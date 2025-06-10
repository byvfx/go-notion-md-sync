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
	ID          string                 `json:"id,omitempty"`
	Object      string                 `json:"object,omitempty"`
	Type        string                 `json:"type"`
	CreatedTime time.Time              `json:"created_time,omitempty"`
	HasChildren bool                   `json:"has_children,omitempty"`
	Content     map[string]interface{} `json:",inline"`
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