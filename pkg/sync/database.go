package sync

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

// DatabaseSync interface for syncing between Notion databases and CSV files
type DatabaseSync interface {
	SyncNotionDatabaseToCSV(ctx context.Context, databaseID, csvPath string) error
	SyncCSVToNotionDatabase(ctx context.Context, csvPath, databaseID string) error
	CreateDatabaseFromCSV(ctx context.Context, csvPath, parentPageID string) (*notion.Database, error)
}

type databaseSync struct {
	client notion.Client
}

// NewDatabaseSync creates a new DatabaseSync instance
func NewDatabaseSync(client notion.Client) DatabaseSync {
	return &databaseSync{
		client: client,
	}
}

// SyncNotionDatabaseToCSV exports a Notion database to a CSV file
func (ds *databaseSync) SyncNotionDatabaseToCSV(ctx context.Context, databaseID, csvPath string) error {
	// Get database schema
	database, err := ds.client.GetDatabase(ctx, databaseID)
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	// Query all rows
	queryResp, err := ds.client.QueryDatabase(ctx, databaseID, &notion.DatabaseQueryRequest{
		PageSize: intPtr(100), // Notion's max page size
	})
	if err != nil {
		return fmt.Errorf("failed to query database: %w", err)
	}

	// Create CSV file
	file, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to close CSV file: %v\n", err)
		}
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header row
	header := ds.buildCSVHeader(database.Properties)
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	allRows := queryResp.Results

	// Handle pagination
	for queryResp.HasMore && queryResp.NextCursor != nil {
		queryResp, err = ds.client.QueryDatabase(ctx, databaseID, &notion.DatabaseQueryRequest{
			StartCursor: queryResp.NextCursor,
			PageSize:    intPtr(100),
		})
		if err != nil {
			return fmt.Errorf("failed to query database (pagination): %w", err)
		}
		allRows = append(allRows, queryResp.Results...)
	}

	for _, row := range allRows {
		csvRow := ds.convertRowToCSV(row, header)
		if err := writer.Write(csvRow); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// SyncCSVToNotionDatabase imports a CSV file to an existing Notion database
func (ds *databaseSync) SyncCSVToNotionDatabase(ctx context.Context, csvPath, databaseID string) error {
	// Read CSV file
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: failed to close CSV file: %v\n", err)
		}
	}()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("CSV file must have at least a header row and one data row")
	}

	// Get database schema
	database, err := ds.client.GetDatabase(ctx, databaseID)
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	header := records[0]
	dataRows := records[1:]

	// Convert and insert rows
	for i, record := range dataRows {
		properties, err := ds.convertCSVRowToProperties(record, header, database.Properties)
		if err != nil {
			return fmt.Errorf("failed to convert CSV row %d: %w", i+2, err)
		}

		_, err = ds.client.CreateDatabaseRow(ctx, databaseID, properties)
		if err != nil {
			return fmt.Errorf("failed to create database row %d: %w", i+2, err)
		}
	}

	return nil
}

// CreateDatabaseFromCSV creates a new Notion database from a CSV file structure
func (ds *databaseSync) CreateDatabaseFromCSV(ctx context.Context, csvPath, parentPageID string) (*notion.Database, error) {
	// Read CSV file to analyze structure
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: failed to close CSV file: %v\n", err)
		}
	}()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 1 {
		return nil, fmt.Errorf("CSV file must have at least a header row")
	}

	header := records[0]

	// Analyze data types if we have data rows
	var sampleData []string
	if len(records) > 1 {
		sampleData = records[1]
	}

	// Create database schema
	properties := ds.inferPropertiesFromCSV(header, sampleData)

	// Create database
	createReq := &notion.CreateDatabaseRequest{
		Parent: notion.Parent{
			Type:   "page_id",
			PageID: parentPageID,
		},
		Title: []notion.RichText{
			{
				Type: "text",
				Text: &notion.TextContent{
					Content: "CSV Import Database",
				},
				PlainText: "CSV Import Database",
			},
		},
		Properties: properties,
	}

	database, err := ds.client.CreateDatabase(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Import data if we have any
	if len(records) > 1 {
		dataRows := records[1:]
		for i, record := range dataRows {
			props, err := ds.convertCSVRowToProperties(record, header, properties)
			if err != nil {
				return nil, fmt.Errorf("failed to convert CSV row %d: %w", i+2, err)
			}

			_, err = ds.client.CreateDatabaseRow(ctx, database.ID, props)
			if err != nil {
				return nil, fmt.Errorf("failed to create database row %d: %w", i+2, err)
			}
		}
	}

	return database, nil
}

// Helper functions

func (ds *databaseSync) buildCSVHeader(properties map[string]notion.Property) []string {
	var header []string
	for name := range properties {
		header = append(header, name)
	}
	return header
}

func (ds *databaseSync) convertRowToCSV(row notion.DatabaseRow, header []string) []string {
	csvRow := make([]string, len(header))

	for i, columnName := range header {
		if prop, exists := row.Properties[columnName]; exists {
			csvRow[i] = ds.propertyValueToString(prop)
		}
	}

	return csvRow
}

func (ds *databaseSync) propertyValueToString(prop notion.PropertyValue) string {
	switch prop.Type {
	case "title":
		return ds.richTextToString(prop.Title)
	case "rich_text":
		return ds.richTextToString(prop.RichText)
	case "number":
		if prop.Number != nil {
			return strconv.FormatFloat(*prop.Number, 'f', -1, 64)
		}
	case "select":
		if prop.Select != nil {
			return prop.Select.Name
		}
	case "multi_select":
		var names []string
		for _, option := range prop.MultiSelect {
			names = append(names, option.Name)
		}
		return strings.Join(names, ", ")
	case "date":
		if prop.Date != nil && prop.Date.Start != nil {
			return prop.Date.Start.Format("2006-01-02")
		}
	case "checkbox":
		if prop.Checkbox != nil {
			return strconv.FormatBool(*prop.Checkbox)
		}
	case "url":
		if prop.URL != nil {
			return *prop.URL
		}
	case "email":
		if prop.Email != nil {
			return *prop.Email
		}
	case "phone_number":
		if prop.PhoneNumber != nil {
			return *prop.PhoneNumber
		}
	}
	return ""
}

func (ds *databaseSync) richTextToString(richTexts []notion.RichText) string {
	var text strings.Builder
	for _, rt := range richTexts {
		text.WriteString(rt.PlainText)
	}
	return text.String()
}

func (ds *databaseSync) convertCSVRowToProperties(record, header []string, schema map[string]notion.Property) (map[string]notion.PropertyValue, error) {
	properties := make(map[string]notion.PropertyValue)

	for i, value := range record {
		if i >= len(header) {
			break
		}

		columnName := header[i]
		prop, exists := schema[columnName]
		if !exists {
			continue
		}

		propValue, err := ds.stringToPropertyValue(value, prop.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value for column %s: %w", columnName, err)
		}

		properties[columnName] = propValue
	}

	return properties, nil
}

func (ds *databaseSync) stringToPropertyValue(value, propType string) (notion.PropertyValue, error) {
	switch propType {
	case "title":
		return notion.PropertyValue{
			Type: "title",
			Title: []notion.RichText{
				{
					Type:      "text",
					Text:      &notion.TextContent{Content: value},
					PlainText: value,
				},
			},
		}, nil
	case "rich_text":
		return notion.PropertyValue{
			Type: "rich_text",
			RichText: []notion.RichText{
				{
					Type:      "text",
					Text:      &notion.TextContent{Content: value},
					PlainText: value,
				},
			},
		}, nil
	case "number":
		if value == "" {
			return notion.PropertyValue{Type: "number"}, nil
		}
		num, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return notion.PropertyValue{}, fmt.Errorf("invalid number: %s", value)
		}
		return notion.PropertyValue{
			Type:   "number",
			Number: &num,
		}, nil
	case "select":
		if value == "" {
			return notion.PropertyValue{Type: "select"}, nil
		}
		return notion.PropertyValue{
			Type: "select",
			Select: &notion.SelectOption{
				Name: value,
			},
		}, nil
	case "multi_select":
		if value == "" {
			return notion.PropertyValue{Type: "multi_select"}, nil
		}
		// Split by comma for multi-select values
		options := []notion.SelectOption{}
		for _, v := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(v)
			if trimmed != "" {
				options = append(options, notion.SelectOption{Name: trimmed})
			}
		}
		return notion.PropertyValue{
			Type:        "multi_select",
			MultiSelect: options,
		}, nil
	case "checkbox":
		if value == "" {
			return notion.PropertyValue{Type: "checkbox"}, nil
		}
		checkbox, err := strconv.ParseBool(value)
		if err != nil {
			return notion.PropertyValue{}, fmt.Errorf("invalid boolean: %s", value)
		}
		return notion.PropertyValue{
			Type:     "checkbox",
			Checkbox: &checkbox,
		}, nil
	case "url":
		return notion.PropertyValue{
			Type: "url",
			URL:  &value,
		}, nil
	case "email":
		return notion.PropertyValue{
			Type:  "email",
			Email: &value,
		}, nil
	case "phone_number":
		return notion.PropertyValue{
			Type:        "phone_number",
			PhoneNumber: &value,
		}, nil
	case "date":
		if value == "" {
			return notion.PropertyValue{Type: "date"}, nil
		}
		date, err := time.Parse("2006-01-02", value)
		if err != nil {
			return notion.PropertyValue{}, fmt.Errorf("invalid date format (expected YYYY-MM-DD): %s", value)
		}
		notionDate := &notion.NotionDate{Time: date}
		return notion.PropertyValue{
			Type: "date",
			Date: &notion.DateValue{Start: notionDate},
		}, nil
	default:
		// Default to rich text for unknown types
		return notion.PropertyValue{
			Type: "rich_text",
			RichText: []notion.RichText{
				{
					Type:      "text",
					Text:      &notion.TextContent{Content: value},
					PlainText: value,
				},
			},
		}, nil
	}
}

func (ds *databaseSync) inferPropertiesFromCSV(header, sampleData []string) map[string]notion.Property {
	properties := make(map[string]notion.Property)

	for i, columnName := range header {
		var propType string

		// Analyze sample data to infer type
		if i < len(sampleData) && sampleData[i] != "" {
			value := sampleData[i]
			propType = ds.inferPropertyType(value)
		} else {
			propType = "rich_text" // Default type
		}

		// First column is typically the title, override any inferred type
		if i == 0 {
			propType = "title"
		}

		var prop notion.Property
		prop.Type = propType

		// Set type-specific properties (only set the field for the specific type)
		switch propType {
		case "title":
			prop.Title = &notion.TitleProperty{}
		case "rich_text":
			prop.Text = &notion.TextProperty{}
		case "number":
			prop.Number = &notion.NumberProperty{}
		case "checkbox":
			prop.Checkbox = &notion.CheckboxProperty{}
		case "url":
			prop.URL = &notion.URLProperty{}
		case "email":
			prop.Email = &notion.EmailProperty{}
		case "phone_number":
			prop.PhoneNumber = &notion.PhoneNumberProperty{}
		case "date":
			prop.Date = &notion.DateProperty{}
		}

		properties[columnName] = prop
	}

	return properties
}

func (ds *databaseSync) inferPropertyType(value string) string {
	// Try to infer type from value
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return "number"
	}

	if _, err := strconv.ParseBool(value); err == nil {
		return "checkbox"
	}

	if _, err := time.Parse("2006-01-02", value); err == nil {
		return "date"
	}

	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return "url"
	}

	if strings.Contains(value, "@") && strings.Contains(value, ".") {
		return "email"
	}

	// Default to rich text
	return "rich_text"
}

func intPtr(i int) *int {
	return &i
}
