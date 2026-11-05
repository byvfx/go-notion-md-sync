package util

import (
	"path/filepath"
	"testing"
)

func TestMatchExcludePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"extension at root", "*.tmp", "a.tmp", true},
		{"extension nested", "*.tmp", "docs/sub/a.tmp", true},
		{"extension no match", "*.tmp", "docs/a.md", false},
		{"bare name matches dir component", "node_modules", "web/node_modules/x.md", true},
		{"bare name exact file", "README.md", "docs/README.md", true},
		{"bare name partial no match", "node", "node_modules/x.md", false},
		{"dir globstar at root", "node_modules/**", "node_modules/pkg/readme.md", true},
		{"dir globstar nested", "node_modules/**", "web/node_modules/pkg/readme.md", true},
		{"dir globstar dir itself", ".git/**", ".git", true},
		{"dir globstar hidden", ".git/**", ".git/objects/ab/cd", true},
		{"dir globstar no match", "node_modules/**", "docs/node_modules.md", false},
		{"dir globstar sibling prefix", "excluded/**", "excluded2/a.md", false},
		{"multi segment dir globstar anchored", "docs/drafts/**", "docs/drafts/a/b.md", true},
		{"multi segment dir globstar not nested", "docs/drafts/**", "x/docs/drafts/a.md", false},
		{"middle globstar zero segments", "a/**/b.md", "a/b.md", true},
		{"middle globstar many segments", "a/**/b.md", "a/x/y/b.md", true},
		{"middle globstar no match", "a/**/b.md", "c/x/b.md", false},
		{"leading globstar", "**/drafts/*.md", "x/y/drafts/a.md", true},
		{"anchored relative pattern", "docs/*.md", "docs/a.md", true},
		{"anchored relative no deep match", "docs/*.md", "docs/sub/a.md", false},
		{"leading slash ignored", "/docs/*.md", "docs/a.md", true},
		{"leading dot slash ignored", "./docs/*.md", "docs/a.md", true},
		{"trailing slash ignored", "build/", "build/out.md", true},
		{"dot slash path", "*.tmp", "./a.tmp", true},
		{"os separators in path", "docs/*.md", filepath.Join("docs", "a.md"), true},
		{"empty pattern", "", "a.md", false},
		{"empty path", "*.md", "", false},
		{"malformed pattern", "[", "a.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchExcludePattern(tt.pattern, tt.path); got != tt.want {
				t.Errorf("MatchExcludePattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestIsExcluded(t *testing.T) {
	root := t.TempDir()
	defaults := []string{"*.tmp", "node_modules/**", ".git/**"}

	tests := []struct {
		name     string
		patterns []string
		root     string
		path     string
		want     bool
	}{
		{"absolute path under root", defaults, root, filepath.Join(root, "docs", "a.tmp"), true},
		{"absolute path not excluded", defaults, root, filepath.Join(root, "docs", "a.md"), false},
		{"absolute git path", defaults, root, filepath.Join(root, ".git", "HEAD"), true},
		{"node modules nested", defaults, root, filepath.Join(root, "x", "node_modules", "y", "z.md"), true},
		{"relative path with empty root", defaults, "", "node_modules/a.md", true},
		{"relative path with absolute root", defaults, root, "docs/a.tmp", true},
		{"parent dir prefix matches", []string{"docs/drafts"}, root, filepath.Join(root, "docs", "drafts", "a", "b.md"), true},
		{"parent dir prefix no match", []string{"docs/drafts"}, root, filepath.Join(root, "docs", "final", "b.md"), false},
		{"root itself", defaults, root, root, false},
		{"no patterns", nil, root, filepath.Join(root, "a.tmp"), false},
		{"pattern relative to root not cwd", []string{"docs/**"}, root, filepath.Join(root, "docs", "a.md"), true},
		{"relative root and path", []string{"sub/*.md"}, "notes", filepath.Join("notes", "sub", "a.md"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsExcluded(tt.patterns, tt.root, tt.path); got != tt.want {
				t.Errorf("IsExcluded(%v, %q, %q) = %v, want %v", tt.patterns, tt.root, tt.path, got, tt.want)
			}
		})
	}
}
