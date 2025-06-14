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
	var files []string

	// Check if it's a directory
	fullPath := pattern
	if !filepath.IsAbs(pattern) {
		fullPath = filepath.Join(workingDir, pattern)
	}

	info, err := os.Stat(fullPath)
	if err == nil && info.IsDir() {
		// It's a directory, find all .md files in it
		err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && strings.HasSuffix(path, ".md") {
				// Convert to relative path
				relPath, err := filepath.Rel(workingDir, path)
				if err != nil {
					return err
				}
				files = append(files, relPath)
			}
			return nil
		})
		return files, err
	}

	// Try as a glob pattern
	matches, err := filepath.Glob(fullPath)
	if err != nil {
		return nil, err
	}

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		// Only include .md files
		if !info.IsDir() && strings.HasSuffix(match, ".md") {
			relPath, err := filepath.Rel(workingDir, match)
			if err != nil {
				continue
			}
			files = append(files, relPath)
		}
	}

	// If no matches found and it looks like a file, try to add it directly
	if len(files) == 0 {
		relPath := pattern
		if filepath.IsAbs(pattern) {
			relPath, err = filepath.Rel(workingDir, pattern)
			if err != nil {
				return nil, err
			}
		}

		// Check if the file exists and is a .md file
		fullPath = filepath.Join(workingDir, relPath)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() && strings.HasSuffix(relPath, ".md") {
			files = append(files, relPath)
		}
	}

	return files, nil
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