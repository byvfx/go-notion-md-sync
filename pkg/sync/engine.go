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
	"github.com/byvfx/go-notion-md-sync/pkg/util"
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
	workerCount      int // Configurable worker count
}

func NewEngine(cfg *config.Config) Engine {
	// Create the appropriate client based on configuration
	var client notion.Client
	if cfg.Performance.UseMultiClient {
		// Use multi-client approach for maximum throughput
		client = notion.NewBatchClient(cfg.Notion.Token, cfg.Performance.ClientCount)
	} else {
		// Use standard client (proven best performance)
		client = notion.NewClient(cfg.Notion.Token)
	}

	return &engine{
		config:           cfg,
		notion:           client,
		parser:           markdown.NewParser(),
		converter:        NewConverter(),
		conflictResolver: NewConflictResolver(cfg.Sync.ConflictResolution),
		workerCount:      cfg.Performance.Workers, // Use configured worker count
	}
}

// NewEngineWithWorkers creates an engine with a specific worker count
func NewEngineWithWorkers(cfg *config.Config, workers int) Engine {
	return &engine{
		config:           cfg,
		notion:           notion.NewClient(cfg.Notion.Token),
		parser:           markdown.NewParser(),
		converter:        NewConverter(),
		conflictResolver: NewConflictResolver(cfg.Sync.ConflictResolution),
		workerCount:      workers,
	}
}

// NewEngineWithClient creates an engine with a custom client
func NewEngineWithClient(cfg *config.Config, client notion.Client) Engine {
	return &engine{
		config:           cfg,
		notion:           client,
		parser:           markdown.NewParser(),
		converter:        NewConverter(),
		conflictResolver: NewConflictResolver(cfg.Sync.ConflictResolution),
		workerCount:      0,
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

	// Check for child databases and export them
	databaseRefs, err := e.exportChildDatabases(ctx, pageID, filePath, title)
	if err != nil {
		// Log warning but don't fail the page sync
		util.WithError(err, "Failed to export databases for page %s", pageID)
	}

	// Convert blocks to markdown
	content, err := e.converter.BlocksToMarkdown(blocks)
	if err != nil {
		return fmt.Errorf("failed to convert blocks to markdown: %w", err)
	}

	// Add database references to content if any databases were exported
	if len(databaseRefs) > 0 {
		content = e.addDatabaseReferences(content, databaseRefs)
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
	// Check if we should use streaming for large workspaces
	if e.shouldUseStreaming(ctx) {
		return e.syncAllNotionToMarkdownStreaming(ctx)
	}

	// Use original implementation for smaller workspaces
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

	// Use concurrent processing for better performance
	return e.syncPagesConcurrently(ctx, pages, pageParentMap)
}

// syncPagesConcurrently processes multiple pages concurrently using simple goroutines
func (e *engine) syncPagesConcurrently(ctx context.Context, pages []notion.Page, pageParentMap map[string]string) error {
	// Configure concurrency based on page count or custom setting
	workerCount := e.workerCount
	if workerCount == 0 {
		// Auto-detect based on page count - optimized based on performance testing
		workerCount = 30 // Optimal performance found with 30 workers
		if len(pages) < 5 {
			workerCount = len(pages) // Use fewer workers for very small batches
		} else if len(pages) < 15 {
			workerCount = 20 // Good performance for medium batches
		}
		// For 15+ pages, use 30 workers (proven optimal)
	}

	// Cap at 50 workers to avoid overwhelming the API
	if workerCount > 50 {
		workerCount = 50
	}

	fmt.Printf("🚀 Using concurrent processing with %d workers for %d pages\n", workerCount, len(pages))

	// Create channels for work distribution
	pageJobs := make(chan pageJob, len(pages))
	results := make(chan syncResult, len(pages))

	// Start workers
	for i := 0; i < workerCount; i++ {
		go e.syncWorker(ctx, pageJobs, results)
	}

	// Send jobs to workers
	for i, page := range pages {
		title := e.extractTitleFromPage(&page)
		filePath := e.buildFilePathForPage(&page, title, pageParentMap, pages)

		pageJobs <- pageJob{
			page:     page,
			title:    title,
			filePath: filePath,
			index:    i + 1,
			total:    len(pages),
		}
	}
	close(pageJobs)

	// Collect results
	var errors []string
	successCount := 0
	for i := 0; i < len(pages); i++ {
		result := <-results
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("Page %s: %v", result.pageID, result.err))
		} else {
			successCount++
		}
	}

	fmt.Printf("\n🎉 Concurrent sync complete! %d/%d pages successful\n", successCount, len(pages))

	if len(errors) > 0 {
		util.ErrorMsg("%d pages failed", len(errors))
		for _, errMsg := range errors {
			util.Error("  - %s", errMsg)
		}
		return fmt.Errorf("%d pages failed to sync", len(errors))
	}

	return nil
}

// pageJob represents a page sync job
type pageJob struct {
	page     notion.Page
	title    string
	filePath string
	index    int
	total    int
}

// syncResult represents the result of a sync operation
type syncResult struct {
	pageID string
	err    error
}

// syncWorker processes page sync jobs concurrently
func (e *engine) syncWorker(ctx context.Context, jobs <-chan pageJob, results chan<- syncResult) {
	for job := range jobs {
		result := syncResult{pageID: job.page.ID}

		// Print progress
		fmt.Printf("[%d/%d] Pulling page: %s\n", job.index, job.total, job.title)
		fmt.Printf("  Notion ID: %s\n", job.page.ID)
		fmt.Printf("  Saving to: %s\n", job.filePath)

		// Create parent directory if needed
		dir := filepath.Dir(job.filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			result.err = fmt.Errorf("failed to create directory %s: %w", dir, err)
			results <- result
			continue
		}

		// Sync the page
		if err := e.SyncNotionToFile(ctx, job.page.ID, job.filePath); err != nil {
			result.err = fmt.Errorf("failed to sync page %s: %w", job.page.ID, err)
		} else {
			fmt.Printf("  ✓ Successfully pulled %s\n", job.title)
		}

		results <- result
	}
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
			util.Warning("Cycle detected in page hierarchy for page %s", currentPageID)
			break
		}
		visited[currentPageID] = true

		// Find the parent page to get its title
		parentFound := false
		for _, p := range allPages {
			if p.ID == parentID {
				parentTitle := e.extractTitleFromPage(&p)
				// Sanitize the parent title to prevent path traversal
				parentTitle = util.SanitizeFileName(parentTitle)
				// Add parent title as directory
				pathParts = append([]string{parentTitle}, pathParts...)
				currentPageID = parentID
				parentFound = true
				break
			}
		}

		// If we couldn't find the parent page, break to avoid infinite loop
		if !parentFound {
			util.Warning("Parent page %s not found in page list", parentID)
			break
		}
	}

	// Add the parent page directory at the beginning
	for _, p := range allPages {
		if p.ID == e.config.Notion.ParentPageID {
			parentTitle := e.extractTitleFromPage(&p)
			// Sanitize the parent title
			parentTitle = util.SanitizeFileName(parentTitle)
			pathParts = append([]string{parentTitle}, pathParts...)
			break
		}
	}

	// Sanitize the current page title
	safeTitle := util.SanitizeFileName(title)

	// Add the current page as a directory and then the filename
	pathParts = append(pathParts, safeTitle)
	pathParts = append(pathParts, safeTitle+".md")

	// Construct the full path securely
	fullPath, err := util.SecureJoin(e.config.Directories.MarkdownRoot, pathParts...)
	if err != nil {
		// If path traversal is detected, fall back to a safe default
		util.Warning("Potential path traversal detected for page %s, using safe path", page.ID)
		fullPath = filepath.Join(e.config.Directories.MarkdownRoot, safeTitle, safeTitle+".md")
	}
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

// DatabaseReference represents a reference to an exported database
type DatabaseReference struct {
	DatabaseID string
	Title      string
	CSVPath    string
}

// exportChildDatabases finds and exports all child databases of a page
func (e *engine) exportChildDatabases(ctx context.Context, pageID, filePath, pageTitle string) ([]DatabaseReference, error) {
	var databaseRefs []DatabaseReference

	// Get child databases - we need to check if the Notion API provides child database blocks
	// For now, we'll look for database blocks in the page content
	blocks, err := e.notion.GetPageBlocks(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get page blocks: %w", err)
	}

	databaseCount := 0

	// Check blocks for child_database type
	for _, block := range blocks {
		if block.Type == "child_database" {
			databaseCount++

			// Debug: print block structure
			fmt.Printf("  Debug: Found child_database block, ID: %s\n", block.ID)

			// Try to extract database ID - it might be the block ID itself
			databaseID := block.ID

			// Also check if we have the ChildDatabase field populated
			if block.ChildDatabase != nil && block.ChildDatabase.DatabaseID != "" {
				databaseID = block.ChildDatabase.DatabaseID
			}

			if databaseID == "" {
				fmt.Printf("  Warning: Found child_database block but couldn't extract database ID\n")
				continue
			}

			// Get database title and create better CSV filename
			baseDir := filepath.Dir(filePath)
			dbTitle := fmt.Sprintf("Database %d", databaseCount)
			var csvFileName string

			if database, err := e.notion.GetDatabase(ctx, databaseID); err == nil {
				if len(database.Title) > 0 && database.Title[0].PlainText != "" {
					dbTitle = database.Title[0].PlainText
					// Use the database title as the CSV filename
					sanitizedDbTitle := e.sanitizeFilename(dbTitle)
					csvFileName = fmt.Sprintf("%s.csv", sanitizedDbTitle)
				} else {
					// Fallback to page name with counter if database has no title
					sanitizedTitle := e.sanitizeFilename(pageTitle)
					csvFileName = fmt.Sprintf("%s_db%d.csv", sanitizedTitle, databaseCount)
				}
			} else {
				// Fallback if we can't get database info
				fmt.Printf("  Warning: Could not get database info for %s: %v\n", databaseID, err)
				sanitizedTitle := e.sanitizeFilename(pageTitle)
				csvFileName = fmt.Sprintf("%s_db%d.csv", sanitizedTitle, databaseCount)
			}

			csvPath := filepath.Join(baseDir, csvFileName)

			// Export database to CSV
			fmt.Printf("  Exporting database '%s' to: %s\n", dbTitle, csvFileName)

			// Create database sync instance and export
			dbSync := NewDatabaseSync(e.notion)
			if err := dbSync.SyncNotionDatabaseToCSV(ctx, databaseID, csvPath); err != nil {
				fmt.Printf("  Warning: Failed to export database %s: %v\n", databaseID, err)
				continue
			}

			databaseRefs = append(databaseRefs, DatabaseReference{
				DatabaseID: databaseID,
				Title:      dbTitle,
				CSVPath:    csvFileName, // Store relative path for markdown reference
			})
		}
	}

	if len(databaseRefs) > 0 {
		fmt.Printf("  Exported %d database(s)\n", len(databaseRefs))
	}

	return databaseRefs, nil
}

// addDatabaseReferences adds database references to the markdown content
func (e *engine) addDatabaseReferences(content string, databaseRefs []DatabaseReference) string {
	if len(databaseRefs) == 0 {
		return content
	}

	// Add database references at the end of the content
	content += "\n\n## Databases\n\n"

	for _, ref := range databaseRefs {
		content += fmt.Sprintf("- [%s](./%s)\n", ref.Title, ref.CSVPath)
	}

	return content
}

// sanitizeFilename removes characters that are invalid in filenames
func (e *engine) sanitizeFilename(filename string) string {
	// Replace common problematic characters
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, ":", "_")
	filename = strings.ReplaceAll(filename, "*", "_")
	filename = strings.ReplaceAll(filename, "?", "_")
	filename = strings.ReplaceAll(filename, "\"", "_")
	filename = strings.ReplaceAll(filename, "<", "_")
	filename = strings.ReplaceAll(filename, ">", "_")
	filename = strings.ReplaceAll(filename, "|", "_")
	filename = strings.ReplaceAll(filename, " ", "_")

	// Remove any leading/trailing dots or spaces
	filename = strings.Trim(filename, ". ")

	// Ensure it's not empty
	if filename == "" {
		filename = "untitled"
	}

	return filename
}

// shouldUseStreaming determines if we should use streaming based on workspace size
func (e *engine) shouldUseStreaming(ctx context.Context) bool {
	// Quick count of direct children to estimate workspace size
	directChildren, err := e.notion.GetChildPages(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		// If we can't count, err on the side of caution and use streaming
		return true
	}

	// Use streaming if there are more than 100 direct children
	// This is a heuristic - large workspaces often have many top-level pages
	return len(directChildren) > 100
}

// syncAllNotionToMarkdownStreaming uses streaming to handle large workspaces
func (e *engine) syncAllNotionToMarkdownStreaming(ctx context.Context) error {
	fmt.Println("🌊 Using streaming mode for large workspace")

	// Get parent page first
	parentPage, err := e.notion.GetPage(ctx, e.config.Notion.ParentPageID)
	if err != nil {
		return fmt.Errorf("failed to get parent page: %w", err)
	}

	// Process parent page first
	parentTitle := e.extractTitleFromPage(parentPage)
	parentPath := e.buildFilePathForPageStreaming(*parentPage, parentTitle)

	util.Progress("Processing parent page: %s", parentTitle)
	if err := e.syncNotionPageToFile(ctx, *parentPage, parentPath); err != nil {
		util.WithError(err, "Failed to sync parent page")
	}

	processedCount := 0
	errorCount := 0

	// Stream and process descendant pages
	stream := e.notion.StreamDescendantPages(ctx, e.config.Notion.ParentPageID)

	for {
		select {
		case page, ok := <-stream.Pages():
			if !ok {
				fmt.Printf("\n🎉 Streaming sync complete! %d/%d pages successful\n", processedCount-errorCount, processedCount+1) // +1 for parent
				return nil
			}

			processedCount++
			title := e.extractTitleFromPage(&page)
			filePath := e.buildFilePathForPageStreaming(page, title)

			util.Progress("[%d] Processing page: %s", processedCount, title)

			if err := e.syncNotionPageToFile(ctx, page, filePath); err != nil {
				errorCount++
				util.ErrorMsg("Error: %v", err)
			} else {
				util.Success("Successfully synced: %s", title)
			}

			// Progress indicator for large operations
			if processedCount%50 == 0 {
				util.Progress("\n--- Progress: %d pages processed ---", processedCount)
			}

		case err := <-stream.Errors():
			errorCount++
			util.Warning("Streaming error: %v", err)

		case <-ctx.Done():
			return fmt.Errorf("sync cancelled: %w", ctx.Err())
		}
	}
}

// buildFilePathForPageStreaming builds file path without needing all pages in memory
func (e *engine) buildFilePathForPageStreaming(page notion.Page, title string) string {
	// For streaming, we use a simpler path construction
	// This avoids needing to keep all pages in memory to build the hierarchy
	safeTitle := util.SanitizeFileName(title)

	// Create a simple path: markdown_root/page_title/page_title.md
	fullPath, err := util.SecureJoin(e.config.Directories.MarkdownRoot, safeTitle, safeTitle+".md")
	if err != nil {
		// Fallback to safe path
		util.Warning("Path construction failed for %s, using fallback", page.ID)
		fullPath = filepath.Join(e.config.Directories.MarkdownRoot, safeTitle, safeTitle+".md")
	}

	return fullPath
}

// syncNotionPageToFile syncs a single Notion page to a markdown file
func (e *engine) syncNotionPageToFile(ctx context.Context, page notion.Page, filePath string) error {
	// Get page blocks
	blocks, err := e.notion.GetPageBlocks(ctx, page.ID)
	if err != nil {
		return fmt.Errorf("failed to get page blocks: %w", err)
	}

	// Convert to markdown
	converter := NewConverter()
	markdown, err := converter.BlocksToMarkdown(blocks)
	if err != nil {
		return fmt.Errorf("failed to convert blocks to markdown: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, []byte(markdown), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
