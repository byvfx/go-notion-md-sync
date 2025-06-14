package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [direction]",
	Short: "Sync between markdown and Notion",
	Long: `Sync files between markdown and Notion in the specified direction.
Directions: push (markdown → Notion), pull (Notion → markdown), bidirectional (both ways)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSync,
}

var (
	syncFile      string
	syncDirection string
	syncDirectory string
	dryRun        bool
)

func init() {
	syncCmd.Flags().StringVarP(&syncFile, "file", "f", "", "specific file to sync")
	syncCmd.Flags().StringVarP(&syncDirection, "direction", "d", "push", "sync direction (push, pull, bidirectional)")
	syncCmd.Flags().StringVar(&syncDirectory, "directory", "", "directory containing markdown files (defaults to config's markdown_root)")
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be synced without making changes")
}

func runSync(cmd *cobra.Command, args []string) error {
	// Use argument as direction if provided
	if len(args) > 0 {
		syncDirection = args[0]
	}

	// Validate direction
	if syncDirection != "push" && syncDirection != "pull" && syncDirection != "bidirectional" {
		return fmt.Errorf("invalid direction: %s (must be push, pull, or bidirectional)", syncDirection)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	printVerbose("Loaded configuration from: %s", configPath)
	printVerbose("Sync direction: %s", syncDirection)

	// Create sync engine
	engine := sync.NewEngine(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Determine the working directory
	workingDir := syncDirectory
	if workingDir == "" {
		workingDir = cfg.Directories.MarkdownRoot
	}

	if dryRun {
		fmt.Println("DRY RUN: No actual changes will be made")
		return performDryRun(ctx, workingDir, syncFile, syncDirection)
	}

	// Sync specific file or all files
	if syncFile != "" {
		printVerbose("Syncing file: %s", syncFile)
		return syncSingleFile(ctx, engine, syncFile, syncDirection)
	}

	printVerbose("Syncing all files in directory: %s", workingDir)
	if err := performDirectorySync(ctx, engine, workingDir, syncDirection); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Printf("✓ Sync completed successfully (%s)\n", syncDirection)
	return nil
}

func syncSingleFile(ctx context.Context, engine sync.Engine, filePath, direction string) error {
	return syncSingleFileHelper(ctx, engine, filePath, direction)
}

func performDryRun(ctx context.Context, workingDir, specificFile, direction string) error {
	if specificFile != "" {
		fmt.Printf("Would sync file: %s (%s)\n", specificFile, direction)
		return nil
	}

	// Find all markdown files in directory
	files, err := findMarkdownFiles(workingDir)
	if err != nil {
		return fmt.Errorf("failed to find markdown files: %w", err)
	}

	if len(files) == 0 {
		fmt.Printf("No markdown files found in %s\n", workingDir)
		return nil
	}

	actionVerb := getActionVerb(direction)
	fmt.Printf("Found %d markdown files in %s\n", len(files), workingDir)
	fmt.Printf("The following files would be %s:\n", actionVerb)

	for _, file := range files {
		fmt.Printf("  %s\n", file)
	}

	return nil
}

func getActionVerb(direction string) string {
	switch direction {
	case "push":
		return "pushed to Notion"
	case "pull":
		return "pulled from Notion"
	case "bidirectional":
		return "synced bidirectionally"
	default:
		return "synced"
	}
}
