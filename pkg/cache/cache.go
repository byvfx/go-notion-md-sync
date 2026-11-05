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

// Get retrieves a value from the cache. It takes the write lock because it
// updates the hit/miss counters and removes expired entries.
func (mc *MemoryCache) Get(key string) (interface{}, bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

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

	// Evict only when adding a new key to a full cache; overwriting an
	// existing key does not grow it. maxSize <= 0 means unbounded.
	if _, exists := mc.entries[key]; !exists && mc.maxSize > 0 {
		for len(mc.entries) >= mc.maxSize {
			if !mc.evictLRU() {
				break
			}
		}
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

// evictLRU removes one entry, preferring an expired one, otherwise the one
// closest to expiry. It reports whether an entry was removed. The caller
// must hold mc.mu.
func (mc *MemoryCache) evictLRU() bool {
	var victim string
	var victimExpiry time.Time
	found := false
	for key, entry := range mc.entries {
		if entry.IsExpired() {
			victim, found = key, true
			break
		}
		if !found || entry.ExpiresAt.Before(victimExpiry) {
			victim, victimExpiry, found = key, entry.ExpiresAt, true
		}
	}
	if !found {
		return false
	}
	delete(mc.entries, victim)
	mc.evictions++
	return true
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
		if page, ok := value.(*notion.Page); ok && page != nil {
			return copyPage(page), true
		}
	}
	return nil, false
}

// SetPage stores a page in the cache
func (nc *NotionCacheImpl) SetPage(pageID string, page *notion.Page, ttl time.Duration) {
	if page == nil {
		return
	}
	key := fmt.Sprintf("page:%s", pageID)
	nc.cache.Set(key, copyPage(page), ttl)
}

// GetPageBlocks retrieves page blocks from the cache
func (nc *NotionCacheImpl) GetPageBlocks(ctx context.Context, pageID string) ([]notion.Block, bool) {
	key := fmt.Sprintf("blocks:%s", pageID)
	if value, exists := nc.cache.Get(key); exists {
		if blocks, ok := value.([]notion.Block); ok {
			return copyBlocks(blocks), true
		}
	}
	return nil, false
}

// SetPageBlocks stores page blocks in the cache
func (nc *NotionCacheImpl) SetPageBlocks(pageID string, blocks []notion.Block, ttl time.Duration) {
	key := fmt.Sprintf("blocks:%s", pageID)
	nc.cache.Set(key, copyBlocks(blocks), ttl)
}

// GetDatabase retrieves a database from the cache
func (nc *NotionCacheImpl) GetDatabase(ctx context.Context, databaseID string) (*notion.Database, bool) {
	key := fmt.Sprintf("database:%s", databaseID)
	if value, exists := nc.cache.Get(key); exists {
		if database, ok := value.(*notion.Database); ok && database != nil {
			return copyDatabase(database), true
		}
	}
	return nil, false
}

// SetDatabase stores a database in the cache
func (nc *NotionCacheImpl) SetDatabase(databaseID string, database *notion.Database, ttl time.Duration) {
	if database == nil {
		return
	}
	key := fmt.Sprintf("database:%s", databaseID)
	nc.cache.Set(key, copyDatabase(database), ttl)
}

// copyPage returns a shallow copy so callers cannot modify the cached
// struct. Nested maps are still shared.
func copyPage(page *notion.Page) *notion.Page {
	cp := *page
	return &cp
}

// copyDatabase returns a shallow copy of database.
func copyDatabase(database *notion.Database) *notion.Database {
	cp := *database
	return &cp
}

// copyBlocks returns a copy of the slice so callers cannot modify the cached
// one (appending to or reordering it, or replacing elements).
func copyBlocks(blocks []notion.Block) []notion.Block {
	if blocks == nil {
		return nil
	}
	return append([]notion.Block(nil), blocks...)
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

// CreatePage creates a page and invalidates the parent, whose child blocks
// now include the new page.
func (c *CachedNotionClient) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*notion.Page, error) {
	page, err := c.client.CreatePage(ctx, parentID, properties)
	c.cache.InvalidatePage(parentID)
	return page, err
}

// UpdatePageBlocks replaces a page's blocks. The page is invalidated before
// the write and again after it, so a read racing with the update cannot
// leave stale blocks cached.
func (c *CachedNotionClient) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	c.cache.InvalidatePage(pageID)
	err := c.client.UpdatePageBlocks(ctx, pageID, blocks)
	c.cache.InvalidatePage(pageID)
	return err
}

// DeletePage deletes a page and invalidates it.
func (c *CachedNotionClient) DeletePage(ctx context.Context, pageID string) error {
	c.cache.InvalidatePage(pageID)
	err := c.client.DeletePage(ctx, pageID)
	c.cache.InvalidatePage(pageID)
	return err
}

// RecreatePageWithBlocks creates a page with content and invalidates the
// parent and the resulting page.
func (c *CachedNotionClient) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*notion.Page, error) {
	page, err := c.client.RecreatePageWithBlocks(ctx, parentID, properties, blocks)
	c.cache.InvalidatePage(parentID)
	if page != nil {
		c.cache.InvalidatePage(page.ID)
	}
	return page, err
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

func (c *CachedNotionClient) StreamDescendantPages(ctx context.Context, parentID string) *notion.PageStream {
	return c.client.StreamDescendantPages(ctx, parentID)
}

func (c *CachedNotionClient) StreamDatabaseRows(ctx context.Context, databaseID string) *notion.DatabaseRowStream {
	return c.client.StreamDatabaseRows(ctx, databaseID)
}

func (c *CachedNotionClient) QueryDatabase(ctx context.Context, databaseID string, request *notion.DatabaseQueryRequest) (*notion.DatabaseQueryResponse, error) {
	return c.client.QueryDatabase(ctx, databaseID, request)
}

// CreateDatabase creates a database and invalidates the parent page, whose
// child blocks now include it.
func (c *CachedNotionClient) CreateDatabase(ctx context.Context, request *notion.CreateDatabaseRequest) (*notion.Database, error) {
	database, err := c.client.CreateDatabase(ctx, request)
	if request != nil && request.Parent.PageID != "" {
		c.cache.InvalidatePage(request.Parent.PageID)
	}
	return database, err
}

// CreateDatabaseRow creates a row and invalidates the database.
func (c *CachedNotionClient) CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	row, err := c.client.CreateDatabaseRow(ctx, databaseID, properties)
	c.cache.InvalidateDatabase(databaseID)
	return row, err
}

// UpdateDatabaseRow updates a row (a page) and invalidates it.
func (c *CachedNotionClient) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	c.cache.InvalidatePage(pageID)
	row, err := c.client.UpdateDatabaseRow(ctx, pageID, properties)
	c.cache.InvalidatePage(pageID)
	return row, err
}

// CacheKey generates a cache key for the given parameters
func CacheKey(parts ...string) string {
	hasher := md5.New()
	for _, part := range parts {
		hasher.Write([]byte(part))
	}
	return fmt.Sprintf("%x", hasher.Sum(nil))
}
