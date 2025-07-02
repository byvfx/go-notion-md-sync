package notion

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// NotionDate handles multiple date formats from Notion API
type NotionDate struct {
	time.Time
}

func (nd *NotionDate) UnmarshalJSON(data []byte) error {
	// Remove quotes from JSON string
	s := strings.Trim(string(data), `"`)
	if s == "null" || s == "" {
		return nil
	}

	// Try different date formats
	formats := []string{
		time.RFC3339,           // "2006-01-02T15:04:05Z07:00"
		time.RFC3339Nano,       // "2006-01-02T15:04:05.999999999Z07:00"
		"2006-01-02T15:04:05Z", // "2006-01-02T15:04:05Z"
		"2006-01-02T15:04:05",  // "2006-01-02T15:04:05"
		"2006-01-02",           // "2006-01-02" (date only)
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			nd.Time = t
			return nil
		}
	}

	return fmt.Errorf("unable to parse date: %s", s)
}

func (nd NotionDate) MarshalJSON() ([]byte, error) {
	if nd.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(nd.Format(time.RFC3339))
}

type User struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Name   string `json:"name"`
}

type Page struct {
	ID          string                 `json:"id"`
	Object      string                 `json:"object"`
	CreatedTime time.Time              `json:"created_time"`
	CreatedBy   User                   `json:"created_by"`
	Properties  map[string]interface{} `json:"properties"`
	URL         string                 `json:"url"`
	Parent      Parent                 `json:"parent"`
}

type Parent struct {
	Type   string `json:"type"`
	PageID string `json:"page_id,omitempty"`
}

type Block struct {
	ID          string    `json:"id,omitempty"`
	Object      string    `json:"object,omitempty"`
	Type        string    `json:"type"`
	CreatedTime time.Time `json:"created_time,omitempty"`
	HasChildren bool      `json:"has_children,omitempty"`

	// Block type specific content - these are mutually exclusive based on Type
	Paragraph        *RichTextBlock `json:"paragraph,omitempty"`
	Heading1         *RichTextBlock `json:"heading_1,omitempty"`
	Heading2         *RichTextBlock `json:"heading_2,omitempty"`
	Heading3         *RichTextBlock `json:"heading_3,omitempty"`
	BulletedListItem *RichTextBlock `json:"bulleted_list_item,omitempty"`
	NumberedListItem *RichTextBlock `json:"numbered_list_item,omitempty"`
	Code             *CodeBlock     `json:"code,omitempty"`
	Quote            *RichTextBlock `json:"quote,omitempty"`
	Table            *TableBlock    `json:"table,omitempty"`
	TableRow         *TableRowBlock `json:"table_row,omitempty"`
	Image            *ImageBlock    `json:"image,omitempty"`
	Callout          *CalloutBlock  `json:"callout,omitempty"`
	Toggle           *ToggleBlock   `json:"toggle,omitempty"`
	Bookmark         *BookmarkBlock `json:"bookmark,omitempty"`
	Divider          *DividerBlock  `json:"divider,omitempty"`
	Equation         *EquationBlock `json:"equation,omitempty"`

	// For unknown block types, keep the raw content
	Content map[string]interface{} `json:",inline"`
}

type RichTextBlock struct {
	RichText []RichText `json:"rich_text"`
}

type CodeBlock struct {
	RichText []RichText `json:"rich_text"`
	Language string     `json:"language"`
}

type RichText struct {
	Type        string       `json:"type"`
	Text        *TextContent `json:"text,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	PlainText   string       `json:"plain_text"`
}

type TextContent struct {
	Content string `json:"content"`
	Link    *Link  `json:"link,omitempty"`
}

type Link struct {
	URL string `json:"url"`
}

type Annotations struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

type CreatePageRequest struct {
	Parent     Parent                 `json:"parent"`
	Properties map[string]interface{} `json:"properties"`
	Children   []Block                `json:"children,omitempty"`
}

type SearchRequest struct {
	Query  string `json:"query"`
	Filter Filter `json:"filter,omitempty"`
}

type Filter struct {
	Value    string `json:"value"`
	Property string `json:"property"`
}

type SearchResponse struct {
	Results []Page `json:"results"`
}

type BlocksResponse struct {
	Results []Block `json:"results"`
}

type TableBlock struct {
	TableWidth      int  `json:"table_width"`
	HasColumnHeader bool `json:"has_column_header"`
	HasRowHeader    bool `json:"has_row_header"`
}

type TableRowBlock struct {
	Cells [][]RichText `json:"cells"`
}

type ImageBlock struct {
	Type     string        `json:"type"`
	External *ExternalFile `json:"external,omitempty"`
	File     *InternalFile `json:"file,omitempty"`
	Caption  []RichText    `json:"caption,omitempty"`
}

type ExternalFile struct {
	URL string `json:"url"`
}

type InternalFile struct {
	URL        string    `json:"url"`
	ExpiryTime time.Time `json:"expiry_time"`
}

type CalloutBlock struct {
	RichText []RichText   `json:"rich_text"`
	Icon     *CalloutIcon `json:"icon,omitempty"`
	Color    string       `json:"color,omitempty"`
}

type CalloutIcon struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
}

type ToggleBlock struct {
	RichText []RichText `json:"rich_text"`
}

type BookmarkBlock struct {
	URL     string     `json:"url"`
	Caption []RichText `json:"caption,omitempty"`
}

type DividerBlock struct {
	// Divider blocks have no content
}

type EquationBlock struct {
	Expression string `json:"expression"`
}

// Database types
type Database struct {
	ID          string              `json:"id"`
	Object      string              `json:"object"`
	CreatedTime time.Time           `json:"created_time"`
	CreatedBy   User                `json:"created_by"`
	Title       []RichText          `json:"title"`
	Properties  map[string]Property `json:"properties"`
	Parent      Parent              `json:"parent"`
	URL         string              `json:"url"`
}

type Property struct {
	ID             string                  `json:"id"`
	Type           string                  `json:"type"`
	Name           string                  `json:"name,omitempty"`
	Title          *TitleProperty          `json:"title,omitempty"`
	Text           *TextProperty           `json:"rich_text,omitempty"`
	Number         *NumberProperty         `json:"number,omitempty"`
	Select         *SelectProperty         `json:"select,omitempty"`
	MultiSelect    *MultiSelectProperty    `json:"multi_select,omitempty"`
	Date           *DateProperty           `json:"date,omitempty"`
	People         *PeopleProperty         `json:"people,omitempty"`
	Files          *FilesProperty          `json:"files,omitempty"`
	Checkbox       *CheckboxProperty       `json:"checkbox,omitempty"`
	URL            *URLProperty            `json:"url,omitempty"`
	Email          *EmailProperty          `json:"email,omitempty"`
	PhoneNumber    *PhoneNumberProperty    `json:"phone_number,omitempty"`
	Formula        *FormulaProperty        `json:"formula,omitempty"`
	Relation       *RelationProperty       `json:"relation,omitempty"`
	Rollup         *RollupProperty         `json:"rollup,omitempty"`
	CreatedTime    *CreatedTimeProperty    `json:"created_time,omitempty"`
	CreatedBy      *CreatedByProperty      `json:"created_by,omitempty"`
	LastEditedTime *LastEditedTimeProperty `json:"last_edited_time,omitempty"`
	LastEditedBy   *LastEditedByProperty   `json:"last_edited_by,omitempty"`
}

// Property type definitions
type TitleProperty struct{}
type TextProperty struct{}
type NumberProperty struct {
	Format string `json:"format,omitempty"`
}
type SelectProperty struct {
	Options []SelectOption `json:"options"`
}
type MultiSelectProperty struct {
	Options []SelectOption `json:"options"`
}
type DateProperty struct{}
type PeopleProperty struct{}
type FilesProperty struct{}
type CheckboxProperty struct{}
type URLProperty struct{}
type EmailProperty struct{}
type PhoneNumberProperty struct{}
type FormulaProperty struct {
	Expression string `json:"expression"`
}
type RelationProperty struct {
	DatabaseID         string `json:"database_id"`
	SyncedPropertyID   string `json:"synced_property_id,omitempty"`
	SyncedPropertyName string `json:"synced_property_name,omitempty"`
}
type RollupProperty struct {
	RelationPropertyName string `json:"relation_property_name"`
	RelationPropertyID   string `json:"relation_property_id"`
	RollupPropertyName   string `json:"rollup_property_name"`
	RollupPropertyID     string `json:"rollup_property_id"`
	Function             string `json:"function"`
}
type CreatedTimeProperty struct{}
type CreatedByProperty struct{}
type LastEditedTimeProperty struct{}
type LastEditedByProperty struct{}

type SelectOption struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// Database row (page in a database)
type DatabaseRow struct {
	ID          string                   `json:"id"`
	Object      string                   `json:"object"`
	CreatedTime time.Time                `json:"created_time"`
	CreatedBy   User                     `json:"created_by"`
	Properties  map[string]PropertyValue `json:"properties"`
	Parent      Parent                   `json:"parent"`
	URL         string                   `json:"url"`
}

// Property values in database rows
type PropertyValue struct {
	ID             string          `json:"id,omitempty"`
	Type           string          `json:"type"`
	Title          []RichText      `json:"title,omitempty"`
	RichText       []RichText      `json:"rich_text,omitempty"`
	Number         *float64        `json:"number,omitempty"`
	Select         *SelectOption   `json:"select,omitempty"`
	MultiSelect    []SelectOption  `json:"multi_select,omitempty"`
	Date           *DateValue      `json:"date,omitempty"`
	People         []User          `json:"people,omitempty"`
	Files          []FileValue     `json:"files,omitempty"`
	Checkbox       *bool           `json:"checkbox,omitempty"`
	URL            *string         `json:"url,omitempty"`
	Email          *string         `json:"email,omitempty"`
	PhoneNumber    *string         `json:"phone_number,omitempty"`
	Formula        *FormulaValue   `json:"formula,omitempty"`
	Relation       []RelationValue `json:"relation,omitempty"`
	Rollup         *RollupValue    `json:"rollup,omitempty"`
	CreatedTime    *time.Time      `json:"created_time,omitempty"`
	CreatedBy      *User           `json:"created_by,omitempty"`
	LastEditedTime *time.Time      `json:"last_edited_time,omitempty"`
	LastEditedBy   *User           `json:"last_edited_by,omitempty"`
}

type DateValue struct {
	Start    *NotionDate `json:"start"`
	End      *NotionDate `json:"end,omitempty"`
	TimeZone *string     `json:"time_zone,omitempty"`
}

type FileValue struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	File     *InternalFile `json:"file,omitempty"`
	External *ExternalFile `json:"external,omitempty"`
}

type FormulaValue struct {
	Type    string     `json:"type"`
	String  *string    `json:"string,omitempty"`
	Number  *float64   `json:"number,omitempty"`
	Boolean *bool      `json:"boolean,omitempty"`
	Date    *DateValue `json:"date,omitempty"`
}

type RelationValue struct {
	ID string `json:"id"`
}

type RollupValue struct {
	Type   string          `json:"type"`
	Number *float64        `json:"number,omitempty"`
	Date   *DateValue      `json:"date,omitempty"`
	Array  []PropertyValue `json:"array,omitempty"`
}

// Database query and creation requests
type DatabaseQueryRequest struct {
	Filter      *Filter `json:"filter,omitempty"`
	Sorts       []Sort  `json:"sorts,omitempty"`
	StartCursor *string `json:"start_cursor,omitempty"`
	PageSize    *int    `json:"page_size,omitempty"`
}

type Sort struct {
	Property  string `json:"property,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Direction string `json:"direction"`
}

type DatabaseQueryResponse struct {
	Results    []DatabaseRow `json:"results"`
	NextCursor *string       `json:"next_cursor"`
	HasMore    bool          `json:"has_more"`
}

type CreateDatabaseRequest struct {
	Parent     Parent              `json:"parent"`
	Title      []RichText          `json:"title"`
	Properties map[string]Property `json:"properties"`
}
