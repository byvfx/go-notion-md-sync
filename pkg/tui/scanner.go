package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/charmbracelet/bubbles/list"
)

// FileScanner scans the file system for markdown files
type FileScanner struct {
	config *config.Config
}

// NewFileScanner creates a new file scanner
func NewFileScanner(cfg *config.Config) *FileScanner {
	return &FileScanner{
		config: cfg,
	}
}

// ScanFiles scans the markdown root directory and returns file items
func (fs *FileScanner) ScanFiles() ([]list.Item, error) {
	var items []list.Item

	root := fs.config.Directories.MarkdownRoot
	if root == "" {
		root = "./"
	}

	// Make root absolute
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Walk the directory tree
	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check excluded patterns
		for _, pattern := range fs.config.Directories.ExcludedPatterns {
			matched, err := filepath.Match(pattern, info.Name())
			if err == nil && matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Get relative path for display
		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			relPath = path
		}

		// For directories, only include if they contain markdown files
		if info.IsDir() {
			// Skip root directory itself
			if path == absRoot {
				return nil
			}
			// Add directory items
			item := FileItem{
				Path:        path,
				Name:        relPath,
				Status:      StatusSynced, // Default status
				IsDirectory: true,
				Desc:        formatFileInfo(info),
			}
			items = append(items, item)
		} else if strings.HasSuffix(info.Name(), ".md") {
			// Add markdown files
			item := FileItem{
				Path:        path,
				Name:        relPath,
				Status:      getFileStatus(info),
				Size:        info.Size(),
				IsDirectory: false,
				Desc:        formatFileInfo(info),
			}
			items = append(items, item)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return items, nil
}

// getFileStatus determines the sync status of a file
func getFileStatus(info os.FileInfo) SyncStatus {
	// Check modification time
	modTime := info.ModTime()
	now := time.Now()

	// If modified in the last hour, mark as modified
	if now.Sub(modTime) < time.Hour {
		return StatusModified
	}

	// Otherwise assume synced (we'd need actual sync metadata for accuracy)
	return StatusSynced
}

// formatFileInfo formats file information for display
func formatFileInfo(info os.FileInfo) string {
	modTime := info.ModTime()
	now := time.Now()

	// Format relative time
	duration := now.Sub(modTime)
	switch {
	case duration < time.Minute:
		return "Just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return modTime.Format("Jan 2, 2006")
	}
}
