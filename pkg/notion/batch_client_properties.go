package notion

import "context"

// UpdatePageProperties uses round-robin client selection
func (bc *BatchClient) UpdatePageProperties(ctx context.Context, pageID string, properties map[string]interface{}) (*Page, error) {
	return bc.GetClient().UpdatePageProperties(ctx, pageID, properties)
}
