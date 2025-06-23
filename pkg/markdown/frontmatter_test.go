package markdown

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     *FrontmatterFields
		wantErr  bool
	}{
		{
			name: "complete frontmatter",
			metadata: map[string]interface{}{
				"title":        "Test Title",
				"notion_id":    "abc123",
				"created_at":   "2023-01-01T10:00:00Z",
				"updated_at":   "2023-01-02T11:00:00Z",
				"tags":         []interface{}{"tag1", "tag2"},
				"status":       "published",
				"sync_enabled": true,
				"properties": map[string]interface{}{
					"custom": "value",
				},
			},
			want: &FrontmatterFields{
				Title:       "Test Title",
				NotionID:    "abc123",
				CreatedAt:   mustParseTime("2023-01-01T10:00:00Z"),
				UpdatedAt:   mustParseTime("2023-01-02T11:00:00Z"),
				Tags:        []string{"tag1", "tag2"},
				Status:      "published",
				SyncEnabled: true,
				Properties: map[string]interface{}{
					"custom": "value",
				},
			},
			wantErr: false,
		},
		{
			name: "minimal frontmatter",
			metadata: map[string]interface{}{
				"title": "Just Title",
			},
			want: &FrontmatterFields{
				Title:       "Just Title",
				SyncEnabled: true, // Default
			},
			wantErr: false,
		},
		{
			name:     "empty metadata",
			metadata: map[string]interface{}{},
			want: &FrontmatterFields{
				SyncEnabled: true, // Default
			},
			wantErr: false,
		},
		{
			name: "sync disabled",
			metadata: map[string]interface{}{
				"title":        "Test",
				"sync_enabled": false,
			},
			want: &FrontmatterFields{
				Title:       "Test",
				SyncEnabled: false,
			},
			wantErr: false,
		},
		{
			name: "invalid tag types ignored",
			metadata: map[string]interface{}{
				"tags": []interface{}{"valid", 123, "another"}, // 123 should be ignored
			},
			want: &FrontmatterFields{
				Tags:        []string{"valid", "another"},
				SyncEnabled: true,
			},
			wantErr: false,
		},
		{
			name: "invalid time ignored",
			metadata: map[string]interface{}{
				"created_at": "invalid-time",
				"updated_at": 12345, // Invalid type
			},
			want: &FrontmatterFields{
				SyncEnabled: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractFrontmatter(tt.metadata)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want.Title, got.Title)
			assert.Equal(t, tt.want.NotionID, got.NotionID)
			assert.Equal(t, tt.want.Tags, got.Tags)
			assert.Equal(t, tt.want.Status, got.Status)
			assert.Equal(t, tt.want.SyncEnabled, got.SyncEnabled)
			assert.Equal(t, tt.want.Properties, got.Properties)

			if tt.want.CreatedAt != nil {
				require.NotNil(t, got.CreatedAt)
				assert.True(t, tt.want.CreatedAt.Equal(*got.CreatedAt))
			} else {
				assert.Nil(t, got.CreatedAt)
			}

			if tt.want.UpdatedAt != nil {
				require.NotNil(t, got.UpdatedAt)
				assert.True(t, tt.want.UpdatedAt.Equal(*got.UpdatedAt))
			} else {
				assert.Nil(t, got.UpdatedAt)
			}
		})
	}
}

func TestFrontmatterFields_ToMetadata(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		fm   *FrontmatterFields
		want map[string]interface{}
	}{
		{
			name: "complete frontmatter",
			fm: &FrontmatterFields{
				Title:       "Test Title",
				NotionID:    "abc123",
				CreatedAt:   &now,
				UpdatedAt:   &now,
				Tags:        []string{"tag1", "tag2"},
				Status:      "published",
				SyncEnabled: true,
				Properties: map[string]interface{}{
					"custom": "value",
				},
			},
			want: map[string]interface{}{
				"title":        "Test Title",
				"notion_id":    "abc123",
				"created_at":   now.Format(time.RFC3339),
				"updated_at":   now.Format(time.RFC3339),
				"tags":         []string{"tag1", "tag2"},
				"status":       "published",
				"sync_enabled": true,
				"properties": map[string]interface{}{
					"custom": "value",
				},
			},
		},
		{
			name: "minimal frontmatter",
			fm: &FrontmatterFields{
				Title:       "Just Title",
				SyncEnabled: false,
			},
			want: map[string]interface{}{
				"title":        "Just Title",
				"sync_enabled": false,
			},
		},
		{
			name: "empty frontmatter",
			fm: &FrontmatterFields{
				SyncEnabled: true,
			},
			want: map[string]interface{}{
				"sync_enabled": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fm.ToMetadata()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name      string
		timeVal   interface{}
		wantErr   bool
		checkTime func(t *testing.T, result time.Time)
	}{
		{
			name:    "RFC3339 string",
			timeVal: "2023-01-01T10:00:00Z",
			wantErr: false,
			checkTime: func(t *testing.T, result time.Time) {
				expected := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)
				assert.True(t, expected.Equal(result))
			},
		},
		{
			name:    "RFC3339Nano string",
			timeVal: "2023-01-01T10:00:00.123456789Z",
			wantErr: false,
			checkTime: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2023, result.Year())
				assert.Equal(t, time.January, result.Month())
			},
		},
		{
			name:    "Date only string",
			timeVal: "2023-01-01",
			wantErr: false,
			checkTime: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2023, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 1, result.Day())
			},
		},
		{
			name:    "time.Time value",
			timeVal: time.Date(2023, 5, 15, 12, 30, 0, 0, time.UTC),
			wantErr: false,
			checkTime: func(t *testing.T, result time.Time) {
				expected := time.Date(2023, 5, 15, 12, 30, 0, 0, time.UTC)
				assert.True(t, expected.Equal(result))
			},
		},
		{
			name:    "invalid string",
			timeVal: "not-a-time",
			wantErr: true,
		},
		{
			name:    "invalid type",
			timeVal: 12345,
			wantErr: true,
		},
		{
			name:    "nil value",
			timeVal: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTime(tt.timeVal)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.checkTime != nil {
				tt.checkTime(t, result)
			}
		})
	}
}

func TestFrontmatterRoundTrip(t *testing.T) {
	// Test that extracting frontmatter and converting back to metadata preserves data
	now := time.Now().Truncate(time.Second) // Truncate to avoid nano precision issues

	original := map[string]interface{}{
		"title":        "Round Trip Test",
		"notion_id":    "test123",
		"created_at":   now.Format(time.RFC3339),
		"updated_at":   now.Format(time.RFC3339),
		"tags":         []interface{}{"go", "testing"},
		"status":       "draft",
		"sync_enabled": true,
		"properties": map[string]interface{}{
			"priority": "high",
		},
	}

	// Extract frontmatter
	fm, err := ExtractFrontmatter(original)
	require.NoError(t, err)

	// Convert back to metadata
	result := fm.ToMetadata()

	// Verify key fields are preserved
	assert.Equal(t, original["title"], result["title"])
	assert.Equal(t, original["notion_id"], result["notion_id"])
	assert.Equal(t, original["status"], result["status"])
	assert.Equal(t, original["sync_enabled"], result["sync_enabled"])

	// Tags should be converted to []string
	assert.Equal(t, []string{"go", "testing"}, result["tags"])

	// Times should be in RFC3339 format
	assert.Equal(t, now.Format(time.RFC3339), result["created_at"])
	assert.Equal(t, now.Format(time.RFC3339), result["updated_at"])

	// Properties should be preserved
	assert.Equal(t, original["properties"], result["properties"])
}

// Helper function for tests
func mustParseTime(timeStr string) *time.Time {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		panic(err)
	}
	return &t
}
