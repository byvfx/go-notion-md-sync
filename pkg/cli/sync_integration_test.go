// +build integration

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSyncCommand_Integration tests the sync command with real API calls
// This test requires NOTION_API_TOKEN and NOTION_PARENT_PAGE_ID env vars
func TestSyncCommand_Integration(t *testing.T) {
	token := os.Getenv("NOTION_API_TOKEN")
	parentID := os.Getenv("NOTION_PARENT_PAGE_ID")
	
	if token == "" || parentID == "" {
		t.Skip("Skipping integration test: NOTION_API_TOKEN or NOTION_PARENT_PAGE_ID not set")
	}

	// Create temporary directory for test files
	tempDir := t.TempDir()

	// Create test config file with real credentials
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
notion:
  token: ` + token + `
  parent_page_id: ` + parentID + `
sync:
  direction: push
directories:
  markdown_root: ` + tempDir + `
`
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	// Create test markdown files
	testMdFile := filepath.Join(tempDir, "test.md")
	require.NoError(t, os.WriteFile(testMdFile, []byte("# Test\nContent"), 0644))

	// Test push
	cmd := &cobra.Command{
		Use:  "sync",
		RunE: runSync,
	}
	
	cmd.Flags().StringVarP(&syncFile, "file", "f", "", "specific file to sync")
	cmd.Flags().StringVarP(&syncDirection, "direction", "d", "push", "sync direction")
	cmd.Flags().StringVar(&syncDirectory, "directory", "", "directory containing markdown files")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be synced")
	cmd.Flags().StringVarP(&configPath, "config", "c", configFile, "config file path")

	// Reset flags
	syncFile = ""
	syncDirection = "push"
	syncDirectory = ""
	dryRun = false
	configPath = configFile

	cmd.SetArgs([]string{"push"})
	
	err := cmd.Execute()
	require.NoError(t, err)
}