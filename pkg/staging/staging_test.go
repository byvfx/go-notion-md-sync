package staging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStagingArea_AddFile(t *testing.T) {
	tempDir := t.TempDir()
	
	sa := NewStagingArea(tempDir)
	
	// Create a test file
	testFile := filepath.Join(tempDir, "test.md")
	content := []byte("# Test\n\nThis is a test.")
	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Add file to staging
	err = sa.AddFile(testFile)
	if err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}
	
	// Check if file is staged
	files, err := sa.GetStagedFiles()
	if err != nil {
		t.Fatalf("GetStagedFiles() error = %v", err)
	}
	
	if len(files) != 1 {
		t.Errorf("Expected 1 staged file, got %d", len(files))
	}
	
	if files[0] != testFile {
		t.Errorf("Expected staged file %s, got %s", testFile, files[0])
	}
}

func TestStagingArea_GetStatus(t *testing.T) {
	tempDir := t.TempDir()
	
	sa := NewStagingArea(tempDir)
	
	// Create test files
	file1 := filepath.Join(tempDir, "file1.md")
	file2 := filepath.Join(tempDir, "file2.md")
	
	content1 := []byte("# File 1")
	content2 := []byte("# File 2")
	
	err := os.WriteFile(file1, content1, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	err = os.WriteFile(file2, content2, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Add one file to staging
	err = sa.AddFile(file1)
	if err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}
	
	// Get status
	status, err := sa.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	
	// Check status
	if len(status) != 2 {
		t.Errorf("Expected 2 files in status, got %d", len(status))
	}
	
	if status[file1] != StatusStaged {
		t.Errorf("Expected file1 to be staged, got %v", status[file1])
	}
	
	if status[file2] != StatusModified {
		t.Errorf("Expected file2 to be modified, got %v", status[file2])
	}
}

func TestStagingArea_ResetFile(t *testing.T) {
	tempDir := t.TempDir()
	
	sa := NewStagingArea(tempDir)
	
	// Create a test file
	testFile := filepath.Join(tempDir, "test.md")
	content := []byte("# Test")
	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Add file to staging
	err = sa.AddFile(testFile)
	if err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}
	
	// Reset file
	err = sa.ResetFile(testFile)
	if err != nil {
		t.Fatalf("ResetFile() error = %v", err)
	}
	
	// Check if file is no longer staged
	files, err := sa.GetStagedFiles()
	if err != nil {
		t.Fatalf("GetStagedFiles() error = %v", err)
	}
	
	if len(files) != 0 {
		t.Errorf("Expected 0 staged files after reset, got %d", len(files))
	}
}

func TestStagingArea_MarkSynced(t *testing.T) {
	tempDir := t.TempDir()
	
	sa := NewStagingArea(tempDir)
	
	// Create a test file
	testFile := filepath.Join(tempDir, "test.md")
	content := []byte("# Test")
	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Add file to staging
	err = sa.AddFile(testFile)
	if err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}
	
	// Mark as synced
	err = sa.MarkSynced([]string{testFile})
	if err != nil {
		t.Fatalf("MarkSynced() error = %v", err)
	}
	
	// Check status
	status, err := sa.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	
	if len(status) != 0 {
		t.Errorf("Expected no files in status after sync, got %d", len(status))
	}
}

func TestStagingArea_Persistence(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create first staging area instance
	sa1 := NewStagingArea(tempDir)
	
	// Create a test file
	testFile := filepath.Join(tempDir, "test.md")
	content := []byte("# Test")
	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Add file to staging
	err = sa1.AddFile(testFile)
	if err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}
	
	// Create second staging area instance (should load from file)
	sa2 := NewStagingArea(tempDir)
	
	// Check if file is still staged
	files, err := sa2.GetStagedFiles()
	if err != nil {
		t.Fatalf("GetStagedFiles() error = %v", err)
	}
	
	if len(files) != 1 {
		t.Errorf("Expected 1 staged file after reload, got %d", len(files))
	}
	
	if files[0] != testFile {
		t.Errorf("Expected staged file %s after reload, got %s", testFile, files[0])
	}
}