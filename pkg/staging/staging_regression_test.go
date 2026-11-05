package staging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestArea(t *testing.T) (*StagingArea, string) {
	t.Helper()
	dir := t.TempDir()
	sa := NewStagingArea(dir)
	if err := sa.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	return sa, dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func setOld(t *testing.T, path string, age time.Duration) {
	t.Helper()
	old := time.Now().Add(-age)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
}

// TestGetStatus_Concurrent is a regression test for concurrent map writes
// to the index from the status workers. Run with -race.
func TestGetStatus_Concurrent(t *testing.T) {
	sa, dir := newTestArea(t)

	var keys []string
	for i := 0; i < 40; i++ {
		key := fmt.Sprintf("docs/file%02d.md", i)
		writeFile(t, filepath.Join(dir, key), fmt.Sprintf("# File %d", i))
		keys = append(keys, key)
	}
	for _, key := range keys {
		if err := sa.AddFile(key); err != nil {
			t.Fatalf("AddFile(%s) error = %v", key, err)
		}
	}
	if err := sa.MarkSynced(keys); err != nil {
		t.Fatalf("MarkSynced() error = %v", err)
	}

	// Touch every file so the workers want to refresh timestamps.
	for _, key := range keys {
		setOld(t, filepath.Join(dir, key), time.Minute)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, err := sa.GetStatus()
			if err != nil {
				errs <- err
				return
			}
			if len(status) != 0 {
				errs <- fmt.Errorf("expected no changes, got %v", status)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// The index must still be valid JSON with every entry.
	data, err := os.ReadFile(filepath.Join(dir, StagingDir, IndexFile))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var index map[string]FileEntry
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("index is corrupt: %v", err)
	}
	if len(index) != len(keys) {
		t.Errorf("index has %d entries, want %d", len(index), len(keys))
	}

	// No temp files left behind.
	leftovers, err := filepath.Glob(filepath.Join(dir, StagingDir, IndexFile+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(leftovers) != 0 {
		t.Errorf("temporary index files left behind: %v", leftovers)
	}
}

// TestMarkSynced_RefreshesHash is a regression test: a push writes notion_id
// into the frontmatter, and the file must not show as modified afterwards.
func TestMarkSynced_RefreshesHash(t *testing.T) {
	sa, dir := newTestArea(t)
	path := filepath.Join(dir, "page.md")
	writeFile(t, path, "# Page\n")

	if err := sa.AddFile("page.md"); err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}

	// Simulate the engine writing frontmatter back during push.
	writeFile(t, path, "---\nnotion_id: abc123\n---\n# Page\n")

	if err := sa.MarkSynced([]string{"page.md"}); err != nil {
		t.Fatalf("MarkSynced() error = %v", err)
	}

	status, err := sa.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if st, ok := status["page.md"]; ok {
		t.Errorf("expected page.md unmodified after MarkSynced, got %v", st)
	}

	// A later real edit is detected.
	writeFile(t, path, "---\nnotion_id: abc123\n---\n# Page\n\nEdited\n")
	status, err = sa.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if status["page.md"] != StatusModified {
		t.Errorf("expected page.md modified, got %v", status["page.md"])
	}
}

// TestGetStatus_UntrackedIsNew is a regression test: untracked files used to
// be reported unmodified when their mtime was old, so `add .` skipped them.
func TestGetStatus_UntrackedIsNew(t *testing.T) {
	tests := []struct {
		name    string
		content string
		age     time.Duration
	}{
		{"recent plain file", "# New\n", 0},
		{"old plain file", "# Old\n", 48 * time.Hour},
		{"old file with notion_id", "---\nnotion_id: abc\n---\n# Old\n", 72 * time.Hour},
		{"recent file with notion_id", "---\nnotion_id: abc\n---\n# Recent\n", time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa, dir := newTestArea(t)
			path := filepath.Join(dir, "sub", "file.md")
			writeFile(t, path, tt.content)
			if tt.age > 0 {
				setOld(t, path, tt.age)
			}

			status, err := sa.GetStatus()
			if err != nil {
				t.Fatalf("GetStatus() error = %v", err)
			}
			if status["sub/file.md"] != StatusNew {
				t.Errorf("expected untracked file to be new, got %v (status %v)", status["sub/file.md"], status)
			}
		})
	}
}

// TestGetStatus_TrackedUsesHash ensures tracked files are compared by
// content, not modification time.
func TestGetStatus_TrackedUsesHash(t *testing.T) {
	tests := []struct {
		name       string
		newContent string
		touchOld   bool
		want       FileStatus
	}{
		{"touched but unchanged", "# Same\n", false, StatusUnmodified},
		{"changed with old mtime", "# Different\n", true, StatusModified},
		{"changed with new mtime", "# Different\n", false, StatusModified},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa, dir := newTestArea(t)
			path := filepath.Join(dir, "a.md")
			writeFile(t, path, "# Same\n")
			if err := sa.AddFile("a.md"); err != nil {
				t.Fatalf("AddFile() error = %v", err)
			}
			if err := sa.MarkSynced([]string{"a.md"}); err != nil {
				t.Fatalf("MarkSynced() error = %v", err)
			}

			writeFile(t, path, tt.newContent)
			if tt.touchOld {
				setOld(t, path, 48*time.Hour)
			}

			status, err := sa.GetStatus()
			if err != nil {
				t.Fatalf("GetStatus() error = %v", err)
			}
			got, ok := status["a.md"]
			if !ok {
				got = StatusUnmodified
			}
			if got != tt.want {
				t.Errorf("status = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPathNormalization is a regression test: "./docs/a.md", "docs/a.md"
// and the absolute path must all refer to the same index entry.
func TestPathNormalization(t *testing.T) {
	sa, dir := newTestArea(t)
	writeFile(t, filepath.Join(dir, "docs", "a.md"), "# A\n")

	variants := []string{
		"./docs/a.md",
		"docs/a.md",
		"docs//a.md",
		"docs/../docs/a.md",
		filepath.Join(dir, "docs", "a.md"),
	}
	for _, v := range variants {
		if err := sa.AddFile(v); err != nil {
			t.Fatalf("AddFile(%q) error = %v", v, err)
		}
	}

	staged, err := sa.GetStagedFiles()
	if err != nil {
		t.Fatalf("GetStagedFiles() error = %v", err)
	}
	if len(staged) != 1 || staged[0] != "docs/a.md" {
		t.Fatalf("staged = %v, want [docs/a.md]", staged)
	}

	status, err := sa.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if len(status) != 1 || status["docs/a.md"] != StatusStaged {
		t.Errorf("status = %v, want only docs/a.md staged", status)
	}

	// Reset and MarkSynced accept the other spellings too.
	if err := sa.ResetFile(filepath.Join(dir, "docs", "a.md")); err != nil {
		t.Fatalf("ResetFile() error = %v", err)
	}
	staged, err = sa.GetStagedFiles()
	if err != nil {
		t.Fatalf("GetStagedFiles() error = %v", err)
	}
	if len(staged) != 0 {
		t.Errorf("expected nothing staged after reset, got %v", staged)
	}

	if err := sa.AddFile("docs/a.md"); err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}
	if err := sa.MarkSynced([]string{"./docs/a.md"}); err != nil {
		t.Fatalf("MarkSynced() error = %v", err)
	}
	status, err = sa.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if len(status) != 0 {
		t.Errorf("expected clean status after MarkSynced, got %v", status)
	}
}

// TestLoadIndex_MergesLegacyDuplicateKeys ensures indexes written by older
// versions with un-normalised keys are merged on load.
func TestLoadIndex_MergesLegacyDuplicateKeys(t *testing.T) {
	sa, dir := newTestArea(t)
	writeFile(t, filepath.Join(dir, "docs", "a.md"), "# A\n")

	legacy := map[string]FileEntry{
		"./docs/a.md": {Path: "./docs/a.md", Hash: "old", Staged: true},
		"docs/a.md":   {Path: "docs/a.md", Hash: "new", LastModified: time.Now()},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, StagingDir, IndexFile), data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	staged, err := sa.GetStagedFiles()
	if err != nil {
		t.Fatalf("GetStagedFiles() error = %v", err)
	}
	if len(staged) != 1 || staged[0] != "docs/a.md" {
		t.Errorf("staged = %v, want [docs/a.md]", staged)
	}
}

func TestNormalizePath(t *testing.T) {
	dir := t.TempDir()
	sa := NewStagingArea(dir)

	tests := []struct {
		in   string
		want string
	}{
		{"a.md", "a.md"},
		{"./a.md", "a.md"},
		{"docs/./b.md", "docs/b.md"},
		{filepath.Join("docs", "sub", "c.md"), "docs/sub/c.md"},
		{filepath.Join(dir, "docs", "d.md"), "docs/d.md"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := sa.NormalizePath(tt.in); got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
