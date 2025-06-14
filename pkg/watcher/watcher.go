package watcher

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	fsWatcher *fsnotify.Watcher
	engine    sync.Engine
	config    *config.Config
	debouncer *debouncer
}

type debouncer struct {
	interval time.Duration
	pending  map[string]*time.Timer
}

func NewWatcher(cfg *config.Config, engine sync.Engine) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Add markdown root directory to watcher
	if err := fsWatcher.Add(cfg.Directories.MarkdownRoot); err != nil {
		fsWatcher.Close()
		return nil, fmt.Errorf("failed to watch directory %s: %w", cfg.Directories.MarkdownRoot, err)
	}

	return &Watcher{
		fsWatcher: fsWatcher,
		engine:    engine,
		config:    cfg,
		debouncer: &debouncer{
			interval: 2 * time.Second,
			pending:  make(map[string]*time.Timer),
		},
	}, nil
}

func (w *Watcher) Start(ctx context.Context) error {
	defer w.fsWatcher.Close()

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
			fmt.Printf("Watcher error: %v\n", err)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (w *Watcher) Close() error {
	if w.fsWatcher != nil {
		return w.fsWatcher.Close()
	}
	return nil
}

func (w *Watcher) handleEvent(ctx context.Context, event fsnotify.Event) {
	// Only process markdown files
	if !strings.HasSuffix(event.Name, ".md") {
		return
	}

	// Check if file should be excluded
	if w.isExcluded(event.Name) {
		return
	}

	// Only process write events
	if event.Op&fsnotify.Write != fsnotify.Write {
		return
	}

	fmt.Printf("📝 File changed: %s\n", event.Name)

	// Debounce the event
	w.debouncer.debounce(event.Name, func() {
		w.syncFile(ctx, event.Name)
	})
}

func (w *Watcher) syncFile(ctx context.Context, filePath string) {
	if err := w.engine.SyncFileToNotion(ctx, filePath); err != nil {
		fmt.Printf("❌ Failed to sync %s: %v\n", filePath, err)
	} else {
		fmt.Printf("✅ Synced %s to Notion\n", filePath)
	}
}

func (w *Watcher) isExcluded(path string) bool {
	for _, pattern := range w.config.Directories.ExcludedPatterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}

func (d *debouncer) debounce(key string, fn func()) {
	// Cancel existing timer for this key
	if timer, exists := d.pending[key]; exists {
		timer.Stop()
	}

	// Create new timer
	d.pending[key] = time.AfterFunc(d.interval, func() {
		fn()
		delete(d.pending, key)
	})
}
