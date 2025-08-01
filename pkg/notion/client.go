package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	BaseURL        = "https://api.notion.com/v1"
	NotionVersion  = "2022-06-28"
	DefaultTimeout = 30 * time.Second
)

type Client interface {
	GetPage(ctx context.Context, pageID string) (*Page, error)
	GetPageBlocks(ctx context.Context, pageID string) ([]Block, error)
	CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*Page, error)
	UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error
	DeletePage(ctx context.Context, pageID string) error
	RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*Page, error)
	SearchPages(ctx context.Context, query string) ([]Page, error)
	GetChildPages(ctx context.Context, parentID string) ([]Page, error)
	GetAllDescendantPages(ctx context.Context, parentID string) ([]Page, error)

	// Streaming methods for large operations
	StreamDescendantPages(ctx context.Context, parentID string) *PageStream
	StreamDatabaseRows(ctx context.Context, databaseID string) *DatabaseRowStream

	// Database methods
	GetDatabase(ctx context.Context, databaseID string) (*Database, error)
	QueryDatabase(ctx context.Context, databaseID string, request *DatabaseQueryRequest) (*DatabaseQueryResponse, error)
	CreateDatabase(ctx context.Context, request *CreateDatabaseRequest) (*Database, error)
	CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]PropertyValue) (*DatabaseRow, error)
	UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]PropertyValue) (*DatabaseRow, error)
}

type client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

type NotionAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	PageID  string `json:"-"`
}

func (e *NotionAPIError) Error() string {
	if e.PageID != "" {
		return fmt.Sprintf("notion api error %d: %s (page: %s)", e.Code, e.Message, e.PageID)
	}
	return fmt.Sprintf("notion api error %d: %s", e.Code, e.Message)
}

func NewClient(token string) Client {
	return &client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		token:   token,
		baseURL: BaseURL,
	}
}

func (c *client) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", NotionVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				// Log error but don't fail the operation
				fmt.Printf("Warning: failed to close response body: %v\n", err)
			}
		}()
		var apiErr NotionAPIError
		bodyBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(bodyBytes, &apiErr); err != nil {
			return nil, fmt.Errorf("http error %d: %s", resp.StatusCode, string(bodyBytes))
		}
		apiErr.Code = resp.StatusCode
		return nil, &apiErr
	}

	return resp, nil
}

func (c *client) GetPage(ctx context.Context, pageID string) (*Page, error) {
	resp, err := c.doRequest(ctx, "GET", "/pages/"+pageID, nil)
	if err != nil {
		if apiErr, ok := err.(*NotionAPIError); ok {
			apiErr.PageID = pageID
		}
		return nil, fmt.Errorf("failed to get page %s: %w", pageID, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var page Page
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("failed to decode page response: %w", err)
	}

	return &page, nil
}

func (c *client) GetPageBlocks(ctx context.Context, pageID string) ([]Block, error) {
	blocks, err := c.getBlocksRecursive(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocks for page %s: %w", pageID, err)
	}
	return blocks, nil
}

func (c *client) getBlocksRecursive(ctx context.Context, blockID string) ([]Block, error) {
	resp, err := c.doRequest(ctx, "GET", "/blocks/"+blockID+"/children", nil)
	if err != nil {
		if apiErr, ok := err.(*NotionAPIError); ok {
			apiErr.PageID = blockID
		}
		return nil, fmt.Errorf("failed to get blocks for %s: %w", blockID, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var blocksResp BlocksResponse
	if err := json.NewDecoder(resp.Body).Decode(&blocksResp); err != nil {
		return nil, fmt.Errorf("failed to decode blocks response: %w", err)
	}

	var allBlocks []Block
	for _, block := range blocksResp.Results {
		allBlocks = append(allBlocks, block)

		// If this block has children, fetch them recursively
		if block.HasChildren {
			childBlocks, err := c.getBlocksRecursive(ctx, block.ID)
			if err != nil {
				// Log the error but continue - don't fail the entire operation
				fmt.Printf("Warning: failed to get child blocks for %s: %v\n", block.ID, err)
				continue
			}
			allBlocks = append(allBlocks, childBlocks...)
		}
	}

	return allBlocks, nil
}

func (c *client) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*Page, error) {
	createReq := CreatePageRequest{
		Parent: Parent{
			Type:   "page_id",
			PageID: parentID,
		},
		Properties: properties,
	}

	resp, err := c.doRequest(ctx, "POST", "/pages", createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var page Page
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("failed to decode create page response: %w", err)
	}

	return &page, nil
}

func (c *client) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	// Clear existing blocks first using sequential deletion for reliability
	if err := c.clearPageBlocks(ctx, pageID); err != nil {
		return fmt.Errorf("failed to clear existing blocks: %w", err)
	}

	// Wait a bit for Notion to process deletions
	time.Sleep(200 * time.Millisecond)

	// Add new blocks in chunks
	const maxBlocksPerRequest = 100

	for i := 0; i < len(blocks); i += maxBlocksPerRequest {
		end := i + maxBlocksPerRequest
		if end > len(blocks) {
			end = len(blocks)
		}

		chunk := blocks[i:end]

		updateReq := map[string]interface{}{
			"children": chunk,
		}

		resp, err := c.doRequest(ctx, "PATCH", "/blocks/"+pageID+"/children", updateReq)
		if err != nil {
			if apiErr, ok := err.(*NotionAPIError); ok {
				apiErr.PageID = pageID
			}
			return fmt.Errorf("failed to update blocks for page %s (chunk %d-%d): %w", pageID, i+1, end, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fmt.Printf("Warning: failed to close response body: %v\n", err)
			}
		}()

		// Small delay between chunks to avoid rate limiting
		if end < len(blocks) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

func (c *client) clearPageBlocks(ctx context.Context, pageID string) error {
	// Get existing blocks
	existingBlocks, err := c.GetPageBlocks(ctx, pageID)
	if err != nil {
		return fmt.Errorf("failed to get existing blocks: %w", err)
	}

	// Delete existing blocks sequentially for reliability
	for _, block := range existingBlocks {
		_, err := c.doRequest(ctx, "DELETE", "/blocks/"+block.ID, nil)
		if err != nil {
			// Log warning but continue - some blocks might not be deletable
			fmt.Printf("Warning: failed to delete block %s: %v\n", block.ID, err)
			continue
		}

		// Small delay to avoid rate limiting
		time.Sleep(50 * time.Millisecond)
	}

	return nil
}

func (c *client) DeletePage(ctx context.Context, pageID string) error {
	// Archive the page (Notion doesn't allow true deletion)
	updateReq := map[string]interface{}{
		"archived": true,
	}

	resp, err := c.doRequest(ctx, "PATCH", "/pages/"+pageID, updateReq)
	if err != nil {
		if apiErr, ok := err.(*NotionAPIError); ok {
			apiErr.PageID = pageID
		}
		return fmt.Errorf("failed to delete page %s: %w", pageID, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	return nil
}

func (c *client) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*Page, error) {
	// Create the page with initial content
	createReq := map[string]interface{}{
		"parent": map[string]interface{}{
			"type":    "page_id",
			"page_id": parentID,
		},
		"properties": properties,
		"children":   blocks,
	}

	resp, err := c.doRequest(ctx, "POST", "/pages", createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to recreate page: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var page Page
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("failed to decode recreated page response: %w", err)
	}

	return &page, nil
}

func (c *client) SearchPages(ctx context.Context, query string) ([]Page, error) {
	searchReq := SearchRequest{
		Query: query,
		Filter: Filter{
			Value:    "page",
			Property: "object",
		},
	}

	resp, err := c.doRequest(ctx, "POST", "/search", searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search pages: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return searchResp.Results, nil
}

func (c *client) GetChildPages(ctx context.Context, parentID string) ([]Page, error) {
	resp, err := c.doRequest(ctx, "GET", "/blocks/"+parentID+"/children", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get child pages: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var blocksResp BlocksResponse
	if err := json.NewDecoder(resp.Body).Decode(&blocksResp); err != nil {
		return nil, fmt.Errorf("failed to decode child pages response: %w", err)
	}

	var pages []Page
	for _, block := range blocksResp.Results {
		if block.Type == "child_page" {
			page, err := c.GetPage(ctx, block.ID)
			if err != nil {
				continue
			}
			pages = append(pages, *page)
		}
	}

	return pages, nil
}

func (c *client) GetAllDescendantPages(ctx context.Context, parentID string) ([]Page, error) {
	var allPages []Page

	// Get direct children first
	directChildren, err := c.GetChildPages(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get child pages: %w", err)
	}

	// Add direct children to results
	allPages = append(allPages, directChildren...)

	// Recursively get children of each child page
	for _, page := range directChildren {
		descendants, err := c.GetAllDescendantPages(ctx, page.ID)
		if err != nil {
			// Log error but continue with other pages
			fmt.Printf("Warning: failed to get descendants of page %s: %v\n", page.ID, err)
			continue
		}
		allPages = append(allPages, descendants...)
	}

	return allPages, nil
}

// Database methods

func (c *client) GetDatabase(ctx context.Context, databaseID string) (*Database, error) {
	endpoint := fmt.Sprintf("/databases/%s", databaseID)
	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var database Database
	if err := json.NewDecoder(resp.Body).Decode(&database); err != nil {
		return nil, fmt.Errorf("failed to decode database response: %w", err)
	}

	return &database, nil
}

func (c *client) QueryDatabase(ctx context.Context, databaseID string, request *DatabaseQueryRequest) (*DatabaseQueryResponse, error) {
	endpoint := fmt.Sprintf("/databases/%s/query", databaseID)
	resp, err := c.doRequest(ctx, "POST", endpoint, request)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var queryResp DatabaseQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode query response: %w", err)
	}

	return &queryResp, nil
}

func (c *client) CreateDatabase(ctx context.Context, request *CreateDatabaseRequest) (*Database, error) {
	endpoint := "/databases"
	resp, err := c.doRequest(ctx, "POST", endpoint, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var database Database
	if err := json.NewDecoder(resp.Body).Decode(&database); err != nil {
		return nil, fmt.Errorf("failed to decode create database response: %w", err)
	}

	return &database, nil
}

func (c *client) CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]PropertyValue) (*DatabaseRow, error) {
	endpoint := "/pages"

	payload := map[string]interface{}{
		"parent": map[string]interface{}{
			"database_id": databaseID,
		},
		"properties": properties,
	}

	resp, err := c.doRequest(ctx, "POST", endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create database row: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var row DatabaseRow
	if err := json.NewDecoder(resp.Body).Decode(&row); err != nil {
		return nil, fmt.Errorf("failed to decode create row response: %w", err)
	}

	return &row, nil
}

func (c *client) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]PropertyValue) (*DatabaseRow, error) {
	endpoint := fmt.Sprintf("/pages/%s", pageID)

	payload := map[string]interface{}{
		"properties": properties,
	}

	resp, err := c.doRequest(ctx, "PATCH", endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to update database row: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	var row DatabaseRow
	if err := json.NewDecoder(resp.Body).Decode(&row); err != nil {
		return nil, fmt.Errorf("failed to decode update row response: %w", err)
	}

	return &row, nil
}
