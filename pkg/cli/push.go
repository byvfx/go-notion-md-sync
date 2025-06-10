package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [file]",
	Short: "Push markdown files to Notion",
	Long:  `Push markdown files to Notion pages. If no file is specified, pushes all markdown files.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPush,
}

var (
	pushDirectory string
	pushDryRun    bool
)

func init() {
	pushCmd.Flags().StringVar(&pushDirectory, "directory", "", "directory containing markdown files (defaults to config's markdown_root)")
	pushCmd.Flags().BoolVar(&pushDryRun, "dry-run", false, "show what would be pushed without actually pushing")
}

func runPush(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	printVerbose("Loaded configuration")
	printVerbose("Direction: push (markdown → Notion)")

	// Create sync engine
	engine := sync.NewEngine(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Determine working directory
	workingDir := pushDirectory
	if workingDir == "" {
		workingDir = cfg.Directories.MarkdownRoot
	}

	if pushDryRun {
		fmt.Println("DRY RUN: No actual changes will be made")
		if len(args) > 0 {
			fmt.Printf("Would push file: %s\n", args[0])
		} else {
			// Find all markdown files to show what would be pushed
			files, err := findMarkdownFiles(workingDir)
			if err != nil {
				return fmt.Errorf("failed to find markdown files: %w", err)
			}
			fmt.Printf("Would push %d files from %s to Notion:\n", len(files), workingDir)
			for _, file := range files {
				fmt.Printf("  %s\n", file)
			}
		}
		return nil
	}

	// Push specific file or all files
	if len(args) > 0 {
		filePath := args[0]
		printVerbose("Pushing file: %s", filePath)
		
		if err := engine.SyncFileToNotion(ctx, filePath); err != nil {
			return fmt.Errorf("failed to push file: %w", err)
		}
		
		fmt.Printf("✓ Successfully pushed %s to Notion\n", filePath)
	} else {
		printVerbose("Pushing all markdown files from: %s", workingDir)
		return performDirectorySync(ctx, engine, workingDir, "push")
	}

	return nil
}