package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/notion"
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
	parentPageTitle, err := getParentPageTitle()
	if err != nil {
		parentPageTitle = "Unknown parent page"
	}

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

	files := categorizeFiles(status)
	printStatus(parentPageTitle, files)

	return nil
}

type categorizedFiles struct {
	staged   []string
	modified []string
	new      []string
	deleted  []string
}

func categorizeFiles(status map[string]staging.FileStatus) *categorizedFiles {
	files := &categorizedFiles{}

	for file, fileStatus := range status {
		switch fileStatus {
		case staging.StatusStaged:
			files.staged = append(files.staged, file)
		case staging.StatusModified:
			files.modified = append(files.modified, file)
		case staging.StatusNew:
			files.new = append(files.new, file)
		case staging.StatusDeleted:
			files.deleted = append(files.deleted, file)
		}
	}

	// Sort all slices for consistent output
	sort.Strings(files.staged)
	sort.Strings(files.modified)
	sort.Strings(files.new)
	sort.Strings(files.deleted)

	return files
}

func printStatus(parentPageTitle string, files *categorizedFiles) {
	isEmpty := len(files.staged) == 0 && len(files.modified) == 0 &&
		len(files.new) == 0 && len(files.deleted) == 0

	fmt.Printf("On parent page: %s\n", parentPageTitle)

	if isEmpty {
		fmt.Println("nothing to commit, working tree clean")
		return
	}

	printStagedFiles(files.staged)
	printUnstagedChanges(files.modified, files.deleted)
	printUntrackedFiles(files.new)
	printSummary(files)
}

func printStagedFiles(stagedFiles []string) {
	if len(stagedFiles) == 0 {
		return
	}

	fmt.Println("\nChanges staged for sync:")
	fmt.Println("  (use \"notion-md-sync reset <file>...\" to unstage)")
	fmt.Println()
	for _, file := range stagedFiles {
		fmt.Printf("        \033[32mstaged:\033[0m   %s\n", file)
	}
}

func printUnstagedChanges(modifiedFiles, deletedFiles []string) {
	if len(modifiedFiles) == 0 && len(deletedFiles) == 0 {
		return
	}

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

func printUntrackedFiles(newFiles []string) {
	if len(newFiles) == 0 {
		return
	}

	fmt.Println("\nUntracked files:")
	fmt.Println("  (use \"notion-md-sync add <file>...\" to include in what will be synced)")
	fmt.Println()
	for _, file := range newFiles {
		fmt.Printf("        \033[31m%s\033[0m\n", file)
	}
}

func printSummary(files *categorizedFiles) {
	hasUnstaged := len(files.modified) > 0 || len(files.deleted) > 0

	fmt.Println()
	if hasUnstaged || len(files.new) > 0 {
		if len(files.staged) > 0 {
			fmt.Println("You have staged changes ready to sync and unstaged changes.")
		} else {
			fmt.Println("No changes staged for sync.")
		}
		fmt.Println("Use \"notion-md-sync add <file>...\" to stage changes for sync.")
	}

	if len(files.staged) > 0 {
		fmt.Println("Use \"notion-md-sync push\" to sync staged changes to Notion.")
	}
}

func getParentPageTitle() (string, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", err
	}

	if cfg.Notion.ParentPageID == "" {
		return "", fmt.Errorf("no parent page ID configured")
	}

	client := notion.NewClient(cfg.Notion.Token)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	page, err := client.GetPage(ctx, cfg.Notion.ParentPageID)
	if err != nil {
		return "", err
	}

	return extractPageTitle(page), nil
}

// extractPageTitle extracts the title from a Notion page
func extractPageTitle(page *notion.Page) string {
	if titleProp, ok := page.Properties["title"]; ok {
		if titleData, ok := titleProp.(map[string]interface{}); ok {
			if titleList, ok := titleData["title"].([]interface{}); ok && len(titleList) > 0 {
				if titleItem, ok := titleList[0].(map[string]interface{}); ok {
					if plainText, ok := titleItem["plain_text"].(string); ok {
						return plainText
					}
				}
			}
		}
	}
	return "Untitled"
}
