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

// performDirectorySync syncs the whole markdown root (the engine must have
// been created with workingDir as its markdown root) in the given direction.
func performDirectorySync(ctx context.Context, engine sync.Engine, workingDir, direction string) error {
	switch direction {
	case "push", "pull", "bidirectional":
	default:
		return fmt.Errorf("invalid direction: %s", direction)
	}
	return engine.SyncAll(ctx, direction)
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
