package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/byoung/go-notion-md-sync/pkg/config"
	"github.com/byoung/go-notion-md-sync/pkg/sync"
	"github.com/byoung/go-notion-md-sync/pkg/watcher"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for file changes and auto-sync",
	Long:  `Watch the markdown directory for changes and automatically sync files to Notion.`,
	RunE:  runWatch,
}

var (
	watchInterval time.Duration
)

func init() {
	watchCmd.Flags().DurationVar(&watchInterval, "interval", 1*time.Second, "debounce interval for file changes")
}

func runWatch(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	printVerbose("Loaded configuration")
	printVerbose("Watching directory: %s", cfg.Directories.MarkdownRoot)

	// Create sync engine
	engine := sync.NewEngine(cfg)

	// Create file watcher
	w, err := watcher.NewWatcher(cfg, engine)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer w.Close()

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	fmt.Printf("🔍 Watching for changes in %s\n", cfg.Directories.MarkdownRoot)
	fmt.Println("Press Ctrl+C to stop")

	// Start watching
	if err := w.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("watcher error: %w", err)
	}

	fmt.Println("✓ Watch stopped")
	return nil
}