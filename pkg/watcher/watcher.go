package watcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/byvfx/go-notion-md-sync/pkg/util"
	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the markdown root (recursively) and pushes changed
// markdown files to Notion.
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	engine    sync.Engine
	config    *config.Config
	debouncer *debouncer
	root      string
	out       io.Writer

	// syncMu serialises sync callbacks so the engine never runs two pushes at
	// once and the content hash is recorded before the next callback runs.
	syncMu gosync.Mutex

	// hashMu guards lastSynced.
	hashMu gosync.Mutex
	// lastSynced holds the content hash of each file as it was after its
	// last successful sync. Events whose file content still matches are
	// ignored, which suppresses events triggered by the engine rewriting
	// frontmatter after a push.
	lastSynced map[string]string
}

type debouncer struct {
	interval time.Duration

	mu      gosync.Mutex
	pending map[string]*time.Timer
	stopped bool
}

// NewWatcher creates a watcher for cfg.Directories.MarkdownRoot and all of
// its non-hidden, non-excluded subdirectories.
func NewWatcher(cfg *config.Config, engine sync.Engine) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	root := cfg.Directories.MarkdownRoot

	// The root itself must be watchable; subdirectory failures are reported
	// but do not abort the watcher.
	if err := fsWatcher.Add(root); err != nil {
		_ = fsWatcher.Close()
		return nil, fmt.Errorf("failed to watch directory %s: %w", root, err)
	}

	w := &Watcher{
		fsWatcher: fsWatcher,
		engine:    engine,
		config:    cfg,
		root:      root,
		out:       os.Stdout,
		debouncer: &debouncer{
			interval: 2 * time.Second,
			pending:  make(map[string]*time.Timer),
		},
		lastSynced: make(map[string]string),
	}

	w.addDirRecursive(root)

	return w, nil
}

// Start processes file system events until ctx is cancelled or the
// underlying watcher is closed.
func (w *Watcher) Start(ctx context.Context) error {
	defer func() { _ = w.Close() }()

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return nil
			}
			w.handleEvent(ctx, event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return nil
			}
			w.logf("Watcher error: %v\n", err)

		case <-ctx.Done():
			w.debouncer.stop()
			return ctx.Err()
		}
	}
}

// Close stops pending debounced syncs and releases the file system watcher.
// It is safe to call more than once.
func (w *Watcher) Close() error {
	if w.debouncer != nil {
		w.debouncer.stop()
	}
	if w.fsWatcher != nil {
		return w.fsWatcher.Close()
	}
	return nil
}

func (w *Watcher) handleEvent(ctx context.Context, event fsnotify.Event) {
	// Newly created directories (e.g. from a pull) must be watched too.
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			w.addDirRecursive(event.Name)
			return
		}
	}

	// Only process markdown files
	if !strings.HasSuffix(event.Name, ".md") {
		return
	}

	// Check if file should be excluded
	if w.isExcluded(event.Name) {
		return
	}

	if !w.isContentEvent(event) {
		return
	}

	// Debounce the event
	w.debouncer.debounce(event.Name, func() {
		w.syncFile(ctx, event.Name)
	})
}

// isContentEvent reports whether the event may have changed a file's
// content. Writes and creates count; a create is also how a rename into the
// watched directory (atomic save in vim, JetBrains IDEs, ...) is reported.
// A rename event names the old path, so it only counts when a file exists
// there again. Remove and chmod events are ignored.
func (w *Watcher) isContentEvent(event fsnotify.Event) bool {
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
		return true
	}
	if event.Has(fsnotify.Rename) {
		info, err := os.Stat(event.Name)
		return err == nil && info.Mode().IsRegular()
	}
	return false
}

func (w *Watcher) syncFile(ctx context.Context, filePath string) {
	w.syncMu.Lock()
	defer w.syncMu.Unlock()

	if ctx.Err() != nil {
		return
	}

	hash, err := fileHash(filePath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			w.logf("❌ Failed to read %s: %v\n", filePath, err)
		}
		return
	}

	if w.lastSyncedHash(filePath) == hash {
		// Content unchanged since our last sync (typically the engine
		// writing frontmatter back after a push).
		return
	}

	w.logf("📝 File changed: %s\n", filePath)

	if err := w.engine.SyncFileToNotion(ctx, filePath); err != nil {
		w.logf("❌ Failed to sync %s: %v\n", filePath, err)
		return
	}
	w.logf("✅ Synced %s to Notion\n", filePath)

	// Record the content as it is after the sync, including any frontmatter
	// the engine wrote, so the resulting event is not treated as a change.
	if newHash, err := fileHash(filePath); err == nil {
		w.setLastSyncedHash(filePath, newHash)
	}
}

func (w *Watcher) lastSyncedHash(filePath string) string {
	w.hashMu.Lock()
	defer w.hashMu.Unlock()
	return w.lastSynced[filePath]
}

func (w *Watcher) setLastSyncedHash(filePath, hash string) {
	w.hashMu.Lock()
	defer w.hashMu.Unlock()
	w.lastSynced[filePath] = hash
}

func (w *Watcher) isExcluded(path string) bool {
	return util.IsExcluded(w.config.Directories.ExcludedPatterns, w.root, path)
}

// addDirRecursive adds dir and all of its subdirectories to the watcher,
// skipping hidden and excluded directories. Errors are reported and skipped.
func (w *Watcher) addDirRecursive(dir string) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == dir {
				return err
			}
			// Unreadable subdirectory: skip it, keep walking the rest.
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != w.root && w.skipDir(path, d.Name()) {
			return filepath.SkipDir
		}
		if err := w.fsWatcher.Add(path); err != nil {
			w.logf("Watcher warning: failed to watch %s: %v\n", path, err)
		}
		return nil
	})
	if err != nil {
		w.logf("Watcher warning: failed to scan %s: %v\n", dir, err)
	}
}

func (w *Watcher) skipDir(path, name string) bool {
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		return true
	}
	return w.isExcluded(path)
}

func fileHash(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// debounce runs fn once no further call for key has been made within the
// interval.
func (d *debouncer) debounce(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	// Cancel existing timer for this key
	if timer, exists := d.pending[key]; exists {
		timer.Stop()
	}

	var timer *time.Timer
	timer = time.AfterFunc(d.interval, func() {
		d.mu.Lock()
		// A newer timer may have replaced this one (Stop does not prevent a
		// callback that has already started), or the debouncer may have
		// been stopped. Only the current timer runs and removes its entry.
		if d.pending[key] != timer {
			d.mu.Unlock()
			return
		}
		delete(d.pending, key)
		d.mu.Unlock()

		fn()
	})
	d.pending[key] = timer
}

// stop cancels all pending timers and rejects future debounce calls.
func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true
	for key, timer := range d.pending {
		timer.Stop()
		delete(d.pending, key)
	}
}

// pendingCount returns the number of scheduled callbacks.
func (d *debouncer) pendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}

// logf writes a status line to the watcher's output.
func (w *Watcher) logf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(w.out, format, args...)
}
