package main

// End-to-end test: builds the real binary and drives it against a fake
// Notion API (internal/notionfake) the way a user would. Run with
//
//	go test ./cmd/notion-md-sync -run TestEndToEnd -v
//
// It is skipped with -short.

import (
	"bytes"
	"encoding/csv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/byvfx/go-notion-md-sync/internal/notionfake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type e2e struct {
	t    *testing.T
	bin  string
	dir  string
	api  *notionfake.Server
	root string
	env  []string
}

func (e *e2e) run(args ...string) string {
	e.t.Helper()
	out, err := e.runErr(args...)
	require.NoError(e.t, err, "notion-md-sync %s failed:\n%s", strings.Join(args, " "), out)
	return out
}

func (e *e2e) runErr(args ...string) (string, error) {
	e.t.Helper()
	cmd := exec.Command(e.bin, append(args, "--config", "config.yaml")...)
	cmd.Dir = e.dir
	cmd.Env = e.env
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	e.t.Logf("$ notion-md-sync %s\n%s", strings.Join(args, " "), buf.String())
	return buf.String(), err
}

func (e *e2e) path(rel string) string {
	return filepath.Join(e.dir, "docs", filepath.FromSlash(rel))
}

func (e *e2e) read(rel string) string {
	e.t.Helper()
	b, err := os.ReadFile(e.path(rel))
	require.NoError(e.t, err)
	return string(b)
}

func (e *e2e) write(rel, content string) {
	e.t.Helper()
	require.NoError(e.t, os.MkdirAll(filepath.Dir(e.path(rel)), 0755))
	require.NoError(e.t, os.WriteFile(e.path(rel), []byte(content), 0644))
}

func (e *e2e) replace(rel, old, new string) {
	e.t.Helper()
	content := e.read(rel)
	require.Contains(e.t, content, old)
	e.write(rel, strings.Replace(content, old, new, 1))
}

func (e *e2e) files() []string {
	var out []string
	_ = filepath.Walk(filepath.Join(e.dir, "docs"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(filepath.Join(e.dir, "docs"), p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func rich(parts ...map[string]interface{}) []interface{} {
	out := make([]interface{}, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out
}

func span(text string, ann map[string]interface{}) map[string]interface{} {
	m := map[string]interface{}{"type": "text", "text": map[string]interface{}{"content": text}, "plain_text": text}
	if ann != nil {
		m["annotations"] = ann
	}
	return m
}

func setup(t *testing.T) *e2e {
	if testing.Short() {
		t.Skip("end-to-end test")
	}
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "notion-md-sync")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build failed: %s", out)

	api := notionfake.New()
	t.Cleanup(api.Close)

	// Workspace:
	//   Team Wiki (rich content, a database and columns)
	//   ├── Meeting Notes
	//   │   └── Q1 Planning
	//   └── Ideas
	root := api.AddPage("", "Team Wiki",
		notionfake.Block("heading_1", map[string]interface{}{"rich_text": notionfake.Text("Welcome")}),
		notionfake.Block("paragraph", map[string]interface{}{"rich_text": rich(
			span("Hello ", nil), span("bold", map[string]interface{}{"bold": true}), span(" world", nil))}),
		notionfake.Block("bulleted_list_item", map[string]interface{}{"rich_text": notionfake.Text("parent item"),
			"children": []interface{}{notionfake.Block("bulleted_list_item", map[string]interface{}{"rich_text": notionfake.Text("nested item")})}}),
		notionfake.Block("to_do", map[string]interface{}{"rich_text": notionfake.Text("ship it"), "checked": true}),
		notionfake.Block("table", map[string]interface{}{"table_width": 2, "has_column_header": true, "children": []interface{}{
			notionfake.Block("table_row", map[string]interface{}{"cells": []interface{}{notionfake.Text("Name"), notionfake.Text("Role")}}),
			notionfake.Block("table_row", map[string]interface{}{"cells": []interface{}{notionfake.Text("Ada"), notionfake.Text("Engineer")}}),
		}}),
		notionfake.Block("toggle", map[string]interface{}{"rich_text": notionfake.Text("Details"),
			"children": []interface{}{notionfake.Para("hidden detail")}}),
		notionfake.Block("column_list", map[string]interface{}{"children": []interface{}{
			notionfake.Block("column", map[string]interface{}{"children": []interface{}{notionfake.Para("left column")}}),
			notionfake.Block("column", map[string]interface{}{"children": []interface{}{notionfake.Para("right column")}}),
		}}),
	)
	api.AddDatabase(root, "Tasks", []string{"Status"}, []map[string]string{
		{"Name": "Write docs", "Status": "Done"},
		{"Name": "Fix bugs", "Status": "Doing"},
	})
	notes := api.AddPage(root, "Meeting Notes", notionfake.Para("notes text"))
	api.AddPage(notes, "Q1 Planning", notionfake.Para("plan text"))
	api.AddPage(root, "Ideas", notionfake.Para("idea text"))

	dir := filepath.Join(tmp, "project")
	require.NoError(t, os.MkdirAll(dir, 0755))
	config := `notion:
  token: ""
  parent_page_id: ""
sync:
  direction: push
  conflict_resolution: newer
directories:
  markdown_root: ./docs
  excluded_patterns:
    - "*.tmp"
    - "node_modules/**"
    - ".git/**"
mapping:
  strategy: frontmatter
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(config), 0644))

	env := []string{
		"HOME=" + tmp, // keep ~/.notion-md-sync/.env out of the test
		"PATH=" + os.Getenv("PATH"),
		"NOTION_MD_SYNC_API_URL=" + api.URL,
		"NOTION_MD_SYNC_NOTION_TOKEN=ntn_test_token_0123456789",
		"NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=" + root,
	}
	return &e2e{t: t, bin: bin, dir: dir, api: api, root: root, env: env}
}

func TestEndToEnd(t *testing.T) {
	e := setup(t)
	api := e.api

	// --- First pull into a docs folder that doesn't exist yet.
	e.run("pull")
	assert.ElementsMatch(t, []string{
		"Team Wiki/Team Wiki.md",
		"Team Wiki/Tasks.csv",
		"Team Wiki/Meeting Notes/Meeting Notes.md",
		"Team Wiki/Meeting Notes/Q1 Planning/Q1 Planning.md",
		"Team Wiki/Ideas/Ideas.md",
	}, e.files())
	assert.Equal(t, 0, api.Writes, "pull must not change Notion")
	assert.Contains(t, e.run("status"), "nothing to commit", "freshly pulled files must not show as new")

	wiki := e.read("Team Wiki/Team Wiki.md")
	for _, want := range []string{
		"# Welcome",
		"Hello **bold** world",
		"- parent item\n  - nested item",
		"- [x] ship it",
		"| Name | Role |\n| --- | --- |\n| Ada | Engineer |",
		"<summary>Details</summary>",
		"left column",
		"[🗃️ Tasks](./Tasks.csv)",
		"[📄 Meeting Notes](<./Meeting Notes/Meeting Notes.md>)",
		"[📄 Ideas](./Ideas/Ideas.md)",
	} {
		assert.Contains(t, wiki, want)
	}
	// Issue #7: sub-page content is not appended to the parent.
	assert.NotContains(t, wiki, "notes text")
	assert.NotContains(t, wiki, "plan text")

	f, err := os.Open(e.path("Team Wiki/Tasks.csv"))
	require.NoError(t, err)
	records, err := csv.NewReader(f).ReadAll()
	_ = f.Close()
	require.NoError(t, err)
	assert.Equal(t, []string{"Name", "Status"}, records[0])
	assert.Len(t, records, 3)

	// --- Pushing unchanged files changes nothing.
	e.run("sync", "--direction", "push")
	assert.Equal(t, 0, api.Writes, "unchanged files must not be rewritten in Notion")

	// --- Edit the parent page and push: sub-pages, the database and the
	// columns must survive, and the text stays in place. The edited block is
	// changed in place and the others are left alone, so all keep their IDs.
	helloBlock := api.FindBlock(e.root, "Hello")
	welcomeBlock := api.FindBlock(e.root, "Welcome")
	writesBefore := api.Writes
	e.replace("Team Wiki/Team Wiki.md", "Hello **bold** world", "Hello **bold** world, edited locally")
	e.run("sync", "--direction", "push")
	assert.Equal(t, helloBlock, api.FindBlock(e.root, "edited locally"), "an edited block keeps its ID")
	assert.Equal(t, welcomeBlock, api.FindBlock(e.root, "Welcome"), "an unchanged block keeps its ID")
	assert.Equal(t, 1, api.Updates)
	assert.Equal(t, writesBefore+1, api.Writes, "a one-line edit is one write")
	subPages := api.SubPages(e.root)
	require.Contains(t, subPages, "Meeting Notes")
	require.Contains(t, subPages, "Ideas")
	assert.False(t, api.InTrash(subPages["Meeting Notes"]), "pushing a parent must not trash its sub-pages")
	outline := api.Outline(e.root)
	assert.Equal(t, []string{
		"heading_1: Welcome",
		"paragraph: Hello bold world, edited locally",
		"bulleted_list_item: parent item",
		"  bulleted_list_item: nested item",
		"to_do: [x] ship it",
		"table: ",
		"  table_row: Name | Role",
		"  table_row: Ada | Engineer",
		"toggle: Details",
		"  paragraph: hidden detail",
		"column_list: ",
		"  column: ",
		"    paragraph: left column",
		"  column: ",
		"    paragraph: right column",
		"child_database: Tasks",
		"child_page: Meeting Notes",
		"child_page: Ideas",
	}, outline)

	// --- Git-like staging: a new file next to Meeting Notes.md becomes its
	// sub-page.
	e.write("Team Wiki/Meeting Notes/Retro.md", "# Retro\n\nWhat went well")
	status := e.run("status")
	assert.Contains(t, status, "Retro.md")
	e.run("add", "docs/Team Wiki/Meeting Notes/Retro.md")
	e.run("push")
	notesPages := api.SubPages(subPages["Meeting Notes"])
	require.Contains(t, notesPages, "Retro", "new file should be created under the page of its folder")
	assert.Contains(t, e.read("Team Wiki/Meeting Notes/Retro.md"), "notion_id: "+notesPages["Retro"])
	assert.NotContains(t, e.run("status"), "Retro.md", "a pushed file must not show as changed")

	// --- A new folder gets its own page.
	e.write("Team Wiki/Projects/Launch.md", "launch plan")
	e.run("sync", "--direction", "push")
	projects, ok := api.SubPages(e.root)["Projects"]
	require.True(t, ok, "folder page created")
	assert.Contains(t, api.SubPages(projects), "Launch")
	assert.FileExists(t, e.path("Team Wiki/Projects/Projects.md"))
	assert.NotContains(t, e.run("status"), "Projects", "a created folder page must not show as new")

	// --- Content Notion is strict about: long text, deep nesting, tables,
	// code containing $$, local links.
	e.write("Team Wiki/Big.md", strings.Repeat("long text ", 400)+"\n\n"+
		"- a\n  - b\n    - c\n      - d\n\n"+
		"| x | y |\n| --- | --- |\n| 1 |\n\n"+
		"```sh\necho $$\n```\n\n"+
		"see [ideas](./Ideas/Ideas.md) and ![diagram](./diagram.png)\n")
	e.run("sync", "--direction", "push")
	big, ok := api.SubPages(e.root)["Big"]
	require.True(t, ok)
	assert.Contains(t, api.Outline(big), "      bulleted_list_item: d")

	// --- Bidirectional sync: no duplicates; a Notion edit is pulled.
	before := e.files()
	e.run("sync", "--direction", "bidirectional")
	assert.ElementsMatch(t, before, e.files(), "sync must not create duplicate files")

	ideas := subPages["Ideas"]
	api.SetText(api.FindBlock(ideas, "idea text"), "idea edited in Notion")
	e.run("sync", "--direction", "bidirectional")
	assert.Contains(t, e.read("Team Wiki/Ideas/Ideas.md"), "idea edited in Notion")
	assert.ElementsMatch(t, before, e.files())

	// --- A rename in Notion is not reverted by a push, and pull picks it up.
	api.Rename(ideas, "Ideas v2")
	e.replace("Team Wiki/Ideas/Ideas.md", "idea edited in Notion", "idea edited in both")
	e.run("sync", "--direction", "push")
	assert.Equal(t, "Ideas v2", api.Title(ideas))
	e.run("pull")
	assert.Contains(t, e.read("Team Wiki/Ideas/Ideas.md"), "title: Ideas v2", "the file stays where it is")
	assert.Contains(t, e.run("status"), "nothing to commit", "re-pulled files must not show as modified")

	// --- Pull a single nested page by name.
	require.NoError(t, os.Remove(e.path("Team Wiki/Meeting Notes/Q1 Planning/Q1 Planning.md")))
	e.run("pull", "--page", "Q1 Planning.md")
	assert.Contains(t, e.read("Team Wiki/Meeting Notes/Q1 Planning/Q1 Planning.md"), "plan text")

	// --- watch: an edit to a nested file is pushed.
	if runtime.GOOS != "windows" {
		cmd := exec.Command(e.bin, "watch", "--interval", "200ms", "--config", "config.yaml")
		cmd.Dir = e.dir
		cmd.Env = e.env
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		require.NoError(t, cmd.Start())
		time.Sleep(1 * time.Second) // let the watcher start
		e.replace("Team Wiki/Meeting Notes/Q1 Planning/Q1 Planning.md", "plan text", "plan changed while watching")
		q1 := api.SubPages(subPages["Meeting Notes"])["Q1 Planning"]
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) && api.FindBlock(q1, "plan changed while watching") == "" {
			time.Sleep(100 * time.Millisecond)
		}
		_ = cmd.Process.Signal(syscall.SIGINT)
		_ = cmd.Wait()
		t.Logf("watch output:\n%s", out.String())
		assert.NotEmpty(t, api.FindBlock(q1, "plan changed while watching"), "watch should push edits in nested folders")
	}

	// Nothing was ever moved to the trash by accident.
	for name, id := range api.SubPages(e.root) {
		assert.False(t, api.InTrash(id), "%s in the trash", name)
	}
}

func TestEndToEnd_InitAcceptsNewTokenFormat(t *testing.T) {
	e := setup(t)
	dir := filepath.Join(filepath.Dir(e.dir), "fresh")
	require.NoError(t, os.MkdirAll(dir, 0755))

	cmd := exec.Command(e.bin, "init")
	cmd.Dir = dir
	cmd.Env = e.env
	// Issue #6: tokens now start with ntn_. Also paste a page URL.
	cmd.Stdin = strings.NewReader("ntn_123456789012345678901234567890\nhttps://www.notion.so/team/Team-Wiki-1e369015c2408021a930e3ee43ea0ef6\n\n")
	done := make(chan error, 1)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	require.NoError(t, cmd.Start())
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		t.Logf("init output:\n%s", out.String())
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("init did not finish (prompting forever?):\n%s", out.String())
	}
	env, err := os.ReadFile(filepath.Join(dir, ".env"))
	require.NoError(t, err)
	assert.Contains(t, string(env), "ntn_123456789012345678901234567890")
	assert.Contains(t, string(env), "1e369015c2408021a930e3ee43ea0ef6")

	// With the input cut short, init stops instead of looping.
	other := filepath.Join(filepath.Dir(e.dir), "fresh2")
	require.NoError(t, os.MkdirAll(other, 0755))
	cmd = exec.Command(e.bin, "init")
	cmd.Dir = other
	cmd.Env = e.env
	cmd.Stdin = strings.NewReader("")
	finished := make(chan error, 1)
	go func() { _, err := cmd.CombinedOutput(); finished <- err }()
	select {
	case err := <-finished:
		assert.Error(t, err, "init without input should fail")
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("init loops forever when stdin is closed")
	}
}
