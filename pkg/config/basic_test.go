package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBasicConfigLoading tests basic config functionality without environment interference
func TestBasicConfigLoading(t *testing.T) {
	// This test will work in CI where no .env file exists
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
notion:
  token: "test_token_basic"
  parent_page_id: "test_page_basic"

sync:
  direction: "push"
  conflict_resolution: "newer"

directories:
  markdown_root: "./docs"

mapping:
  strategy: "frontmatter"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Clear environment to ensure clean test
	originalToken := os.Getenv("NOTION_MD_SYNC_NOTION_TOKEN")
	originalPageID := os.Getenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID")

	_ = os.Unsetenv("NOTION_MD_SYNC_NOTION_TOKEN")
	_ = os.Unsetenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID")

	defer func() {
		if originalToken != "" {
			_ = os.Setenv("NOTION_MD_SYNC_NOTION_TOKEN", originalToken)
		}
		if originalPageID != "" {
			_ = os.Setenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID", originalPageID)
		}
	}()

	// Test loading the config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify basic config loading works
	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}

	if cfg.Sync.Direction != "push" {
		t.Errorf("Expected direction 'push', got '%s'", cfg.Sync.Direction)
	}
}
