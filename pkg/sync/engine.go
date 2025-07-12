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
	SyncSpecificFile(ctx context.Context, filename, direction string) error
}

type engine struct {
	config           *config.Config
	notion           notion.Client
	parser           markdown.Parser
	converter        Converter
	conflictResolver *ConflictResolver
}

func NewEngine(cfg *config.Config) Engine {
	return &engine{
		config:           cfg,
		notion:           notion.NewClient(cfg.Notion.Token),
		parser:           markdown.NewParser(),
		converter:        NewConverter(),
		conflictResolver: NewConflictResolver(cfg.Sync.ConflictResolution),
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

	// Extract and display page title
	title := e.extractTitleFromPage(page)
	fmt.Printf("  Page title: %s\n", title)

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
		Title:       title,
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
	// Get the parent page itself first
	parentPage, err := e.notion.GetPage(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		return fmt.Errorf("failed to get parent page: %w", err)
	}

	// Get all descendant pages (including nested sub-pages)
	descendantPages, err := e.notion.GetAllDescendantPages(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		return fmt.Errorf("failed to get descendant pages: %w", err)
	}

	// Combine parent page with descendants
	pages := append([]notion.Page{*parentPage}, descendantPages...)

	fmt.Printf("Found %d pages under parent %s (including parent and sub-pages)\n", len(pages), e.config.Notion.ParentPageID)
	fmt.Println()

	// Build a map of page IDs to their parent IDs for path construction
	pageParentMap := make(map[string]string)
	for _, page := range pages {
		if page.Parent.Type == "page_id" {
			pageParentMap[page.ID] = page.Parent.PageID
		}
	}

	for i, page := range pages {
		title := e.extractTitleFromPage(&page)

		// Build the file path including parent directories
		filePath := e.buildFilePathForPage(&page, title, pageParentMap, pages)

		fmt.Printf("[%d/%d] Pulling page: %s\n", i+1, len(pages), title)
		fmt.Printf("  Notion ID: %s\n", page.ID)
		fmt.Printf("  Saving to: %s\n", filePath)

		// Create parent directory if needed
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		if err := e.SyncNotionToFile(ctx, page.ID, filePath); err != nil {
			return fmt.Errorf("failed to sync page %s: %w", page.ID, err)
		}

		fmt.Printf("  ✓ Successfully pulled\n\n")
	}

	return nil
}

func (e *engine) syncBidirectional(ctx context.Context) error {
	// Get all descendant pages from Notion (including sub-pages)
	pages, err := e.notion.GetAllDescendantPages(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		return fmt.Errorf("failed to get descendant pages: %w", err)
	}

	// Check each file for conflicts
	err = filepath.Walk(e.config.Directories.MarkdownRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".md") || e.isExcluded(path) {
			return nil
		}

		return e.syncFileWithConflictDetection(ctx, path, pages)
	})
	if err != nil {
		return fmt.Errorf("failed to sync markdown files: %w", err)
	}

	// Sync any Notion pages that don't have corresponding markdown files
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

func (e *engine) syncFileWithConflictDetection(ctx context.Context, filePath string, notionPages []notion.Page) error {
	// Parse local markdown file
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

	// If no NotionID, push to Notion (new file)
	if frontmatter.NotionID == "" {
		return e.SyncFileToNotion(ctx, filePath)
	}

	// Find corresponding Notion page
	var notionPage *notion.Page
	for _, page := range notionPages {
		if page.ID == frontmatter.NotionID {
			notionPage = &page
			break
		}
	}

	if notionPage == nil {
		// Page doesn't exist in Notion anymore, create new one
		return e.SyncFileToNotion(ctx, filePath)
	}

	// Get remote content from Notion
	blocks, err := e.notion.GetPageBlocks(ctx, frontmatter.NotionID)
	if err != nil {
		return fmt.Errorf("failed to get page blocks: %w", err)
	}

	remoteContent, err := e.converter.BlocksToMarkdown(blocks)
	if err != nil {
		return fmt.Errorf("failed to convert blocks to markdown: %w", err)
	}

	// Check for conflicts
	if HasConflict(doc.Content, remoteContent) {
		// Resolve conflict
		resolvedContent, err := e.conflictResolver.ResolveConflict(doc.Content, remoteContent, filePath)
		if err != nil {
			// User chose to skip or there was an error
			fmt.Printf("Skipping file %s: %v\n", filePath, err)
			return nil
		}

		// Determine which direction to sync based on resolved content
		if resolvedContent == doc.Content {
			// Local version chosen, push to Notion
			return e.SyncFileToNotion(ctx, filePath)
		} else {
			// Remote version chosen, pull from Notion
			return e.SyncNotionToFile(ctx, frontmatter.NotionID, filePath)
		}
	} else {
		// No conflict, sync normally (push local to Notion)
		return e.SyncFileToNotion(ctx, filePath)
	}
}

// Helper functions

// buildFilePathForPage constructs the file path for a page, including nested directory structure
func (e *engine) buildFilePathForPage(page *notion.Page, title string, pageParentMap map[string]string, allPages []notion.Page) string {
	// Special handling for the parent page itself
	if page.ID == e.config.Notion.ParentPageID {
		// Parent page gets its own directory with its markdown file inside
		return filepath.Join(e.config.Directories.MarkdownRoot, title, title+".md")
	}

	// Build the path from root to this page
	var pathParts []string

	// Traverse up the parent chain
	currentPageID := page.ID
	visited := make(map[string]bool) // Track visited pages to prevent infinite loops

	for {
		parentID, hasParent := pageParentMap[currentPageID]
		if !hasParent || parentID == e.config.Notion.ParentPageID {
			// Reached the root parent or no parent found
			break
		}

		// Safety check to prevent infinite loops
		if visited[currentPageID] {
			fmt.Printf("Warning: cycle detected in page hierarchy for page %s\n", currentPageID)
			break
		}
		visited[currentPageID] = true

		// Find the parent page to get its title
		parentFound := false
		for _, p := range allPages {
			if p.ID == parentID {
				parentTitle := e.extractTitleFromPage(&p)
				// Add parent title as directory
				pathParts = append([]string{parentTitle}, pathParts...)
				currentPageID = parentID
				parentFound = true
				break
			}
		}

		// If we couldn't find the parent page, break to avoid infinite loop
		if !parentFound {
			fmt.Printf("Warning: parent page %s not found in page list\n", parentID)
			break
		}
	}

	// Add the parent page directory at the beginning
	for _, p := range allPages {
		if p.ID == e.config.Notion.ParentPageID {
			parentTitle := e.extractTitleFromPage(&p)
			pathParts = append([]string{parentTitle}, pathParts...)
			break
		}
	}

	// Add the current page as a directory and then the filename
	pathParts = append(pathParts, title)
	pathParts = append(pathParts, title+".md")

	// Construct the full path
	fullPath := filepath.Join(e.config.Directories.MarkdownRoot, filepath.Join(pathParts...))
	return fullPath
}

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

func (e *engine) SyncSpecificFile(ctx context.Context, filename, direction string) error {
	switch direction {
	case "pull":
		return e.syncSpecificNotionToMarkdown(ctx, filename)
	case "push":
		return e.SyncFileToNotion(ctx, filename)
	default:
		return fmt.Errorf("unsupported direction: %s", direction)
	}
}

func (e *engine) syncSpecificNotionToMarkdown(ctx context.Context, filename string) error {
	// Get all child pages from Notion
	pages, err := e.notion.GetChildPages(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		return fmt.Errorf("failed to get child pages: %w", err)
	}

	// Find the page that matches the filename
	var targetPage *notion.Page
	targetTitle := strings.TrimSuffix(filepath.Base(filename), ".md")

	for _, page := range pages {
		pageTitle := e.extractTitleFromPage(&page)
		if pageTitle == targetTitle {
			targetPage = &page
			break
		}
	}

	if targetPage == nil {
		return fmt.Errorf("page with title '%s' not found in Notion", targetTitle)
	}

	// Create the file path
	filePath := filepath.Join(e.config.Directories.MarkdownRoot, filename)

	// Sync this specific page
	fmt.Printf("Pulling page: %s\n", e.extractTitleFromPage(targetPage))
	fmt.Printf("  Notion ID: %s\n", targetPage.ID)
	fmt.Printf("  Saving to: %s\n", filePath)

	if err := e.SyncNotionToFile(ctx, targetPage.ID, filePath); err != nil {
		return fmt.Errorf("failed to sync page %s: %w", targetPage.ID, err)
	}

	return nil
}
