package cli

import (
	"fmt"
	"os"

	"github.com/byvfx/go-notion-md-sync/pkg/staging"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset [file...]",
	Short: "Remove files from the staging area",
	Long: `Remove files from the staging area, keeping changes in the working directory.

Examples:
  notion-md-sync reset file.md           # Unstage a specific file
  notion-md-sync reset .                 # Unstage all staged files
  notion-md-sync reset                   # Unstage all staged files (same as reset .)`,
	RunE: runReset,
}

var (
	resetAll bool
)

func init() {
	resetCmd.Flags().BoolVar(&resetAll, "all", false, "unstage all staged files")
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	stagingArea := staging.NewStagingArea(workingDir)

	// If no args provided or "." is provided or --all flag, reset all staged files
	if len(args) == 0 || resetAll || (len(args) == 1 && args[0] == ".") {
		return resetAllStaged(stagingArea)
	}

	// Reset specific files
	return resetSpecificFiles(stagingArea, args)
}

func resetAllStaged(stagingArea *staging.StagingArea) error {
	stagedFiles, err := stagingArea.GetStagedFiles()
	if err != nil {
		return fmt.Errorf("failed to get staged files: %w", err)
	}

	if len(stagedFiles) == 0 {
		fmt.Println("No files staged for sync.")
		return nil
	}

	for _, file := range stagedFiles {
		if err := stagingArea.ResetFile(file); err != nil {
			printVerbose("Warning: failed to reset %s: %v", file, err)
			continue
		}
		printVerbose("Unstaged %s", file)
	}

	fmt.Printf("Unstaged %d file(s).\n", len(stagedFiles))
	if !verbose {
		fmt.Println("Use \"notion-md-sync status\" to see current status.")
	}

	return nil
}

func resetSpecificFiles(stagingArea *staging.StagingArea, files []string) error {
	stagedFiles, err := stagingArea.GetStagedFiles()
	if err != nil {
		return fmt.Errorf("failed to get staged files: %w", err)
	}

	// Create a map for quick lookup
	stagedMap := make(map[string]bool)
	for _, file := range stagedFiles {
		stagedMap[file] = true
	}

	var resetFiles []string
	var notStagedFiles []string

	for _, file := range files {
		if stagedMap[file] {
			if err := stagingArea.ResetFile(file); err != nil {
				printVerbose("Warning: failed to reset %s: %v", file, err)
				continue
			}
			resetFiles = append(resetFiles, file)
			printVerbose("Unstaged %s", file)
		} else {
			notStagedFiles = append(notStagedFiles, file)
		}
	}

	// Summary
	if len(resetFiles) > 0 {
		fmt.Printf("Unstaged %d file(s).\n", len(resetFiles))
	}

	if len(notStagedFiles) > 0 {
		fmt.Printf("Warning: %d file(s) were not staged:\n", len(notStagedFiles))
		for _, file := range notStagedFiles {
			fmt.Printf("  %s\n", file)
		}
	}

	if len(resetFiles) == 0 && len(notStagedFiles) == 0 {
		fmt.Println("No files to unstage.")
	}

	return nil
}