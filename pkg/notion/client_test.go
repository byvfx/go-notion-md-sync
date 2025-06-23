package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock HTTP server for testing
type mockServer struct {
	*httptest.Server
	requests []recordedRequest
}

type recordedRequest struct {
	Method  string
	Path    string
	Body    string
	Headers http.Header
}

func newMockServer(t *testing.T, handler http.HandlerFunc) *mockServer {
	ms := &mockServer{
		requests: []recordedRequest{},
	}

	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body and store it
		body, _ := io.ReadAll(r.Body)
		ms.requests = append(ms.requests, recordedRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Body:    string(body),
			Headers: r.Header,
		})
		// Reset the body for the handler
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		handler(w, r)
	}))

	return ms
}

func TestNewClient(t *testing.T) {
	token := "test-token"
	c := NewClient(token)

	assert.NotNil(t, c)
	// Since client is an interface, we can't directly access internal fields
	// The test verifies that NewClient returns a non-nil implementation
}

func TestClient_GetPage(t *testing.T) {
	tests := []struct {
		name         string
		pageID       string
		serverStatus int
		serverResp   interface{}
		wantErr      bool
		wantErrMsg   string
		checkPage    func(t *testing.T, page *Page)
	}{
		{
			name:         "successful get page",
			pageID:       "test-page-id",
			serverStatus: http.StatusOK,
			serverResp: Page{
				ID:          "test-page-id",
				Object:      "page",
				CreatedTime: time.Now(),
				URL:         "https://notion.so/test-page",
				Properties: map[string]interface{}{
					"title": map[string]interface{}{
						"title": []map[string]interface{}{
							{"text": map[string]interface{}{"content": "Test Page"}},
						},
					},
				},
			},
			wantErr: false,
			checkPage: func(t *testing.T, page *Page) {
				assert.Equal(t, "test-page-id", page.ID)
				assert.Equal(t, "page", page.Object)
				assert.Equal(t, "https://notion.so/test-page", page.URL)
			},
		},
		{
			name:         "page not found",
			pageID:       "non-existent-page",
			serverStatus: http.StatusNotFound,
			serverResp: NotionAPIError{
				Code:    http.StatusNotFound,
				Message: "Page not found",
			},
			wantErr:    true,
			wantErrMsg: "failed to get page non-existent-page: notion api error 404: Page not found (page: non-existent-page)",
		},
		{
			name:         "server error",
			pageID:       "test-page-id",
			serverStatus: http.StatusInternalServerError,
			serverResp: NotionAPIError{
				Code:    http.StatusInternalServerError,
				Message: "Internal server error",
			},
			wantErr:    true,
			wantErrMsg: "failed to get page test-page-id: notion api error 500: Internal server error (page: test-page-id)",
		},
		{
			name:         "invalid json response",
			pageID:       "test-page-id",
			serverStatus: http.StatusOK,
			serverResp:   "invalid json",
			wantErr:      true,
			wantErrMsg:   "failed to decode page response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/pages/"+tt.pageID, r.URL.Path)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				assert.Equal(t, NotionVersion, r.Header.Get("Notion-Version"))

				w.WriteHeader(tt.serverStatus)
				if s, ok := tt.serverResp.(string); ok {
					w.Write([]byte(s))
				} else {
					json.NewEncoder(w).Encode(tt.serverResp)
				}
			})
			defer server.Close()

			c := &client{
				httpClient: &http.Client{Timeout: DefaultTimeout},
				token:      "test-token",
				baseURL:    server.URL,
			}

			page, err := c.GetPage(context.Background(), tt.pageID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
				assert.Nil(t, page)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, page)
				if tt.checkPage != nil {
					tt.checkPage(t, page)
				}
			}
		})
	}
}

func TestClient_GetPageBlocks(t *testing.T) {
	tests := []struct {
		name        string
		pageID      string
		serverResp  map[string]interface{}
		wantErr     bool
		wantBlocks  int
		checkBlocks func(t *testing.T, blocks []Block)
	}{
		{
			name:   "get blocks without children",
			pageID: "test-page-id",
			serverResp: map[string]interface{}{
				"results": []Block{
					{
						ID:   "block1",
						Type: "heading_1",
						Heading1: &RichTextBlock{
							RichText: []RichText{{PlainText: "Test Heading"}},
						},
					},
					{
						ID:   "block2",
						Type: "paragraph",
						Paragraph: &RichTextBlock{
							RichText: []RichText{{PlainText: "Test paragraph"}},
						},
					},
				},
			},
			wantErr:    false,
			wantBlocks: 2,
			checkBlocks: func(t *testing.T, blocks []Block) {
				assert.Equal(t, "block1", blocks[0].ID)
				assert.Equal(t, "heading_1", blocks[0].Type)
				assert.Equal(t, "block2", blocks[1].ID)
				assert.Equal(t, "paragraph", blocks[1].Type)
			},
		},
		{
			name:   "get blocks with nested children",
			pageID: "test-page-id",
			serverResp: map[string]interface{}{
				"results": []Block{
					{
						ID:          "block1",
						Type:        "bulleted_list_item",
						HasChildren: true,
						BulletedListItem: &RichTextBlock{
							RichText: []RichText{{PlainText: "Parent item"}},
						},
					},
				},
			},
			wantErr:    false,
			wantBlocks: 1, // Would be more with recursive calls
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				callCount++
				assert.Equal(t, "GET", r.Method)

				// First call is for the page blocks
				if callCount == 1 {
					assert.Equal(t, "/blocks/"+tt.pageID+"/children", r.URL.Path)
				}

				w.WriteHeader(http.StatusOK)
				if callCount == 1 {
					json.NewEncoder(w).Encode(tt.serverResp)
				} else {
					// Return empty results for child block requests
					json.NewEncoder(w).Encode(map[string]interface{}{"results": []Block{}})
				}
			})
			defer server.Close()

			c := &client{
				httpClient: &http.Client{Timeout: DefaultTimeout},
				token:      "test-token",
				baseURL:    server.URL,
			}

			blocks, err := c.GetPageBlocks(context.Background(), tt.pageID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, blocks, tt.wantBlocks)
				if tt.checkBlocks != nil {
					tt.checkBlocks(t, blocks)
				}
			}
		})
	}
}

func TestClient_CreatePage(t *testing.T) {
	tests := []struct {
		name         string
		parentID     string
		properties   map[string]interface{}
		serverStatus int
		serverResp   interface{}
		wantErr      bool
		checkPage    func(t *testing.T, page *Page)
	}{
		{
			name:     "successful page creation",
			parentID: "parent-page-id",
			properties: map[string]interface{}{
				"title": map[string]interface{}{
					"title": []map[string]interface{}{
						{"text": map[string]interface{}{"content": "New Page"}},
					},
				},
			},
			serverStatus: http.StatusOK,
			serverResp: Page{
				ID:     "new-page-id",
				Object: "page",
				URL:    "https://notion.so/new-page",
			},
			wantErr: false,
			checkPage: func(t *testing.T, page *Page) {
				assert.Equal(t, "new-page-id", page.ID)
				assert.Equal(t, "https://notion.so/new-page", page.URL)
			},
		},
		{
			name:     "creation failed",
			parentID: "parent-page-id",
			properties: map[string]interface{}{
				"title": map[string]interface{}{},
			},
			serverStatus: http.StatusBadRequest,
			serverResp: NotionAPIError{
				Code:    http.StatusBadRequest,
				Message: "Invalid properties",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/pages", r.URL.Path)

				body, _ := io.ReadAll(r.Body)
				var req CreatePageRequest
				json.Unmarshal(body, &req)
				assert.Equal(t, tt.parentID, req.Parent.PageID)
				assert.Equal(t, "page_id", req.Parent.Type)

				w.WriteHeader(tt.serverStatus)
				json.NewEncoder(w).Encode(tt.serverResp)
			})
			defer server.Close()

			c := &client{
				httpClient: &http.Client{Timeout: DefaultTimeout},
				token:      "test-token",
				baseURL:    server.URL,
			}

			page, err := c.CreatePage(context.Background(), tt.parentID, tt.properties)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, page)
				if tt.checkPage != nil {
					tt.checkPage(t, page)
				}
			}
		})
	}
}

func TestClient_UpdatePageBlocks(t *testing.T) {
	tests := []struct {
		name           string
		pageID         string
		blocks         []map[string]interface{}
		existingBlocks []Block
		wantErr        bool
	}{
		{
			name:   "successful update with clearing",
			pageID: "test-page-id",
			blocks: []map[string]interface{}{
				{
					"type": "heading_1",
					"heading_1": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{"text": map[string]interface{}{"content": "New Heading"}},
						},
					},
				},
			},
			existingBlocks: []Block{
				{ID: "old-block-1", Type: "paragraph"},
				{ID: "old-block-2", Type: "heading_2"},
			},
			wantErr: false,
		},
		{
			name:           "update with many blocks (chunking)",
			pageID:         "test-page-id",
			blocks:         make([]map[string]interface{}, 150), // More than maxBlocksPerRequest
			existingBlocks: []Block{},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			deleteCalls := 0

			server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				callCount++

				switch r.Method {
				case "GET":
					// Getting existing blocks
					assert.Equal(t, "/blocks/"+tt.pageID+"/children", r.URL.Path)
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(BlocksResponse{Results: tt.existingBlocks})

				case "DELETE":
					// Deleting existing blocks
					deleteCalls++
					w.WriteHeader(http.StatusOK)

				case "PATCH":
					// Adding new blocks
					assert.Equal(t, "/blocks/"+tt.pageID+"/children", r.URL.Path)
					body, _ := io.ReadAll(r.Body)
					var req map[string]interface{}
					json.Unmarshal(body, &req)
					if children, ok := req["children"].([]interface{}); ok {
						assert.LessOrEqual(t, len(children), 100) // Check chunking
					}
					w.WriteHeader(http.StatusOK)
				}
			})
			defer server.Close()

			c := &client{
				httpClient: &http.Client{Timeout: DefaultTimeout},
				token:      "test-token",
				baseURL:    server.URL,
			}

			// Initialize blocks for chunking test
			if tt.name == "update with many blocks (chunking)" {
				for i := range tt.blocks {
					tt.blocks[i] = map[string]interface{}{
						"type": "paragraph",
						"paragraph": map[string]interface{}{
							"rich_text": []map[string]interface{}{
								{"text": map[string]interface{}{"content": fmt.Sprintf("Block %d", i)}},
							},
						},
					}
				}
			}

			err := c.UpdatePageBlocks(context.Background(), tt.pageID, tt.blocks)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.existingBlocks), deleteCalls)
			}
		})
	}
}

func TestClient_DeletePage(t *testing.T) {
	tests := []struct {
		name         string
		pageID       string
		serverStatus int
		wantErr      bool
	}{
		{
			name:         "successful deletion (archive)",
			pageID:       "test-page-id",
			serverStatus: http.StatusOK,
			wantErr:      false,
		},
		{
			name:         "deletion failed",
			pageID:       "non-existent-page",
			serverStatus: http.StatusNotFound,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PATCH", r.Method)
				assert.Equal(t, "/pages/"+tt.pageID, r.URL.Path)

				body, _ := io.ReadAll(r.Body)
				var req map[string]interface{}
				json.Unmarshal(body, &req)
				assert.Equal(t, true, req["archived"])

				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus != http.StatusOK {
					json.NewEncoder(w).Encode(NotionAPIError{
						Code:    tt.serverStatus,
						Message: "Error",
					})
				}
			})
			defer server.Close()

			c := &client{
				httpClient: &http.Client{Timeout: DefaultTimeout},
				token:      "test-token",
				baseURL:    server.URL,
			}

			err := c.DeletePage(context.Background(), tt.pageID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_SearchPages(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		serverResp interface{}
		wantErr    bool
		wantPages  int
	}{
		{
			name:  "successful search",
			query: "test query",
			serverResp: SearchResponse{
				Results: []Page{
					{ID: "page1", Object: "page"},
					{ID: "page2", Object: "page"},
				},
			},
			wantErr:   false,
			wantPages: 2,
		},
		{
			name:       "empty results",
			query:      "no matches",
			serverResp: SearchResponse{Results: []Page{}},
			wantErr:    false,
			wantPages:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/search", r.URL.Path)

				body, _ := io.ReadAll(r.Body)
				var req SearchRequest
				json.Unmarshal(body, &req)
				assert.Equal(t, tt.query, req.Query)
				assert.Equal(t, "page", req.Filter.Value)
				assert.Equal(t, "object", req.Filter.Property)

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(tt.serverResp)
			})
			defer server.Close()

			c := &client{
				httpClient: &http.Client{Timeout: DefaultTimeout},
				token:      "test-token",
				baseURL:    server.URL,
			}

			pages, err := c.SearchPages(context.Background(), tt.query)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, pages, tt.wantPages)
			}
		})
	}
}

func TestClient_GetChildPages(t *testing.T) {
	parentID := "parent-page-id"
	childPageID := "child-page-id"

	callCount := 0
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if callCount == 1 {
			// First call: get blocks
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/blocks/"+parentID+"/children", r.URL.Path)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(BlocksResponse{
				Results: []Block{
					{ID: childPageID, Type: "child_page"},
					{ID: "other-block", Type: "paragraph"},
				},
			})
		} else {
			// Second call: get page details
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/pages/"+childPageID, r.URL.Path)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Page{
				ID:     childPageID,
				Object: "page",
				URL:    "https://notion.so/child-page",
			})
		}
	})
	defer server.Close()

	c := &client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		token:      "test-token",
		baseURL:    server.URL,
	}

	pages, err := c.GetChildPages(context.Background(), parentID)

	assert.NoError(t, err)
	assert.Len(t, pages, 1)
	assert.Equal(t, childPageID, pages[0].ID)
}

func TestClient_RecreatePageWithBlocks(t *testing.T) {
	parentID := "parent-page-id"
	properties := map[string]interface{}{
		"title": map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]interface{}{"content": "Recreated Page"}},
			},
		},
	}
	blocks := []map[string]interface{}{
		{
			"type": "paragraph",
			"paragraph": map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{"text": map[string]interface{}{"content": "Initial content"}},
				},
			},
		},
	}

	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/pages", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("Failed to unmarshal request: %v", err)
		}

		parent, ok := req["parent"].(map[string]interface{})
		if !ok {
			t.Fatal("Parent field is not a map")
		}
		assert.Equal(t, parentID, parent["page_id"])
		assert.Equal(t, "page_id", parent["type"])

		// Check that both properties and children are included
		assert.NotNil(t, req["properties"])
		assert.NotNil(t, req["children"])

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Page{
			ID:     "recreated-page-id",
			Object: "page",
			URL:    "https://notion.so/recreated-page",
		})
	})
	defer server.Close()

	c := &client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		token:      "test-token",
		baseURL:    server.URL,
	}

	page, err := c.RecreatePageWithBlocks(context.Background(), parentID, properties, blocks)

	assert.NoError(t, err)
	assert.NotNil(t, page)
	assert.Equal(t, "recreated-page-id", page.ID)
}

func TestNotionAPIError(t *testing.T) {
	tests := []struct {
		name string
		err  *NotionAPIError
		want string
	}{
		{
			name: "error with page ID",
			err: &NotionAPIError{
				Code:    404,
				Message: "Page not found",
				PageID:  "test-page-id",
			},
			want: "notion api error 404: Page not found (page: test-page-id)",
		},
		{
			name: "error without page ID",
			err: &NotionAPIError{
				Code:    400,
				Message: "Bad request",
			},
			want: "notion api error 400: Bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Page{ID: "test"})
	}))
	defer server.Close()

	c := &client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		token:      "test-token",
		baseURL:    server.URL,
	}

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetPage(ctx, "test-page-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestClient_doRequest_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		endpoint    string
		body        interface{}
		setupFunc   func() *client
		wantErr     bool
		errContains string
	}{
		{
			name:     "marshal error",
			method:   "POST",
			endpoint: "/test",
			body:     make(chan int), // Channels can't be marshaled to JSON
			setupFunc: func() *client {
				return &client{
					httpClient: &http.Client{},
					token:      "test",
					baseURL:    "http://test",
				}
			},
			wantErr:     true,
			errContains: "failed to marshal request body",
		},
		{
			name:     "invalid URL",
			method:   "GET",
			endpoint: "/test",
			body:     nil,
			setupFunc: func() *client {
				return &client{
					httpClient: &http.Client{},
					token:      "test",
					baseURL:    "http://[::1]:namedport", // Invalid URL
				}
			},
			wantErr:     true,
			errContains: "failed to create request",
		},
		{
			name:     "network error",
			method:   "GET",
			endpoint: "/test",
			body:     nil,
			setupFunc: func() *client {
				return &client{
					httpClient: &http.Client{
						Timeout: 1 * time.Millisecond,
					},
					token:   "test",
					baseURL: "http://192.0.2.0", // Non-routable IP
				}
			},
			wantErr:     true,
			errContains: "request failed",
		},
		{
			name:     "malformed error response",
			method:   "GET",
			endpoint: "/test",
			body:     nil,
			setupFunc: func() *client {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("not json"))
				}))
				return &client{
					httpClient: &http.Client{},
					token:      "test",
					baseURL:    server.URL,
				}
			},
			wantErr:     true,
			errContains: "http error 400: not json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupFunc()
			_, err := c.doRequest(context.Background(), tt.method, tt.endpoint, tt.body)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_RateLimiting(t *testing.T) {
	// Test behavior with multiple rapid requests
	requestCount := 0
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= 2 {
			// Simulate rate limit on first two requests
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(NotionAPIError{
				Code:    http.StatusTooManyRequests,
				Message: "Rate limited",
			})
		} else {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Page{ID: "test"})
		}
	})
	defer server.Close()

	c := &client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		token:      "test-token",
		baseURL:    server.URL,
	}

	// Current implementation doesn't retry, so it should fail
	_, err := c.GetPage(context.Background(), "test-page-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestClient_LargeBlockUpdate(t *testing.T) {
	// Test updating with exactly 100, 101, and 200 blocks to verify chunking
	testCases := []struct {
		blockCount int
		wantChunks int
	}{
		{blockCount: 50, wantChunks: 1},
		{blockCount: 100, wantChunks: 1},
		{blockCount: 101, wantChunks: 2},
		{blockCount: 200, wantChunks: 2},
		{blockCount: 250, wantChunks: 3},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d_blocks", tc.blockCount), func(t *testing.T) {
			patchCount := 0
			server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case "GET":
					// Getting existing blocks (empty)
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(BlocksResponse{Results: []Block{}})
				case "PATCH":
					// Counting update requests
					patchCount++
					body, _ := io.ReadAll(r.Body)
					var req map[string]interface{}
					json.Unmarshal(body, &req)
					if children, ok := req["children"].([]interface{}); ok {
						assert.LessOrEqual(t, len(children), 100)
					}
					w.WriteHeader(http.StatusOK)
				}
			})
			defer server.Close()

			c := &client{
				httpClient: &http.Client{Timeout: DefaultTimeout},
				token:      "test-token",
				baseURL:    server.URL,
			}

			// Create blocks
			blocks := make([]map[string]interface{}, tc.blockCount)
			for i := range blocks {
				blocks[i] = map[string]interface{}{
					"type": "paragraph",
					"paragraph": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{"text": map[string]interface{}{"content": fmt.Sprintf("Block %d", i)}},
						},
					},
				}
			}

			err := c.UpdatePageBlocks(context.Background(), "test-page", blocks)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantChunks, patchCount)
		})
	}
}

func TestClient_Headers(t *testing.T) {
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify all required headers are present
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, NotionVersion, r.Header.Get("Notion-Version"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Page{ID: "test"})
	})
	defer server.Close()

	c := &client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		token:      "test-token",
		baseURL:    server.URL,
	}

	_, err := c.GetPage(context.Background(), "test-page")
	assert.NoError(t, err)

	// Verify the request was recorded
	assert.Len(t, server.requests, 1)
	assert.Equal(t, "GET", server.requests[0].Method)
	assert.Equal(t, "/pages/test-page", server.requests[0].Path)
}

func TestClient_RecursiveBlockFetching(t *testing.T) {
	// Test deep nesting and error handling in recursive calls
	callMap := map[string]BlocksResponse{
		"page-id": {
			Results: []Block{
				{ID: "block1", Type: "paragraph", HasChildren: true},
				{ID: "block2", Type: "heading_1", HasChildren: false},
			},
		},
		"block1": {
			Results: []Block{
				{ID: "block1-1", Type: "bulleted_list_item", HasChildren: true},
				{ID: "block1-2", Type: "bulleted_list_item", HasChildren: false},
			},
		},
		"block1-1": {
			Results: []Block{
				{ID: "block1-1-1", Type: "bulleted_list_item", HasChildren: false},
			},
		},
	}

	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)

		// Extract block ID from path
		parts := strings.Split(r.URL.Path, "/")
		blockID := parts[2]

		if resp, ok := callMap[blockID]; ok {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(NotionAPIError{
				Code:    http.StatusNotFound,
				Message: "Block not found",
			})
		}
	})
	defer server.Close()

	c := &client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
		token:      "test-token",
		baseURL:    server.URL,
	}

	blocks, err := c.GetPageBlocks(context.Background(), "page-id")
	assert.NoError(t, err)

	// Should have all blocks including nested ones
	assert.Len(t, blocks, 5) // 2 top-level + 2 from block1 + 1 from block1-1

	// Verify order (depth-first traversal)
	expectedIDs := []string{"block1", "block1-1", "block1-1-1", "block1-2", "block2"}
	for i, expectedID := range expectedIDs {
		assert.Equal(t, expectedID, blocks[i].ID)
	}
}
