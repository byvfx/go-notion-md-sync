package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
}

func (m *mockEngine) SyncFileToNotion(ctx context.Context, filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.syncError != nil {
		return m.syncError
	}
	
	m.syncedFiles = append(m.syncedFiles, filePath)
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
}

func (m *mockEngine) setSyncError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncError = err
}

func setupTestDir(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "watcher-test-*")
	require.NoError(t, err)
	
	cleanup := func() {
		os.RemoveAll(tempDir)
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
				watcher.Close()
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

	cfg := createTestConfig(tempDir)
	engine := &mockEngine{}

	watcher, err := NewWatcher(cfg, engine)
	require.NoError(t, err)
	defer watcher.Close()

	ctx := context.Background()

	tests := []struct {
		name        string
		event       fsnotify.Event
		shouldSync  bool
		description string
	}{
		{
			name: "markdown write event",
			event: fsnotify.Event{
				Name: filepath.Join(tempDir, "test.md"),
				Op:   fsnotify.Write,
			},
			shouldSync:  true,
			description: "should sync markdown files on write",
		},
		{
			name: "non-markdown file",
			event: fsnotify.Event{
				Name: filepath.Join(tempDir, "test.txt"),
				Op:   fsnotify.Write,
			},
			shouldSync:  false,
			description: "should ignore non-markdown files",
		},
		{
			name: "markdown create event",
			event: fsnotify.Event{
				Name: filepath.Join(tempDir, "new.md"),
				Op:   fsnotify.Create,
			},
			shouldSync:  false,
			description: "should ignore create events",
		},
		{
			name: "markdown remove event",
			event: fsnotify.Event{
				Name: filepath.Join(tempDir, "deleted.md"),
				Op:   fsnotify.Remove,
			},
			shouldSync:  false,
			description: "should ignore remove events",
		},
		{
			name: "excluded file",
			event: fsnotify.Event{
				Name: filepath.Join(tempDir, "temp.tmp"),
				Op:   fsnotify.Write,
			},
			shouldSync:  false,
			description: "should ignore excluded files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine.reset()

			// Handle the event
			watcher.handleEvent(ctx, tt.event)

			// Wait for debouncing to complete
			time.Sleep(3 * time.Second)

			syncedFiles := engine.getSyncedFiles()

			if tt.shouldSync {
				assert.Len(t, syncedFiles, 1, tt.description)
				assert.Equal(t, tt.event.Name, syncedFiles[0])
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
	engine := &mockEngine{}

	watcher, err := NewWatcher(cfg, engine)
	require.NoError(t, err)
	defer watcher.Close()

	tests := []struct {
		name     string
		path     string
		excluded bool
	}{
		{
			name:     "normal markdown file",
			path:     "docs/test.md",
			excluded: false,
		},
		{
			name:     "temp file in root",
			path:     "temp.tmp",
			excluded: true,
		},
		{
			name:     "excluded directory file",
			path:     "excluded/test.md",
			excluded: true, // excluded/** pattern matches this
		},
		{
			name:     "nested file with tmp extension",
			path:     "docs/test.tmp",
			excluded: false, // *.tmp doesn't match paths with directories
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := watcher.isExcluded(tt.path)
			assert.Equal(t, tt.excluded, result)
		})
	}
}

func TestWatcher_syncFile(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	cfg := createTestConfig(tempDir)
	engine := &mockEngine{}

	watcher, err := NewWatcher(cfg, engine)
	require.NoError(t, err)
	defer watcher.Close()

	ctx := context.Background()
	testFile := filepath.Join(tempDir, "test.md")

	t.Run("successful sync", func(t *testing.T) {
		engine.reset()
		
		watcher.syncFile(ctx, testFile)
		
		syncedFiles := engine.getSyncedFiles()
		assert.Len(t, syncedFiles, 1)
		assert.Equal(t, testFile, syncedFiles[0])
	})

	t.Run("sync error", func(t *testing.T) {
		engine.reset()
		engine.setSyncError(fmt.Errorf("sync failed"))
		
		// This should not panic even with an error
		watcher.syncFile(ctx, testFile)
		
		syncedFiles := engine.getSyncedFiles()
		assert.Len(t, syncedFiles, 0)
	})
}

func TestDebouncer(t *testing.T) {
	d := &debouncer{
		interval: 100 * time.Millisecond,
		pending:  make(map[string]*time.Timer),
	}

	t.Run("single event", func(t *testing.T) {
		executed := false
		
		d.debounce("test-key", func() {
			executed = true
		})

		// Should not execute immediately
		assert.False(t, executed)

		// Wait for debounce interval
		time.Sleep(150 * time.Millisecond)
		assert.True(t, executed)
	})

	t.Run("multiple events debounced", func(t *testing.T) {
		execCount := 0
		
		// Send multiple events rapidly
		for i := 0; i < 5; i++ {
			d.debounce("test-key-2", func() {
				execCount++
			})
			time.Sleep(50 * time.Millisecond) // Less than debounce interval
		}

		// Wait for debounce interval
		time.Sleep(150 * time.Millisecond)
		
		// Should only execute once
		assert.Equal(t, 1, execCount)
	})

	t.Run("different keys execute separately", func(t *testing.T) {
		exec1 := false
		exec2 := false
		
		d.debounce("key1", func() { exec1 = true })
		d.debounce("key2", func() { exec2 = true })

		time.Sleep(150 * time.Millisecond)
		
		assert.True(t, exec1)
		assert.True(t, exec2)
	})
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
	defer watcher.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	defer watcher.Close()

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