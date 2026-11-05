package cache

import (
	"context"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

// UpdatePageProperties updates a page's properties and drops the cached page.
func (c *CachedNotionClient) UpdatePageProperties(ctx context.Context, pageID string, properties map[string]interface{}) (*notion.Page, error) {
	page, err := c.client.UpdatePageProperties(ctx, pageID, properties)
	c.cache.InvalidatePage(pageID)
	return page, err
}
