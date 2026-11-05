package watcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEngine implements sync.Engine for testing
type mockEngine struct {
	mu          sync.Mutex
	syncedFiles []string
	syncError   error
	// onSync, when set, runs after a successful sync (e.g. to simulate the
	// engine writing frontmatter back into the file).
	onSync func(filePath string)
}

func (m *mockEngine) SyncFileToNotion(ctx context.Context, filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.syncError != nil {
		return m.syncError
	}

	m.syncedFiles = append(m.syncedFiles, filePath)
	if m.onSync != nil {
		m.onSync(filePath)
	}
	return nil
}

func (m *mockEngine) SyncNotionToFile(ctx context.Context, pageID, filePath string) error {
	return nil
}

func (m *mockEngine) SyncAll(ctx context.Context, direction string) error {
	return nil
}

func (m *mockEngine) SyncSpecificFile(ctx context.Context, filename, direction string) error {
	return nil
}

func (m *mockEngine) getSyncedFiles() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.syncedFiles...)
}

func (m *mockEngine) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncedFiles = nil
	m.syncError = nil
	m.onSync = nil
}

func (m *mockEngine) setSyncError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncError = err
}

func (m *mockEngine) setOnSync(fn func(filePath string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSync = fn
}

// countSyncs returns how many times filePath has been synced.
func (m *mockEngine) countSyncs(filePath string) int {
	count := 0
	for _, f := range m.getSyncedFiles() {
		if f == filePath {
			count++
		}
	}
	return count
}

// newFastWatcher creates a watcher with a short debounce interval.
func newFastWatcher(t *testing.T, root string, engine *mockEngine) *Watcher {
	t.Helper()
	w, err := NewWatcher(createTestConfig(root), engine)
	require.NoError(t, err)
	w.debouncer.interval = 50 * time.Millisecond
	w.out = io.Discard
	t.Cleanup(func() { _ = w.Close() })
	return w
}

func setupTestDir(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "watcher-test-*")
	require.NoError(t, err)

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func createTestConfig(markdownRoot string) *config.Config {
	cfg := &config.Config{}
	cfg.Directories.MarkdownRoot = markdownRoot
	cfg.Directories.ExcludedPatterns = []string{
		"*.tmp",
		"excluded/**",
	}
	cfg.Notion.Token = "test-token"
	cfg.Notion.ParentPageID = "test-page-id"
	cfg.Sync.Direction = "push"
	cfg.Sync.ConflictResolution = "newer"
	return cfg
}

func TestNewWatcher(t *testing.T) {
	tests := []struct {
		name          string
		setupDir      bool
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid directory",
			setupDir:    true,
			expectError: false,
		},
		{
			name:          "non-existent directory",
			setupDir:      false,
			expectError:   true,
			errorContains: "failed to watch directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var markdownRoot string
			var cleanup func()

			if tt.setupDir {
				markdownRoot, cleanup = setupTestDir(t)
				defer cleanup()
			} else {
				markdownRoot = "/non/existent/directory"
			}

			cfg := createTestConfig(markdownRoot)
			engine := &mockEngine{}

			watcher, err := NewWatcher(cfg, engine)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, watcher)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, watcher)
				assert.NotNil(t, watcher.fsWatcher)
				assert.Equal(t, engine, watcher.engine)
				assert.Equal(t, cfg, watcher.config)
				assert.NotNil(t, watcher.debouncer)
				assert.Equal(t, 2*time.Second, watcher.debouncer.interval)

				// Clean up
				_ = watcher.Close()
			}
		})
	}
}

func TestWatcher_Close(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	cfg := createTestConfig(tempDir)
	engine := &mockEngine{}

	watcher, err := NewWatcher(cfg, engine)
	require.NoError(t, err)

	// Test closing
	err = watcher.Close()
	assert.NoError(t, err)

	// Test closing again (should not panic)
	err = watcher.Close()
	assert.NoError(t, err)
}

func TestWatcher_handleEvent(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	engine := &mockEngine{}
	watcher := newFastWatcher(t, tempDir, engine)
	ctx := context.Background()

	tests := []struct {
		name        string
		file        string
		createFile  bool
		op          fsnotify.Op
		shouldSync  bool
		description string
	}{
		{
			name:        "markdown write event",
			file:        "test.md",
			createFile:  true,
			op:          fsnotify.Write,
			shouldSync:  true,
			description: "should sync markdown files on write",
		},
		{
			name:        "non-markdown file",
			file:        "test.txt",
			createFile:  true,
			op:          fsnotify.Write,
			shouldSync:  false,
			description: "should ignore non-markdown files",
		},
		{
			name:        "markdown create event",
			file:        "new.md",
			createFile:  true,
			op:          fsnotify.Create,
			shouldSync:  true,
			description: "should sync on create (also how rename-into is reported)",
		},
		{
			name:        "markdown rename event with file present",
			file:        "renamed.md",
			createFile:  true,
			op:          fsnotify.Rename,
			shouldSync:  true,
			description: "should sync when a file exists at the renamed path",
		},
		{
			name:        "markdown rename event with file gone",
			file:        "moved-away.md",
			createFile:  false,
			op:          fsnotify.Rename,
			shouldSync:  false,
			description: "should ignore renames away from the path",
		},
		{
			name:        "markdown remove event",
			file:        "deleted.md",
			createFile:  false,
			op:          fsnotify.Remove,
			shouldSync:  false,
			description: "should ignore remove events",
		},
		{
			name:        "markdown chmod event",
			file:        "chmod.md",
			createFile:  true,
			op:          fsnotify.Chmod,
			shouldSync:  false,
			description: "should ignore chmod events",
		},
		{
			name:        "excluded file",
			file:        "temp.tmp",
			createFile:  true,
			op:          fsnotify.Write,
			shouldSync:  false,
			description: "should ignore excluded files",
		},
		{
			name:        "markdown in excluded dir",
			file:        filepath.Join("excluded", "a.md"),
			createFile:  true,
			op:          fsnotify.Write,
			shouldSync:  false,
			description: "should ignore files under excluded/**",
		},
		{
			name:        "write to missing file",
			file:        "missing.md",
			createFile:  false,
			op:          fsnotify.Write,
			shouldSync:  false,
			description: "should not sync files that no longer exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine.reset()

			path := filepath.Join(tempDir, tt.file)
			if tt.createFile {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
				require.NoError(t, os.WriteFile(path, []byte("# "+tt.name), 0644))
			}

			watcher.handleEvent(ctx, fsnotify.Event{Name: path, Op: tt.op})

			// Wait for debouncing to complete
			time.Sleep(200 * time.Millisecond)

			syncedFiles := engine.getSyncedFiles()
			if tt.shouldSync {
				require.Len(t, syncedFiles, 1, tt.description)
				assert.Equal(t, path, syncedFiles[0])
			} else {
				assert.Len(t, syncedFiles, 0, tt.description)
			}
		})
	}
}

func TestWatcher_isExcluded(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	cfg := createTestConfig(tempDir)
	cfg.Directories.ExcludedPatterns = append(cfg.Directories.ExcludedPatterns, "node_modules/**", ".git/**")
	engine := &mockEngine{}

	watcher, err := NewWatcher(cfg, engine)
	require.NoError(t, err)
	defer func() { _ = watcher.Close() }()

	tests := []struct {
		name     string
		path     string
		excluded bool
	}{
		{"normal markdown file", "docs/test.md", false},
		{"temp file in root", "temp.tmp", true},
		{"excluded directory file", "excluded/test.md", true},
		{"nested file with tmp extension", "docs/test.tmp", true},
		{"absolute nested tmp", filepath.Join(tempDir, "a", "b", "c.tmp"), true},
		{"absolute normal file", filepath.Join(tempDir, "a", "b", "c.md"), false},
		{"nested node_modules", filepath.Join(tempDir, "web", "node_modules", "x.md"), true},
		{"git dir", filepath.Join(tempDir, ".git", "x.md"), true},
		{"excluded dir itself", filepath.Join(tempDir, "excluded"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.excluded, watcher.isExcluded(tt.path))
		})
	}
}

func TestWatcher_syncFile(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	engine := &mockEngine{}
	watcher := newFastWatcher(t, tempDir, engine)

	ctx := context.Background()
	testFile := filepath.Join(tempDir, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test"), 0644))

	t.Run("successful sync", func(t *testing.T) {
		engine.reset()
		watcher.syncFile(ctx, testFile)

		syncedFiles := engine.getSyncedFiles()
		assert.Len(t, syncedFiles, 1)
		assert.Equal(t, testFile, syncedFiles[0])
	})

	t.Run("unchanged content is not synced again", func(t *testing.T) {
		engine.reset()
		watcher.syncFile(ctx, testFile)
		assert.Len(t, engine.getSyncedFiles(), 0)
	})

	t.Run("sync error", func(t *testing.T) {
		engine.reset()
		engine.setSyncError(fmt.Errorf("sync failed"))
		require.NoError(t, os.WriteFile(testFile, []byte("# Changed"), 0644))

		// This should not panic even with an error
		watcher.syncFile(ctx, testFile)
		assert.Len(t, engine.getSyncedFiles(), 0)
	})

	t.Run("failed sync is retried on next event", func(t *testing.T) {
		engine.reset()
		watcher.syncFile(ctx, testFile)
		assert.Len(t, engine.getSyncedFiles(), 1)
	})

	t.Run("cancelled context skips sync", func(t *testing.T) {
		engine.reset()
		require.NoError(t, os.WriteFile(testFile, []byte("# Changed again"), 0644))
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		watcher.syncFile(cancelled, testFile)
		assert.Len(t, engine.getSyncedFiles(), 0)
	})
}

// TestWatcher_SelfTriggeredEventSuppressed is a regression test: the engine
// writes frontmatter (notion_id) back into a newly pushed file, which fires
// another write event. That event must not cause a second push.
func TestWatcher_SelfTriggeredEventSuppressed(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	engine := &mockEngine{}
	watcher := newFastWatcher(t, tempDir, engine)
	ctx := context.Background()

	testFile := filepath.Join(tempDir, "page.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Page\n"), 0644))

	engine.setOnSync(func(filePath string) {
		// Simulate the engine writing frontmatter, then the resulting event.
		content, err := os.ReadFile(filePath)
		if err != nil {
			return
		}
		if !strings.HasPrefix(string(content), "---") {
			_ = os.WriteFile(filePath, append([]byte("---\nnotion_id: abc\n---\n"), content...), 0644)
		}
		watcher.handleEvent(ctx, fsnotify.Event{Name: filePath, Op: fsnotify.Write})
	})

	watcher.handleEvent(ctx, fsnotify.Event{Name: testFile, Op: fsnotify.Write})
	time.Sleep(400 * time.Millisecond)

	assert.Equal(t, 1, engine.countSyncs(testFile), "frontmatter write-back must not trigger a second push")

	// A real user edit afterwards is still synced.
	engine.setOnSync(nil)
	require.NoError(t, os.WriteFile(testFile, []byte("---\nnotion_id: abc\n---\n# Page\n\nEdited\n"), 0644))
	watcher.handleEvent(ctx, fsnotify.Event{Name: testFile, Op: fsnotify.Write})
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 2, engine.countSyncs(testFile))
}

func TestDebouncer(t *testing.T) {
	d := &debouncer{
		interval: 100 * time.Millisecond,
		pending:  make(map[string]*time.Timer),
	}

	t.Run("single event", func(t *testing.T) {
		var executed atomic.Bool

		d.debounce("test-key", func() {
			executed.Store(true)
		})

		// Should not execute immediately
		assert.False(t, executed.Load())

		// Wait for debounce interval
		time.Sleep(150 * time.Millisecond)
		assert.True(t, executed.Load())
	})

	t.Run("multiple events debounced", func(t *testing.T) {
		var execCount atomic.Int32

		// Send multiple events rapidly
		for i := 0; i < 5; i++ {
			d.debounce("test-key-2", func() {
				execCount.Add(1)
			})
			time.Sleep(50 * time.Millisecond) // Less than debounce interval
		}

		// Wait for debounce interval
		time.Sleep(150 * time.Millisecond)

		// Should only execute once
		assert.Equal(t, int32(1), execCount.Load())
	})

	t.Run("different keys execute separately", func(t *testing.T) {
		var exec1, exec2 atomic.Bool

		d.debounce("key1", func() { exec1.Store(true) })
		d.debounce("key2", func() { exec2.Store(true) })

		time.Sleep(150 * time.Millisecond)

		assert.True(t, exec1.Load())
		assert.True(t, exec2.Load())
	})

	t.Run("pending entries are removed after firing", func(t *testing.T) {
		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, 0, d.pendingCount())
	})
}

// TestDebouncer_Concurrent is a regression test for the unsynchronised
// pending map; run with -race.
func TestDebouncer_Concurrent(t *testing.T) {
	d := &debouncer{
		interval: time.Millisecond,
		pending:  make(map[string]*time.Timer),
	}

	var calls atomic.Int32
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				d.debounce(fmt.Sprintf("key-%d", (g+i)%5), func() { calls.Add(1) })
			}
		}(g)
	}
	wg.Wait()

	assert.Eventually(t, func() bool { return d.pendingCount() == 0 }, 2*time.Second, 10*time.Millisecond)
	assert.Greater(t, calls.Load(), int32(0))
}

// TestDebouncer_StaleTimerKeepsNewerEntry ensures a timer that was replaced
// does not delete the entry of the timer that replaced it.
func TestDebouncer_StaleTimerKeepsNewerEntry(t *testing.T) {
	d := &debouncer{
		interval: 60 * time.Millisecond,
		pending:  make(map[string]*time.Timer),
	}

	var first, second atomic.Int32
	d.debounce("k", func() { first.Add(1) })
	time.Sleep(20 * time.Millisecond)
	d.debounce("k", func() { second.Add(1) })

	// After the first timer would have fired, the second must still be
	// pending.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, d.pendingCount())

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(0), first.Load())
	assert.Equal(t, int32(1), second.Load())
	assert.Equal(t, 0, d.pendingCount())
}

func TestDebouncer_Stop(t *testing.T) {
	d := &debouncer{
		interval: 50 * time.Millisecond,
		pending:  make(map[string]*time.Timer),
	}

	var calls atomic.Int32
	d.debounce("a", func() { calls.Add(1) })
	d.debounce("b", func() { calls.Add(1) })
	d.stop()

	// Calls after stop are ignored.
	d.debounce("c", func() { calls.Add(1) })

	time.Sleep(120 * time.Millisecond)
	assert.Equal(t, int32(0), calls.Load())
	assert.Equal(t, 0, d.pendingCount())
}

// TestWatcher_CloseStopsPendingSyncs ensures a debounced sync scheduled
// before Close never runs.
func TestWatcher_CloseStopsPendingSyncs(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	engine := &mockEngine{}
	watcher := newFastWatcher(t, tempDir, engine)

	testFile := filepath.Join(tempDir, "a.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# A"), 0644))

	watcher.handleEvent(context.Background(), fsnotify.Event{Name: testFile, Op: fsnotify.Write})
	require.NoError(t, watcher.Close())

	time.Sleep(150 * time.Millisecond)
	assert.Len(t, engine.getSyncedFiles(), 0)
}

// TestWatcher_NestedDirectories is a regression test: pull creates nested
// directories (root/Parent/Child/Child.md), all of which must be watched,
// including directories created after the watcher started.
func TestWatcher_NestedDirectories(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	existing := filepath.Join(tempDir, "Parent", "Child")
	require.NoError(t, os.MkdirAll(existing, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".git", "objects"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".notion-sync"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "excluded", "deep"), 0755))

	engine := &mockEngine{}
	watcher := newFastWatcher(t, tempDir, engine)

	watched := watcher.fsWatcher.WatchList()
	assert.Contains(t, watched, tempDir)
	assert.Contains(t, watched, filepath.Join(tempDir, "Parent"))
	assert.Contains(t, watched, existing)
	for _, dir := range watched {
		rel, err := filepath.Rel(tempDir, dir)
		require.NoError(t, err)
		assert.NotContains(t, rel, ".git", "hidden directories must not be watched")
		assert.NotContains(t, rel, ".notion-sync", "hidden directories must not be watched")
		assert.NotContains(t, rel, "excluded", "excluded directories must not be watched")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- watcher.Start(ctx) }()

	// File in a pre-existing nested directory.
	nestedFile := filepath.Join(existing, "Child.md")
	require.NoError(t, os.WriteFile(nestedFile, []byte("# Child"), 0644))
	assert.Eventually(t, func() bool { return engine.countSyncs(nestedFile) == 1 }, 2*time.Second, 20*time.Millisecond)

	// Directory created after start, then a file inside it.
	newDir := filepath.Join(tempDir, "New Page", "Sub")
	require.NoError(t, os.MkdirAll(newDir, 0755))
	assert.Eventually(t, func() bool {
		for _, dir := range watcher.fsWatcher.WatchList() {
			if dir == newDir {
				return true
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond, "new directories must be watched")

	newFile := filepath.Join(newDir, "Sub.md")
	require.NoError(t, os.WriteFile(newFile, []byte("# Sub"), 0644))
	assert.Eventually(t, func() bool { return engine.countSyncs(newFile) == 1 }, 2*time.Second, 20*time.Millisecond)

	cancel()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}
}

// TestWatcher_AtomicSave is a regression test for editors that save by
// writing a temp file and renaming it over the original.
func TestWatcher_AtomicSave(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	target := filepath.Join(tempDir, "doc.md")
	require.NoError(t, os.WriteFile(target, []byte("# Original"), 0644))

	engine := &mockEngine{}
	watcher := newFastWatcher(t, tempDir, engine)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- watcher.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	tmp := filepath.Join(tempDir, "doc.md.tmp~")
	require.NoError(t, os.WriteFile(tmp, []byte("# Saved atomically"), 0644))
	require.NoError(t, os.Rename(tmp, target))

	assert.Eventually(t, func() bool { return engine.countSyncs(target) == 1 }, 2*time.Second, 20*time.Millisecond)

	cancel()
	<-done
}

func TestWatcher_Start_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	cfg := createTestConfig(tempDir)
	engine := &mockEngine{}

	watcher, err := NewWatcher(cfg, engine)
	require.NoError(t, err)
	defer func() { _ = watcher.Close() }()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start watcher in goroutine
	watcherErr := make(chan error, 1)
	go func() {
		watcherErr <- watcher.Start(ctx)
	}()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create and modify a test file
	testFile := filepath.Join(tempDir, "integration-test.md")
	testContent := "# Test Document\n\nThis is a test."

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Wait for file to be processed (debounce + processing time)
	time.Sleep(3 * time.Second)

	// Check that file was synced
	syncedFiles := engine.getSyncedFiles()
	assert.Contains(t, syncedFiles, testFile)

	// Modify the file again
	modifiedContent := testContent + "\n\nModified content."
	err = os.WriteFile(testFile, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Wait for second sync
	time.Sleep(3 * time.Second)

	// Should have synced twice
	syncedFiles = engine.getSyncedFiles()
	syncCount := 0
	for _, file := range syncedFiles {
		if file == testFile {
			syncCount++
		}
	}
	assert.GreaterOrEqual(t, syncCount, 2)

	// Cancel context to stop watcher
	cancel()

	// Wait for watcher to stop
	select {
	case err := <-watcherErr:
		// Accept either canceled or deadline exceeded
		assert.True(t, err == context.Canceled || err == context.DeadlineExceeded,
			"Expected context.Canceled or context.DeadlineExceeded, got %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Watcher did not stop within timeout")
	}
}

func TestWatcher_ExcludedPatterns_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create excluded subdirectory
	excludedDir := filepath.Join(tempDir, "excluded")
	err := os.MkdirAll(excludedDir, 0755)
	require.NoError(t, err)

	cfg := createTestConfig(tempDir)
	engine := &mockEngine{}

	watcher, err := NewWatcher(cfg, engine)
	require.NoError(t, err)
	defer func() { _ = watcher.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watcher
	watcherErr := make(chan error, 1)
	go func() {
		watcherErr <- watcher.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create files that should be excluded
	tmpFile := filepath.Join(tempDir, "temp.tmp")
	excludedFile := filepath.Join(excludedDir, "test.md")

	err = os.WriteFile(tmpFile, []byte("temp content"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(excludedFile, []byte("# Excluded"), 0644)
	require.NoError(t, err)

	// Create file that should be included
	normalFile := filepath.Join(tempDir, "normal.md")
	err = os.WriteFile(normalFile, []byte("# Normal"), 0644)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(3 * time.Second)

	syncedFiles := engine.getSyncedFiles()

	// Should only contain the normal file
	normalFileFound := false
	tmpFileFound := false
	for _, file := range syncedFiles {
		if strings.Contains(file, "temp.tmp") {
			tmpFileFound = true
		}
		if file == normalFile {
			normalFileFound = true
		}
	}

	assert.True(t, normalFileFound, "Normal markdown file should have been synced")
	assert.False(t, tmpFileFound, "Temp file should not have been synced")

	// Cancel context to stop watcher
	cancel()

	// Check watcher error
	select {
	case err := <-watcherErr:
		assert.True(t, err == context.Canceled || err == context.DeadlineExceeded,
			"Expected context.Canceled or context.DeadlineExceeded, got %v", err)
	case <-time.After(1 * time.Second):
		t.Error("Watcher did not return within timeout")
	}
}
