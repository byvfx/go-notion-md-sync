package sync

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/byvfx/go-notion-md-sync/pkg/util"
)

// StreamingDatabaseSync provides streaming database operations to prevent OOM issues
type StreamingDatabaseSync struct {
	client notion.Client
}

// NewStreamingDatabaseSync creates a new streaming database sync instance
func NewStreamingDatabaseSync(client notion.Client) *StreamingDatabaseSync {
	return &StreamingDatabaseSync{
		client: client,
	}
}

// ExportToCSVStreaming exports database to CSV using streaming to handle large datasets
func (sds *StreamingDatabaseSync) ExportToCSVStreaming(ctx context.Context, databaseID, csvPath string) error {
	// Get database schema first
	database, err := sds.client.GetDatabase(ctx, databaseID)
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	// Create CSV file
	file, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			util.WithError(cerr, "Failed to close CSV file")
		}
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header row
	header := sds.buildCSVHeader(database.Properties)
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Stream database rows and write them directly to CSV
	stream := sds.client.StreamDatabaseRows(ctx, databaseID)

	rowCount := 0
	for {
		select {
		case row, ok := <-stream.Rows():
			if !ok {
				// The stream is finished; it may have stopped because of an error.
				if err := stream.Err(); err != nil {
					return fmt.Errorf("failed to export database after %d rows: %w", rowCount, err)
				}
				writer.Flush()
				if err := writer.Error(); err != nil {
					return fmt.Errorf("failed to write CSV file: %w", err)
				}
				util.Success("Successfully exported %d rows to %s", rowCount, csvPath)
				return nil
			}

			if err := writer.Write(sds.buildCSVRow(row, header)); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
			rowCount++

			// Flush every 100 rows to ensure data is written
			if rowCount%100 == 0 {
				writer.Flush()
				if err := writer.Error(); err != nil {
					return fmt.Errorf("failed to flush CSV writer: %w", err)
				}
			}

		case <-ctx.Done():
			return fmt.Errorf("context cancelled while exporting database: %w", ctx.Err())
		}
	}
}

// buildCSVHeader builds the CSV header: the row ID, then the database
// properties in a stable order (title first).
func (sds *StreamingDatabaseSync) buildCSVHeader(properties map[string]notion.Property) []string {
	return append([]string{"ID"}, orderedPropertyNames(properties)...)
}

// buildCSVRow builds a CSV row whose values line up with header.
func (sds *StreamingDatabaseSync) buildCSVRow(dbRow notion.DatabaseRow, header []string) []string {
	row := make([]string, len(header))
	for i, name := range header {
		if i == 0 {
			row[i] = dbRow.ID
			continue
		}
		if prop, ok := dbRow.Properties[name]; ok {
			row[i] = sds.extractPropertyValueFromPropertyValue(prop)
		}
	}
	return row
}

// extractPropertyValueFromPropertyValue extracts string value from PropertyValue
func (sds *StreamingDatabaseSync) extractPropertyValueFromPropertyValue(property notion.PropertyValue) string {
	switch property.Type {
	case "title":
		if len(property.Title) > 0 {
			return property.Title[0].PlainText
		}
	case "rich_text":
		if len(property.RichText) > 0 {
			return property.RichText[0].PlainText
		}
	case "number":
		if property.Number != nil {
			return fmt.Sprintf("%.2f", *property.Number)
		}
	case "select":
		if property.Select != nil {
			return property.Select.Name
		}
	case "multi_select":
		if len(property.MultiSelect) > 0 {
			var names []string
			for _, option := range property.MultiSelect {
				names = append(names, option.Name)
			}
			return strings.Join(names, ", ")
		}
	case "checkbox":
		if property.Checkbox != nil && *property.Checkbox {
			return "true"
		}
		return "false"
	case "date":
		if property.Date != nil && property.Date.Start != nil {
			return property.Date.Start.Format("2006-01-02")
		}
		// Add more property types as needed
	}

	return ""
}

// StreamingPageProcessor processes pages one by one to prevent memory issues
type StreamingPageProcessor struct {
	engine Engine
}

// NewStreamingPageProcessor creates a new streaming page processor
func NewStreamingPageProcessor(engine Engine) *StreamingPageProcessor {
	return &StreamingPageProcessor{
		engine: engine,
	}
}

// ProcessPagesStreaming processes pages using streaming to handle large workspaces
func (spp *StreamingPageProcessor) ProcessPagesStreaming(ctx context.Context, parentID string, processor func(page notion.Page) error) error {
	// Use the streaming client to get pages
	if client, ok := spp.engine.(*engine); ok {
		stream := client.notion.StreamDescendantPages(ctx, parentID)

		processedCount := 0
		errorCount := 0

		for {
			select {
			case page, ok := <-stream.Pages():
				if !ok {
					// Channel closed, we're done
					fmt.Printf("Successfully processed %d pages\n", processedCount)
					return nil
				}

				// Process the page
				if err := processor(page); err != nil {
					errorCount++
					fmt.Printf("Warning: failed to process page %s: %v\n", page.ID, err)

					// If too many errors, fail
					if errorCount > 50 {
						return fmt.Errorf("too many processing errors: %w", err)
					}
				} else {
					processedCount++
				}

				// Progress indication for large operations
				if processedCount%100 == 0 {
					fmt.Printf("Processed %d pages...\n", processedCount)
				}

			case err := <-stream.Errors():
				errorCount++
				fmt.Printf("Warning: streaming error: %v\n", err)

				if errorCount > 10 {
					return fmt.Errorf("too many streaming errors: %w", err)
				}

			case <-ctx.Done():
				return fmt.Errorf("context cancelled while processing pages: %w", ctx.Err())
			}
		}
	}

	return fmt.Errorf("engine does not support streaming")
}
