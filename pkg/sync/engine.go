package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/markdown"
	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/byvfx/go-notion-md-sync/pkg/util"
)

type Engine interface {
	SyncFileToNotion(ctx context.Context, filePath string) error
	SyncNotionToFile(ctx context.Context, pageID, filePath string) error
	SyncAll(ctx context.Context, direction string) error
	SyncSpecificFile(ctx context.Context, filename, direction string) error
}

type engine struct {
	config           *config.Config
	notion           notion.Client
	parser           markdown.Parser
	converter        Converter
	conflictResolver *ConflictResolver
	workerCount      int // Configurable worker count

	// createMu serialises creating new pages, so two files pushed at once
	// can't both create the same folder page.
	createMu gosync.Mutex
	// folderPages caches the Notion page for each local folder.
	folderPages map[string]string
}

func newEngine(cfg *config.Config, client notion.Client, workers int) *engine {
	return &engine{
		config:           cfg,
		notion:           client,
		parser:           markdown.NewParser(),
		converter:        NewConverter(),
		conflictResolver: NewConflictResolver(cfg.Sync.ConflictResolution),
		workerCount:      workers,
		folderPages:      make(map[string]string),
	}
}

func NewEngine(cfg *config.Config) Engine {
	// Create the appropriate client based on configuration
	var client notion.Client
	if cfg.Performance.UseMultiClient {
		// Use multi-client approach for maximum throughput
		client = notion.NewBatchClient(cfg.Notion.Token, cfg.Performance.ClientCount)
	} else {
		// Use standard client (proven best performance)
		client = notion.NewClient(cfg.Notion.Token)
	}
	return newEngine(cfg, client, cfg.Performance.Workers)
}

// NewEngineWithWorkers creates an engine with a specific worker count
func NewEngineWithWorkers(cfg *config.Config, workers int) Engine {
	return newEngine(cfg, notion.NewClient(cfg.Notion.Token), workers)
}

// NewEngineWithClient creates an engine with a custom client
func NewEngineWithClient(cfg *config.Config, client notion.Client) Engine {
	return newEngine(cfg, client, 0)
}

// ---------------------------------------------------------------- push

// SyncFileToNotion pushes one Markdown file. A file without a notion_id
// becomes a new page under the page that matches its folder (see layout.go);
// a file with a notion_id replaces that page's content.
func (e *engine) SyncFileToNotion(ctx context.Context, filePath string) error {
	doc, err := e.parser.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse markdown file: %w", err)
	}

	frontmatter, err := markdown.ExtractFrontmatter(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to extract frontmatter: %w", err)
	}

	if !frontmatter.SyncEnabled {
		return nil
	}

	if frontmatter.NotionID != "" {
		return e.pushExisting(ctx, doc, frontmatter)
	}

	e.createMu.Lock()
	defer e.createMu.Unlock()
	_, err = e.pushNewLocked(ctx, filePath)
	return err
}

// pushExisting replaces the content (and, if changed, the title) of the page
// a file is linked to. Nothing is sent when Notion already has the same
// content. After a push the file's updated_at records Notion's edit time.
func (e *engine) pushExisting(ctx context.Context, doc *markdown.Document, fm *markdown.FrontmatterFields) error {
	blocks, err := e.converter.MarkdownToBlocks(doc.Content)
	if err != nil {
		return fmt.Errorf("failed to convert markdown to blocks: %w", err)
	}

	if remote, err := e.notion.GetPageBlocks(ctx, fm.NotionID); err == nil {
		if remoteMD, err := e.converter.BlocksToMarkdown(remote); err == nil && !HasConflict(doc.Content, remoteMD) {
			return e.updateTitle(ctx, fm)
		}
	}

	if err := e.updateTitle(ctx, fm); err != nil {
		return err
	}
	if err := e.updateNotionPage(ctx, fm.NotionID, fm.Title, blocks); err != nil {
		return err
	}
	if doc.FilePath != "" {
		e.markSynced(ctx, doc.FilePath, fm.NotionID)
	}
	return nil
}

// pushNewLocked creates a page for a file that has no notion_id yet and
// writes the new ID into its frontmatter. createMu must be held.
func (e *engine) pushNewLocked(ctx context.Context, filePath string) (string, error) {
	// Parse again under the lock: the file may have just been created as a
	// folder page by another push.
	doc, err := e.parser.ParseFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse markdown file: %w", err)
	}
	fm, err := markdown.ExtractFrontmatter(doc.Metadata)
	if err != nil {
		return "", fmt.Errorf("failed to extract frontmatter: %w", err)
	}
	if fm.NotionID != "" {
		return fm.NotionID, e.pushExisting(ctx, doc, fm)
	}

	blocks, err := e.converter.MarkdownToBlocks(doc.Content)
	if err != nil {
		return "", fmt.Errorf("failed to convert markdown to blocks: %w", err)
	}

	title := fm.Title
	if title == "" {
		title = e.getTitleFromFilename(filePath)
	}

	parentID, err := e.parentForFileLocked(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to find the parent page for %s: %w", filePath, err)
	}

	pageID, err := e.createNotionPage(ctx, parentID, title, blocks)
	if err != nil {
		return "", err
	}

	if err := e.writeWithFrontmatter(filePath, doc.Metadata, map[string]interface{}{
		"notion_id":  pageID,
		"updated_at": time.Now().Format(time.RFC3339),
	}, doc.Content); err != nil {
		return pageID, fmt.Errorf("created page %s but failed to save its ID in %s: %w", pageID, filePath, err)
	}

	if abs, err := filepath.Abs(filePath); err == nil && isFolderFile(abs) {
		e.folderPages[filepath.Dir(abs)] = pageID
	}
	return pageID, nil
}

// isFolderFile reports whether path is the page file of its folder
// (Folder/Folder.md).
func isFolderFile(path string) bool {
	dir := filepath.Dir(path)
	return strings.EqualFold(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), filepath.Base(dir))
}

// parentForFileLocked returns the Notion page a new file should be created
// under, following the layout in layout.go:
//
//	root/A/A.md       -> under the page for root (the configured parent)
//	root/A/B/B.md     -> under the page for folder A
//	root/A/notes.md   -> under the page for folder A
//
// Files outside the markdown root go under the configured parent page.
func (e *engine) parentForFileLocked(ctx context.Context, filePath string) (string, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	root := e.absRoot()
	dir := filepath.Dir(abs)
	if !isWithin(root, dir) {
		return e.config.Notion.ParentPageID, nil
	}
	if isFolderFile(abs) {
		dir = filepath.Dir(dir)
	}
	return e.folderPageLocked(ctx, dir, root, 0)
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// folderPageLocked returns the Notion page that a local folder stands for,
// creating it (and its parents) when it doesn't exist yet.
func (e *engine) folderPageLocked(ctx context.Context, dir, root string, depth int) (string, error) {
	if !isWithin(root, dir) || dir == root || depth > 64 {
		return e.config.Notion.ParentPageID, nil
	}
	if id, ok := e.folderPages[dir]; ok {
		return id, nil
	}

	name := filepath.Base(dir)
	ownFile := filepath.Join(dir, name+".md")

	// The folder's own page file, or a README/index with a notion_id.
	for _, candidate := range []string{ownFile, filepath.Join(dir, "README.md"), filepath.Join(dir, "readme.md"), filepath.Join(dir, "index.md")} {
		doc, err := e.readExisting(candidate)
		if err != nil || doc == nil {
			continue
		}
		if id, _ := doc.Metadata["notion_id"].(string); id != "" {
			e.folderPages[dir] = id
			return id, nil
		}
	}

	// The folder's own file exists but hasn't been pushed yet: push it first
	// so it becomes the folder's page.
	if doc, err := e.readExisting(ownFile); err == nil && doc != nil {
		if fm, err := markdown.ExtractFrontmatter(doc.Metadata); err == nil && fm.SyncEnabled {
			id, err := e.pushNewLocked(ctx, ownFile)
			if err != nil {
				return "", err
			}
			e.folderPages[dir] = id
			return id, nil
		}
	}

	parentID, err := e.folderPageLocked(ctx, filepath.Dir(dir), root, depth+1)
	if err != nil {
		return "", err
	}

	// Reuse an existing sub-page with the folder's name.
	if pages, err := e.notion.GetChildPages(ctx, parentID); err == nil {
		for i := range pages {
			title := e.extractTitleFromPage(&pages[i])
			if title == name || util.SanitizeFileName(title) == name {
				e.folderPages[dir] = pages[i].ID
				return pages[i].ID, nil
			}
		}
	}

	id, err := e.createNotionPage(ctx, parentID, name, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create a page for folder %s: %w", dir, err)
	}
	// Save the folder page locally so the next push and pull agree on it.
	if err := e.parser.CreateMarkdownWithFrontmatter(ownFile, map[string]interface{}{
		"title":        name,
		"notion_id":    id,
		"sync_enabled": true,
		"updated_at":   time.Now().Format(time.RFC3339),
	}, ""); err != nil {
		util.Warning("Created page %s for folder %s but could not write %s: %v", id, dir, ownFile, err)
	}
	e.folderPages[dir] = id
	return id, nil
}

func titleProperties(title string) map[string]interface{} {
	return map[string]interface{}{
		"title": map[string]interface{}{
			"title": []notion.RichText{
				{
					Type:      "text",
					Text:      &notion.TextContent{Content: title},
					PlainText: title,
				},
			},
		},
	}
}

// createNotionPage creates a page with the given content. If the content
// can't be added, the new page is archived again and an error is returned,
// so a failed push never leaves an empty page linked to the file.
func (e *engine) createNotionPage(ctx context.Context, parentID string, title string, blocks []map[string]interface{}) (string, error) {
	page, err := e.notion.CreatePage(ctx, parentID, titleProperties(title))
	if err != nil {
		return "", fmt.Errorf("failed to create page: %w", err)
	}

	if len(blocks) > 0 {
		if err := e.notion.UpdatePageBlocks(ctx, page.ID, blocks); err != nil {
			if delErr := e.notion.DeletePage(ctx, page.ID); delErr != nil {
				util.Warning("Could not remove incomplete page %s: %v", page.ID, delErr)
			}
			return "", fmt.Errorf("failed to add content to new page %q: %w", title, err)
		}
	}

	return page.ID, nil
}

func (e *engine) updateNotionPage(ctx context.Context, pageID, title string, blocks []map[string]interface{}) error {
	// Replace the content in place, so the page keeps its ID, links and
	// comments. UpdatePageBlocks never removes sub-pages or databases.
	return e.notion.UpdatePageBlocks(ctx, pageID, blocks)
}

// updateTitle renames the page when the frontmatter title differs. It
// doesn't when the page changed in Notion since the file was last synced:
// then the difference is more likely a rename in Notion, which must not be
// reverted.
func (e *engine) updateTitle(ctx context.Context, fm *markdown.FrontmatterFields) error {
	if fm.Title == "" {
		return nil
	}
	page, err := e.notion.GetPage(ctx, fm.NotionID)
	if err != nil {
		return fmt.Errorf("failed to get page %s: %w", fm.NotionID, err)
	}
	current := e.extractTitleFromPage(page)
	if page.Properties == nil || current == fm.Title {
		return nil
	}
	if fm.UpdatedAt != nil && page.LastEditedTime.After(*fm.UpdatedAt) {
		util.Warning("Not renaming %q to %q: the page changed in Notion since the last sync. Pull first to take the Notion title.", current, fm.Title)
		return nil
	}
	if _, err := e.notion.UpdatePageProperties(ctx, fm.NotionID, titleProperties(fm.Title)); err != nil {
		return fmt.Errorf("failed to rename page %s: %w", fm.NotionID, err)
	}
	return nil
}

func (e *engine) syncAllMarkdownToNotion(ctx context.Context) error {
	var files []string
	if err := e.walkMarkdown(func(path string) error {
		files = append(files, path)
		return nil
	}); err != nil {
		return err
	}
	var failed []string
	for _, path := range files {
		if err := e.SyncFileToNotion(ctx, path); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			util.ErrorMsg("Failed to push %s: %v", path, err)
			failed = append(failed, path)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d files failed to push", len(failed), len(files))
	}
	return nil
}

// ---------------------------------------------------------------- pull

// SyncNotionToFile pulls one page into filePath. An existing file keeps its
// own frontmatter fields; a file with sync_enabled: false is left alone.
func (e *engine) SyncNotionToFile(ctx context.Context, pageID, filePath string) error {
	page, err := e.notion.GetPage(ctx, pageID)
	if err != nil {
		return fmt.Errorf("failed to get Notion page: %w", err)
	}
	return e.pullPage(ctx, page, filePath, nil)
}

// pullPage writes a page to filePath. plan (optional) says where sub-pages
// are, so links to them are correct.
func (e *engine) pullPage(ctx context.Context, page *notion.Page, filePath string, plan *pagePlan) error {
	existing, err := e.readExisting(filePath)
	if err != nil {
		return fmt.Errorf("failed to read existing file %s: %w", filePath, err)
	}
	var existingMeta map[string]interface{}
	if existing != nil {
		existingMeta = existing.Metadata
		if fm, err := markdown.ExtractFrontmatter(existing.Metadata); err == nil && !fm.SyncEnabled {
			fmt.Printf("  Skipping %s (sync_enabled is false)\n", filePath)
			return nil
		}
	}

	title := e.extractTitleFromPage(page)

	blocks, err := e.notion.GetPageBlocks(ctx, page.ID)
	if err != nil {
		return fmt.Errorf("failed to get page blocks: %w", err)
	}

	content, err := e.converter.BlocksToMarkdown(blocks)
	if err != nil {
		return fmt.Errorf("failed to convert blocks to markdown: %w", err)
	}

	databases := e.exportChildDatabases(ctx, blocks, filePath, title)

	content = relinkMarkers(content, func(blockType, id string) (string, bool) {
		switch blockType {
		case "child_database":
			name, ok := databases[normalizeID(id)]
			return "./" + name, ok
		case "child_page":
			if plan == nil {
				return "", false
			}
			target, ok := plan.path(id)
			if !ok {
				return "", false
			}
			return relativeLink(filePath, target)
		}
		return "", false
	})

	updatedAt := page.LastEditedTime
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	updates := map[string]interface{}{
		"title":      title,
		"notion_id":  page.ID,
		"updated_at": updatedAt.UTC().Format(time.RFC3339),
	}
	if !page.CreatedTime.IsZero() {
		updates["created_at"] = page.CreatedTime.UTC().Format(time.RFC3339)
	}
	if _, ok := existingMeta["sync_enabled"]; !ok {
		updates["sync_enabled"] = true
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(filePath), err)
	}
	return e.writeWithFrontmatter(filePath, existingMeta, updates, content)
}

// allPages returns the root page followed by all its descendants, parents
// before children.
func (e *engine) allPages(ctx context.Context) ([]notion.Page, error) {
	parentPage, err := e.notion.GetPage(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent page: %w", err)
	}
	descendants, err := e.notion.GetAllDescendantPages(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get descendant pages: %w", err)
	}
	return append([]notion.Page{*parentPage}, descendants...), nil
}

func (e *engine) syncAllNotionToMarkdown(ctx context.Context) error {
	pages, err := e.allPages(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d pages under parent %s (including parent and sub-pages)\n", len(pages), e.config.Notion.ParentPageID)
	fmt.Println()

	local, err := e.localIndex()
	if err != nil {
		return fmt.Errorf("failed to scan %s: %w", e.config.Directories.MarkdownRoot, err)
	}
	plan := e.planPaths(pages, local)

	return e.syncPagesConcurrently(ctx, pages, plan)
}

// syncPagesConcurrently processes multiple pages concurrently using simple goroutines
func (e *engine) syncPagesConcurrently(ctx context.Context, pages []notion.Page, plan *pagePlan) error {
	if len(pages) == 0 {
		return nil
	}
	workerCount := e.workerCount
	if workerCount <= 0 {
		// Auto-detect based on page count - optimized based on performance testing
		workerCount = 30
		if len(pages) < 5 {
			workerCount = len(pages)
		} else if len(pages) < 15 {
			workerCount = 20
		}
	}
	if workerCount > 50 {
		workerCount = 50
	}
	if workerCount > len(pages) {
		workerCount = len(pages)
	}

	fmt.Printf("🚀 Using concurrent processing with %d workers for %d pages\n", workerCount, len(pages))

	pageJobs := make(chan pageJob, len(pages))
	results := make(chan syncResult, len(pages))

	for i := 0; i < workerCount; i++ {
		go e.syncWorker(ctx, pageJobs, results, plan)
	}

	for i := range pages {
		path, _ := plan.path(pages[i].ID)
		pageJobs <- pageJob{
			page:     pages[i],
			title:    e.extractTitleFromPage(&pages[i]),
			filePath: path,
			index:    i + 1,
			total:    len(pages),
		}
	}
	close(pageJobs)

	var errs []string
	successCount := 0
	for i := 0; i < len(pages); i++ {
		result := <-results
		if result.err != nil {
			errs = append(errs, fmt.Sprintf("Page %s: %v", result.pageID, result.err))
		} else {
			successCount++
		}
	}

	fmt.Printf("\n🎉 Concurrent sync complete! %d/%d pages successful\n", successCount, len(pages))

	if len(errs) > 0 {
		util.ErrorMsg("%d pages failed", len(errs))
		for _, errMsg := range errs {
			util.Error("  - %s", errMsg)
		}
		return fmt.Errorf("%d pages failed to sync", len(errs))
	}
	return nil
}

// pageJob represents a page sync job
type pageJob struct {
	page     notion.Page
	title    string
	filePath string
	index    int
	total    int
}

// syncResult represents the result of a sync operation
type syncResult struct {
	pageID string
	err    error
}

// syncWorker processes page sync jobs concurrently
func (e *engine) syncWorker(ctx context.Context, jobs <-chan pageJob, results chan<- syncResult, plan *pagePlan) {
	for job := range jobs {
		result := syncResult{pageID: job.page.ID}

		fmt.Printf("[%d/%d] Pulling page: %s\n  Notion ID: %s\n  Saving to: %s\n",
			job.index, job.total, job.title, job.page.ID, e.describePath(job.filePath))

		page := job.page
		if err := e.pullPage(ctx, &page, job.filePath, plan); err != nil {
			result.err = fmt.Errorf("failed to sync page %s: %w", job.page.ID, err)
		} else {
			fmt.Printf("  ✓ Successfully pulled %s\n", job.title)
		}

		results <- result
	}
}

// ---------------------------------------------------------------- both ways

func (e *engine) SyncAll(ctx context.Context, direction string) error {
	switch direction {
	case "push":
		return e.syncAllMarkdownToNotion(ctx)
	case "pull":
		return e.syncAllNotionToMarkdown(ctx)
	case "bidirectional":
		return e.syncBidirectional(ctx)
	default:
		return fmt.Errorf("unsupported sync direction: %s", direction)
	}
}

func (e *engine) syncBidirectional(ctx context.Context) error {
	pages, err := e.allPages(ctx)
	if err != nil {
		return err
	}
	byID := make(map[string]*notion.Page, len(pages))
	for i := range pages {
		byID[normalizeID(pages[i].ID)] = &pages[i]
	}

	local, err := e.localIndex()
	if err != nil {
		return fmt.Errorf("failed to scan %s: %w", e.config.Directories.MarkdownRoot, err)
	}
	plan := e.planPaths(pages, local)

	var files []string
	if err := e.walkMarkdown(func(path string) error {
		files = append(files, path)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to list markdown files: %w", err)
	}

	var failed int
	for _, path := range files {
		if err := e.syncFileWithConflictDetection(ctx, path, byID, plan); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			util.ErrorMsg("Failed to sync %s: %v", path, err)
			failed++
		}
	}

	// Pull Notion pages that have no local file yet.
	for i := range pages {
		key := normalizeID(pages[i].ID)
		if _, ok := local[key]; ok {
			continue
		}
		path, _ := plan.path(pages[i].ID)
		if _, err := os.Stat(path); err == nil {
			// Something else is already there (for example a page that
			// was just pushed); don't overwrite it.
			continue
		}
		fmt.Printf("Pulling new page %s to %s\n", e.extractTitleFromPage(&pages[i]), e.describePath(path))
		if err := e.pullPage(ctx, &pages[i], path, plan); err != nil {
			util.ErrorMsg("Failed to pull page %s: %v", pages[i].ID, err)
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d items failed to sync", failed)
	}
	return nil
}

func (e *engine) syncFileWithConflictDetection(ctx context.Context, filePath string, pages map[string]*notion.Page, plan *pagePlan) error {
	doc, err := e.parser.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse markdown file: %w", err)
	}

	fm, err := markdown.ExtractFrontmatter(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to extract frontmatter: %w", err)
	}

	if !fm.SyncEnabled {
		return nil
	}

	// New local file: create it in Notion.
	if fm.NotionID == "" {
		return e.SyncFileToNotion(ctx, filePath)
	}

	page, ok := pages[normalizeID(fm.NotionID)]
	if !ok {
		// Not under the configured parent. Only sync it if the page still
		// exists; never silently recreate or overwrite.
		p, err := e.notion.GetPage(ctx, fm.NotionID)
		if err != nil {
			return fmt.Errorf("page %s could not be read (deleted, moved or not shared with the integration?): %w", fm.NotionID, err)
		}
		if p.Archived || p.InTrash {
			util.Warning("Skipping %s: its Notion page %s is in the trash. Remove notion_id from the file to create a new page.", filePath, fm.NotionID)
			return nil
		}
		page = p
	}

	blocks, err := e.notion.GetPageBlocks(ctx, fm.NotionID)
	if err != nil {
		return fmt.Errorf("failed to get page blocks: %w", err)
	}
	remoteContent, err := e.converter.BlocksToMarkdown(blocks)
	if err != nil {
		return fmt.Errorf("failed to convert blocks to markdown: %w", err)
	}

	if !HasConflict(doc.Content, remoteContent) {
		return e.updateTitle(ctx, fm)
	}

	remoteChanged := fm.UpdatedAt == nil || page.LastEditedTime.After(*fm.UpdatedAt)
	if !remoteChanged {
		// Only the local file changed since the last sync.
		return e.SyncFileToNotion(ctx, filePath)
	}

	var localTime time.Time
	if info, err := os.Stat(filePath); err == nil {
		localTime = info.ModTime()
	}
	resolved, err := e.conflictResolver.ResolveConflictAt(doc.Content, remoteContent, filePath, localTime, page.LastEditedTime)
	if err != nil {
		fmt.Printf("Skipping file %s: %v\n", filePath, err)
		return nil
	}

	if resolved == remoteContent && HasConflict(resolved, doc.Content) {
		return e.pullPage(ctx, page, filePath, plan)
	}
	return e.SyncFileToNotion(ctx, filePath)
}

// markSynced records in the file's frontmatter when the page was last synced,
// using Notion's own edit time so the next sync can tell whether the page was
// changed in Notion since. It is best effort: the push already succeeded.
func (e *engine) markSynced(ctx context.Context, filePath, pageID string) {
	page, err := e.notion.GetPage(ctx, pageID)
	if err != nil {
		return
	}
	doc, err := e.parser.ParseFile(filePath)
	if err != nil {
		return
	}
	stamp := page.LastEditedTime
	if stamp.IsZero() {
		stamp = time.Now()
	}
	if err := e.writeWithFrontmatter(filePath, doc.Metadata, map[string]interface{}{
		"updated_at": stamp.UTC().Format(time.RFC3339),
	}, doc.Content); err != nil {
		util.Warning("Pushed %s but could not update its frontmatter: %v", filePath, err)
	}
}

// ---------------------------------------------------------------- single file

func (e *engine) SyncSpecificFile(ctx context.Context, filename, direction string) error {
	switch direction {
	case "pull":
		return e.syncSpecificNotionToMarkdown(ctx, filename)
	case "push":
		return e.SyncFileToNotion(ctx, filename)
	default:
		return fmt.Errorf("unsupported direction: %s", direction)
	}
}

// syncSpecificNotionToMarkdown pulls one page, chosen by file name. A local
// file with that name and a notion_id is refreshed from its page; otherwise
// the page with the matching title is found anywhere under the parent page.
func (e *engine) syncSpecificNotionToMarkdown(ctx context.Context, filename string) error {
	// An existing file, as given or relative to the markdown root.
	for _, candidate := range []string{filename, filepath.Join(e.config.Directories.MarkdownRoot, filename)} {
		doc, err := e.readExisting(candidate)
		if err != nil || doc == nil {
			continue
		}
		if id, _ := doc.Metadata["notion_id"].(string); id != "" {
			fmt.Printf("Pulling page %s into %s\n", id, candidate)
			return e.SyncNotionToFile(ctx, id, candidate)
		}
	}

	targetTitle := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	local, err := e.localIndex()
	if err != nil {
		return err
	}
	// A local file with that name anywhere under the root.
	for id, path := range local {
		if strings.EqualFold(filepath.Base(path), targetTitle+".md") {
			fmt.Printf("Pulling page %s into %s\n", id, e.describePath(path))
			return e.SyncNotionToFile(ctx, id, path)
		}
	}

	pages, err := e.allPages(ctx)
	if err != nil {
		return err
	}
	var target *notion.Page
	for i := range pages {
		title := e.extractTitleFromPage(&pages[i])
		if title == targetTitle || util.SanitizeFileName(title) == targetTitle {
			target = &pages[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("page with title '%s' not found in Notion", targetTitle)
	}

	plan := e.planPaths(pages, local)
	filePath, _ := plan.path(target.ID)

	fmt.Printf("Pulling page: %s\n  Notion ID: %s\n  Saving to: %s\n", e.extractTitleFromPage(target), target.ID, e.describePath(filePath))
	if err := e.pullPage(ctx, target, filePath, plan); err != nil {
		return fmt.Errorf("failed to sync page %s: %w", target.ID, err)
	}
	return nil
}

// ---------------------------------------------------------------- helpers

func (e *engine) getTitleFromFilename(filePath string) string {
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func (e *engine) extractTitleFromPage(page *notion.Page) string {
	// The title property is called "title" on regular pages, but database
	// rows can name it anything; find it by type.
	for _, prop := range page.Properties {
		titleData, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := titleData["type"].(string); t != "" && t != "title" {
			continue
		}
		titleList, ok := titleData["title"].([]interface{})
		if !ok {
			continue
		}
		var b strings.Builder
		for _, item := range titleList {
			if m, ok := item.(map[string]interface{}); ok {
				if s, ok := m["plain_text"].(string); ok {
					b.WriteString(s)
				}
			}
		}
		if b.Len() > 0 {
			return b.String()
		}
	}
	return "Untitled"
}

// exportChildDatabases exports every database on the page (including ones
// nested in toggles or columns) to a CSV next to the page's file. It returns
// the CSV file name for each database ID. Failures are reported but don't
// fail the page.
func (e *engine) exportChildDatabases(ctx context.Context, blocks []notion.Block, filePath, pageTitle string) map[string]string {
	names := make(map[string]string)
	used := make(map[string]bool)
	baseDir := filepath.Dir(filePath)

	var walk func(bs []notion.Block)
	walk = func(bs []notion.Block) {
		for i := range bs {
			b := &bs[i]
			walk(b.Children)
			if b.Type != "child_database" {
				continue
			}
			databaseID := b.ID
			if b.ChildDatabase != nil && b.ChildDatabase.DatabaseID != "" {
				databaseID = b.ChildDatabase.DatabaseID
			}
			title := ""
			if b.ChildDatabase != nil {
				title = b.ChildDatabase.Title
			}
			if title == "" {
				if db, err := e.notion.GetDatabase(ctx, databaseID); err == nil && len(db.Title) > 0 {
					title = db.Title[0].PlainText
				}
			}

			csvName := databaseCSVFileName(title, b.ID)
			for n := 2; used[strings.ToLower(csvName)]; n++ {
				csvName = strings.TrimSuffix(databaseCSVFileName(title, b.ID), ".csv") + fmt.Sprintf("_%d.csv", n)
			}
			used[strings.ToLower(csvName)] = true

			label := title
			if label == "" {
				label = "Database in " + pageTitle
			}
			fmt.Printf("  Exporting database '%s' to: %s\n", label, csvName)

			if err := os.MkdirAll(baseDir, 0755); err != nil {
				util.Warning("Failed to export database %s: %v", databaseID, err)
				continue
			}
			dbSync := NewDatabaseSync(e.notion)
			if err := dbSync.SyncNotionDatabaseToCSV(ctx, databaseID, filepath.Join(baseDir, csvName)); err != nil {
				util.Warning("Failed to export database %s: %v", databaseID, err)
				continue
			}
			names[normalizeID(b.ID)] = csvName
		}
	}
	walk(blocks)
	return names
}
