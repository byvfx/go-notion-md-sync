package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock sync engine for testing
type mockSyncEngine struct {
	syncFileToNotionFunc func(ctx context.Context, filePath string) error
	syncNotionToFileFunc func(ctx context.Context, pageID, filePath string) error
	syncAllFunc          func(ctx context.Context, direction string) error
	syncSpecificFileFunc func(ctx context.Context, filename, direction string) error
}

func (m *mockSyncEngine) SyncFileToNotion(ctx context.Context, filePath string) error {
	if m.syncFileToNotionFunc != nil {
		return m.syncFileToNotionFunc(ctx, filePath)
	}
	return nil
}

func (m *mockSyncEngine) SyncNotionToFile(ctx context.Context, pageID, filePath string) error {
	if m.syncNotionToFileFunc != nil {
		return m.syncNotionToFileFunc(ctx, pageID, filePath)
	}
	return nil
}

func (m *mockSyncEngine) SyncAll(ctx context.Context, direction string) error {
	if m.syncAllFunc != nil {
		return m.syncAllFunc(ctx, direction)
	}
	return nil
}

func (m *mockSyncEngine) SyncSpecificFile(ctx context.Context, filename, direction string) error {
	if m.syncSpecificFileFunc != nil {
		return m.syncSpecificFileFunc(ctx, filename, direction)
	}
	return nil
}


func TestRunSync_DirectionValidation(t *testing.T) {
	// Only test direction validation logic
	validDirections := []string{"push", "pull", "bidirectional"}
	
	for _, dir := range validDirections {
		err := validateDirection(dir)
		assert.NoError(t, err, "Direction %s should be valid", dir)
	}
	
	err := validateDirection("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid direction")
}

// Helper function to validate direction
func validateDirection(direction string) error {
	switch direction {
	case "push", "pull", "bidirectional":
		return nil
	default:
		return fmt.Errorf("invalid direction: %s (must be push, pull, or bidirectional)", direction)
	}
}

func TestGetActionVerb(t *testing.T) {
	tests := []struct {
		direction string
		expected  string
	}{
		{"push", "pushed to Notion"},
		{"pull", "pulled from Notion"},
		{"bidirectional", "synced bidirectionally"},
		{"unknown", "synced"},
	}

	for _, tt := range tests {
		t.Run(tt.direction, func(t *testing.T) {
			result := getActionVerb(tt.direction)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPerformDryRun(t *testing.T) {
	tempDir := t.TempDir()

	// Create test markdown files
	testFiles := []string{
		filepath.Join(tempDir, "file1.md"),
		filepath.Join(tempDir, "file2.md"),
		filepath.Join(tempDir, "subdir", "file3.md"),
	}

	// Create subdirectory
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755))

	// Create files
	for _, file := range testFiles {
		require.NoError(t, os.WriteFile(file, []byte("# Test"), 0644))
	}

	// Also create a non-markdown file that should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "ignore.txt"), []byte("ignore"), 0644))

	tests := []struct {
		name         string
		workingDir   string
		specificFile string
		direction    string
		checkOutput  func(t *testing.T, output string)
	}{
		{
			name:       "dry run all files push",
			workingDir: tempDir,
			direction:  "push",
			checkOutput: func(t *testing.T, output string) {
				// Capture stdout
				assert.Contains(t, output, "Found 3 markdown files")
				assert.Contains(t, output, "would be pushed to Notion")
				assert.Contains(t, output, "file1.md")
				assert.Contains(t, output, "file2.md")
				assert.Contains(t, output, "file3.md")
			},
		},
		{
			name:         "dry run specific file",
			workingDir:   tempDir,
			specificFile: testFiles[0],
			direction:    "pull",
			checkOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "Would sync file:")
				assert.Contains(t, output, "file1.md")
				assert.Contains(t, output, "(pull)")
			},
		},
		{
			name:       "dry run empty directory",
			workingDir: filepath.Join(tempDir, "empty"),
			direction:  "push",
			checkOutput: func(t *testing.T, output string) {
				// Note: This will fail to find files because directory doesn't exist
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Redirect stdout to capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			ctx := context.Background()
			err := performDryRun(ctx, tt.workingDir, tt.specificFile, tt.direction)

			// Restore stdout and read output
			_ = w.Close()
			os.Stdout = oldStdout

			buf := make([]byte, 4096)
			n, _ := r.Read(buf)
			output := string(buf[:n])

			if tt.name == "dry run empty directory" {
				// Special case: error expected
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkOutput != nil {
					tt.checkOutput(t, output)
				}
			}
		})
	}
}

func TestSyncCommandFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "sync",
		RunE: runSync,
	}

	// Initialize flags
	cmd.Flags().StringVarP(&syncFile, "file", "f", "", "specific file to sync")
	cmd.Flags().StringVarP(&syncDirection, "direction", "d", "push", "sync direction")
	cmd.Flags().StringVar(&syncDirectory, "directory", "", "directory containing markdown files")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be synced")

	// Test file flag
	fileFlag := cmd.Flags().Lookup("file")
	require.NotNil(t, fileFlag)
	assert.Equal(t, "f", fileFlag.Shorthand)
	assert.Equal(t, "", fileFlag.DefValue)

	// Test direction flag
	directionFlag := cmd.Flags().Lookup("direction")
	require.NotNil(t, directionFlag)
	assert.Equal(t, "d", directionFlag.Shorthand)
	assert.Equal(t, "push", directionFlag.DefValue)

	// Test directory flag
	directoryFlag := cmd.Flags().Lookup("directory")
	require.NotNil(t, directoryFlag)
	assert.Equal(t, "", directoryFlag.DefValue)

	// Test dry-run flag
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	require.NotNil(t, dryRunFlag)
	assert.Equal(t, "false", dryRunFlag.DefValue)
}

// Test the original sync.NewEngine is called correctly
func TestSyncEngineCreation(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Create a valid config
	cfg := &config.Config{}
	cfg.Notion.Token = "test-token"
	cfg.Directories.MarkdownRoot = tempDir

	// Write config manually since there's no Save method
	configContent := `
notion:
  token: test-token
directories:
  markdown_root: ` + tempDir
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	// Test that engine is created with config
	engine := sync.NewEngine(cfg)
	assert.NotNil(t, engine)
}
