package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindMarkdownFiles(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"file1.md":               "# File 1",
		"file2.MD":               "# File 2",
		"file3.markdown":         "# File 3",
		"file4.MARKDOWN":         "# File 4",
		"subdir/file5.md":        "# File 5",
		"subdir/nested/file6.md": "# File 6",
		"notmd.txt":              "Not markdown",
		"README":                 "No extension",
	}

	// Create subdirectories
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "subdir", "nested"), 0755))

	// Create files
	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	// Find markdown files
	found, err := findMarkdownFiles(tempDir)
	require.NoError(t, err)

	// Should find all .md and .markdown files (case insensitive)
	assert.Len(t, found, 6)

	// Verify all markdown files are found
	expectedFiles := []string{
		"file1.md",
		"file2.MD",
		"file3.markdown",
		"file4.MARKDOWN",
		filepath.Join("subdir", "file5.md"),
		filepath.Join("subdir", "nested", "file6.md"),
	}

	for _, expected := range expectedFiles {
		fullPath := filepath.Join(tempDir, expected)
		assert.Contains(t, found, fullPath)
	}

	// Verify non-markdown files are not found
	assert.NotContains(t, found, filepath.Join(tempDir, "notmd.txt"))
	assert.NotContains(t, found, filepath.Join(tempDir, "README"))
}

func TestFindMarkdownFiles_Errors(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		wantErr bool
	}{
		{
			name:    "non-existent directory",
			dir:     "/non/existent/path",
			wantErr: true,
		},
		{
			name:    "empty directory",
			dir:     t.TempDir(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := findMarkdownFiles(tt.dir)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.name == "empty directory" {
					assert.Empty(t, files)
				}
			}
		})
	}
}

func TestPerformDirectorySync(t *testing.T) {
	tempDir := t.TempDir()

	// Create test markdown files
	testFiles := []string{
		filepath.Join(tempDir, "file1.md"),
		filepath.Join(tempDir, "file2.md"),
	}

	for _, file := range testFiles {
		content := `---
notion_id: test-id
---
# Test File`
		require.NoError(t, os.WriteFile(file, []byte(content), 0644))
	}

	tests := []struct {
		name        string
		direction   string
		engineSetup func(*mockSyncEngine)
		checkOutput func(t *testing.T, output string)
		expectError bool
	}{
		{
			name:      "successful push",
			direction: "push",
			engineSetup: func(m *mockSyncEngine) {
				m.syncFileToNotionFunc = func(ctx context.Context, filePath string) error {
					return nil
				}
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Found 2 markdown files")
				assert.Contains(t, output, "✅ Pushed")
				assert.Contains(t, output, "2 succeeded, 0 failed, 0 skipped")
			},
			expectError: false,
		},
		{
			name:      "push with failures",
			direction: "push",
			engineSetup: func(m *mockSyncEngine) {
				m.syncFileToNotionFunc = func(ctx context.Context, filePath string) error {
					if strings.Contains(filePath, "file1") {
						return errors.New("sync failed")
					}
					return nil
				}
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "❌ Failed to push")
				assert.Contains(t, output, "✅ Pushed")
				assert.Contains(t, output, "1 succeeded, 1 failed, 0 skipped")
			},
			expectError: false,
		},
		{
			name:      "pull with notion_id",
			direction: "pull",
			engineSetup: func(m *mockSyncEngine) {
				// syncSingleFileHelper will be called instead
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Found 2 markdown files")
				// Files will be skipped because mock engine doesn't implement full flow
			},
			expectError: false,
		},
		{
			name:      "bidirectional sync",
			direction: "bidirectional",
			engineSetup: func(m *mockSyncEngine) {
				m.syncFileToNotionFunc = func(ctx context.Context, filePath string) error {
					return nil
				}
			},
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "✅ Pushed")
				assert.Contains(t, output, "2 succeeded, 0 failed, 0 skipped")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create mock engine
			engine := &mockSyncEngine{}
			if tt.engineSetup != nil {
				tt.engineSetup(engine)
			}

			// Run function
			ctx := context.Background()
			err := performDirectorySync(ctx, engine, tempDir, tt.direction)

			// Restore stdout and read output
			w.Close()
			os.Stdout = oldStdout

			buf := make([]byte, 4096)
			n, _ := r.Read(buf)
			output := string(buf[:n])

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, output)
			}
		})
	}
}

func TestSyncSingleFileHelper(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file with frontmatter
	testFile := filepath.Join(tempDir, "test.md")
	content := `---
title: Test Page
notion_id: test-notion-id
---
# Test Content`
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	// Create test file without notion_id
	testFileNoID := filepath.Join(tempDir, "test-no-id.md")
	contentNoID := `---
title: Test Page
---
# Test Content`
	require.NoError(t, os.WriteFile(testFileNoID, []byte(contentNoID), 0644))

	tests := []struct {
		name        string
		filePath    string
		direction   string
		engineSetup func(*mockSyncEngine)
		wantErr     bool
		errContains string
	}{
		{
			name:      "push file",
			filePath:  testFile,
			direction: "push",
			engineSetup: func(m *mockSyncEngine) {
				m.syncFileToNotionFunc = func(ctx context.Context, filePath string) error {
					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "push file error",
			filePath:  testFile,
			direction: "push",
			engineSetup: func(m *mockSyncEngine) {
				m.syncFileToNotionFunc = func(ctx context.Context, filePath string) error {
					return errors.New("push failed")
				}
			},
			wantErr:     true,
			errContains: "push failed",
		},
		{
			name:      "pull file with notion_id",
			filePath:  testFile,
			direction: "pull",
			engineSetup: func(m *mockSyncEngine) {
				m.syncNotionToFileFunc = func(ctx context.Context, pageID, filePath string) error {
					assert.Equal(t, "test-notion-id", pageID)
					return nil
				}
			},
			wantErr: false,
		},
		{
			name:        "pull file without notion_id",
			filePath:    testFileNoID,
			direction:   "pull",
			engineSetup: func(m *mockSyncEngine) {},
			wantErr:     true,
			errContains: "no notion_id found in frontmatter",
		},
		{
			name:        "bidirectional not supported",
			filePath:    testFile,
			direction:   "bidirectional",
			engineSetup: func(m *mockSyncEngine) {},
			wantErr:     true,
			errContains: "bidirectional sync not supported for single file",
		},
		{
			name:        "invalid direction",
			filePath:    testFile,
			direction:   "invalid",
			engineSetup: func(m *mockSyncEngine) {},
			wantErr:     true,
			errContains: "invalid direction",
		},
		{
			name:        "non-existent file",
			filePath:    filepath.Join(tempDir, "non-existent.md"),
			direction:   "pull",
			engineSetup: func(m *mockSyncEngine) {},
			wantErr:     true,
			errContains: "failed to parse file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &mockSyncEngine{}
			if tt.engineSetup != nil {
				tt.engineSetup(engine)
			}

			ctx := context.Background()
			err := syncSingleFileHelper(ctx, engine, tt.filePath, tt.direction)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
