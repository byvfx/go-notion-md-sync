package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/config"
	"github.com/byvfx/go-notion-md-sync/pkg/markdown"
	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

type Engine interface {
	SyncFileToNotion(ctx context.Context, filePath string) error
	SyncNotionToFile(ctx context.Context, pageID, filePath string) error
	SyncAll(ctx context.Context, direction string) error
}

type engine struct {
	config    *config.Config
	notion    notion.Client
	parser    markdown.Parser
	converter Converter
}

func NewEngine(cfg *config.Config) Engine {
	return &engine{
		config:    cfg,
		notion:    notion.NewClient(cfg.Notion.Token),
		parser:    markdown.NewParser(),
		converter: NewConverter(),
	}
}

func (e *engine) SyncFileToNotion(ctx context.Context, filePath string) error {
	// Parse markdown file
	doc, err := e.parser.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse markdown file: %w", err)
	}

	// Extract frontmatter
	frontmatter, err := markdown.ExtractFrontmatter(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to extract frontmatter: %w", err)
	}

	// Skip if sync is disabled
	if !frontmatter.SyncEnabled {
		return nil
	}

	// Convert markdown to Notion blocks
	blocks, err := e.converter.MarkdownToBlocks(doc.Content)
	if err != nil {
		return fmt.Errorf("failed to convert markdown to blocks: %w", err)
	}

	// Determine title
	title := frontmatter.Title
	if title == "" {
		title = e.getTitleFromFilename(filePath)
	}

	// Create or update page
	if frontmatter.NotionID != "" {
		// Update existing page
		err = e.updateNotionPage(ctx, frontmatter.NotionID, title, blocks)
	} else {
		// Create new page
		pageID, err := e.createNotionPage(ctx, title, blocks)
		if err != nil {
			return err
		}
		
		// Update frontmatter with new page ID
		frontmatter.NotionID = pageID
		frontmatter.UpdatedAt = &time.Time{}
		*frontmatter.UpdatedAt = time.Now()
		
		// Write back to file
		return e.parser.CreateMarkdownWithFrontmatter(
			filePath,
			frontmatter.ToMetadata(),
			doc.Content,
		)
	}

	return err
}

func (e *engine) SyncNotionToFile(ctx context.Context, pageID, filePath string) error {
	// Get page from Notion
	page, err := e.notion.GetPage(ctx, pageID)
	if err != nil {
		return fmt.Errorf("failed to get Notion page: %w", err)
	}

	// Get page blocks
	blocks, err := e.notion.GetPageBlocks(ctx, pageID)
	if err != nil {
		return fmt.Errorf("failed to get page blocks: %w", err)
	}

	// Convert blocks to markdown
	content, err := e.converter.BlocksToMarkdown(blocks)
	if err != nil {
		return fmt.Errorf("failed to convert blocks to markdown: %w", err)
	}

	// Create frontmatter
	frontmatter := &markdown.FrontmatterFields{
		Title:       e.extractTitleFromPage(page),
		NotionID:    pageID,
		CreatedAt:   &page.CreatedTime,
		UpdatedAt:   &time.Time{},
		SyncEnabled: true,
	}
	*frontmatter.UpdatedAt = time.Now()

	// Write markdown file
	return e.parser.CreateMarkdownWithFrontmatter(
		filePath,
		frontmatter.ToMetadata(),
		content,
	)
}

func (e *engine) SyncAll(ctx context.Context, direction string) error {
	switch direction {
	case "push":
		return e.syncAllMarkdownToNotion(ctx)
	case "pull":
		return e.syncAllNotionToMarkdown(ctx)
	case "bidirectional":
		return e.syncBidirectional(ctx)
	default:
		return fmt.Errorf("unsupported sync direction: %s", direction)
	}
}

func (e *engine) syncAllMarkdownToNotion(ctx context.Context) error {
	return filepath.Walk(e.config.Directories.MarkdownRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		if e.isExcluded(path) {
			return nil
		}

		return e.SyncFileToNotion(ctx, path)
	})
}

func (e *engine) syncAllNotionToMarkdown(ctx context.Context) error {
	// Get all child pages from parent
	pages, err := e.notion.GetChildPages(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		return fmt.Errorf("failed to get child pages: %w", err)
	}

	for _, page := range pages {
		title := e.extractTitleFromPage(&page)
		filePath := filepath.Join(e.config.Directories.MarkdownRoot, title+".md")
		
		if err := e.SyncNotionToFile(ctx, page.ID, filePath); err != nil {
			return fmt.Errorf("failed to sync page %s: %w", page.ID, err)
		}
	}

	return nil
}

func (e *engine) syncBidirectional(ctx context.Context) error {
	// First, sync all markdown files to Notion
	if err := e.syncAllMarkdownToNotion(ctx); err != nil {
		return fmt.Errorf("failed to sync markdown to Notion: %w", err)
	}

	// Then, sync any Notion pages that don't have corresponding markdown files
	pages, err := e.notion.GetChildPages(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		return fmt.Errorf("failed to get child pages: %w", err)
	}

	for _, page := range pages {
		title := e.extractTitleFromPage(&page)
		filePath := filepath.Join(e.config.Directories.MarkdownRoot, title+".md")
		
		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if err := e.SyncNotionToFile(ctx, page.ID, filePath); err != nil {
				return fmt.Errorf("failed to sync page %s: %w", page.ID, err)
			}
		}
	}

	return nil
}

// Helper functions

func (e *engine) createNotionPage(ctx context.Context, title string, blocks []map[string]interface{}) (string, error) {
	properties := map[string]interface{}{
		"title": map[string]interface{}{
			"title": []notion.RichText{
				{
					Type:      "text",
					Text:      &notion.TextContent{Content: title},
					PlainText: title,
				},
			},
		},
	}

	page, err := e.notion.CreatePage(ctx, e.config.Notion.ParentPageID, properties)
	if err != nil {
		return "", fmt.Errorf("failed to create page: %w", err)
	}

	// Add blocks to the page (only if we have blocks)
	if len(blocks) > 0 {
		if err := e.notion.UpdatePageBlocks(ctx, page.ID, blocks); err != nil {
			// Log the error but don't fail - the page was created successfully
			fmt.Printf("Warning: failed to update page blocks: %v\n", err)
		}
	}

	return page.ID, nil
}

func (e *engine) updateNotionPage(ctx context.Context, pageID, title string, blocks []map[string]interface{}) error {
	// Use the original slower but safer method for updates to preserve page IDs
	// The delete-and-recreate approach would change page IDs and break links
	return e.notion.UpdatePageBlocks(ctx, pageID, blocks)
}

func (e *engine) getTitleFromFilename(filePath string) string {
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func (e *engine) extractTitleFromPage(page *notion.Page) string {
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

func (e *engine) isExcluded(path string) bool {
	for _, pattern := range e.config.Directories.ExcludedPatterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}