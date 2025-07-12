package cache

import (
	"context"
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

// Cache provides a generic caching interface
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
	Size() int
}

// NotionCache provides caching for Notion API calls
type NotionCache interface {
	GetPage(ctx context.Context, pageID string) (*notion.Page, bool)
	SetPage(pageID string, page *notion.Page, ttl time.Duration)
	GetPageBlocks(ctx context.Context, pageID string) ([]notion.Block, bool)
	SetPageBlocks(pageID string, blocks []notion.Block, ttl time.Duration)
	GetDatabase(ctx context.Context, databaseID string) (*notion.Database, bool)
	SetDatabase(databaseID string, database *notion.Database, ttl time.Duration)
	InvalidatePage(pageID string)
	InvalidateDatabase(databaseID string)
	Clear()
	Stats() CacheStats
}

// CacheStats provides statistics about cache usage
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int
}

// CacheEntry represents a cached item with expiration
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// IsExpired checks if the cache entry has expired
func (ce *CacheEntry) IsExpired() bool {
	return time.Now().After(ce.ExpiresAt)
}

// MemoryCache implements a simple in-memory cache
type MemoryCache struct {
	mu         sync.RWMutex
	entries    map[string]*CacheEntry
	hits       int64
	misses     int64
	evictions  int64
	maxSize    int
	defaultTTL time.Duration
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxSize int, defaultTTL time.Duration) *MemoryCache {
	return &MemoryCache{
		entries:    make(map[string]*CacheEntry),
		maxSize:    maxSize,
		defaultTTL: defaultTTL,
	}
}

// Get retrieves a value from the cache
func (mc *MemoryCache) Get(key string) (interface{}, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, exists := mc.entries[key]
	if !exists {
		mc.misses++
		return nil, false
	}

	if entry.IsExpired() {
		// Remove expired entry
		delete(mc.entries, key)
		mc.misses++
		return nil, false
	}

	mc.hits++
	return entry.Value, true
}

// Set stores a value in the cache
func (mc *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = mc.defaultTTL
	}

	// Check if we need to evict entries
	if len(mc.entries) >= mc.maxSize {
		// Simple eviction: remove oldest expired entry or first entry
		mc.evictLRU()
	}

	mc.entries[key] = &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a value from the cache
func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.entries, key)
}

// Clear removes all entries from the cache
func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.entries = make(map[string]*CacheEntry)
}

// Size returns the current size of the cache
func (mc *MemoryCache) Size() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return len(mc.entries)
}

// evictLRU removes the least recently used entry
func (mc *MemoryCache) evictLRU() {
	// Simple eviction strategy: remove first expired entry, or first entry
	for key, entry := range mc.entries {
		if entry.IsExpired() {
			delete(mc.entries, key)
			mc.evictions++
			return
		}
	}

	// No expired entries, remove first entry
	for key := range mc.entries {
		delete(mc.entries, key)
		mc.evictions++
		return
	}
}

// NotionCacheImpl implements NotionCache using MemoryCache
type NotionCacheImpl struct {
	cache *MemoryCache
}

// NewNotionCache creates a new Notion cache
func NewNotionCache(maxSize int, defaultTTL time.Duration) NotionCache {
	return &NotionCacheImpl{
		cache: NewMemoryCache(maxSize, defaultTTL),
	}
}

// GetPage retrieves a page from the cache
func (nc *NotionCacheImpl) GetPage(ctx context.Context, pageID string) (*notion.Page, bool) {
	key := fmt.Sprintf("page:%s", pageID)
	if value, exists := nc.cache.Get(key); exists {
		if page, ok := value.(*notion.Page); ok {
			return page, true
		}
	}
	return nil, false
}

// SetPage stores a page in the cache
func (nc *NotionCacheImpl) SetPage(pageID string, page *notion.Page, ttl time.Duration) {
	key := fmt.Sprintf("page:%s", pageID)
	nc.cache.Set(key, page, ttl)
}

// GetPageBlocks retrieves page blocks from the cache
func (nc *NotionCacheImpl) GetPageBlocks(ctx context.Context, pageID string) ([]notion.Block, bool) {
	key := fmt.Sprintf("blocks:%s", pageID)
	if value, exists := nc.cache.Get(key); exists {
		if blocks, ok := value.([]notion.Block); ok {
			return blocks, true
		}
	}
	return nil, false
}

// SetPageBlocks stores page blocks in the cache
func (nc *NotionCacheImpl) SetPageBlocks(pageID string, blocks []notion.Block, ttl time.Duration) {
	key := fmt.Sprintf("blocks:%s", pageID)
	nc.cache.Set(key, blocks, ttl)
}

// GetDatabase retrieves a database from the cache
func (nc *NotionCacheImpl) GetDatabase(ctx context.Context, databaseID string) (*notion.Database, bool) {
	key := fmt.Sprintf("database:%s", databaseID)
	if value, exists := nc.cache.Get(key); exists {
		if database, ok := value.(*notion.Database); ok {
			return database, true
		}
	}
	return nil, false
}

// SetDatabase stores a database in the cache
func (nc *NotionCacheImpl) SetDatabase(databaseID string, database *notion.Database, ttl time.Duration) {
	key := fmt.Sprintf("database:%s", databaseID)
	nc.cache.Set(key, database, ttl)
}

// InvalidatePage removes a page and its blocks from the cache
func (nc *NotionCacheImpl) InvalidatePage(pageID string) {
	nc.cache.Delete(fmt.Sprintf("page:%s", pageID))
	nc.cache.Delete(fmt.Sprintf("blocks:%s", pageID))
}

// InvalidateDatabase removes a database from the cache
func (nc *NotionCacheImpl) InvalidateDatabase(databaseID string) {
	nc.cache.Delete(fmt.Sprintf("database:%s", databaseID))
}

// Clear removes all entries from the cache
func (nc *NotionCacheImpl) Clear() {
	nc.cache.Clear()
}

// Stats returns cache statistics
func (nc *NotionCacheImpl) Stats() CacheStats {
	nc.cache.mu.RLock()
	defer nc.cache.mu.RUnlock()

	return CacheStats{
		Hits:      nc.cache.hits,
		Misses:    nc.cache.misses,
		Evictions: nc.cache.evictions,
		Size:      len(nc.cache.entries),
	}
}

// CachedNotionClient wraps a notion.Client with caching capabilities
type CachedNotionClient struct {
	client notion.Client
	cache  NotionCache
}

// NewCachedNotionClient creates a new cached notion client
func NewCachedNotionClient(client notion.Client, cache NotionCache) *CachedNotionClient {
	return &CachedNotionClient{
		client: client,
		cache:  cache,
	}
}

// GetPage implements notion.Client interface with caching
func (c *CachedNotionClient) GetPage(ctx context.Context, pageID string) (*notion.Page, error) {
	// Check cache first
	if page, exists := c.cache.GetPage(ctx, pageID); exists {
		return page, nil
	}

	// Fetch from API
	page, err := c.client.GetPage(ctx, pageID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.cache.SetPage(pageID, page, 0) // Use default TTL
	return page, nil
}

// GetPageBlocks implements notion.Client interface with caching
func (c *CachedNotionClient) GetPageBlocks(ctx context.Context, pageID string) ([]notion.Block, error) {
	// Check cache first
	if blocks, exists := c.cache.GetPageBlocks(ctx, pageID); exists {
		return blocks, nil
	}

	// Fetch from API
	blocks, err := c.client.GetPageBlocks(ctx, pageID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.cache.SetPageBlocks(pageID, blocks, 0) // Use default TTL
	return blocks, nil
}

// GetDatabase implements notion.Client interface with caching
func (c *CachedNotionClient) GetDatabase(ctx context.Context, databaseID string) (*notion.Database, error) {
	// Check cache first
	if database, exists := c.cache.GetDatabase(ctx, databaseID); exists {
		return database, nil
	}

	// Fetch from API
	database, err := c.client.GetDatabase(ctx, databaseID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.cache.SetDatabase(databaseID, database, 0) // Use default TTL
	return database, nil
}

// All other methods delegate to the underlying client

func (c *CachedNotionClient) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*notion.Page, error) {
	return c.client.CreatePage(ctx, parentID, properties)
}

func (c *CachedNotionClient) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	// Invalidate cache when updating
	c.cache.InvalidatePage(pageID)
	return c.client.UpdatePageBlocks(ctx, pageID, blocks)
}

func (c *CachedNotionClient) DeletePage(ctx context.Context, pageID string) error {
	// Invalidate cache when deleting
	c.cache.InvalidatePage(pageID)
	return c.client.DeletePage(ctx, pageID)
}

func (c *CachedNotionClient) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*notion.Page, error) {
	return c.client.RecreatePageWithBlocks(ctx, parentID, properties, blocks)
}

func (c *CachedNotionClient) SearchPages(ctx context.Context, query string) ([]notion.Page, error) {
	return c.client.SearchPages(ctx, query)
}

func (c *CachedNotionClient) GetChildPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	return c.client.GetChildPages(ctx, parentID)
}

func (c *CachedNotionClient) GetAllDescendantPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	return c.client.GetAllDescendantPages(ctx, parentID)
}

func (c *CachedNotionClient) QueryDatabase(ctx context.Context, databaseID string, request *notion.DatabaseQueryRequest) (*notion.DatabaseQueryResponse, error) {
	return c.client.QueryDatabase(ctx, databaseID, request)
}

func (c *CachedNotionClient) CreateDatabase(ctx context.Context, request *notion.CreateDatabaseRequest) (*notion.Database, error) {
	return c.client.CreateDatabase(ctx, request)
}

func (c *CachedNotionClient) CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return c.client.CreateDatabaseRow(ctx, databaseID, properties)
}

func (c *CachedNotionClient) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return c.client.UpdateDatabaseRow(ctx, pageID, properties)
}

// CacheKey generates a cache key for the given parameters
func CacheKey(parts ...string) string {
	hasher := md5.New()
	for _, part := range parts {
		hasher.Write([]byte(part))
	}
	return fmt.Sprintf("%x", hasher.Sum(nil))
}
