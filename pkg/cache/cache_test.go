package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

// Mock Notion client for testing
type mockNotionClient struct {
	pages            map[string]*notion.Page
	blocks           map[string][]notion.Block
	databases        map[string]*notion.Database
	getPageCalls     int
	getBlocksCalls   int
	getDatabaseCalls int
	getPageErr       error
	getBlocksErr     error
	getDatabaseErr   error
}

func (m *mockNotionClient) GetPage(ctx context.Context, pageID string) (*notion.Page, error) {
	m.getPageCalls++
	if m.getPageErr != nil {
		return nil, m.getPageErr
	}
	page, exists := m.pages[pageID]
	if !exists {
		return nil, errors.New("page not found")
	}
	return page, nil
}

func (m *mockNotionClient) GetPageBlocks(ctx context.Context, pageID string) ([]notion.Block, error) {
	m.getBlocksCalls++
	if m.getBlocksErr != nil {
		return nil, m.getBlocksErr
	}
	blocks, exists := m.blocks[pageID]
	if !exists {
		return nil, errors.New("blocks not found")
	}
	return blocks, nil
}

func (m *mockNotionClient) GetDatabase(ctx context.Context, databaseID string) (*notion.Database, error) {
	m.getDatabaseCalls++
	if m.getDatabaseErr != nil {
		return nil, m.getDatabaseErr
	}
	database, exists := m.databases[databaseID]
	if !exists {
		return nil, errors.New("database not found")
	}
	return database, nil
}

func (m *mockNotionClient) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*notion.Page, error) {
	return nil, errors.New("not implemented")
}

func (m *mockNotionClient) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	return errors.New("not implemented")
}

func (m *mockNotionClient) DeletePage(ctx context.Context, pageID string) error {
	return errors.New("not implemented")
}

func (m *mockNotionClient) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*notion.Page, error) {
	return nil, errors.New("not implemented")
}

func (m *mockNotionClient) SearchPages(ctx context.Context, query string) ([]notion.Page, error) {
	return nil, errors.New("not implemented")
}

func (m *mockNotionClient) GetChildPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	return nil, errors.New("not implemented")
}

func (m *mockNotionClient) GetAllDescendantPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	return nil, errors.New("not implemented")
}

func (m *mockNotionClient) StreamDescendantPages(ctx context.Context, parentID string) *notion.PageStream {
	stream := notion.NewPageStream()
	go func() {
		defer stream.Close()
	}()
	return stream
}

func (m *mockNotionClient) StreamDatabaseRows(ctx context.Context, databaseID string) *notion.DatabaseRowStream {
	stream := notion.NewDatabaseRowStream()
	go func() {
		defer stream.Close()
	}()
	return stream
}

func (m *mockNotionClient) QueryDatabase(ctx context.Context, databaseID string, request *notion.DatabaseQueryRequest) (*notion.DatabaseQueryResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockNotionClient) CreateDatabase(ctx context.Context, request *notion.CreateDatabaseRequest) (*notion.Database, error) {
	return nil, errors.New("not implemented")
}

func (m *mockNotionClient) CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return nil, errors.New("not implemented")
}

func (m *mockNotionClient) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return nil, errors.New("not implemented")
}

func TestMemoryCache_GetSet(t *testing.T) {
	cache := NewMemoryCache(10, 1*time.Hour)

	// Test miss
	_, exists := cache.Get("nonexistent")
	if exists {
		t.Error("Expected cache miss for nonexistent key")
	}

	// Test set and get
	cache.Set("key1", "value1", 0)
	value, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected cache hit for existing key")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}
}

func TestMemoryCache_Expiration(t *testing.T) {
	cache := NewMemoryCache(10, 1*time.Hour)

	// Set with short TTL
	cache.Set("key1", "value1", 10*time.Millisecond)

	// Should be available immediately
	_, exists := cache.Get("key1")
	if !exists {
		t.Error("Expected cache hit for fresh entry")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Should be expired
	_, exists = cache.Get("key1")
	if exists {
		t.Error("Expected cache miss for expired entry")
	}
}

func TestMemoryCache_Eviction(t *testing.T) {
	cache := NewMemoryCache(2, 1*time.Hour)

	// Fill cache to capacity
	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)

	// Add third item should trigger eviction
	cache.Set("key3", "value3", 0)

	// Should have evicted something
	if cache.Size() > 2 {
		t.Errorf("Cache size %d exceeds max size 2", cache.Size())
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(10, 1*time.Hour)

	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected empty cache after clear, got size %d", cache.Size())
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	cache := NewMemoryCache(10, 1*time.Hour)

	cache.Set("key1", "value1", 0)
	cache.Delete("key1")

	_, exists := cache.Get("key1")
	if exists {
		t.Error("Expected cache miss after delete")
	}
}

func TestNotionCache_Page(t *testing.T) {
	cache := NewNotionCache(10, 1*time.Hour)
	ctx := context.Background()

	page := &notion.Page{
		ID: "page-1",
		Properties: map[string]interface{}{
			"title": "Test Page",
		},
	}

	// Test miss
	_, exists := cache.GetPage(ctx, "page-1")
	if exists {
		t.Error("Expected cache miss for nonexistent page")
	}

	// Test set and get
	cache.SetPage("page-1", page, 0)
	cachedPage, exists := cache.GetPage(ctx, "page-1")
	if !exists {
		t.Error("Expected cache hit for existing page")
	}
	if cachedPage.ID != page.ID {
		t.Errorf("Expected page ID %s, got %s", page.ID, cachedPage.ID)
	}
}

func TestNotionCache_PageBlocks(t *testing.T) {
	cache := NewNotionCache(10, 1*time.Hour)
	ctx := context.Background()

	blocks := []notion.Block{
		{ID: "block-1", Type: "paragraph"},
		{ID: "block-2", Type: "heading_1"},
	}

	// Test miss
	_, exists := cache.GetPageBlocks(ctx, "page-1")
	if exists {
		t.Error("Expected cache miss for nonexistent blocks")
	}

	// Test set and get
	cache.SetPageBlocks("page-1", blocks, 0)
	cachedBlocks, exists := cache.GetPageBlocks(ctx, "page-1")
	if !exists {
		t.Error("Expected cache hit for existing blocks")
	}
	if len(cachedBlocks) != len(blocks) {
		t.Errorf("Expected %d blocks, got %d", len(blocks), len(cachedBlocks))
	}
}

func TestNotionCache_Database(t *testing.T) {
	cache := NewNotionCache(10, 1*time.Hour)
	ctx := context.Background()

	database := &notion.Database{
		ID: "database-1",
		Title: []notion.RichText{
			{PlainText: "Test Database"},
		},
	}

	// Test miss
	_, exists := cache.GetDatabase(ctx, "database-1")
	if exists {
		t.Error("Expected cache miss for nonexistent database")
	}

	// Test set and get
	cache.SetDatabase("database-1", database, 0)
	cachedDatabase, exists := cache.GetDatabase(ctx, "database-1")
	if !exists {
		t.Error("Expected cache hit for existing database")
	}
	if cachedDatabase.ID != database.ID {
		t.Errorf("Expected database ID %s, got %s", database.ID, cachedDatabase.ID)
	}
}

func TestNotionCache_Invalidation(t *testing.T) {
	cache := NewNotionCache(10, 1*time.Hour)

	page := &notion.Page{ID: "page-1"}
	blocks := []notion.Block{{ID: "block-1"}}
	database := &notion.Database{ID: "database-1"}

	// Set items
	cache.SetPage("page-1", page, 0)
	cache.SetPageBlocks("page-1", blocks, 0)
	cache.SetDatabase("database-1", database, 0)

	// Invalidate page
	cache.InvalidatePage("page-1")

	// Page and blocks should be gone
	ctx := context.Background()
	_, exists := cache.GetPage(ctx, "page-1")
	if exists {
		t.Error("Expected page to be invalidated")
	}
	_, exists = cache.GetPageBlocks(ctx, "page-1")
	if exists {
		t.Error("Expected blocks to be invalidated")
	}

	// Database should still exist
	_, exists = cache.GetDatabase(ctx, "database-1")
	if !exists {
		t.Error("Expected database to still exist")
	}

	// Invalidate database
	cache.InvalidateDatabase("database-1")
	_, exists = cache.GetDatabase(ctx, "database-1")
	if exists {
		t.Error("Expected database to be invalidated")
	}
}

func TestNotionCache_Stats(t *testing.T) {
	cache := NewNotionCache(10, 1*time.Hour)
	ctx := context.Background()

	// Initial stats
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("Expected initial stats to be zero, got hits=%d, misses=%d", stats.Hits, stats.Misses)
	}

	// Test miss
	_, _ = cache.GetPage(ctx, "page-1")
	stats = cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	// Set and test hit
	page := &notion.Page{ID: "page-1"}
	cache.SetPage("page-1", page, 0)
	_, _ = cache.GetPage(ctx, "page-1")
	stats = cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
}

func TestCachedNotionClient_GetPage(t *testing.T) {
	mockClient := &mockNotionClient{
		pages: map[string]*notion.Page{
			"page-1": {ID: "page-1", Properties: map[string]interface{}{"title": "Test Page"}},
		},
	}

	cache := NewNotionCache(10, 1*time.Hour)
	cachedClient := NewCachedNotionClient(mockClient, cache)

	ctx := context.Background()

	// First call should hit the API
	page, err := cachedClient.GetPage(ctx, "page-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if page.ID != "page-1" {
		t.Errorf("Expected page ID page-1, got %s", page.ID)
	}
	if mockClient.getPageCalls != 1 {
		t.Errorf("Expected 1 API call, got %d", mockClient.getPageCalls)
	}

	// Second call should hit the cache
	page, err = cachedClient.GetPage(ctx, "page-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if page.ID != "page-1" {
		t.Errorf("Expected page ID page-1, got %s", page.ID)
	}
	if mockClient.getPageCalls != 1 {
		t.Errorf("Expected 1 API call (cached), got %d", mockClient.getPageCalls)
	}
}

func TestCachedNotionClient_GetPageBlocks(t *testing.T) {
	mockClient := &mockNotionClient{
		blocks: map[string][]notion.Block{
			"page-1": {
				{ID: "block-1", Type: "paragraph"},
				{ID: "block-2", Type: "heading_1"},
			},
		},
	}

	cache := NewNotionCache(10, 1*time.Hour)
	cachedClient := NewCachedNotionClient(mockClient, cache)

	ctx := context.Background()

	// First call should hit the API
	blocks, err := cachedClient.GetPageBlocks(ctx, "page-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(blocks) != 2 {
		t.Errorf("Expected 2 blocks, got %d", len(blocks))
	}
	if mockClient.getBlocksCalls != 1 {
		t.Errorf("Expected 1 API call, got %d", mockClient.getBlocksCalls)
	}

	// Second call should hit the cache
	blocks, err = cachedClient.GetPageBlocks(ctx, "page-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(blocks) != 2 {
		t.Errorf("Expected 2 blocks, got %d", len(blocks))
	}
	if mockClient.getBlocksCalls != 1 {
		t.Errorf("Expected 1 API call (cached), got %d", mockClient.getBlocksCalls)
	}
}

func TestCachedNotionClient_GetDatabase(t *testing.T) {
	mockClient := &mockNotionClient{
		databases: map[string]*notion.Database{
			"database-1": {ID: "database-1", Title: []notion.RichText{{PlainText: "Test DB"}}},
		},
	}

	cache := NewNotionCache(10, 1*time.Hour)
	cachedClient := NewCachedNotionClient(mockClient, cache)

	ctx := context.Background()

	// First call should hit the API
	database, err := cachedClient.GetDatabase(ctx, "database-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if database.ID != "database-1" {
		t.Errorf("Expected database ID database-1, got %s", database.ID)
	}
	if mockClient.getDatabaseCalls != 1 {
		t.Errorf("Expected 1 API call, got %d", mockClient.getDatabaseCalls)
	}

	// Second call should hit the cache
	database, err = cachedClient.GetDatabase(ctx, "database-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if database.ID != "database-1" {
		t.Errorf("Expected database ID database-1, got %s", database.ID)
	}
	if mockClient.getDatabaseCalls != 1 {
		t.Errorf("Expected 1 API call (cached), got %d", mockClient.getDatabaseCalls)
	}
}

func TestCachedNotionClient_Invalidation(t *testing.T) {
	mockClient := &mockNotionClient{
		pages: map[string]*notion.Page{
			"page-1": {ID: "page-1"},
		},
	}

	cache := NewNotionCache(10, 1*time.Hour)
	cachedClient := NewCachedNotionClient(mockClient, cache)

	ctx := context.Background()

	// First call caches the page
	_, err := cachedClient.GetPage(ctx, "page-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Update page blocks should invalidate cache
	err = cachedClient.UpdatePageBlocks(ctx, "page-1", []map[string]interface{}{})
	if err == nil {
		t.Error("Expected error from mock client")
	}

	// Next call should hit API again (cache invalidated)
	_, err = cachedClient.GetPage(ctx, "page-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if mockClient.getPageCalls != 2 {
		t.Errorf("Expected 2 API calls after invalidation, got %d", mockClient.getPageCalls)
	}
}

func TestCacheKey(t *testing.T) {
	key1 := CacheKey("page", "123")
	key2 := CacheKey("page", "123")
	key3 := CacheKey("page", "456")

	if key1 != key2 {
		t.Error("Expected same keys for same input")
	}

	if key1 == key3 {
		t.Error("Expected different keys for different input")
	}
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	cache := NewMemoryCache(1000, 1*time.Hour)
	cache.Set("key", "value", 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("key")
	}
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	cache := NewMemoryCache(1000, 1*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("key", "value", 0)
	}
}

func BenchmarkCachedNotionClient_GetPage(b *testing.B) {
	mockClient := &mockNotionClient{
		pages: map[string]*notion.Page{
			"page-1": {ID: "page-1"},
		},
	}

	cache := NewNotionCache(1000, 1*time.Hour)
	cachedClient := NewCachedNotionClient(mockClient, cache)

	ctx := context.Background()

	// Prime the cache
	_, _ = cachedClient.GetPage(ctx, "page-1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cachedClient.GetPage(ctx, "page-1")
	}
}
