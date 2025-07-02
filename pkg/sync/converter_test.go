package sync

import (
	"reflect"
	"testing"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

func TestConverter_MarkdownToBlocks(t *testing.T) {
	converter := NewConverter()

	tests := []struct {
		name     string
		markdown string
		want     []map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "simple heading",
			markdown: "# Hello World",
			want: []map[string]interface{}{
				{
					"type": "heading_1",
					"heading_1": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "Hello World",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "paragraph",
			markdown: "This is a paragraph.",
			want: []map[string]interface{}{
				{
					"type": "paragraph",
					"paragraph": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "This is a paragraph.",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "fenced code block",
			markdown: "```python\ndef hello():\n    print(\"Hello!\")\n```",
			want: []map[string]interface{}{
				{
					"type": "code",
					"code": map[string]interface{}{
						"language": "python",
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "def hello():\n    print(\"Hello!\")",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "code block with unknown language",
			markdown: "```unknownlang\nsome code\n```",
			want: []map[string]interface{}{
				{
					"type": "code",
					"code": map[string]interface{}{
						"language": "plain text",
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "some code",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "code block with js alias",
			markdown: "```js\nconsole.log('hello');\n```",
			want: []map[string]interface{}{
				{
					"type": "code",
					"code": map[string]interface{}{
						"language": "javascript",
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "console.log('hello');",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "bulleted list",
			markdown: "- First item\n- Second item",
			want: []map[string]interface{}{
				{
					"type": "bulleted_list_item",
					"bulleted_list_item": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "First item",
								},
							},
						},
					},
				},
				{
					"type": "bulleted_list_item",
					"bulleted_list_item": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "Second item",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "numbered list",
			markdown: "1. First item\n2. Second item",
			want: []map[string]interface{}{
				{
					"type": "numbered_list_item",
					"numbered_list_item": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "First item",
								},
							},
						},
					},
				},
				{
					"type": "numbered_list_item",
					"numbered_list_item": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "Second item",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "multiple headings",
			markdown: "# H1\n## H2\n### H3\n#### H4",
			want: []map[string]interface{}{
				{
					"type": "heading_1",
					"heading_1": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "H1",
								},
							},
						},
					},
				},
				{
					"type": "heading_2",
					"heading_2": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "H2",
								},
							},
						},
					},
				},
				{
					"type": "heading_3",
					"heading_3": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "H3",
								},
							},
						},
					},
				},
				{
					"type": "heading_3", // H4 becomes H3 in Notion
					"heading_3": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "H4",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "empty content",
			markdown: "",
			want:     []map[string]interface{}{},
			wantErr:  false,
		},
		{
			name: "simple table",
			markdown: `| Header 1 | Header 2 |
| --- | --- |
| Cell 1 | Cell 2 |
| Cell 3 | Cell 4 |`,
			want: []map[string]interface{}{
				{
					"type": "table",
					"table": map[string]interface{}{
						"table_width":       2,
						"has_column_header": true,
						"has_row_header":    false,
					},
				},
				{
					"type": "table_row",
					"table_row": map[string]interface{}{
						"cells": [][]map[string]interface{}{
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Header 1",
									},
								},
							},
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Header 2",
									},
								},
							},
						},
					},
				},
				{
					"type": "table_row",
					"table_row": map[string]interface{}{
						"cells": [][]map[string]interface{}{
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Cell 1",
									},
								},
							},
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Cell 2",
									},
								},
							},
						},
					},
				},
				{
					"type": "table_row",
					"table_row": map[string]interface{}{
						"cells": [][]map[string]interface{}{
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Cell 3",
									},
								},
							},
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Cell 4",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "table with three columns",
			markdown: `| Name | Age | City |
| --- | --- | --- |
| Alice | 30 | New York |
| Bob | 25 | London |`,
			want: []map[string]interface{}{
				{
					"type": "table",
					"table": map[string]interface{}{
						"table_width":       3,
						"has_column_header": true,
						"has_row_header":    false,
					},
				},
				{
					"type": "table_row",
					"table_row": map[string]interface{}{
						"cells": [][]map[string]interface{}{
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Name",
									},
								},
							},
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Age",
									},
								},
							},
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "City",
									},
								},
							},
						},
					},
				},
				{
					"type": "table_row",
					"table_row": map[string]interface{}{
						"cells": [][]map[string]interface{}{
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Alice",
									},
								},
							},
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "30",
									},
								},
							},
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "New York",
									},
								},
							},
						},
					},
				},
				{
					"type": "table_row",
					"table_row": map[string]interface{}{
						"cells": [][]map[string]interface{}{
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "Bob",
									},
								},
							},
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "25",
									},
								},
							},
							{
								{
									"type": "text",
									"text": map[string]interface{}{
										"content": "London",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "image block",
			markdown: "![Alt text](https://example.com/image.png)",
			want: []map[string]interface{}{
				{
					"type": "image",
					"image": map[string]interface{}{
						"type": "external",
						"external": map[string]interface{}{
							"url": "https://example.com/image.png",
						},
						"caption": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "Alt text",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "blockquote as callout",
			markdown: "> This is a callout",
			want: []map[string]interface{}{
				{
					"type": "callout",
					"callout": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "This is a callout",
								},
							},
						},
						"color": "gray_background",
					},
				},
			},
			wantErr: false,
		},
		{
			name:     "divider block",
			markdown: "---",
			want: []map[string]interface{}{
				{
					"type":    "divider",
					"divider": map[string]interface{}{},
				},
			},
			wantErr: false,
		},
		{
			name:     "toggle block",
			markdown: "<details>\n<summary>Click to expand</summary>\n</details>",
			want: []map[string]interface{}{
				{
					"type": "toggle",
					"toggle": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "Click to expand",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nested list",
			markdown: `- Item 1
  - Nested item 1
  - Nested item 2
    - Deep nested item
- Item 2`,
			want: []map[string]interface{}{
				{
					"type": "bulleted_list_item",
					"bulleted_list_item": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "Item 1",
								},
							},
						},
					},
				},
				{
					"type": "bulleted_list_item",
					"bulleted_list_item": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "  Nested item 1",
								},
							},
						},
					},
				},
				{
					"type": "bulleted_list_item",
					"bulleted_list_item": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "  Nested item 2",
								},
							},
						},
					},
				},
				{
					"type": "bulleted_list_item",
					"bulleted_list_item": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "    Deep nested item",
								},
							},
						},
					},
				},
				{
					"type": "bulleted_list_item",
					"bulleted_list_item": map[string]interface{}{
						"rich_text": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"content": "Item 2",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := converter.MarkdownToBlocks(tt.markdown)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarkdownToBlocks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Handle empty slice vs nil comparison
			if len(got) == 0 && len(tt.want) == 0 {
				// Both are empty, test passes
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MarkdownToBlocks() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConverter_BlocksToMarkdown(t *testing.T) {
	converter := NewConverter()

	tests := []struct {
		name    string
		blocks  []notion.Block
		want    string
		wantErr bool
	}{
		{
			name: "heading blocks",
			blocks: []notion.Block{
				{
					Type: "heading_1",
					Heading1: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "Hello World"},
						},
					},
				},
				{
					Type: "heading_2",
					Heading2: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "Subtitle"},
						},
					},
				},
			},
			want:    "# Hello World\n\n## Subtitle",
			wantErr: false,
		},
		{
			name: "paragraph block",
			blocks: []notion.Block{
				{
					Type: "paragraph",
					Paragraph: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "This is a paragraph."},
						},
					},
				},
			},
			want:    "This is a paragraph.",
			wantErr: false,
		},
		{
			name: "code block",
			blocks: []notion.Block{
				{
					Type: "code",
					Code: &notion.CodeBlock{
						RichText: []notion.RichText{
							{PlainText: "def hello():\n    print(\"Hello!\")"},
						},
						Language: "python",
					},
				},
			},
			want:    "```python\ndef hello():\n    print(\"Hello!\")\n```",
			wantErr: false,
		},
		{
			name: "list blocks",
			blocks: []notion.Block{
				{
					Type: "bulleted_list_item",
					BulletedListItem: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "First item"},
						},
					},
				},
				{
					Type: "numbered_list_item",
					NumberedListItem: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "Second item"},
						},
					},
				},
			},
			want:    "- First item\n1. Second item",
			wantErr: false,
		},
		{
			name: "quote block",
			blocks: []notion.Block{
				{
					Type: "quote",
					Quote: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "This is a quote"},
						},
					},
				},
			},
			want:    "> This is a quote",
			wantErr: false,
		},
		{
			name:    "empty blocks",
			blocks:  []notion.Block{},
			want:    "",
			wantErr: false,
		},
		{
			name: "table blocks",
			blocks: []notion.Block{
				{
					Type: "table",
					Table: &notion.TableBlock{
						TableWidth:      2,
						HasColumnHeader: true,
						HasRowHeader:    false,
					},
				},
				{
					Type: "table_row",
					TableRow: &notion.TableRowBlock{
						Cells: [][]notion.RichText{
							{{PlainText: "Header 1"}},
							{{PlainText: "Header 2"}},
						},
					},
				},
				{
					Type: "table_row",
					TableRow: &notion.TableRowBlock{
						Cells: [][]notion.RichText{
							{{PlainText: "Cell 1"}},
							{{PlainText: "Cell 2"}},
						},
					},
				},
			},
			want:    "| Header 1 | Header 2 |\n| --- | --- |\n| Cell 1 | Cell 2 |",
			wantErr: false,
		},
		{
			name: "image block",
			blocks: []notion.Block{
				{
					Type: "image",
					Image: &notion.ImageBlock{
						Type: "external",
						External: &notion.ExternalFile{
							URL: "https://example.com/image.png",
						},
						Caption: []notion.RichText{
							{PlainText: "Test image"},
						},
					},
				},
			},
			want:    "![Test image](https://example.com/image.png)",
			wantErr: false,
		},
		{
			name: "callout block",
			blocks: []notion.Block{
				{
					Type: "callout",
					Callout: &notion.CalloutBlock{
						RichText: []notion.RichText{
							{PlainText: "This is a callout"},
						},
						Icon: &notion.CalloutIcon{
							Type:  "emoji",
							Emoji: "💡",
						},
						Color: "gray_background",
					},
				},
			},
			want:    "> 💡 This is a callout",
			wantErr: false,
		},
		{
			name: "toggle block",
			blocks: []notion.Block{
				{
					Type: "toggle",
					Toggle: &notion.ToggleBlock{
						RichText: []notion.RichText{
							{PlainText: "Click to expand"},
						},
					},
				},
			},
			want:    "<details>\n<summary>Click to expand</summary>\n\n</details>",
			wantErr: false,
		},
		{
			name: "bookmark block",
			blocks: []notion.Block{
				{
					Type: "bookmark",
					Bookmark: &notion.BookmarkBlock{
						URL: "https://example.com",
						Caption: []notion.RichText{
							{PlainText: "Example website"},
						},
					},
				},
			},
			want:    "[Example website](https://example.com)",
			wantErr: false,
		},
		{
			name: "divider block",
			blocks: []notion.Block{
				{
					Type:    "divider",
					Divider: &notion.DividerBlock{},
				},
			},
			want:    "---",
			wantErr: false,
		},
		{
			name: "nested list blocks",
			blocks: []notion.Block{
				{
					Type: "bulleted_list_item",
					BulletedListItem: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "Item 1"},
						},
					},
				},
				{
					Type: "bulleted_list_item",
					BulletedListItem: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "  Nested item 1"},
						},
					},
				},
				{
					Type: "bulleted_list_item",
					BulletedListItem: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "  Nested item 2"},
						},
					},
				},
				{
					Type: "bulleted_list_item",
					BulletedListItem: &notion.RichTextBlock{
						RichText: []notion.RichText{
							{PlainText: "Item 2"},
						},
					},
				},
			},
			want:    "- Item 1\n-   Nested item 1\n-   Nested item 2\n- Item 2",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := converter.BlocksToMarkdown(tt.blocks)
			if (err != nil) != tt.wantErr {
				t.Errorf("BlocksToMarkdown() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BlocksToMarkdown() got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeNotionLanguage(t *testing.T) {
	tests := []struct {
		name string
		lang string
		want string
	}{
		{"javascript alias", "js", "javascript"},
		{"typescript alias", "ts", "typescript"},
		{"python alias", "py", "python"},
		{"shell alias", "sh", "shell"},
		{"yaml alias", "yml", "yaml"},
		{"valid language", "go", "go"},
		{"valid language case", "JavaScript", "javascript"},
		{"empty language", "", "plain text"},
		{"unknown language", "unknownlang", "plain text"},
		{"dockerfile alias", "dockerfile", "docker"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeNotionLanguage(tt.lang)
			if got != tt.want {
				t.Errorf("normalizeNotionLanguage() = %v, want %v", got, tt.want)
			}
		})
	}
}
