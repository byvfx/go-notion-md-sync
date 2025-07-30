package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/byvfx/go-notion-md-sync/pkg/util"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull content from Notion to markdown files",
	Long:  `Pull all pages from the configured Notion parent page and create corresponding markdown files.`,
	RunE:  runPull,
}

var (
	pullPageID    string
	pullPage      string
	pullOutput    string
	pullDirectory string
	pullDryRun    bool
)

func init() {
	pullCmd.Flags().StringVar(&pullPageID, "page-id", "", "specific Notion page ID to pull")
	pullCmd.Flags().StringVar(&pullPage, "page", "", "specific page filename to pull (e.g., 'Table Page.md')")
	pullCmd.Flags().StringVarP(&pullOutput, "output", "o", "", "output file path (required when using --page-id)")
	pullCmd.Flags().StringVar(&pullDirectory, "directory", "", "directory to save pulled files (defaults to config's markdown_root)")
	pullCmd.Flags().BoolVar(&pullDryRun, "dry-run", false, "show what would be pulled without actually pulling")
}

func runPull(cmd *cobra.Command, args []string) error {
	// Validate inputs
	if pullPageID != "" {
		if err := util.ValidateNotionPageID(pullPageID); err != nil {
			return fmt.Errorf("invalid page ID: %w", err)
		}
		if pullOutput == "" {
			return fmt.Errorf("--output flag is required when pulling a specific page")
		}
		if err := util.ValidateFilePath(pullOutput, false); err != nil {
			return fmt.Errorf("invalid output path: %w", err)
		}
	}

	if pullPage != "" {
		sanitized, err := util.SanitizeAndValidateFilename(pullPage)
		if err != nil {
			return fmt.Errorf("invalid page filename: %w", err)
		}
		pullPage = sanitized
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	printVerbose("Loaded configuration")
	printVerbose("Direction: pull (Notion → markdown)")

	// Create sync engine
	engine := sync.NewEngine(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Determine output directory
	outputDir := pullDirectory
	if outputDir == "" {
		outputDir = cfg.Directories.MarkdownRoot
	}

	// Validate output directory
	if err := util.ValidateDirectoryPath(outputDir, true); err != nil {
		return fmt.Errorf("invalid output directory: %w", err)
	}

	if pullDryRun {
		fmt.Println("DRY RUN: No actual changes will be made")
		if pullPageID != "" {
			if pullOutput == "" {
				return fmt.Errorf("--output flag is required when pulling a specific page")
			}
			fmt.Printf("Would pull page %s to %s\n", pullPageID, pullOutput)
		} else if pullPage != "" {
			fmt.Printf("Would pull page %s\n", pullPage)
		} else {
			fmt.Printf("Would pull all child pages from parent %s to directory %s\n",
				cfg.Notion.ParentPageID, outputDir)
		}
		return nil
	}

	// Pull specific page or all pages
	if pullPageID != "" {
		if pullOutput == "" {
			return fmt.Errorf("--output flag is required when pulling a specific page")
		}

		fmt.Printf("Pulling page from Notion...\n")
		fmt.Printf("  Page ID: %s\n", pullPageID)
		fmt.Printf("  Output: %s\n", pullOutput)
		printVerbose("Pulling page: %s to %s", pullPageID, pullOutput)

		if err := engine.SyncNotionToFile(ctx, pullPageID, pullOutput); err != nil {
			return fmt.Errorf("failed to pull page: %w", err)
		}

		fmt.Printf("\n✓ Successfully pulled page to %s\n", pullOutput)
	} else if pullPage != "" {
		fmt.Printf("Pulling specific page: %s\n", pullPage)
		printVerbose("Pulling page by filename: %s", pullPage)

		if err := engine.SyncSpecificFile(ctx, pullPage, "pull"); err != nil {
			return fmt.Errorf("failed to pull page %s: %w", pullPage, err)
		}

		fmt.Printf("✓ Successfully pulled %s\n", pullPage)
	} else {
		fmt.Printf("Pulling all pages from Notion parent page: %s\n", cfg.Notion.ParentPageID)
		printVerbose("Pulling all pages from parent: %s", cfg.Notion.ParentPageID)

		if err := engine.SyncAll(ctx, "pull"); err != nil {
			return fmt.Errorf("pull failed: %w", err)
		}

		fmt.Println("\n✓ Pull completed successfully")
	}

	return nil
}
