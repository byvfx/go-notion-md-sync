package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

// TestMemoryCache_ConcurrentGetExpired is a regression test: Get deleted
// expired entries and bumped counters under a read lock. Run with -race.
func TestMemoryCache_ConcurrentGetExpired(t *testing.T) {
	mc := NewMemoryCache(1000, time.Hour)
	for i := 0; i < 100; i++ {
		mc.Set(fmt.Sprintf("k%d", i), i, time.Nanosecond)
	}
	time.Sleep(time.Millisecond)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				mc.Get(fmt.Sprintf("k%d", i))
				mc.Get("missing")
			}
		}()
	}
	wg.Wait()

	if mc.Size() != 0 {
		t.Errorf("expired entries not removed, size = %d", mc.Size())
	}
	nc := &NotionCacheImpl{cache: mc}
	if stats := nc.Stats(); stats.Misses != 8*200 {
		t.Errorf("misses = %d, want %d", stats.Misses, 8*200)
	}
}

func TestMemoryCache_OverwriteDoesNotEvict(t *testing.T) {
	mc := NewMemoryCache(2, time.Hour)
	mc.Set("a", 1, 0)
	mc.Set("b", 2, 0)
	mc.Set("a", 3, 0)

	if v, ok := mc.Get("a"); !ok || v != 3 {
		t.Errorf("Get(a) = %v, %v; want 3, true", v, ok)
	}
	if _, ok := mc.Get("b"); !ok {
		t.Error("overwriting an existing key evicted another entry")
	}
}

func TestNotionCache_ReturnsCopies(t *testing.T) {
	nc := NewNotionCache(10, time.Hour)

	blocks := []notion.Block{{ID: "b1"}, {ID: "b2"}}
	nc.SetPageBlocks("p", blocks, 0)
	blocks[0].ID = "mutated-after-set"

	got, ok := nc.GetPageBlocks(context.Background(), "p")
	if !ok || got[0].ID != "b1" {
		t.Fatalf("cache shares the caller's slice: %+v", got)
	}
	got[1].ID = "mutated-after-get"
	again, _ := nc.GetPageBlocks(context.Background(), "p")
	if again[1].ID != "b2" {
		t.Errorf("cache hands out its own slice: %+v", again)
	}

	nc.SetPage("p", &notion.Page{ID: "p", URL: "u"}, 0)
	page, _ := nc.GetPage(context.Background(), "p")
	page.URL = "changed"
	page2, _ := nc.GetPage(context.Background(), "p")
	if page2.URL != "u" {
		t.Errorf("cache hands out its own page: %+v", page2)
	}
}

// writeMockClient extends mockNotionClient with working write operations.
type writeMockClient struct {
	mockNotionClient
	onUpdateBlocks func(pageID string)
}

func (m *writeMockClient) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*notion.Page, error) {
	id := fmt.Sprintf("child-of-%s", parentID)
	m.pages[id] = &notion.Page{ID: id}
	m.blocks[parentID] = append(m.blocks[parentID], notion.Block{ID: id, Type: "child_page"})
	return m.pages[id], nil
}

func (m *writeMockClient) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*notion.Page, error) {
	return m.CreatePage(ctx, parentID, properties)
}

func (m *writeMockClient) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	if m.onUpdateBlocks != nil {
		m.onUpdateBlocks(pageID)
	}
	m.blocks[pageID] = []notion.Block{{ID: "new"}}
	return nil
}

func (m *writeMockClient) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	m.pages[pageID] = &notion.Page{ID: pageID, URL: "updated"}
	return &notion.DatabaseRow{}, nil
}

func newWriteMock() *writeMockClient {
	return &writeMockClient{mockNotionClient: mockNotionClient{
		pages:     map[string]*notion.Page{"parent": {ID: "parent"}, "row": {ID: "row", URL: "old"}},
		blocks:    map[string][]notion.Block{"parent": {}, "page": {{ID: "old"}}},
		databases: map[string]*notion.Database{},
	}}
}

// TestCachedNotionClient_WritesInvalidate is a regression test: create,
// recreate and database row updates left stale entries in the cache.
func TestCachedNotionClient_WritesInvalidate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		write func(c *CachedNotionClient) error
		check func(t *testing.T, c *CachedNotionClient)
	}{
		{
			name: "CreatePage invalidates parent blocks",
			write: func(c *CachedNotionClient) error {
				_, err := c.CreatePage(ctx, "parent", nil)
				return err
			},
			check: func(t *testing.T, c *CachedNotionClient) {
				blocks, err := c.GetPageBlocks(ctx, "parent")
				if err != nil || len(blocks) != 1 {
					t.Errorf("parent blocks = %v, %v; want the new child", blocks, err)
				}
			},
		},
		{
			name: "RecreatePageWithBlocks invalidates parent blocks",
			write: func(c *CachedNotionClient) error {
				_, err := c.RecreatePageWithBlocks(ctx, "parent", nil, nil)
				return err
			},
			check: func(t *testing.T, c *CachedNotionClient) {
				blocks, err := c.GetPageBlocks(ctx, "parent")
				if err != nil || len(blocks) != 1 {
					t.Errorf("parent blocks = %v, %v; want the new child", blocks, err)
				}
			},
		},
		{
			name: "UpdateDatabaseRow invalidates the row page",
			write: func(c *CachedNotionClient) error {
				_, err := c.UpdateDatabaseRow(ctx, "row", nil)
				return err
			},
			check: func(t *testing.T, c *CachedNotionClient) {
				page, err := c.GetPage(ctx, "row")
				if err != nil || page.URL != "updated" {
					t.Errorf("row page = %+v, %v; want updated", page, err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewCachedNotionClient(newWriteMock(), NewNotionCache(10, time.Hour))
			// Warm the cache.
			if _, err := client.GetPageBlocks(ctx, "parent"); err != nil {
				t.Fatalf("GetPageBlocks() error = %v", err)
			}
			if _, err := client.GetPage(ctx, "row"); err != nil {
				t.Fatalf("GetPage() error = %v", err)
			}

			if err := tt.write(client); err != nil {
				t.Fatalf("write error = %v", err)
			}
			tt.check(t, client)
		})
	}
}

// TestCachedNotionClient_UpdatePageBlocksInvalidatesAfterWrite ensures a
// read that happens while the update is in flight cannot leave the old
// blocks cached.
func TestCachedNotionClient_UpdatePageBlocksInvalidatesAfterWrite(t *testing.T) {
	ctx := context.Background()
	mock := newWriteMock()
	client := NewCachedNotionClient(mock, NewNotionCache(10, time.Hour))

	mock.onUpdateBlocks = func(pageID string) {
		// Concurrent reader fetches (and caches) the old blocks before the
		// server applies the update.
		if _, err := client.GetPageBlocks(ctx, pageID); err != nil {
			t.Errorf("GetPageBlocks() error = %v", err)
		}
	}

	if err := client.UpdatePageBlocks(ctx, "page", nil); err != nil {
		t.Fatalf("UpdatePageBlocks() error = %v", err)
	}

	blocks, err := client.GetPageBlocks(ctx, "page")
	if err != nil {
		t.Fatalf("GetPageBlocks() error = %v", err)
	}
	if len(blocks) != 1 || blocks[0].ID != "new" {
		t.Errorf("blocks = %+v, want the updated blocks", blocks)
	}
}

func (m *writeMockClient) UpdatePageProperties(ctx context.Context, pageID string, properties map[string]interface{}) (*notion.Page, error) {
	return &notion.Page{ID: pageID}, nil
}
