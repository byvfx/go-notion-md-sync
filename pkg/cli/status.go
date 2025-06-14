package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/byvfx/go-notion-md-sync/pkg/staging"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the working tree status",
	Long: `Show the status of files in the working directory.
Files are shown in different categories:
- Changes staged for sync (ready to push)
- Changes not staged for sync (modified but not added)
- Untracked files (new files not yet added)`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	stagingArea := staging.NewStagingArea(workingDir)
	if err := stagingArea.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize staging area: %w", err)
	}

	status, err := stagingArea.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// Separate files by status
	var stagedFiles []string
	var modifiedFiles []string
	var newFiles []string
	var deletedFiles []string

	for file, fileStatus := range status {
		switch fileStatus {
		case staging.StatusStaged:
			stagedFiles = append(stagedFiles, file)
		case staging.StatusModified:
			modifiedFiles = append(modifiedFiles, file)
		case staging.StatusNew:
			newFiles = append(newFiles, file)
		case staging.StatusDeleted:
			deletedFiles = append(deletedFiles, file)
		}
	}

	// Sort all slices for consistent output
	sort.Strings(stagedFiles)
	sort.Strings(modifiedFiles)
	sort.Strings(newFiles)
	sort.Strings(deletedFiles)

	// Print status
	if len(stagedFiles) == 0 && len(modifiedFiles) == 0 && len(newFiles) == 0 && len(deletedFiles) == 0 {
		fmt.Println("On branch main")
		fmt.Println("nothing to commit, working tree clean")
		return nil
	}

	fmt.Println("On branch main")

	// Staged changes
	if len(stagedFiles) > 0 {
		fmt.Println("\nChanges staged for sync:")
		fmt.Println("  (use \"notion-md-sync reset <file>...\" to unstage)")
		fmt.Println()
		for _, file := range stagedFiles {
			fmt.Printf("        \033[32mstaged:\033[0m   %s\n", file)
		}
	}

	// Changes not staged
	hasUnstaged := len(modifiedFiles) > 0 || len(deletedFiles) > 0
	if hasUnstaged {
		fmt.Println("\nChanges not staged for sync:")
		fmt.Println("  (use \"notion-md-sync add <file>...\" to stage changes)")
		fmt.Println()
		
		for _, file := range modifiedFiles {
			fmt.Printf("        \033[31mmodified:\033[0m %s\n", file)
		}
		for _, file := range deletedFiles {
			fmt.Printf("        \033[31mdeleted:\033[0m  %s\n", file)
		}
	}

	// Untracked files
	if len(newFiles) > 0 {
		fmt.Println("\nUntracked files:")
		fmt.Println("  (use \"notion-md-sync add <file>...\" to include in what will be synced)")
		fmt.Println()
		for _, file := range newFiles {
			fmt.Printf("        \033[31m%s\033[0m\n", file)
		}
	}

	// Summary message
	fmt.Println()
	if hasUnstaged || len(newFiles) > 0 {
		if len(stagedFiles) > 0 {
			fmt.Println("You have staged changes ready to sync and unstaged changes.")
		} else {
			fmt.Println("No changes staged for sync.")
		}
		fmt.Println("Use \"notion-md-sync add <file>...\" to stage changes for sync.")
	}

	if len(stagedFiles) > 0 {
		fmt.Println("Use \"notion-md-sync push\" to sync staged changes to Notion.")
	}

	return nil
}