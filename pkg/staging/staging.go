package staging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	StagingDir   = ".notion-sync"
	IndexFile    = "index"
	LastSyncFile = "last-sync"
	HashesDir    = "hashes"
)

// FileStatus represents the status of a file
type FileStatus int

const (
	StatusUnknown FileStatus = iota
	StatusUnmodified
	StatusModified
	StatusNew
	StatusDeleted
	StatusStaged
)

func (s FileStatus) String() string {
	switch s {
	case StatusUnmodified:
		return "unmodified"
	case StatusModified:
		return "modified"
	case StatusNew:
		return "new file"
	case StatusDeleted:
		return "deleted"
	case StatusStaged:
		return "staged"
	default:
		return "unknown"
	}
}

// FileEntry represents a tracked file
type FileEntry struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	LastModified time.Time `json:"last_modified"`
	LastSynced   time.Time `json:"last_synced"`
	Staged       bool      `json:"staged"`
}

// StagingArea manages the staging system
type StagingArea struct {
	rootDir string
}

// NewStagingArea creates a new staging area
func NewStagingArea(rootDir string) *StagingArea {
	return &StagingArea{
		rootDir: rootDir,
	}
}

// Initialize creates the staging directory structure
func (s *StagingArea) Initialize() error {
	stagingPath := filepath.Join(s.rootDir, StagingDir)

	// Create main staging directory
	if err := os.MkdirAll(stagingPath, 0755); err != nil {
		return fmt.Errorf("failed to create staging directory: %w", err)
	}

	// Create hashes directory
	hashesPath := filepath.Join(stagingPath, HashesDir)
	if err := os.MkdirAll(hashesPath, 0755); err != nil {
		return fmt.Errorf("failed to create hashes directory: %w", err)
	}

	// Create empty index if it doesn't exist
	indexPath := filepath.Join(stagingPath, IndexFile)
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		if err := s.saveIndex(make(map[string]FileEntry)); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// GetStatus returns the status of all tracked files
func (s *StagingArea) GetStatus() (map[string]FileStatus, error) {
	index, err := s.loadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	filesToCheck, err := s.collectMarkdownFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	status := s.processFiles(filesToCheck, index)
	s.checkDeletedFiles(status, index)

	return status, nil
}

type fileInfo struct {
	path string
	info os.FileInfo
}

func (s *StagingArea) collectMarkdownFiles() ([]fileInfo, error) {
	var filesToCheck []fileInfo

	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if s.shouldSkipFile(path, info) {
			return nil
		}

		filesToCheck = append(filesToCheck, fileInfo{path: path, info: info})
		return nil
	})

	return filesToCheck, err
}

func (s *StagingArea) shouldSkipFile(path string, info os.FileInfo) bool {
	// Skip directories
	if info.IsDir() {
		return true
	}

	// Skip non-markdown files
	if filepath.Ext(path) != ".md" {
		return true
	}

	// Skip staging directory
	return s.isInStagingDir(path)
}

func (s *StagingArea) processFiles(filesToCheck []fileInfo, index map[string]FileEntry) map[string]FileStatus {
	status := make(map[string]FileStatus)
	statusMutex := sync.Mutex{}

	maxWorkers := getMaxWorkers(len(filesToCheck))
	jobs := make(chan fileInfo, len(filesToCheck))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go s.statusWorker(&wg, jobs, index, status, &statusMutex)
	}

	// Send jobs
	for _, file := range filesToCheck {
		jobs <- file
	}
	close(jobs)

	wg.Wait()
	return status
}

func getMaxWorkers(fileCount int) int {
	const defaultMaxWorkers = 5
	if fileCount < defaultMaxWorkers {
		return fileCount
	}
	return defaultMaxWorkers
}

func (s *StagingArea) statusWorker(wg *sync.WaitGroup, jobs <-chan fileInfo, index map[string]FileEntry, status map[string]FileStatus, statusMutex *sync.Mutex) {
	defer wg.Done()

	for job := range jobs {
		relPath, err := filepath.Rel(s.rootDir, job.path)
		if err != nil {
			continue
		}

		fileStatus, err := s.getFileStatus(relPath, job.info, index)
		if err != nil {
			continue
		}

		if fileStatus != StatusUnmodified {
			statusMutex.Lock()
			status[relPath] = fileStatus
			statusMutex.Unlock()
		}
	}
}

func (s *StagingArea) checkDeletedFiles(status map[string]FileStatus, index map[string]FileEntry) {
	for path, entry := range index {
		fullPath := filepath.Join(s.rootDir, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			status[path] = StatusDeleted
		} else if entry.Staged {
			// If file exists and is staged, mark as staged
			if _, exists := status[path]; !exists {
				status[path] = StatusStaged
			}
		}
	}
}

// AddFile stages a file for sync
func (s *StagingArea) AddFile(filePath string) error {
	if err := s.Initialize(); err != nil {
		return err
	}

	index, err := s.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	fullPath := filepath.Join(s.rootDir, filePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	hash, err := s.calculateFileHash(fullPath)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	entry := FileEntry{
		Path:         filePath,
		Hash:         hash,
		LastModified: info.ModTime(),
		Staged:       true,
	}

	// Update last synced time if this is an update to an existing entry
	if existing, exists := index[filePath]; exists {
		entry.LastSynced = existing.LastSynced
	}

	index[filePath] = entry

	return s.saveIndex(index)
}

// ResetFile unstages a file
func (s *StagingArea) ResetFile(filePath string) error {
	index, err := s.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	if entry, exists := index[filePath]; exists {
		entry.Staged = false
		index[filePath] = entry
		return s.saveIndex(index)
	}

	return nil
}

// GetStagedFiles returns all staged files
func (s *StagingArea) GetStagedFiles() ([]string, error) {
	index, err := s.loadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	var staged []string
	for path, entry := range index {
		if entry.Staged {
			staged = append(staged, path)
		}
	}

	return staged, nil
}

// MarkSynced marks files as synced and unstages them
func (s *StagingArea) MarkSynced(filePaths []string) error {
	index, err := s.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	now := time.Now()
	for _, path := range filePaths {
		if entry, exists := index[path]; exists {
			entry.LastSynced = now
			entry.Staged = false
			index[path] = entry
		}
	}

	return s.saveIndex(index)
}

// Helper methods

func (s *StagingArea) getFileStatus(relPath string, info os.FileInfo, index map[string]FileEntry) (FileStatus, error) {
	entry, exists := index[relPath]

	if !exists {
		// File not in index - check if it's truly new or just not tracked yet
		isActuallyNew, err := s.isFileActuallyNew(relPath, info)
		if err != nil {
			return StatusUnknown, err
		}

		if isActuallyNew {
			return StatusNew, nil
		} else {
			// File exists but not tracked, treat as unmodified unless it has recent changes
			return StatusUnmodified, nil
		}
	}

	if entry.Staged {
		return StatusStaged, nil
	}

	// Quick timestamp check first
	if !info.ModTime().After(entry.LastModified) {
		return StatusUnmodified, nil
	}

	// File timestamp suggests change, verify with hash
	fullPath := filepath.Join(s.rootDir, relPath)
	currentHash, err := s.calculateFileHash(fullPath)
	if err != nil {
		return StatusUnknown, fmt.Errorf("failed to calculate hash: %w", err)
	}

	if currentHash != entry.Hash {
		return StatusModified, nil
	}

	// Hash matches, update timestamp in index to avoid future hash calculations
	entry.LastModified = info.ModTime()
	index[relPath] = entry
	_ = s.saveIndex(index) // Async update, ignore errors

	return StatusUnmodified, nil
}

func (s *StagingArea) isFileActuallyNew(relPath string, info os.FileInfo) (bool, error) {
	fullPath := filepath.Join(s.rootDir, relPath)

	// Read the file to check for frontmatter indicating it's been synced before
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return false, err
	}

	// Check if file has frontmatter with notion_id (indicating it's been synced before)
	contentStr := string(content)
	if strings.HasPrefix(contentStr, "---\n") {
		// Look for notion_id in frontmatter
		if strings.Contains(contentStr, "notion_id:") || strings.Contains(contentStr, "notion_page_id:") {
			// File has been synced before, check if it's been modified recently
			// Consider it modified if changed in last 24 hours, otherwise unmodified
			if time.Since(info.ModTime()) < 24*time.Hour {
				return true, nil // Treat as new/modified
			}
			return false, nil // Old file, treat as unmodified
		}
	}

	// No frontmatter with notion_id, check if it's a recently created file
	if time.Since(info.ModTime()) < 1*time.Hour {
		return true, nil // Recently created file
	}

	return false, nil // Old file without notion_id, probably doesn't need syncing
}

func (s *StagingArea) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *StagingArea) isInStagingDir(path string) bool {
	stagingPath := filepath.Join(s.rootDir, StagingDir)
	rel, err := filepath.Rel(stagingPath, path)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "..")
}

func (s *StagingArea) loadIndex() (map[string]FileEntry, error) {
	indexPath := filepath.Join(s.rootDir, StagingDir, IndexFile)

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]FileEntry), nil
		}
		return nil, err
	}

	var index map[string]FileEntry
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	return index, nil
}

func (s *StagingArea) saveIndex(index map[string]FileEntry) error {
	indexPath := filepath.Join(s.rootDir, StagingDir, IndexFile)

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(indexPath, data, 0644)
}
