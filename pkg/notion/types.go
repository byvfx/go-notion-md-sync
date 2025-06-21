package notion

import "time"

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
