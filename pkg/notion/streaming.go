package notion

import (
	"context"
	"fmt"
)

// PageStream represents a stream of pages
type PageStream struct {
	pages  chan Page
	errors chan error
	done   chan struct{}
}

// NewPageStream creates a new PageStream
func NewPageStream() *PageStream {
	return &PageStream{
		pages:  make(chan Page, 100), // Buffer of 100 pages
		errors: make(chan error, 10), // Buffer for errors
		done:   make(chan struct{}),
	}
}

// Pages returns the channel of pages
func (ps *PageStream) Pages() <-chan Page {
	return ps.pages
}

// SendPage sends a page to the stream (for testing)
func (ps *PageStream) SendPage(page Page) {
	ps.pages <- page
}

// Errors returns the channel of errors
func (ps *PageStream) Errors() <-chan error {
	return ps.errors
}

// Done returns the done channel
func (ps *PageStream) Done() <-chan struct{} {
	return ps.done
}

// Close closes all channels
func (ps *PageStream) Close() {
	close(ps.pages)
	close(ps.errors)
	close(ps.done)
}

// StreamDescendantPages streams descendant pages without loading them all into memory
func (c *client) StreamDescendantPages(ctx context.Context, parentID string) *PageStream {
	stream := NewPageStream()

	go func() {
		defer stream.Close()

		if err := c.streamDescendantPagesRecursive(ctx, parentID, stream); err != nil {
			select {
			case stream.errors <- err:
			case <-ctx.Done():
				return
			}
		}
	}()

	return stream
}

// streamDescendantPagesRecursive recursively streams pages without keeping them all in memory
func (c *client) streamDescendantPagesRecursive(ctx context.Context, parentID string, stream *PageStream) error {
	// Get direct children
	directChildren, err := c.GetChildPages(ctx, parentID)
	if err != nil {
		return fmt.Errorf("failed to get child pages for %s: %w", parentID, err)
	}

	// Stream direct children
	for _, page := range directChildren {
		select {
		case stream.pages <- page:
		case <-ctx.Done():
			return ctx.Err()
		}

		// Recursively stream descendants
		if err := c.streamDescendantPagesRecursive(ctx, page.ID, stream); err != nil {
			// Log warning but continue with other pages
			fmt.Printf("Warning: failed to stream descendants of page %s: %v\n", page.ID, err)
		}
	}

	return nil
}

// DatabaseRowStream represents a stream of database rows
type DatabaseRowStream struct {
	rows   chan DatabaseRow
	errors chan error
	done   chan struct{}
}

// NewDatabaseRowStream creates a new DatabaseRowStream
func NewDatabaseRowStream() *DatabaseRowStream {
	return &DatabaseRowStream{
		rows:   make(chan DatabaseRow, 100), // Buffer of 100 rows
		errors: make(chan error, 10),        // Buffer for errors
		done:   make(chan struct{}),
	}
}

// Rows returns the channel of database rows
func (drs *DatabaseRowStream) Rows() <-chan DatabaseRow {
	return drs.rows
}

// Errors returns the channel of errors
func (drs *DatabaseRowStream) Errors() <-chan error {
	return drs.errors
}

// Done returns the done channel
func (drs *DatabaseRowStream) Done() <-chan struct{} {
	return drs.done
}

// Close closes all channels
func (drs *DatabaseRowStream) Close() {
	close(drs.rows)
	close(drs.errors)
	close(drs.done)
}

// StreamDatabaseRows streams database rows without loading them all into memory
func (c *client) StreamDatabaseRows(ctx context.Context, databaseID string) *DatabaseRowStream {
	stream := NewDatabaseRowStream()

	go func() {
		defer stream.Close()

		// Start with the first page
		request := &DatabaseQueryRequest{
			PageSize: intPtr(100), // Use max page size
		}

		for {
			queryResp, err := c.QueryDatabase(ctx, databaseID, request)
			if err != nil {
				select {
				case stream.errors <- fmt.Errorf("failed to query database: %w", err):
				case <-ctx.Done():
					return
				}
				return
			}

			// Stream each row
			for _, row := range queryResp.Results {
				select {
				case stream.rows <- row:
				case <-ctx.Done():
					return
				}
			}

			// Check if there are more pages
			if !queryResp.HasMore || queryResp.NextCursor == nil {
				break
			}

			// Update request for next page
			request.StartCursor = queryResp.NextCursor
		}
	}()

	return stream
}

// Helper function for int pointer (already exists but including for completeness)
func intPtr(i int) *int {
	return &i
}
