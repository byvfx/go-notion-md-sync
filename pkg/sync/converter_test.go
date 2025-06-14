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