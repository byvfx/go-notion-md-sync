package sync

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/byvfx/go-notion-md-sync/pkg/markdown"
	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/byvfx/go-notion-md-sync/pkg/util"
)

// The local layout mirrors the Notion tree. Each page gets a folder with its
// Markdown file inside, and sub-pages go in sub-folders:
//
//	<markdown_root>/
//	└── Parent Page/
//	    ├── Parent Page.md
//	    └── Child Page/
//	        ├── Child Page.md
//	        └── Sub Page/Sub Page.md
//
// Every command (pull, push, sync, watch) uses this same layout.

// pagePlan maps page IDs to the local file each page is synced with.
type pagePlan struct {
	paths map[string]string
}

func (p *pagePlan) path(id string) (string, bool) {
	path, ok := p.paths[normalizeID(id)]
	return path, ok
}

// absRoot returns the markdown root as an absolute, clean path.
func (e *engine) absRoot() string {
	root, err := filepath.Abs(e.config.Directories.MarkdownRoot)
	if err != nil {
		return filepath.Clean(e.config.Directories.MarkdownRoot)
	}
	return root
}

// planPaths decides the local file for each page. A page that already has a
// local file (found by its notion_id) keeps that file, wherever the user put
// it. New pages go in a folder named after the page, inside their parent's
// folder. Sibling pages with the same name get the start of their ID added so
// they don't overwrite each other.
//
// pages must contain the root page; parents must come before children.
func (e *engine) planPaths(pages []notion.Page, local map[string]string) *pagePlan {
	root := e.absRoot()
	rootID := normalizeID(e.config.Notion.ParentPageID)
	plan := &pagePlan{paths: make(map[string]string, len(pages))}

	byID := make(map[string]*notion.Page, len(pages))
	for i := range pages {
		byID[normalizeID(pages[i].ID)] = &pages[i]
	}

	// Folder names, unique among siblings.
	folder := make(map[string]string, len(pages))
	siblings := make(map[string][]*notion.Page)
	for i := range pages {
		p := &pages[i]
		parent := ""
		if p.Parent.Type == "page_id" {
			parent = normalizeID(p.Parent.PageID)
		}
		if normalizeID(p.ID) == rootID {
			parent = ""
		}
		siblings[parent] = append(siblings[parent], p)
	}
	for _, group := range siblings {
		sort.SliceStable(group, func(i, j int) bool {
			if !group[i].CreatedTime.Equal(group[j].CreatedTime) {
				return group[i].CreatedTime.Before(group[j].CreatedTime)
			}
			return group[i].ID < group[j].ID
		})
		used := make(map[string]bool)
		for _, p := range group {
			name := util.SanitizeFileName(e.extractTitleFromPage(p))
			if used[strings.ToLower(name)] {
				short := strings.ReplaceAll(p.ID, "-", "")
				if len(short) > 8 {
					short = short[:8]
				}
				name = name + " (" + short + ")"
			}
			used[strings.ToLower(name)] = true
			folder[normalizeID(p.ID)] = name
		}
	}

	var resolve func(id string, depth int) string
	resolve = func(id string, depth int) string {
		if path, ok := plan.paths[id]; ok {
			return path
		}
		if existing, ok := local[id]; ok {
			plan.paths[id] = existing
			return existing
		}
		name := folder[id]
		if name == "" {
			name = "untitled"
		}
		parentDir := root
		if p, ok := byID[id]; ok && id != rootID && p.Parent.Type == "page_id" && depth < 64 {
			parentID := normalizeID(p.Parent.PageID)
			if _, known := byID[parentID]; known {
				parentDir = filepath.Dir(resolve(parentID, depth+1))
			}
		}
		path, err := util.SecureJoin(parentDir, name, name+".md")
		if err != nil {
			path = filepath.Join(root, "untitled", "untitled.md")
		}
		plan.paths[id] = path
		return path
	}
	for i := range pages {
		resolve(normalizeID(pages[i].ID), 0)
	}
	return plan
}

// walkMarkdown calls fn for every Markdown file under the markdown root,
// skipping excluded paths and hidden folders (like .git and .notion-sync).
func (e *engine) walkMarkdown(fn func(path string) error) error {
	root := e.absRoot()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || e.isExcluded(path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") || e.isExcluded(path) {
			return nil
		}
		return fn(path)
	})
}

// localIndex maps notion_id to file path for the Markdown files under the
// markdown root.
func (e *engine) localIndex() (map[string]string, error) {
	index := make(map[string]string)
	err := e.walkMarkdown(func(path string) error {
		doc, err := e.parser.ParseFile(path)
		if err != nil {
			util.Warning("Skipping %s: %v", path, err)
			return nil
		}
		id, _ := doc.Metadata["notion_id"].(string)
		if id == "" {
			return nil
		}
		key := normalizeID(id)
		if other, dup := index[key]; dup {
			util.Warning("%s and %s have the same notion_id; using %s", other, path, other)
			return nil
		}
		index[key] = path
		return nil
	})
	return index, err
}

func (e *engine) isExcluded(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return util.IsExcluded(e.config.Directories.ExcludedPatterns, e.absRoot(), abs)
}

// writeWithFrontmatter writes content to path with the file's existing
// frontmatter (if any) updated with the given fields. Fields the tool doesn't
// know about are kept.
func (e *engine) writeWithFrontmatter(path string, existing map[string]interface{}, updates map[string]interface{}, content string) error {
	merged := make(map[string]interface{}, len(existing)+len(updates))
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range updates {
		merged[k] = v
	}
	return e.parser.CreateMarkdownWithFrontmatter(path, merged, content)
}

// readExisting returns the metadata of an existing Markdown file, or nil if
// it doesn't exist.
func (e *engine) readExisting(path string) (*markdown.Document, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return e.parser.ParseFile(path)
}

var linkedMarkerRe = regexp.MustCompile(`^(\[[^\n]*\])\((?:<[^>\n]*>|[^)\s]*)\)( <!-- notion:(child_page|child_database) (` + notionIDPattern + `) -->)\s*$`)

// relinkMarkers points the links on sub-page and database marker lines at
// the files they were actually written to.
func relinkMarkers(content string, resolve func(blockType, id string) (string, bool)) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		m := linkedMarkerRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if dest, ok := resolve(m[3], m[4]); ok {
			lines[i] = m[1] + "(" + linkDestination(dest) + ")" + m[2]
		}
	}
	return strings.Join(lines, "\n")
}

// relativeLink returns a "./"-style link from the folder of fromFile to target.
func relativeLink(fromFile, target string) (string, bool) {
	rel, err := filepath.Rel(filepath.Dir(fromFile), target)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") {
		rel = "./" + rel
	}
	return rel, true
}

// describePath shortens a path for messages.
func (e *engine) describePath(path string) string {
	if rel, err := filepath.Rel(e.absRoot(), path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.Join(e.config.Directories.MarkdownRoot, rel)
	}
	return path
}
