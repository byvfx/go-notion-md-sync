package notion

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewOptimizedClient creates a Notion client with optimized HTTP settings
func NewOptimizedClient(token string) Client {
	// Create an optimized HTTP transport
	transport := &http.Transport{
		// Connection pooling settings
		MaxIdleConns:        100,               // Increased from default 100
		MaxIdleConnsPerHost: 50,                // Increased from default 2
		MaxConnsPerHost:     100,               // Limit connections per host
		IdleConnTimeout:     120 * time.Second, // Keep connections alive longer

		// TCP connection settings
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // Connection timeout
			KeepAlive: 60 * time.Second, // TCP keep-alive
		}).DialContext,

		// TLS settings
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false, // Keep security
		},

		// HTTP/2 settings
		ForceAttemptHTTP2:      true,
		MaxResponseHeaderBytes: 4096,

		// Timeouts
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,

		// Disable compression for speed (API responses are usually small)
		DisableCompression: true,
	}

	// Create optimized HTTP client
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Minute, // Increased overall timeout
	}

	return &client{
		baseURL:    "https://api.notion.com/v1",
		token:      token,
		httpClient: httpClient,
	}
}

// NewBurstClient creates a client optimized for burst requests
func NewBurstClient(token string) Client {
	// Even more aggressive settings for burst workloads
	transport := &http.Transport{
		MaxIdleConns:        200,               // Double the connections
		MaxIdleConnsPerHost: 100,               // Much higher per host
		MaxConnsPerHost:     200,               // Allow more concurrent connections
		IdleConnTimeout:     300 * time.Second, // Keep alive even longer

		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,   // Faster connection timeout
			KeepAlive: 120 * time.Second, // Longer keep-alive
		}).DialContext,

		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},

		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   5 * time.Second,  // Faster TLS
		ResponseHeaderTimeout: 15 * time.Second, // Faster headers
		ExpectContinueTimeout: 500 * time.Millisecond,
		DisableCompression:    true,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Minute, // Longer timeout for burst operations
	}

	return &client{
		baseURL:    "https://api.notion.com/v1",
		token:      token,
		httpClient: httpClient,
	}
}

// BatchClient creates multiple clients for true parallel processing
type BatchClient struct {
	clients []Client
	current int
}

// NewBatchClient creates multiple clients to work around connection limits
func NewBatchClient(token string, clientCount int) *BatchClient {
	if clientCount < 1 {
		clientCount = 1
	}
	if clientCount > 10 {
		clientCount = 10 // Cap to avoid overwhelming the API
	}

	clients := make([]Client, clientCount)
	for i := 0; i < clientCount; i++ {
		clients[i] = NewBurstClient(token)
	}

	return &BatchClient{
		clients: clients,
		current: 0,
	}
}

// GetClient returns the next client in round-robin fashion
func (bc *BatchClient) GetClient() Client {
	client := bc.clients[bc.current]
	bc.current = (bc.current + 1) % len(bc.clients)
	return client
}

// GetPage uses round-robin client selection
func (bc *BatchClient) GetPage(ctx context.Context, pageID string) (*Page, error) {
	return bc.GetClient().GetPage(ctx, pageID)
}

// GetPageBlocks uses round-robin client selection
func (bc *BatchClient) GetPageBlocks(ctx context.Context, pageID string) ([]Block, error) {
	return bc.GetClient().GetPageBlocks(ctx, pageID)
}

// GetDatabase uses round-robin client selection
func (bc *BatchClient) GetDatabase(ctx context.Context, databaseID string) (*Database, error) {
	return bc.GetClient().GetDatabase(ctx, databaseID)
}

// CreatePage uses round-robin client selection
func (bc *BatchClient) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*Page, error) {
	return bc.GetClient().CreatePage(ctx, parentID, properties)
}

// UpdatePageBlocks uses round-robin client selection
func (bc *BatchClient) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	return bc.GetClient().UpdatePageBlocks(ctx, pageID, blocks)
}

// DeletePage uses round-robin client selection
func (bc *BatchClient) DeletePage(ctx context.Context, pageID string) error {
	return bc.GetClient().DeletePage(ctx, pageID)
}

// RecreatePageWithBlocks uses round-robin client selection
func (bc *BatchClient) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*Page, error) {
	return bc.GetClient().RecreatePageWithBlocks(ctx, parentID, properties, blocks)
}

// SearchPages uses round-robin client selection
func (bc *BatchClient) SearchPages(ctx context.Context, query string) ([]Page, error) {
	return bc.GetClient().SearchPages(ctx, query)
}

// GetChildPages uses round-robin client selection
func (bc *BatchClient) GetChildPages(ctx context.Context, parentID string) ([]Page, error) {
	return bc.GetClient().GetChildPages(ctx, parentID)
}

// GetAllDescendantPages uses round-robin client selection
func (bc *BatchClient) GetAllDescendantPages(ctx context.Context, parentID string) ([]Page, error) {
	return bc.GetClient().GetAllDescendantPages(ctx, parentID)
}

// QueryDatabase uses round-robin client selection
func (bc *BatchClient) QueryDatabase(ctx context.Context, databaseID string, request *DatabaseQueryRequest) (*DatabaseQueryResponse, error) {
	return bc.GetClient().QueryDatabase(ctx, databaseID, request)
}

// CreateDatabase uses round-robin client selection
func (bc *BatchClient) CreateDatabase(ctx context.Context, request *CreateDatabaseRequest) (*Database, error) {
	return bc.GetClient().CreateDatabase(ctx, request)
}

// CreateDatabaseRow uses round-robin client selection
func (bc *BatchClient) CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]PropertyValue) (*DatabaseRow, error) {
	return bc.GetClient().CreateDatabaseRow(ctx, databaseID, properties)
}

// UpdateDatabaseRow uses round-robin client selection
func (bc *BatchClient) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]PropertyValue) (*DatabaseRow, error) {
	return bc.GetClient().UpdateDatabaseRow(ctx, pageID, properties)
}

// StreamDescendantPages uses round-robin client selection
func (bc *BatchClient) StreamDescendantPages(ctx context.Context, parentID string) *PageStream {
	return bc.GetClient().StreamDescendantPages(ctx, parentID)
}

// StreamDatabaseRows uses round-robin client selection
func (bc *BatchClient) StreamDatabaseRows(ctx context.Context, databaseID string) *DatabaseRowStream {
	return bc.GetClient().StreamDatabaseRows(ctx, databaseID)
}
