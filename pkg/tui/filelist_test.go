package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewFileListModel(t *testing.T) {
	model := NewFileListModel()

	if model.currentPath != "." {
		t.Errorf("Expected initial path to be '.', got %s", model.currentPath)
	}

	if len(model.selectedFiles) != 0 {
		t.Errorf("Expected no selected files initially, got %d", len(model.selectedFiles))
	}
}

func TestFileListFileSelection(t *testing.T) {
	model := NewFileListModel()

	// Test space key for selection
	spaceMsg := tea.KeyMsg{Type: tea.KeySpace}
	_, _ = model.Update(spaceMsg)

	// Note: We can't easily test actual selection without setting up the list properly
	// This test primarily ensures the Update method doesn't panic
}

func TestFileListSelectAll(t *testing.T) {
	model := NewFileListModel()

	// Test 'a' key for select all
	aMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	_, _ = model.Update(aMsg)

	// The actual selection depends on the mock data
	// This test ensures the command runs without error
}

func TestFileListDeselectAll(t *testing.T) {
	model := NewFileListModel()

	// First select some files (mock)
	model.selectedFiles["test.md"] = true

	// Test 'n' key for deselect all
	nMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	model, _ = model.Update(nMsg)

	if len(model.selectedFiles) != 0 {
		t.Errorf("Expected no selected files after deselect all, got %d", len(model.selectedFiles))
	}
}

func TestFileListGetSelectedFiles(t *testing.T) {
	model := NewFileListModel()

	// Add some selected files
	model.selectedFiles["file1.md"] = true
	model.selectedFiles["file2.md"] = true
	model.selectedFiles["file3.md"] = false // This shouldn't be included

	selected := model.GetSelectedFiles()

	if len(selected) != 2 {
		t.Errorf("Expected 2 selected files, got %d", len(selected))
	}

	// Check that the correct files are selected
	found1, found2 := false, false
	for _, file := range selected {
		if file == "file1.md" {
			found1 = true
		}
		if file == "file2.md" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Error("Selected files don't match expected files")
	}
}

func TestFileItemStatusIcon(t *testing.T) {
	tests := []struct {
		status       SyncStatus
		expectedIcon string
	}{
		{StatusSynced, "✅"},
		{StatusPending, "⏳"},
		{StatusModified, "🔄"},
		{StatusError, "❌"},
		{StatusConflict, "⚠️"},
	}

	for _, test := range tests {
		item := FileItem{Status: test.status}
		icon := item.getStatusIcon()

		if icon != test.expectedIcon {
			t.Errorf("Status %v: expected icon %s, got %s", test.status, test.expectedIcon, icon)
		}
	}
}

func TestFileItemTitle(t *testing.T) {
	// Test file
	fileItem := FileItem{
		Name:        "test.md",
		Status:      StatusSynced,
		IsDirectory: false,
	}

	title := fileItem.Title()
	expected := "✅ 📄 test.md"

	if title != expected {
		t.Errorf("Expected title %s, got %s", expected, title)
	}

	// Test directory
	dirItem := FileItem{
		Name:        "docs",
		Status:      StatusSynced,
		IsDirectory: true,
	}

	title = dirItem.Title()
	expected = "✅ 📁 docs"

	if title != expected {
		t.Errorf("Expected title %s, got %s", expected, title)
	}
}
