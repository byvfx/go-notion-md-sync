package notion

import (
	"context"
	"fmt"
	"sync"
)

// streamErrors collects errors from a stream. The first error is kept for
// Err(); every error is also offered on the Errors() channel without blocking.
type streamErrors struct {
	mu    sync.Mutex
	first error
	count int
}

func (se *streamErrors) record(ch chan error, err error) {
	se.mu.Lock()
	if se.first == nil {
		se.first = err
	}
	se.count++
	se.mu.Unlock()
	select {
	case ch <- err:
	default:
	}
}

func (se *streamErrors) err() error {
	se.mu.Lock()
	defer se.mu.Unlock()
	if se.count > 1 {
		return fmt.Errorf("%w (and %d more errors)", se.first, se.count-1)
	}
	return se.first
}

// PageStream represents a stream of pages.
//
// Read pages from Pages() until it is closed, then call Err() to find out
// whether the stream was complete. Errors() is closed together with Pages().
type PageStream struct {
	pages  chan Page
	errors chan error
	done   chan struct{}
	errs   streamErrors
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

// Err returns the first error the stream hit, or nil. It is only meaningful
// after Pages() has been closed.
func (ps *PageStream) Err() error {
	return ps.errs.err()
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

		if err := c.streamDescendantPagesRecursive(ctx, parentID, stream, map[string]bool{parentID: true}); err != nil {
			stream.errs.record(stream.errors, err)
		}
	}()

	return stream
}

// streamDescendantPagesRecursive recursively streams pages without keeping them all in memory
// Pages are sent parents first. A failing branch is recorded as an error and
// the other branches are still streamed.
func (c *client) streamDescendantPagesRecursive(ctx context.Context, parentID string, stream *PageStream, visited map[string]bool) error {
	// Get direct children
	directChildren, err := c.GetChildPages(ctx, parentID)
	if err != nil {
		return fmt.Errorf("failed to get child pages for %s: %w", parentID, err)
	}

	// Stream direct children
	for _, page := range directChildren {
		if visited[page.ID] {
			continue
		}
		visited[page.ID] = true
		select {
		case stream.pages <- page:
		case <-ctx.Done():
			return ctx.Err()
		}

		if err := c.streamDescendantPagesRecursive(ctx, page.ID, stream, visited); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			stream.errs.record(stream.errors, err)
		}
	}

	return nil
}

// DatabaseRowStream represents a stream of database rows.
//
// Read rows from Rows() until it is closed, then call Err().
type DatabaseRowStream struct {
	rows   chan DatabaseRow
	errors chan error
	done   chan struct{}
	errs   streamErrors
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

// Err returns the first error the stream hit, or nil. It is only meaningful
// after Rows() has been closed.
func (drs *DatabaseRowStream) Err() error {
	return drs.errs.err()
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
				stream.errs.record(stream.errors, fmt.Errorf("failed to query database: %w", err))
				return
			}

			// Stream each row
			for _, row := range queryResp.Results {
				select {
				case stream.rows <- row:
				case <-ctx.Done():
					stream.errs.record(stream.errors, ctx.Err())
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
