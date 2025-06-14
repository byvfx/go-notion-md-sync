package markdown

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParser_ParseFile(t *testing.T) {
	parser := NewParser()
	
	tests := []struct {
		name     string
		content  string
		wantErr  bool
		wantMeta map[string]interface{}
	}{
		{
			name: "file with frontmatter",
			content: `---
title: "Test Document"
notion_id: "12345"
sync_enabled: true
---

# Test Document

This is test content.`,
			wantErr: false,
			wantMeta: map[string]interface{}{
				"title":        "Test Document",
				"notion_id":    "12345",
				"sync_enabled": true,
			},
		},
		{
			name: "file without frontmatter",
			content: `# Test Document

This is test content without frontmatter.`,
			wantErr:  false,
			wantMeta: map[string]interface{}{},
		},
		{
			name: "empty file",
			content: "",
			wantErr:  false,
			wantMeta: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, "test.md")
			
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Parse file
			doc, err := parser.ParseFile(filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Check metadata
			for key, expectedValue := range tt.wantMeta {
				if actualValue, exists := doc.Metadata[key]; !exists {
					t.Errorf("Expected metadata key %s not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected metadata %s = %v, got %v", key, expectedValue, actualValue)
				}
			}

			// Check content extraction
			if len(tt.content) > 0 && len(doc.Content) == 0 {
				t.Error("Expected non-empty content")
			}
		})
	}
}

func TestParser_CreateMarkdownWithFrontmatter(t *testing.T) {
	parser := NewParser()
	
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "output.md")
	
	metadata := map[string]interface{}{
		"title":        "Test Document",
		"notion_id":    "12345",
		"sync_enabled": true,
		"created_at":   time.Now(),
	}
	
	content := "# Test Document\n\nThis is test content."
	
	err := parser.CreateMarkdownWithFrontmatter(filePath, metadata, content)
	if err != nil {
		t.Fatalf("CreateMarkdownWithFrontmatter() error = %v", err)
	}
	
	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("Output file was not created")
	}
	
	// Read and parse the created file
	doc, err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse created file: %v", err)
	}
	
	// Verify metadata was preserved
	if doc.Metadata["title"] != "Test Document" {
		t.Errorf("Expected title 'Test Document', got %v", doc.Metadata["title"])
	}
	
	if doc.Metadata["notion_id"] != "12345" {
		t.Errorf("Expected notion_id '12345', got %v", doc.Metadata["notion_id"])
	}
	
	if doc.Metadata["sync_enabled"] != true {
		t.Errorf("Expected sync_enabled true, got %v", doc.Metadata["sync_enabled"])
	}
	
	// Verify content was preserved
	if doc.Content != content {
		t.Errorf("Content mismatch. Expected %q, got %q", content, doc.Content)
	}
}

func TestExtractFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		wantErr  bool
	}{
		{
			name: "complete frontmatter",
			metadata: map[string]interface{}{
				"title":        "Test Document",
				"notion_id":    "12345",
				"sync_enabled": true,
				"created_at":   "2025-01-01T00:00:00Z",
				"updated_at":   "2025-01-01T12:00:00Z",
			},
			wantErr: false,
		},
		{
			name: "minimal frontmatter",
			metadata: map[string]interface{}{
				"sync_enabled": true,
			},
			wantErr: false,
		},
		{
			name:     "empty frontmatter",
			metadata: map[string]interface{}{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frontmatter, err := ExtractFrontmatter(tt.metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// Verify basic structure
			if frontmatter == nil {
				t.Error("Expected non-nil frontmatter")
				return
			}

			// Check that values can be converted back
			backToMetadata := frontmatter.ToMetadata()
			
			// Verify key preservation (some type conversion is expected)
			for key := range tt.metadata {
				if _, exists := backToMetadata[key]; !exists && tt.metadata[key] != nil {
					t.Errorf("Key %s was lost during frontmatter conversion", key)
				}
			}
		})
	}
}

func TestFrontmatterFields_ToMetadata(t *testing.T) {
	now := time.Now()
	
	ff := &FrontmatterFields{
		Title:       "Test Document",
		NotionID:    "12345",
		CreatedAt:   &now,
		UpdatedAt:   &now,
		SyncEnabled: true,
		Tags:        []string{"tag1", "tag2"},
		Status:      "published",
	}
	
	metadata := ff.ToMetadata()
	
	if metadata["title"] != "Test Document" {
		t.Errorf("Expected title 'Test Document', got %v", metadata["title"])
	}
	
	if metadata["notion_id"] != "12345" {
		t.Errorf("Expected notion_id '12345', got %v", metadata["notion_id"])
	}
	
	if metadata["sync_enabled"] != true {
		t.Errorf("Expected sync_enabled true, got %v", metadata["sync_enabled"])
	}
	
	if metadata["status"] != "published" {
		t.Errorf("Expected status 'published', got %v", metadata["status"])
	}
	
	tags, ok := metadata["tags"].([]string)
	if !ok {
		t.Errorf("Expected tags to be []string, got %T", metadata["tags"])
	} else if len(tags) != 2 || tags[0] != "tag1" || tags[1] != "tag2" {
		t.Errorf("Expected tags [tag1, tag2], got %v", tags)
	}
}