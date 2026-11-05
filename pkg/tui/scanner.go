package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/staging"
	"github.com/byvfx/go-notion-md-sync/pkg/util"
	"github.com/charmbracelet/bubbles/list"
)

// FileScanner scans the file system for markdown files
type FileScanner struct {
	config *config.Config
	// stagingRoot is the staging area root used for file status. Empty
	// means the working directory, matching the CLI staging commands.
	stagingRoot string
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

	resolver := fs.newResolver()

	// Walk the directory tree
	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if path != absRoot && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check excluded patterns (relative to the markdown root)
		if path != absRoot && util.IsExcluded(fs.config.Directories.ExcludedPatterns, absRoot, path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
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
				Status:      resolver.status(path),
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

func (fs *FileScanner) newResolver() *statusResolver {
	stagingRoot := fs.stagingRoot
	if stagingRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			return &statusResolver{}
		}
		stagingRoot = wd
	}
	return newStatusResolver(stagingRoot)
}

// statusResolver determines file sync status, preferring the staging area
// and falling back to frontmatter when no staging index is available.
type statusResolver struct {
	area     *staging.StagingArea
	statuses map[string]staging.FileStatus
}

func newStatusResolver(stagingRoot string) *statusResolver {
	indexPath := filepath.Join(stagingRoot, staging.StagingDir, staging.IndexFile)
	if _, err := os.Stat(indexPath); err != nil {
		return &statusResolver{}
	}

	area := staging.NewStagingArea(stagingRoot)
	statuses, err := area.GetStatus()
	if err != nil {
		return &statusResolver{}
	}
	return &statusResolver{area: area, statuses: statuses}
}

// status returns the display status for the markdown file at path.
func (r *statusResolver) status(path string) SyncStatus {
	if r.area != nil {
		key := r.area.NormalizePath(path)
		if !isOutsideRoot(key) {
			st, changed := r.statuses[key]
			if !changed {
				// Tracked and unmodified.
				return StatusSynced
			}
			return stagingToSyncStatus(st)
		}
	}
	return frontmatterStatus(path)
}

// isOutsideRoot reports whether a normalised staging key points outside the
// staging root (or could not be made relative to it).
func isOutsideRoot(key string) bool {
	return key == ".." || strings.HasPrefix(key, "../") || filepath.IsAbs(key)
}

func stagingToSyncStatus(st staging.FileStatus) SyncStatus {
	switch st {
	case staging.StatusModified:
		return StatusModified
	case staging.StatusUnmodified:
		return StatusSynced
	default:
		// New (untracked), staged, or unknown: not yet synced.
		return StatusPending
	}
}

// frontmatterStatus is the fallback without a staging index: a file that
// has a notion_id has been synced at some point; one without has not.
func frontmatterStatus(path string) SyncStatus {
	if hasNotionID(path) {
		return StatusSynced
	}
	return StatusPending
}

// hasNotionID reports whether the file's YAML frontmatter contains a
// non-empty notion_id. Only the frontmatter block is read.
func hasNotionID(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return false
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			return false
		}
		if strings.HasPrefix(line, "notion_id:") {
			value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "notion_id:")), `"'`)
			return value != ""
		}
	}
	return false
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
