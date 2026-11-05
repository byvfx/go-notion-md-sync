package staging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

// StagingArea manages the staging system.
//
// Index keys are paths relative to the staging root, cleaned and
// slash-separated. Every public method accepts either such a relative path
// or an absolute path inside the root and normalises it the same way.
// A StagingArea is safe for concurrent use within one process.
type StagingArea struct {
	rootDir string

	// mu serialises every load-modify-save cycle of the index.
	mu sync.Mutex
}

// NewStagingArea creates a new staging area
func NewStagingArea(rootDir string) *StagingArea {
	return &StagingArea{
		rootDir: rootDir,
	}
}

// Initialize creates the staging directory structure
func (s *StagingArea) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initializeLocked()
}

func (s *StagingArea) initializeLocked() error {
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

// NormalizePath returns the index key for filePath: relative to the staging
// root (when filePath is absolute), cleaned and slash-separated.
func (s *StagingArea) NormalizePath(filePath string) string {
	if filepath.IsAbs(filePath) {
		if rel, ok := s.relToRoot(filePath); ok {
			filePath = rel
		}
	}
	return filepath.ToSlash(filepath.Clean(filePath))
}

// relToRoot makes an absolute path relative to the staging root, resolving
// symlinks when the plain comparison places it outside the root.
func (s *StagingArea) relToRoot(absPath string) (string, bool) {
	root, err := filepath.Abs(s.rootDir)
	if err != nil {
		return "", false
	}

	rel, err := filepath.Rel(root, absPath)
	if err == nil && !isOutside(rel) {
		return rel, true
	}

	realRoot, rootErr := filepath.EvalSymlinks(root)
	realPath, pathErr := filepath.EvalSymlinks(absPath)
	if rootErr == nil && pathErr == nil {
		if r, err := filepath.Rel(realRoot, realPath); err == nil && !isOutside(r) {
			return r, true
		}
	}

	if err != nil {
		return "", false
	}
	return rel, true
}

func isOutside(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// fullPath returns the on-disk path for an index key.
func (s *StagingArea) fullPath(key string) string {
	return filepath.Join(s.rootDir, filepath.FromSlash(key))
}

// GetStatus returns the status of every markdown file that is not
// unmodified: staged, modified, new (untracked) or deleted. Keys are
// normalised index paths.
func (s *StagingArea) GetStatus() (map[string]FileStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	filesToCheck, err := s.collectMarkdownFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	status, refreshed := s.processFiles(filesToCheck, index)
	s.checkDeletedFiles(status, index)

	// Persist refreshed timestamps once, after all workers have finished.
	// This only refreshes cached metadata, so a failure (e.g. a read-only
	// checkout) must not prevent reporting the status.
	// The index is re-read first so a concurrent writer (another process,
	// such as the CLI while the TUI is open) is not overwritten with stale
	// entries; only unchanged entries get the new timestamp.
	if len(refreshed) > 0 {
		if latest, err := s.loadIndex(); err == nil {
			changed := false
			for key, entry := range refreshed {
				cur, ok := latest[key]
				if !ok || cur.Hash != entry.Hash || cur.Staged != entry.Staged {
					continue
				}
				cur.LastModified = entry.LastModified
				latest[key] = cur
				changed = true
			}
			if changed {
				_ = s.saveIndex(latest)
			}
		}
	}

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

		if info.IsDir() && s.isInStagingDir(path) {
			return filepath.SkipDir
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

// processFiles computes the status of each file concurrently. The index is
// only read by the workers; entries whose timestamps should be refreshed
// are returned for the caller to persist.
func (s *StagingArea) processFiles(filesToCheck []fileInfo, index map[string]FileEntry) (map[string]FileStatus, map[string]FileEntry) {
	status := make(map[string]FileStatus)
	refreshed := make(map[string]FileEntry)
	var resultMu sync.Mutex

	maxWorkers := getMaxWorkers(len(filesToCheck))
	jobs := make(chan fileInfo, len(filesToCheck))
	var wg sync.WaitGroup

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				relPath, err := filepath.Rel(s.rootDir, job.path)
				if err != nil {
					continue
				}
				key := filepath.ToSlash(filepath.Clean(relPath))

				result, err := s.getFileStatus(key, job.info, index)
				if err != nil {
					continue
				}

				resultMu.Lock()
				if result.status != StatusUnmodified {
					status[key] = result.status
				}
				if result.refreshed != nil {
					refreshed[key] = *result.refreshed
				}
				resultMu.Unlock()
			}
		}()
	}

	for _, file := range filesToCheck {
		jobs <- file
	}
	close(jobs)

	wg.Wait()
	return status, refreshed
}

func getMaxWorkers(fileCount int) int {
	const defaultMaxWorkers = 5
	if fileCount < defaultMaxWorkers {
		return fileCount
	}
	return defaultMaxWorkers
}

func (s *StagingArea) checkDeletedFiles(status map[string]FileStatus, index map[string]FileEntry) {
	for key, entry := range index {
		if _, err := os.Stat(s.fullPath(key)); os.IsNotExist(err) {
			status[key] = StatusDeleted
		} else if entry.Staged {
			// If file exists and is staged, mark as staged
			if _, exists := status[key]; !exists {
				status[key] = StatusStaged
			}
		}
	}
}

// AddFile stages a file for sync. filePath may be relative to the staging
// root or absolute.
func (s *StagingArea) AddFile(filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.initializeLocked(); err != nil {
		return err
	}

	index, err := s.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	key := s.NormalizePath(filePath)
	fullPath := s.fullPath(key)
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	hash, err := s.calculateFileHash(fullPath)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	entry := FileEntry{
		Path:         key,
		Hash:         hash,
		LastModified: info.ModTime(),
		Staged:       true,
	}

	// Update last synced time if this is an update to an existing entry
	if existing, exists := index[key]; exists {
		entry.LastSynced = existing.LastSynced
	}

	index[key] = entry

	return s.saveIndex(index)
}

// ResetFile unstages a file
func (s *StagingArea) ResetFile(filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	key := s.NormalizePath(filePath)
	if entry, exists := index[key]; exists {
		entry.Staged = false
		index[key] = entry
		return s.saveIndex(index)
	}

	return nil
}

// GetStagedFiles returns all staged files as normalised index paths, sorted.
func (s *StagingArea) GetStagedFiles() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	var staged []string
	for key, entry := range index {
		if entry.Staged {
			staged = append(staged, key)
		}
	}
	sort.Strings(staged)

	return staged, nil
}

// MarkSynced marks files as synced and unstages them. The stored hash and
// modification time are refreshed from disk, because a push may rewrite the
// file (e.g. adding notion_id to the frontmatter).
func (s *StagingArea) MarkSynced(filePaths []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	now := time.Now()
	for _, filePath := range filePaths {
		key := s.NormalizePath(filePath)
		entry, exists := index[key]
		if !exists {
			entry = FileEntry{Path: key}
		}

		fullPath := s.fullPath(key)
		info, statErr := os.Stat(fullPath)
		if statErr != nil {
			if !exists {
				// Nothing on disk and nothing tracked: nothing to record.
				continue
			}
		} else {
			hash, err := s.calculateFileHash(fullPath)
			if err != nil {
				return fmt.Errorf("failed to calculate hash for %s: %w", key, err)
			}
			entry.Hash = hash
			entry.LastModified = info.ModTime()
		}

		entry.Path = key
		entry.LastSynced = now
		entry.Staged = false
		index[key] = entry
	}

	return s.saveIndex(index)
}

// Helper methods

type statusResult struct {
	status FileStatus
	// refreshed is set when the entry's timestamp should be updated in the
	// index (content unchanged but mtime moved).
	refreshed *FileEntry
}

// getFileStatus determines the status of one file. It never mutates index.
// Untracked files are always new; tracked files are compared by content
// hash.
func (s *StagingArea) getFileStatus(key string, info os.FileInfo, index map[string]FileEntry) (statusResult, error) {
	entry, exists := index[key]
	if !exists {
		return statusResult{status: StatusNew}, nil
	}

	if entry.Staged {
		return statusResult{status: StatusStaged}, nil
	}

	currentHash, err := s.calculateFileHash(s.fullPath(key))
	if err != nil {
		return statusResult{status: StatusUnknown}, fmt.Errorf("failed to calculate hash: %w", err)
	}

	if currentHash != entry.Hash {
		return statusResult{status: StatusModified}, nil
	}

	if !info.ModTime().Equal(entry.LastModified) {
		entry.LastModified = info.ModTime()
		return statusResult{status: StatusUnmodified, refreshed: &entry}, nil
	}

	return statusResult{status: StatusUnmodified}, nil
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

// loadIndex reads the index, normalising keys written by older versions
// (e.g. "./docs/a.md" and "docs/a.md") and merging any duplicates.
func (s *StagingArea) loadIndex() (map[string]FileEntry, error) {
	indexPath := filepath.Join(s.rootDir, StagingDir, IndexFile)

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]FileEntry), nil
		}
		return nil, err
	}

	var raw map[string]FileEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	index := make(map[string]FileEntry, len(raw))
	for rawKey, entry := range raw {
		key := s.NormalizePath(rawKey)
		entry.Path = key
		if existing, dup := index[key]; dup {
			entry = mergeEntries(existing, entry)
		}
		index[key] = entry
	}

	return index, nil
}

// mergeEntries combines two index entries for the same file.
func mergeEntries(a, b FileEntry) FileEntry {
	merged := a
	if b.LastModified.After(a.LastModified) {
		merged.Hash = b.Hash
		merged.LastModified = b.LastModified
	}
	if b.LastSynced.After(merged.LastSynced) {
		merged.LastSynced = b.LastSynced
	}
	merged.Staged = a.Staged || b.Staged
	return merged
}

// saveIndex writes the index atomically (temp file + rename) so readers
// never observe a partially written file.
func (s *StagingArea) saveIndex(index map[string]FileEntry) error {
	dir := filepath.Join(s.rootDir, StagingDir)
	indexPath := filepath.Join(dir, IndexFile)

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, IndexFile+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary index: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("failed to write temporary index: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("failed to flush temporary index: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("failed to close temporary index: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		cleanup()
		return fmt.Errorf("failed to set index permissions: %w", err)
	}
	if err := os.Rename(tmpName, indexPath); err != nil {
		cleanup()
		return fmt.Errorf("failed to replace index: %w", err)
	}

	return nil
}
