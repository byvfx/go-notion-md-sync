package markdown

import (
	"fmt"
	"time"
)

// FrontmatterFields defines common frontmatter fields for Notion integration
type FrontmatterFields struct {
	Title       string                 `yaml:"title,omitempty"`
	NotionID    string                 `yaml:"notion_id,omitempty"`
	CreatedAt   *time.Time             `yaml:"created_at,omitempty"`
	UpdatedAt   *time.Time             `yaml:"updated_at,omitempty"`
	Tags        []string               `yaml:"tags,omitempty"`
	Status      string                 `yaml:"status,omitempty"`
	Properties  map[string]interface{} `yaml:"properties,omitempty"`
	SyncEnabled bool                   `yaml:"sync_enabled,omitempty"`
}

// ExtractFrontmatter extracts and validates frontmatter from metadata
func ExtractFrontmatter(metadata map[string]interface{}) (*FrontmatterFields, error) {
	fm := &FrontmatterFields{
		SyncEnabled: true, // Default to enabled
	}

	if title, ok := metadata["title"].(string); ok {
		fm.Title = title
	}

	if notionID, ok := metadata["notion_id"].(string); ok {
		fm.NotionID = notionID
	}

	if createdAt, ok := metadata["created_at"]; ok {
		if t, err := parseTime(createdAt); err == nil {
			fm.CreatedAt = &t
		}
	}

	if updatedAt, ok := metadata["updated_at"]; ok {
		if t, err := parseTime(updatedAt); err == nil {
			fm.UpdatedAt = &t
		}
	}

	if tags, ok := metadata["tags"]; ok {
		if tagList, ok := tags.([]interface{}); ok {
			for _, tag := range tagList {
				if tagStr, ok := tag.(string); ok {
					fm.Tags = append(fm.Tags, tagStr)
				}
			}
		}
	}

	if status, ok := metadata["status"].(string); ok {
		fm.Status = status
	}

	if syncEnabled, ok := metadata["sync_enabled"].(bool); ok {
		fm.SyncEnabled = syncEnabled
	}

	if properties, ok := metadata["properties"].(map[string]interface{}); ok {
		fm.Properties = properties
	}

	return fm, nil
}

// ToMetadata converts frontmatter fields back to metadata map
func (fm *FrontmatterFields) ToMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})

	if fm.Title != "" {
		metadata["title"] = fm.Title
	}

	if fm.NotionID != "" {
		metadata["notion_id"] = fm.NotionID
	}

	if fm.CreatedAt != nil {
		metadata["created_at"] = fm.CreatedAt.Format(time.RFC3339)
	}

	if fm.UpdatedAt != nil {
		metadata["updated_at"] = fm.UpdatedAt.Format(time.RFC3339)
	}

	if len(fm.Tags) > 0 {
		metadata["tags"] = fm.Tags
	}

	if fm.Status != "" {
		metadata["status"] = fm.Status
	}

	metadata["sync_enabled"] = fm.SyncEnabled

	if len(fm.Properties) > 0 {
		metadata["properties"] = fm.Properties
	}

	return metadata
}

// parseTime attempts to parse various time formats
func parseTime(timeVal interface{}) (time.Time, error) {
	switch v := timeVal.(type) {
	case string:
		// Try various formats
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}

		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("unable to parse time string: %s", v)
	case time.Time:
		return v, nil
	default:
		return time.Time{}, fmt.Errorf("unsupported time type: %T", timeVal)
	}
}
