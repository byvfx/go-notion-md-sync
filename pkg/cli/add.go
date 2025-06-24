package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/byvfx/go-notion-md-sync/pkg/staging"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [file...]",
	Short: "Add file contents to the staging area",
	Long: `Stage files for synchronization to Notion.

Examples:
  notion-md-sync add file.md              # Stage a specific file
  notion-md-sync add docs/                # Stage all files in docs directory
  notion-md-sync add .                    # Stage all changed files in current directory
  notion-md-sync add *.md                 # Stage all markdown files`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	stagingArea := staging.NewStagingArea(workingDir)
	if err := stagingArea.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize staging area: %w", err)
	}

	// Collect all files to add
	var filesToAdd []string

	for _, arg := range args {
		if arg == "." {
			// Add all changed files
			files, err := getAllChangedFiles(stagingArea)
			if err != nil {
				return fmt.Errorf("failed to get changed files: %w", err)
			}
			filesToAdd = append(filesToAdd, files...)
		} else {
			// Handle individual files or patterns
			files, err := expandFilePattern(workingDir, arg)
			if err != nil {
				return fmt.Errorf("failed to expand pattern %s: %w", arg, err)
			}
			filesToAdd = append(filesToAdd, files...)
		}
	}

	// Remove duplicates
	filesToAdd = removeDuplicates(filesToAdd)

	if len(filesToAdd) == 0 {
		fmt.Println("No files to add.")
		return nil
	}

	// Add each file
	var addedFiles []string
	for _, file := range filesToAdd {
		if err := stagingArea.AddFile(file); err != nil {
			printVerbose("Warning: failed to add %s: %v", file, err)
			continue
		}
		addedFiles = append(addedFiles, file)
		printVerbose("Added %s", file)
	}

	// Summary
	if len(addedFiles) > 0 {
		fmt.Printf("Added %d file(s) to staging area.\n", len(addedFiles))
		if !verbose {
			fmt.Println("Use \"notion-md-sync status\" to see staged files.")
		}
	}

	if len(addedFiles) != len(filesToAdd) {
		fmt.Printf("Warning: %d file(s) could not be added.\n", len(filesToAdd)-len(addedFiles))
	}

	return nil
}

func getAllChangedFiles(stagingArea *staging.StagingArea) ([]string, error) {
	status, err := stagingArea.GetStatus()
	if err != nil {
		return nil, err
	}

	var files []string
	for file, fileStatus := range status {
		// Add files that are modified or new (but not already staged)
		if fileStatus == staging.StatusModified || fileStatus == staging.StatusNew {
			files = append(files, file)
		}
	}

	return files, nil
}

func expandFilePattern(workingDir, pattern string) ([]string, error) {
	fullPath := getFullPath(workingDir, pattern)

	// Check if it's a directory
	if files, isDir, err := expandDirectory(workingDir, fullPath); isDir {
		return files, err
	}

	// Try as a glob pattern
	if files, err := expandGlobPattern(workingDir, fullPath); err != nil || len(files) > 0 {
		return files, err
	}

	// Try as a single file
	return expandSingleFile(workingDir, pattern)
}

func getFullPath(workingDir, pattern string) string {
	if filepath.IsAbs(pattern) {
		return pattern
	}
	return filepath.Join(workingDir, pattern)
}

func expandDirectory(workingDir, fullPath string) ([]string, bool, error) {
	info, err := os.Stat(fullPath)
	if err != nil || !info.IsDir() {
		return nil, false, nil
	}

	var files []string
	err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if isMarkdownFile(path, info) {
			relPath, err := filepath.Rel(workingDir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		return nil
	})

	return files, true, err
}

func expandGlobPattern(workingDir, fullPath string) ([]string, error) {
	matches, err := filepath.Glob(fullPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		if isMarkdownFile(match, info) {
			relPath, err := filepath.Rel(workingDir, match)
			if err != nil {
				continue
			}
			files = append(files, relPath)
		}
	}

	return files, nil
}

func expandSingleFile(workingDir, pattern string) ([]string, error) {
	relPath := pattern
	if filepath.IsAbs(pattern) {
		var err error
		relPath, err = filepath.Rel(workingDir, pattern)
		if err != nil {
			return nil, err
		}
	}

	fullPath := filepath.Join(workingDir, relPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, nil
	}

	if isMarkdownFile(relPath, info) {
		return []string{relPath}, nil
	}

	return nil, nil
}

func isMarkdownFile(path string, info os.FileInfo) bool {
	return !info.IsDir() && strings.HasSuffix(path, ".md")
}

func removeDuplicates(files []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, file := range files {
		if !seen[file] {
			seen[file] = true
			result = append(result, file)
		}
	}

	return result
}
