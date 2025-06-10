package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/byvfx/go-notion-md-sync/pkg/markdown"
	"github.com/byvfx/go-notion-md-sync/pkg/sync"
)

// findMarkdownFiles recursively finds all markdown files in a directory
func findMarkdownFiles(dir string) ([]string, error) {
	var files []string
	
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && (strings.HasSuffix(strings.ToLower(path), ".md") || 
			strings.HasSuffix(strings.ToLower(path), ".markdown")) {
			files = append(files, path)
		}
		
		return nil
	})
	
	return files, err
}

// performDirectorySync syncs all markdown files in a directory
func performDirectorySync(ctx context.Context, engine sync.Engine, workingDir, direction string) error {
	// Find all markdown files in directory
	files, err := findMarkdownFiles(workingDir)
	if err != nil {
		return fmt.Errorf("failed to find markdown files: %w", err)
	}

	if len(files) == 0 {
		fmt.Printf("No markdown files found in %s\n", workingDir)
		return nil
	}

	fmt.Printf("Found %d markdown files in %s\n", len(files), workingDir)
	
	successCount := 0
	failureCount := 0
	skippedCount := 0

	for _, file := range files {
		printVerbose("Processing: %s", file)
		
		switch direction {
		case "push":
			if err := engine.SyncFileToNotion(ctx, file); err != nil {
				fmt.Printf("❌ Failed to push %s: %v\n", file, err)
				failureCount++
			} else {
				fmt.Printf("✅ Pushed %s\n", file)
				successCount++
			}
		case "pull":
			if err := syncSingleFileHelper(ctx, engine, file, "pull"); err != nil {
				if err.Error() == "no notion_id found in frontmatter - cannot pull without page ID" {
					printVerbose("Skipped %s: No notion_id in frontmatter", file)
					skippedCount++
				} else {
					fmt.Printf("❌ Failed to pull %s: %v\n", file, err)
					failureCount++
				}
			} else {
				fmt.Printf("✅ Pulled %s\n", file)
				successCount++
			}
		case "bidirectional":
			// Try push first, then pull if file has notion_id
			if err := engine.SyncFileToNotion(ctx, file); err != nil {
				fmt.Printf("❌ Failed to push %s: %v\n", file, err)
				failureCount++
			} else {
				fmt.Printf("✅ Pushed %s\n", file)
				successCount++
			}
		}
	}

	fmt.Printf("\n📊 Sync completed: %d succeeded, %d failed, %d skipped\n", 
		successCount, failureCount, skippedCount)
	
	return nil
}

// syncSingleFileHelper handles syncing a single file (shared between commands)
func syncSingleFileHelper(ctx context.Context, engine sync.Engine, filePath, direction string) error {
	switch direction {
	case "push":
		return engine.SyncFileToNotion(ctx, filePath)
	case "pull":
		// For pull, try to get notion_id from frontmatter
		doc, err := markdown.NewParser().ParseFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to parse file for pull: %w", err)
		}
		
		frontmatter, err := markdown.ExtractFrontmatter(doc.Metadata)
		if err != nil {
			return fmt.Errorf("failed to extract frontmatter: %w", err)
		}
		
		if frontmatter.NotionID == "" {
			return fmt.Errorf("no notion_id found in frontmatter - cannot pull without page ID")
		}
		
		return engine.SyncNotionToFile(ctx, frontmatter.NotionID, filePath)
	case "bidirectional":
		return fmt.Errorf("bidirectional sync not supported for single file")
	default:
		return fmt.Errorf("invalid direction: %s", direction)
	}
}