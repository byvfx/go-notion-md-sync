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
			text := extractCodeBlockContent(codeBlock, source)
			language := extractLanguageFromCodeBlock(codeBlock, source)
			block := createCodeBlock(text, language)
			blocks = append(blocks, block)
			return ast.WalkSkipChildren, nil

		case ast.KindFencedCodeBlock:
			fencedCodeBlock := n.(*ast.FencedCodeBlock)
			text := extractFencedCodeBlockContent(fencedCodeBlock, source)
			language := extractLanguageFromFencedCodeBlock(fencedCodeBlock, source)
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
			if block.Heading1 != nil {
				text := extractPlainTextFromRichText(block.Heading1.RichText)
				md.WriteString("# " + text + "\n\n")
			}

		case "heading_2":
			if block.Heading2 != nil {
				text := extractPlainTextFromRichText(block.Heading2.RichText)
				md.WriteString("## " + text + "\n\n")
			}

		case "heading_3":
			if block.Heading3 != nil {
				text := extractPlainTextFromRichText(block.Heading3.RichText)
				md.WriteString("### " + text + "\n\n")
			}

		case "paragraph":
			if block.Paragraph != nil {
				text := extractPlainTextFromRichText(block.Paragraph.RichText)
				if strings.TrimSpace(text) != "" {
					md.WriteString(text + "\n\n")
				}
			}

		case "bulleted_list_item":
			if block.BulletedListItem != nil {
				text := extractPlainTextFromRichText(block.BulletedListItem.RichText)
				md.WriteString("- " + text + "\n")
			}

		case "numbered_list_item":
			if block.NumberedListItem != nil {
				text := extractPlainTextFromRichText(block.NumberedListItem.RichText)
				md.WriteString("1. " + text + "\n")
			}

		case "code":
			if block.Code != nil {
				code := extractPlainTextFromRichText(block.Code.RichText)
				language := block.Code.Language
				md.WriteString("```" + language + "\n" + code + "\n```\n\n")
			}

		case "quote":
			if block.Quote != nil {
				text := extractPlainTextFromRichText(block.Quote.RichText)
				md.WriteString("> " + text + "\n\n")
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
	// Notion requires a valid language, use "plain text" for empty languages
	if language == "" {
		language = "plain text"
	}

	// Validate language against Notion's supported languages
	language = normalizeNotionLanguage(language)

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

func normalizeNotionLanguage(lang string) string {
	// Map common language names to Notion's expected values
	langMap := map[string]string{
		"js":         "javascript",
		"ts":         "typescript",
		"py":         "python",
		"rb":         "ruby",
		"sh":         "shell",
		"yml":        "yaml",
		"dockerfile": "docker",
		"":           "plain text",
	}

	// Convert to lowercase for comparison
	langLower := strings.ToLower(lang)

	// Check if we have a mapping
	if mapped, exists := langMap[langLower]; exists {
		return mapped
	}

	// Check if it's already a valid Notion language
	validLanguages := []string{
		"abap", "agda", "arduino", "ascii art", "assembly", "bash", "basic", "bnf",
		"c", "c#", "c++", "clojure", "coffeescript", "coq", "css", "dart", "dhall",
		"diff", "docker", "ebnf", "elixir", "elm", "erlang", "f#", "flow", "fortran",
		"gherkin", "glsl", "go", "graphql", "groovy", "haskell", "hcl", "html",
		"idris", "java", "javascript", "json", "julia", "kotlin", "latex", "less",
		"lisp", "livescript", "llvm ir", "lua", "makefile", "markdown", "markup",
		"matlab", "mathematica", "mermaid", "nix", "notion formula", "objective-c",
		"ocaml", "pascal", "perl", "php", "plain text", "powershell", "prolog",
		"protobuf", "purescript", "python", "r", "racket", "reason", "ruby", "rust",
		"sass", "scala", "scheme", "scss", "shell", "smalltalk", "solidity", "sql",
		"swift", "toml", "typescript", "vb.net", "verilog", "vhdl", "visual basic",
		"webassembly", "xml", "yaml", "java/c/c++/c#", "notionscript",
	}

	// Check if the language is valid as-is
	for _, validLang := range validLanguages {
		if langLower == validLang {
			return validLang
		}
	}

	// If not found, default to plain text
	return "plain text"
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

func extractPlainTextFromRichText(richTexts []notion.RichText) string {
	var text strings.Builder

	for _, rt := range richTexts {
		text.WriteString(rt.PlainText)
	}

	return text.String()
}

func extractLanguageFromCodeBlock(codeBlock *ast.CodeBlock, source []byte) string {
	// For indented code blocks, there's typically no language info
	return ""
}

func extractCodeBlockContent(codeBlock *ast.CodeBlock, source []byte) string {
	var buf strings.Builder

	// For code blocks, iterate through lines
	for i := 0; i < codeBlock.Lines().Len(); i++ {
		line := codeBlock.Lines().At(i)
		buf.Write(line.Value(source))
	}

	return strings.TrimRight(buf.String(), "\n")
}

func extractFencedCodeBlockContent(fencedCodeBlock *ast.FencedCodeBlock, source []byte) string {
	var buf strings.Builder

	// For fenced code blocks, iterate through lines
	for i := 0; i < fencedCodeBlock.Lines().Len(); i++ {
		line := fencedCodeBlock.Lines().At(i)
		buf.Write(line.Value(source))
	}

	return strings.TrimRight(buf.String(), "\n")
}

func extractLanguageFromFencedCodeBlock(fencedCodeBlock *ast.FencedCodeBlock, source []byte) string {
	// Get the info string from the fenced code block
	if fencedCodeBlock.Info != nil {
		infoText := string(fencedCodeBlock.Info.Text(source))
		// The language is typically the first word in the info string
		if len(infoText) > 0 {
			// Split by space and take the first part
			parts := strings.Fields(infoText)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}
	return ""
}
