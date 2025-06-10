package sync

import (
	"fmt"
	"strings"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Converter interface {
	MarkdownToBlocks(content string) ([]map[string]interface{}, error)
	BlocksToMarkdown(blocks []notion.Block) (string, error)
}

type converter struct{}

func NewConverter() Converter {
	return &converter{}
}

func (c *converter) MarkdownToBlocks(content string) ([]map[string]interface{}, error) {
	// Parse markdown into AST
	parser := goldmark.New().Parser()
	reader := text.NewReader([]byte(content))
	doc := parser.Parse(reader)

	var blocks []map[string]interface{}
	source := []byte(content)

	// Walk the AST and convert nodes to Notion blocks
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n.Kind() {
		case ast.KindHeading:
			heading := n.(*ast.Heading)
			text := extractTextFromNode(heading, source)
			block := createHeadingBlock(heading.Level, text)
			blocks = append(blocks, block)
			return ast.WalkSkipChildren, nil

		case ast.KindParagraph:
			paragraph := n.(*ast.Paragraph)
			text := extractTextFromNode(paragraph, source)
			if strings.TrimSpace(text) != "" {
				block := createParagraphBlock(text)
				blocks = append(blocks, block)
			}
			return ast.WalkSkipChildren, nil

		case ast.KindList:
			list := n.(*ast.List)
			listBlocks := c.convertListToBlocks(list, source)
			blocks = append(blocks, listBlocks...)
			return ast.WalkSkipChildren, nil

		case ast.KindCodeBlock:
			codeBlock := n.(*ast.CodeBlock)
			text := extractTextFromNode(codeBlock, source)
			language := "" // TODO: Extract language from fenced code block info
			block := createCodeBlock(text, language)
			blocks = append(blocks, block)
			return ast.WalkSkipChildren, nil
		}

		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to convert markdown to blocks: %w", err)
	}

	return blocks, nil
}

func (c *converter) BlocksToMarkdown(blocks []notion.Block) (string, error) {
	var md strings.Builder

	for _, block := range blocks {
		switch block.Type {
		case "heading_1":
			if richTexts := getRichTexts(block.Content, "heading_1"); len(richTexts) > 0 {
				md.WriteString("# " + extractPlainText(richTexts) + "\n\n")
			}

		case "heading_2":
			if richTexts := getRichTexts(block.Content, "heading_2"); len(richTexts) > 0 {
				md.WriteString("## " + extractPlainText(richTexts) + "\n\n")
			}

		case "heading_3":
			if richTexts := getRichTexts(block.Content, "heading_3"); len(richTexts) > 0 {
				md.WriteString("### " + extractPlainText(richTexts) + "\n\n")
			}

		case "paragraph":
			if richTexts := getRichTexts(block.Content, "paragraph"); len(richTexts) > 0 {
				md.WriteString(extractPlainText(richTexts) + "\n\n")
			}

		case "bulleted_list_item":
			if richTexts := getRichTexts(block.Content, "bulleted_list_item"); len(richTexts) > 0 {
				md.WriteString("- " + extractPlainText(richTexts) + "\n")
			}

		case "numbered_list_item":
			if richTexts := getRichTexts(block.Content, "numbered_list_item"); len(richTexts) > 0 {
				md.WriteString("1. " + extractPlainText(richTexts) + "\n")
			}

		case "code":
			if codeData, ok := block.Content["code"].(map[string]interface{}); ok {
				if richTexts, ok := codeData["rich_text"].([]interface{}); ok {
					language := ""
					if lang, ok := codeData["language"].(string); ok {
						language = lang
					}
					
					code := extractPlainTextFromInterface(richTexts)
					md.WriteString("```" + language + "\n" + code + "\n```\n\n")
				}
			}

		case "divider":
			md.WriteString("---\n\n")
		}
	}

	return strings.TrimSpace(md.String()), nil
}

// Helper functions

func extractTextFromNode(node ast.Node, source []byte) string {
	var buf strings.Builder
	
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n.Kind() {
			case ast.KindText:
				textNode := n.(*ast.Text)
				buf.Write(textNode.Segment.Value(source))
			case ast.KindString:
				stringNode := n.(*ast.String)
				buf.Write(stringNode.Value)
			}
		}
		return ast.WalkContinue, nil
	})
	
	return buf.String()
}

func createHeadingBlock(level int, text string) map[string]interface{} {
	blockType := fmt.Sprintf("heading_%d", level)
	if level > 3 {
		blockType = "heading_3"
	}

	return map[string]interface{}{
		"type": blockType,
		blockType: map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]interface{}{
						"content": text,
					},
				},
			},
		},
	}
}

func createParagraphBlock(text string) map[string]interface{} {
	return map[string]interface{}{
		"type": "paragraph",
		"paragraph": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]interface{}{
						"content": text,
					},
				},
			},
		},
	}
}

func createCodeBlock(text, language string) map[string]interface{} {
	return map[string]interface{}{
		"type": "code",
		"code": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]interface{}{
						"content": text,
					},
				},
			},
			"language": language,
		},
	}
}

func (c *converter) convertListToBlocks(list *ast.List, source []byte) []map[string]interface{} {
	var blocks []map[string]interface{}
	
	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		if listItem, ok := child.(*ast.ListItem); ok {
			text := extractTextFromNode(listItem, source)
			blockType := "bulleted_list_item"
			if list.IsOrdered() {
				blockType = "numbered_list_item"
			}
			
			block := map[string]interface{}{
				"type": blockType,
				blockType: map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{
							"type": "text",
							"text": map[string]interface{}{
								"content": text,
							},
						},
					},
				},
			}
			blocks = append(blocks, block)
		}
	}
	
	return blocks
}

func getRichTexts(content map[string]interface{}, blockType string) []interface{} {
	if blockData, ok := content[blockType].(map[string]interface{}); ok {
		if richTexts, ok := blockData["rich_text"].([]interface{}); ok {
			return richTexts
		}
	}
	return nil
}

func extractPlainText(richTexts []interface{}) string {
	var text strings.Builder
	
	for _, rt := range richTexts {
		if richText, ok := rt.(map[string]interface{}); ok {
			if plainText, ok := richText["plain_text"].(string); ok {
				text.WriteString(plainText)
			}
		}
	}
	
	return text.String()
}

func extractPlainTextFromInterface(richTexts []interface{}) string {
	var text strings.Builder
	
	for _, rt := range richTexts {
		if richText, ok := rt.(map[string]interface{}); ok {
			if plainText, ok := richText["plain_text"].(string); ok {
				text.WriteString(plainText)
			}
		}
	}
	
	return text.String()
}