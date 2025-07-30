package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/staging"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
	"github.com/byvfx/go-notion-md-sync/pkg/util"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [file]",
	Short: "Push staged files to Notion",
	Long: `Push staged markdown files to Notion pages.

If no file is specified, pushes all files that have been staged with 'notion-md-sync add'.
If a specific file is provided, it will be automatically staged and pushed.

Examples:
  notion-md-sync push                    # Push all staged files
  notion-md-sync push docs/file.md       # Stage and push a specific file
  notion-md-sync push --dry-run          # Show what would be pushed`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPush,
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
	// Validate file argument if provided
	if len(args) > 0 {
		if err := util.ValidateFilePath(args[0], true); err != nil {
			return fmt.Errorf("invalid file path: %w", err)
		}
	}

	workingDir, err := getWorkingDirectory()
	if err != nil {
		return err
	}

	// Validate working directory
	if err := util.ValidateDirectoryPath(workingDir, true); err != nil {
		return fmt.Errorf("invalid working directory: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	printVerbose("Loaded configuration")
	printVerbose("Direction: push (markdown → Notion)")

	stagingArea := staging.NewStagingArea(workingDir)
	if err := stagingArea.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize staging area: %w", err)
	}

	filesToPush, err := getFilesToPush(args, workingDir, stagingArea)
	if err != nil {
		return err
	}

	if len(filesToPush) == 0 {
		fmt.Println("No files staged for sync.")
		fmt.Println("Use \"notion-md-sync add <file>...\" to stage files, or \"notion-md-sync status\" to see changed files.")
		return nil
	}

	if pushDryRun {
		return performDryRunPush(filesToPush)
	}

	return performPush(cfg, workingDir, filesToPush, stagingArea)
}

func getWorkingDirectory() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	if pushDirectory != "" {
		workingDir = pushDirectory
	}

	return workingDir, nil
}

func getFilesToPush(args []string, workingDir string, stagingArea *staging.StagingArea) ([]string, error) {
	if len(args) > 0 {
		return stageAndGetSpecificFile(args[0], workingDir, stagingArea)
	}

	return stagingArea.GetStagedFiles()
}

func stageAndGetSpecificFile(filePath, workingDir string, stagingArea *staging.StagingArea) ([]string, error) {
	// Convert to relative path if needed
	if filepath.IsAbs(filePath) {
		relPath, err := filepath.Rel(workingDir, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get relative path: %w", err)
		}
		filePath = relPath
	}

	// Stage the file
	if err := stagingArea.AddFile(filePath); err != nil {
		return nil, fmt.Errorf("failed to stage file %s: %w", filePath, err)
	}

	printVerbose("Auto-staged and will push: %s", filePath)
	return []string{filePath}, nil
}

func performDryRunPush(filesToPush []string) error {
	fmt.Println("DRY RUN: No actual changes will be made")
	fmt.Printf("Would push %d file(s) to Notion:\n", len(filesToPush))
	for _, file := range filesToPush {
		fmt.Printf("  %s\n", file)
	}
	return nil
}

func performPush(cfg *config.Config, workingDir string, filesToPush []string, stagingArea *staging.StagingArea) error {
	engine := sync.NewEngine(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	results := pushFilesConcurrently(ctx, engine, workingDir, filesToPush)

	return processPushResults(results, stagingArea)
}

type pushResult struct {
	file    string
	success bool
	error   error
}

func pushFilesConcurrently(ctx context.Context, engine sync.Engine, workingDir string, filesToPush []string) []pushResult {
	maxWorkers := 3
	if len(filesToPush) < maxWorkers {
		maxWorkers = len(filesToPush)
	}

	jobs := make(chan string, len(filesToPush))
	results := make(chan pushResult, len(filesToPush))

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		go pushWorker(ctx, engine, workingDir, jobs, results)
	}

	// Send jobs
	for _, file := range filesToPush {
		jobs <- file
	}
	close(jobs)

	// Collect results
	var allResults []pushResult
	for i := 0; i < len(filesToPush); i++ {
		allResults = append(allResults, <-results)
	}

	return allResults
}

func pushWorker(ctx context.Context, engine sync.Engine, workingDir string, jobs <-chan string, results chan<- pushResult) {
	for relPath := range jobs {
		fullPath := filepath.Join(workingDir, relPath)
		printVerbose("Pushing file: %s", relPath)

		err := engine.SyncFileToNotion(ctx, fullPath)
		results <- pushResult{
			file:    relPath,
			success: err == nil,
			error:   err,
		}
	}
}

func processPushResults(results []pushResult, stagingArea *staging.StagingArea) error {
	var successfulPushes []string
	var failedPushes []string

	for _, result := range results {
		if result.success {
			fmt.Printf("✓ Successfully pushed %s to Notion\n", result.file)
			successfulPushes = append(successfulPushes, result.file)
		} else {
			fmt.Printf("✗ Failed to push %s: %v\n", result.file, result.error)
			failedPushes = append(failedPushes, result.file)
		}
	}

	// Mark successfully pushed files as synced
	if len(successfulPushes) > 0 {
		if err := stagingArea.MarkSynced(successfulPushes); err != nil {
			printVerbose("Warning: failed to mark files as synced: %v", err)
		}
	}

	// Print summary
	fmt.Println()
	if len(successfulPushes) > 0 {
		fmt.Printf("Successfully pushed %d file(s) to Notion.\n", len(successfulPushes))
	}

	if len(failedPushes) > 0 {
		fmt.Printf("Failed to push %d file(s). These files remain staged.\n", len(failedPushes))
		return fmt.Errorf("some files failed to push")
	}

	return nil
}
