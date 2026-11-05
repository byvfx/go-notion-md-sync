package tui

import (
	"path/filepath"
	"testing"

	"github.com/byvfx/go-notion-md-sync/pkg/staging"
)

func TestScanner_UsesStagingStatus(t *testing.T) {
	wd := t.TempDir()
	root := filepath.Join(wd, "docs")
	write := func(rel, content string) {
		writeTestFile(t, filepath.Join(root, rel), content)
	}
	write("synced.md", "# Synced\n")
	write("modified.md", "# Before\n")
	write("untracked.md", "---\nnotion_id: abc\n---\n# Untracked\n")
	write("staged.md", "# Staged\n")
	write(filepath.Join("drafts", "skip.md"), "# Excluded\n")
	write(filepath.Join("deep", "x.tmp.md"), "# Kept\n")

	area := staging.NewStagingArea(wd)
	if err := area.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	for _, f := range []string{"synced.md", "modified.md", "staged.md"} {
		if err := area.AddFile(filepath.Join(root, f)); err != nil {
			t.Fatalf("AddFile(%s) error = %v", f, err)
		}
	}
	if err := area.MarkSynced([]string{filepath.Join(root, "synced.md"), filepath.Join(root, "modified.md")}); err != nil {
		t.Fatalf("MarkSynced() error = %v", err)
	}
	write("modified.md", "# After\n")

	scanner := &FileScanner{config: testConfig(root), stagingRoot: wd}
	items, err := scanner.ScanFiles()
	if err != nil {
		t.Fatalf("ScanFiles() error = %v", err)
	}

	got := make(map[string]SyncStatus)
	for _, it := range items {
		fi := it.(FileItem)
		if !fi.IsDirectory {
			got[filepath.ToSlash(fi.Name)] = fi.Status
		}
	}

	want := map[string]SyncStatus{
		"synced.md":     StatusSynced,
		"modified.md":   StatusModified,
		"untracked.md":  StatusPending,
		"staged.md":     StatusPending,
		"deep/x.tmp.md": StatusPending,
	}
	if len(got) != len(want) {
		t.Errorf("scanned files = %v, want %v", got, want)
	}
	for name, st := range want {
		if got[name] != st {
			t.Errorf("%s: status %v, want %v", name, got[name], st)
		}
	}
}

func TestScanner_FallbackWithoutStaging(t *testing.T) {
	wd := t.TempDir()
	root := filepath.Join(wd, "docs")
	writeTestFile(t, filepath.Join(root, "linked.md"), "---\ntitle: x\nnotion_id: \"abc\"\n---\n# Linked\n")
	writeTestFile(t, filepath.Join(root, "local.md"), "# Local only\n")
	writeTestFile(t, filepath.Join(root, "empty-id.md"), "---\nnotion_id: \"\"\n---\n# Empty\n")

	scanner := &FileScanner{config: testConfig(root), stagingRoot: wd}
	items, err := scanner.ScanFiles()
	if err != nil {
		t.Fatalf("ScanFiles() error = %v", err)
	}

	want := map[string]SyncStatus{
		"linked.md":   StatusSynced,
		"local.md":    StatusPending,
		"empty-id.md": StatusPending,
	}
	for _, it := range items {
		fi := it.(FileItem)
		if st, ok := want[fi.Name]; ok && fi.Status != st {
			t.Errorf("%s: status %v, want %v", fi.Name, fi.Status, st)
		}
	}
}
