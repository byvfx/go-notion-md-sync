package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/markdown"
	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workspace builds:
//
//	Root            "root text"
//	├── Child       "child text"
//	│   └── Grand   "grand text"
//	└── Other       "other text"
func workspace(t *testing.T) (*memNotion, *engine, string) {
	t.Helper()
	mem := newMemNotion()
	root := mem.addPage("", "Root", paraBlock("root text"))
	child := mem.addPage(root, "Child", paraBlock("child text"))
	mem.addPage(child, "Grand", paraBlock("grand text"))
	mem.addPage(root, "Other", paraBlock("other text"))

	cfg := &config.Config{}
	cfg.Notion.ParentPageID = root
	cfg.Directories.MarkdownRoot = filepath.Join(t.TempDir(), "docs")
	cfg.Sync.ConflictResolution = "markdown_wins"
	cfg.Performance.Workers = 2
	e := NewEngineWithClient(cfg, mem).(*engine)
	return mem, e, cfg.Directories.MarkdownRoot
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func listMarkdown(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".md") {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func TestPull_NestedLayoutAndNoAppendedSubPages(t *testing.T) {
	_, e, root := workspace(t)
	ctx := context.Background()

	// The markdown root does not exist yet; pull must create it.
	require.NoError(t, e.SyncAll(ctx, "pull"))

	assert.ElementsMatch(t, []string{
		"Root/Root.md",
		"Root/Child/Child.md",
		"Root/Child/Grand/Grand.md",
		"Root/Other/Other.md",
	}, listMarkdown(t, root))

	rootMD := readFile(t, filepath.Join(root, "Root", "Root.md"))
	assert.Contains(t, rootMD, "root text")
	// Issue #7: sub-page content must not be appended to the parent.
	assert.NotContains(t, rootMD, "child text")
	assert.NotContains(t, rootMD, "grand text")
	assert.Contains(t, rootMD, "[📄 Child](./Child/Child.md) <!-- notion:child_page ")
	assert.Contains(t, rootMD, "[📄 Other](./Other/Other.md) <!-- notion:child_page ")

	childMD := readFile(t, filepath.Join(root, "Root", "Child", "Child.md"))
	assert.Contains(t, childMD, "[📄 Grand](./Grand/Grand.md)")
}

func TestPush_UnchangedPulledFilesSendNothing(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))

	require.NoError(t, e.SyncAll(ctx, "push"))
	assert.Equal(t, 0, mem.updateCalls, "unchanged pages must not be rewritten")
	assert.Equal(t, 0, mem.createCalls)
	assert.Len(t, listMarkdown(t, root), 4)
}

func TestPush_EditedParentKeepsSubPages(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))

	path := filepath.Join(root, "Root", "Root.md")
	content := strings.Replace(readFile(t, path), "root text", "edited root text", 1)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	require.NoError(t, e.SyncFileToNotion(ctx, path))
	assert.Equal(t, 1, mem.updateCalls)

	rootID := e.config.Notion.ParentPageID
	assert.Len(t, mem.childrenOf(rootID), 2, "sub-pages must survive pushing their parent")

	blocks, _ := mem.GetPageBlocks(ctx, rootID)
	var kinds []string
	for _, b := range blocks {
		kinds = append(kinds, b.Type)
	}
	assert.Equal(t, []string{"paragraph", "child_page", "child_page"}, kinds, "text stays above the sub-pages")
	assert.Equal(t, "edited root text", blocks[0].Paragraph.RichText[0].PlainText)
}

func TestPush_NewFilesFollowFolders(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))
	rootID := e.config.Notion.ParentPageID
	childID := mem.childrenOf(rootID)[0]

	// A file next to Child.md becomes a sub-page of Child.
	notes := filepath.Join(root, "Root", "Child", "Notes.md")
	require.NoError(t, os.WriteFile(notes, []byte("# Notes\n\nhello"), 0644))
	// A file in a new folder creates a page for the folder first.
	doc := filepath.Join(root, "Root", "Projects", "Plan", "Plan.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(doc), 0755))
	require.NoError(t, os.WriteFile(doc, []byte("the plan"), 0644))

	require.NoError(t, e.SyncFileToNotion(ctx, notes))
	require.NoError(t, e.SyncFileToNotion(ctx, doc))

	var childKids []string
	for _, id := range mem.childrenOf(childID) {
		childKids = append(childKids, mem.title(id))
	}
	assert.Contains(t, childKids, "Notes")

	var rootKids []string
	var projectsID string
	for _, id := range mem.childrenOf(rootID) {
		rootKids = append(rootKids, mem.title(id))
		if mem.title(id) == "Projects" {
			projectsID = id
		}
	}
	assert.Contains(t, rootKids, "Projects")
	require.NotEmpty(t, projectsID)
	planKids := mem.childrenOf(projectsID)
	require.Len(t, planKids, 1)
	assert.Equal(t, "Plan", mem.title(planKids[0]))

	// The folder page was saved locally, and the new IDs were written back.
	parser := markdown.NewParser()
	folderDoc, err := parser.ParseFile(filepath.Join(root, "Root", "Projects", "Projects.md"))
	require.NoError(t, err)
	assert.Equal(t, projectsID, folderDoc.Metadata["notion_id"])
	planDoc, err := parser.ParseFile(doc)
	require.NoError(t, err)
	assert.Equal(t, planKids[0], planDoc.Metadata["notion_id"])

	// Pushing again creates nothing new.
	creates := mem.createCalls
	require.NoError(t, e.SyncFileToNotion(ctx, doc))
	assert.Equal(t, creates, mem.createCalls)
}

func TestPush_ConcurrentNewFilesShareOneFolderPage(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))

	dir := filepath.Join(root, "Root", "Batch")
	require.NoError(t, os.MkdirAll(dir, 0755))
	var files []string
	for _, name := range []string{"a", "b", "c", "d"} {
		p := filepath.Join(dir, name+".md")
		require.NoError(t, os.WriteFile(p, []byte(name), 0644))
		files = append(files, p)
	}
	errs := make(chan error, len(files))
	for _, f := range files {
		go func(f string) { errs <- e.SyncFileToNotion(ctx, f) }(f)
	}
	for range files {
		require.NoError(t, <-errs)
	}

	count := 0
	for _, id := range mem.childrenOf(e.config.Notion.ParentPageID) {
		if mem.title(id) == "Batch" {
			count++
		}
	}
	assert.Equal(t, 1, count, "only one page for the folder")
}

func TestPush_FailedNewPageIsRemoved(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, os.MkdirAll(root, 0755))
	path := filepath.Join(root, "new.md")
	require.NoError(t, os.WriteFile(path, []byte("content"), 0644))

	mem.failUpdate = true
	require.Error(t, e.SyncFileToNotion(ctx, path))
	assert.NotContains(t, readFile(t, path), "notion_id", "a failed push must not link the file")
	for _, id := range mem.childrenOf(e.config.Notion.ParentPageID) {
		assert.NotEqual(t, "new", mem.title(id), "the half-created page is archived")
	}
}

func TestBidirectional_NoDuplicatesAfterPull(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))
	before := listMarkdown(t, root)

	require.NoError(t, e.SyncAll(ctx, "bidirectional"))
	assert.ElementsMatch(t, before, listMarkdown(t, root), "sync must not create flat duplicates")
	assert.Equal(t, 0, mem.updateCalls)
	assert.Equal(t, 0, mem.createCalls)

	// A new Notion page is pulled into its parent's folder.
	childID := mem.childrenOf(e.config.Notion.ParentPageID)[0]
	mem.addPage(childID, "Fresh", paraBlock("fresh text"))
	require.NoError(t, e.SyncAll(ctx, "bidirectional"))
	assert.FileExists(t, filepath.Join(root, "Root", "Child", "Fresh", "Fresh.md"))
}

func TestBidirectional_LocalEditPushesWithoutConflict(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))
	e.conflictResolver = NewConflictResolver("notion_wins") // must not be consulted

	path := filepath.Join(root, "Root", "Other", "Other.md")
	content := strings.Replace(readFile(t, path), "other text", "changed locally", 1)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	require.NoError(t, e.SyncAll(ctx, "bidirectional"))
	assert.Equal(t, 1, mem.updateCalls)
	assert.Contains(t, readFile(t, path), "changed locally")
}

func TestPull_KeepsLocalFrontmatterAndRespectsSyncDisabled(t *testing.T) {
	_, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))

	path := filepath.Join(root, "Root", "Other", "Other.md")
	parser := markdown.NewParser()
	doc, err := parser.ParseFile(path)
	require.NoError(t, err)
	doc.Metadata["tags"] = []interface{}{"keep"}
	doc.Metadata["custom"] = "value"
	require.NoError(t, parser.CreateMarkdownWithFrontmatter(path, doc.Metadata, doc.Content))

	require.NoError(t, e.SyncAll(ctx, "pull"))
	after := readFile(t, path)
	assert.Contains(t, after, "custom: value")
	assert.Contains(t, after, "- keep")

	disabled := filepath.Join(root, "Root", "Child", "Child.md")
	doc, err = parser.ParseFile(disabled)
	require.NoError(t, err)
	doc.Metadata["sync_enabled"] = false
	require.NoError(t, parser.CreateMarkdownWithFrontmatter(disabled, doc.Metadata, "my local version"))
	require.NoError(t, e.SyncAll(ctx, "pull"))
	assert.Contains(t, readFile(t, disabled), "my local version")
}

func TestPull_DuplicateTitlesGetSeparateFolders(t *testing.T) {
	mem, e, root := workspace(t)
	rootID := e.config.Notion.ParentPageID
	mem.addPage(rootID, "Other", paraBlock("second other"))
	require.NoError(t, e.SyncAll(context.Background(), "pull"))

	files := listMarkdown(t, root)
	assert.Contains(t, files, "Root/Other/Other.md")
	var dup string
	for _, f := range files {
		if strings.HasPrefix(f, "Root/Other (") {
			dup = f
		}
	}
	require.NotEmpty(t, dup, "the second page named Other needs its own folder: %v", files)
	assert.Contains(t, readFile(t, filepath.Join(root, dup)), "second other")
	// The parent's link points at the right folder.
	rootMD := readFile(t, filepath.Join(root, "Root", "Root.md"))
	assert.Contains(t, rootMD, "(<./"+strings.TrimSuffix(dup[len("Root/"):], "/"+filepath.Base(dup))+"/"+filepath.Base(dup)+">)")
}

func TestPull_KeepsFilesWhereTheUserMovedThem(t *testing.T) {
	_, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))

	from := filepath.Join(root, "Root", "Other", "Other.md")
	to := filepath.Join(root, "Elsewhere.md")
	require.NoError(t, os.Rename(from, to))

	require.NoError(t, e.SyncAll(ctx, "pull"))
	assert.NoFileExists(t, from, "a moved file must not be pulled again at the old path")
	assert.Contains(t, readFile(t, to), "other text")
}

func TestPullSpecificFile_FindsNestedPages(t *testing.T) {
	_, e, root := workspace(t)
	require.NoError(t, e.SyncSpecificFile(context.Background(), "Grand.md", "pull"))
	assert.FileExists(t, filepath.Join(root, "Root", "Child", "Grand", "Grand.md"))
}

func TestExcludedAndHiddenFoldersAreSkipped(t *testing.T) {
	mem, e, root := workspace(t)
	e.config.Directories.ExcludedPatterns = []string{"node_modules/**", "*.tmp.md"}
	for _, p := range []string{"node_modules/pkg/README.md", ".git/x.md", ".notion-sync/y.md", "a.tmp.md"} {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
		require.NoError(t, os.WriteFile(full, []byte("x"), 0644))
	}
	require.NoError(t, e.SyncAll(context.Background(), "push"))
	assert.Equal(t, 0, mem.createCalls)
}

func TestRelinkMarkers(t *testing.T) {
	id := "12345678-1234-1234-1234-123456789abc"
	in := "[📄 A](./A/A.md) " + markerComment("child_page", id) + "\ntext"
	out := relinkMarkers(in, func(blockType, got string) (string, bool) {
		assert.Equal(t, "child_page", blockType)
		assert.Equal(t, id, got)
		return "./A (1)/A (1).md", true
	})
	assert.Equal(t, "[📄 A](<./A (1)/A (1).md>) "+markerComment("child_page", id)+"\ntext", out)
}

func TestPush_TitleRename(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))
	otherID := mem.childrenOf(e.config.Notion.ParentPageID)[1]
	path := filepath.Join(root, "Root", "Other", "Other.md")

	// Renamed locally: the new title is pushed.
	content := strings.Replace(readFile(t, path), "title: Other", "title: Renamed", 1)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	require.NoError(t, e.SyncFileToNotion(ctx, path))
	assert.Equal(t, "Renamed", mem.title(otherID))

	// Renamed in Notion after the last sync: pushing must not revert it.
	_, _ = mem.UpdatePageProperties(ctx, otherID, titleProperties("Renamed in Notion"))
	mem.mu.Lock()
	mem.now = mem.now.Add(10 * time.Minute)
	mem.pages[otherID].LastEditedTime = mem.now
	mem.mu.Unlock()
	content = strings.Replace(readFile(t, path), "other text", "edited", 1)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	require.NoError(t, e.SyncFileToNotion(ctx, path))
	assert.Equal(t, "Renamed in Notion", mem.title(otherID))
}

func TestBidirectional_ChangeDetection(t *testing.T) {
	setup := func(t *testing.T, strategy string) (*memNotion, *engine, string, string) {
		mem, e, root := workspace(t)
		require.NoError(t, e.SyncAll(context.Background(), "pull"))
		e.conflictResolver = NewConflictResolver(strategy)
		otherID := mem.childrenOf(e.config.Notion.ParentPageID)[1]
		return mem, e, filepath.Join(root, "Root", "Other", "Other.md"), otherID
	}
	editRemote := func(mem *memNotion, id, text string) {
		mem.mu.Lock()
		defer mem.mu.Unlock()
		mem.content[id] = []notion.Block{paraBlock(text)}
		mem.now = mem.now.Add(5 * time.Minute)
		mem.pages[id].LastEditedTime = mem.now
	}
	editLocal := func(t *testing.T, path, text string) {
		content := strings.Replace(readFile(t, path), "other text", text, 1)
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
		future := time.Now().Add(time.Minute) // clearly after the last sync
		require.NoError(t, os.Chtimes(path, future, future))
	}

	t.Run("only Notion changed: pull", func(t *testing.T) {
		mem, e, path, id := setup(t, "markdown_wins")
		editRemote(mem, id, "remote edit")
		require.NoError(t, e.SyncAll(context.Background(), "bidirectional"))
		assert.Contains(t, readFile(t, path), "remote edit")
		assert.Equal(t, 0, mem.updateCalls, "the Notion edit must not be overwritten")
	})

	t.Run("both changed: resolver decides", func(t *testing.T) {
		mem, e, path, id := setup(t, "notion_wins")
		editRemote(mem, id, "remote edit")
		editLocal(t, path, "local edit")
		require.NoError(t, e.SyncAll(context.Background(), "bidirectional"))
		assert.Contains(t, readFile(t, path), "remote edit")

		mem, e, path, id = setup(t, "markdown_wins")
		editRemote(mem, id, "remote edit")
		editLocal(t, path, "local edit")
		require.NoError(t, e.SyncAll(context.Background(), "bidirectional"))
		blocks, _ := mem.GetPageBlocks(context.Background(), id)
		assert.Equal(t, "local edit", blocks[0].Paragraph.RichText[0].PlainText)
	})
}

func TestPull_NewPageNeverTakesAnotherPagesFile(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))

	// "Other" is renamed in Notion to "Renamed"; its file stays at Other/.
	// A new page called "Other" then appears.
	rootID := e.config.Notion.ParentPageID
	otherID := mem.childrenOf(rootID)[1]
	_, _ = mem.UpdatePageProperties(ctx, otherID, titleProperties("Renamed"))
	newID := mem.addPage(rootID, "Other", paraBlock("brand new"))

	require.NoError(t, e.SyncAll(ctx, "pull"))
	original := readFile(t, filepath.Join(root, "Root", "Other", "Other.md"))
	assert.Contains(t, original, "notion_id: "+otherID, "the existing file keeps its page")
	assert.Contains(t, original, "other text")

	var newFile string
	for _, f := range listMarkdown(t, root) {
		if strings.Contains(readFile(t, filepath.Join(root, f)), "notion_id: "+newID) {
			newFile = f
		}
	}
	require.NotEmpty(t, newFile, "the new page needs its own file")
	assert.NotEqual(t, "Root/Other/Other.md", newFile)
}

func TestPull_SkipsExcludedDestinations(t *testing.T) {
	mem, e, root := workspace(t)
	mem.addPage(e.config.Notion.ParentPageID, "Drafts", paraBlock("draft"))
	e.config.Directories.ExcludedPatterns = []string{"Root/Drafts/**"}
	require.NoError(t, e.SyncAll(context.Background(), "pull"))
	assert.NoFileExists(t, filepath.Join(root, "Root", "Drafts", "Drafts.md"))
	assert.FileExists(t, filepath.Join(root, "Root", "Other", "Other.md"))

	require.NoError(t, e.SyncAll(context.Background(), "bidirectional"))
	assert.NoFileExists(t, filepath.Join(root, "Root", "Drafts", "Drafts.md"))
}

func TestRewritingFrontmatterDoesNotAddBlankLines(t *testing.T) {
	_, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))
	path := filepath.Join(root, "Root", "Other", "Other.md")

	// Each push rewrites the frontmatter (updated_at, synced_at).
	for i, text := range []string{"edit one", "edit two", "edit three"} {
		content := readFile(t, path)
		prev := "other text"
		if i > 0 {
			prev = []string{"edit one", "edit two"}[i-1]
		}
		require.NoError(t, os.WriteFile(path, []byte(strings.Replace(content, prev, text, 1)), 0644))
		require.NoError(t, e.SyncFileToNotion(ctx, path))
	}
	assert.Contains(t, readFile(t, path), "---\n\nedit three", "exactly one blank line after the frontmatter")
}

func TestPull_DoesNotOverwriteUnpushedDraft(t *testing.T) {
	mem, e, root := workspace(t)
	ctx := context.Background()
	require.NoError(t, e.SyncAll(ctx, "pull"))

	// A local draft (no notion_id yet) sits where a new Notion page would go.
	draft := filepath.Join(root, "Root", "Notes", "Notes.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(draft), 0755))
	require.NoError(t, os.WriteFile(draft, []byte("my unpushed draft"), 0644))
	newID := mem.addPage(e.config.Notion.ParentPageID, "Notes", paraBlock("from notion"))

	require.NoError(t, e.SyncAll(ctx, "pull"))
	assert.Equal(t, "my unpushed draft", readFile(t, draft))
	var found bool
	for _, f := range listMarkdown(t, root) {
		if strings.Contains(readFile(t, filepath.Join(root, f)), "notion_id: "+newID) {
			found = true
		}
	}
	assert.True(t, found, "the Notion page is pulled to another file")
}
