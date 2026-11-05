package notion

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// newTunedTransport returns a copy of http.DefaultTransport (so proxy
// settings from HTTPS_PROXY/HTTP_PROXY/NO_PROXY, default TLS and HTTP/2
// behaviour are preserved) with tune applied on top.
func newTunedTransport(tune func(t *http.Transport)) *http.Transport {
	var transport *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = dt.Clone()
	} else {
		transport = &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			ForceAttemptHTTP2: true,
		}
	}
	if transport.Proxy == nil {
		transport.Proxy = http.ProxyFromEnvironment
	}
	tune(transport)
	return transport
}

// NewOptimizedClient creates a Notion client with optimized HTTP settings
func NewOptimizedClient(token string) Client {
	transport := newTunedTransport(func(t *http.Transport) {
		// Connection pooling settings
		t.MaxIdleConns = 100
		t.MaxIdleConnsPerHost = 50 // default is 2
		t.MaxConnsPerHost = 100
		t.IdleConnTimeout = 120 * time.Second

		// TCP connection settings
		t.DialContext = (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext

		t.ForceAttemptHTTP2 = true
		t.MaxResponseHeaderBytes = 4096

		// Timeouts
		t.TLSHandshakeTimeout = 10 * time.Second
		t.ResponseHeaderTimeout = 30 * time.Second
		t.ExpectContinueTimeout = 1 * time.Second

		// Disable compression for speed (API responses are usually small)
		t.DisableCompression = true
	})

	return &client{
		baseURL: "https://api.notion.com/v1",
		token:   token,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Minute,
		},
	}
}

// NewBurstClient creates a client optimized for burst requests
func NewBurstClient(token string) Client {
	transport := newTunedTransport(func(t *http.Transport) {
		t.MaxIdleConns = 200
		t.MaxIdleConnsPerHost = 100
		t.MaxConnsPerHost = 200
		t.IdleConnTimeout = 300 * time.Second

		t.DialContext = (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 120 * time.Second,
		}).DialContext

		t.ForceAttemptHTTP2 = true
		t.TLSHandshakeTimeout = 5 * time.Second
		t.ResponseHeaderTimeout = 15 * time.Second
		t.ExpectContinueTimeout = 500 * time.Millisecond
		t.DisableCompression = true
	})

	return &client{
		baseURL: "https://api.notion.com/v1",
		token:   token,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Minute, // Longer timeout for burst operations
		},
	}
}

// BatchClient creates multiple clients for true parallel processing
type BatchClient struct {
	clients []Client
	// next is the round-robin counter; it is shared by all workers.
	next atomic.Uint64
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

	return &BatchClient{clients: clients}
}

// GetClient returns the next client in round-robin fashion. It is safe for
// concurrent use.
func (bc *BatchClient) GetClient() Client {
	n := bc.next.Add(1) - 1
	return bc.clients[n%uint64(len(bc.clients))]
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
