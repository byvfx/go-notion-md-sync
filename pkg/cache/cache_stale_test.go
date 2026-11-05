package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slowReadClient serves page blocks; the first read is held until released,
// to model a read that started before a write and finishes after it.
type slowReadClient struct {
	notion.Client
	mu       sync.Mutex
	content  string
	hold     chan struct{}
	started  chan struct{}
	readOnce sync.Once
}

func (c *slowReadClient) GetPageBlocks(ctx context.Context, pageID string) ([]notion.Block, error) {
	c.mu.Lock()
	content := c.content
	c.mu.Unlock()
	first := false
	c.readOnce.Do(func() { first = true })
	if first {
		close(c.started)
		<-c.hold
	}
	return []notion.Block{{ID: content, Type: "paragraph"}}, nil
}

func (c *slowReadClient) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	c.mu.Lock()
	c.content = "new"
	c.mu.Unlock()
	return nil
}

func TestCachedClient_ReadFinishingAfterWriteIsNotCached(t *testing.T) {
	client := &slowReadClient{content: "old", hold: make(chan struct{}), started: make(chan struct{})}
	cached := NewCachedNotionClient(client, NewNotionCache(10, time.Hour))
	ctx := context.Background()

	done := make(chan []notion.Block, 1)
	go func() {
		blocks, _ := cached.GetPageBlocks(ctx, "p")
		done <- blocks
	}()
	<-client.started // the read has fetched "old"
	require.NoError(t, cached.UpdatePageBlocks(ctx, "p", nil))
	close(client.hold) // now the old read completes

	assert.Equal(t, "old", (<-done)[0].ID)
	blocks, err := cached.GetPageBlocks(ctx, "p")
	require.NoError(t, err)
	assert.Equal(t, "new", blocks[0].ID, "the stale read must not have been cached")
}
