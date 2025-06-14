package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Clear environment variables to avoid interference
	os.Unsetenv("NOTION_MD_SYNC_NOTION_TOKEN")
	os.Unsetenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID")
	defer func() {
		// Clean up environment after test
		os.Unsetenv("NOTION_MD_SYNC_NOTION_TOKEN")
		os.Unsetenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID")
	}()

	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
notion:
  token: "test_token"
  parent_page_id: "test_page_id"

sync:
  direction: "push"
  conflict_resolution: "newer"

directories:
  markdown_root: "./docs"
  excluded_patterns:
    - "*.tmp"
    - ".git/**"

mapping:
  strategy: "frontmatter"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test loading the config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify the loaded config
	if cfg.Notion.Token != "test_token" {
		t.Errorf("Expected token 'test_token', got '%s'", cfg.Notion.Token)
	}

	if cfg.Notion.ParentPageID != "test_page_id" {
		t.Errorf("Expected parent_page_id 'test_page_id', got '%s'", cfg.Notion.ParentPageID)
	}

	if cfg.Sync.Direction != "push" {
		t.Errorf("Expected direction 'push', got '%s'", cfg.Sync.Direction)
	}

	if cfg.Directories.MarkdownRoot != "./docs" {
		t.Errorf("Expected markdown_root './docs', got '%s'", cfg.Directories.MarkdownRoot)
	}

	if len(cfg.Directories.ExcludedPatterns) != 2 {
		t.Errorf("Expected 2 excluded patterns, got %d", len(cfg.Directories.ExcludedPatterns))
	}

	if cfg.Mapping.Strategy != "filename" {
		t.Errorf("Expected default strategy 'filename', got '%s'", cfg.Mapping.Strategy)
	}
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("NOTION_MD_SYNC_NOTION_TOKEN", "env_token")
	os.Setenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID", "env_page_id")
	defer func() {
		os.Unsetenv("NOTION_MD_SYNC_NOTION_TOKEN")
		os.Unsetenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID")
	}()

	// Create a minimal config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
sync:
  direction: "pull"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test loading the config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Environment variables should override config file values
	if cfg.Notion.Token != "env_token" {
		t.Errorf("Expected token from env 'env_token', got '%s'", cfg.Notion.Token)
	}

	if cfg.Notion.ParentPageID != "env_page_id" {
		t.Errorf("Expected parent_page_id from env 'env_page_id', got '%s'", cfg.Notion.ParentPageID)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("nonexistent_config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent config file, got nil")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	// Create a temporary config file with invalid YAML
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid_config.yaml")

	invalidContent := `
notion:
  token: "test_token"
  invalid_yaml: [unclosed bracket
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoadMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "missing token",
			content: `
notion:
  parent_page_id: "valid_page_id"
`,
			wantErr: true,
		},
		{
			name: "missing parent_page_id",
			content: `
notion:
  token: "valid_token"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "test_config.yaml")

			err := os.WriteFile(configPath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}

			_, err = Load(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	// Create a minimal config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "minimal_config.yaml")

	// Set required environment variables
	os.Setenv("NOTION_MD_SYNC_NOTION_TOKEN", "test_token")
	os.Setenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID", "test_page_id")
	defer func() {
		os.Unsetenv("NOTION_MD_SYNC_NOTION_TOKEN")
		os.Unsetenv("NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID")
	}()

	configContent := `# minimal config`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check defaults
	if cfg.Sync.Direction != "push" {
		t.Errorf("Expected default direction 'push', got '%s'", cfg.Sync.Direction)
	}

	if cfg.Sync.ConflictResolution != "newer" {
		t.Errorf("Expected default conflict_resolution 'newer', got '%s'", cfg.Sync.ConflictResolution)
	}

	if cfg.Directories.MarkdownRoot != "./" {
		t.Errorf("Expected default markdown_root './', got '%s'", cfg.Directories.MarkdownRoot)
	}

	if cfg.Mapping.Strategy != "filename" {
		t.Errorf("Expected default strategy 'filename', got '%s'", cfg.Mapping.Strategy)
	}
}