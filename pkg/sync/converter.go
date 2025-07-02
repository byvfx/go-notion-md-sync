package sync

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
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
	// Pre-process content to extract math blocks and replace with placeholders
	content, mathBlocks := c.extractMathBlocks(content)

	// Parse markdown into AST with table extension
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table),
	)
	reader := text.NewReader([]byte(content))
	doc := md.Parser().Parse(reader)

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
			// Check if paragraph contains only an image
			if imageBlock := c.extractImageFromParagraph(paragraph, source); imageBlock != nil {
				blocks = append(blocks, imageBlock)
			} else {
				text := extractTextFromNode(paragraph, source)
				if strings.TrimSpace(text) != "" {
					// Check if this is a math block placeholder
					if strings.HasPrefix(text, "MATH_BLOCK_") {
						index := strings.TrimPrefix(text, "MATH_BLOCK_")
						if i := parseInt(index); i < len(mathBlocks) {
							block := createEquationBlock(mathBlocks[i])
							blocks = append(blocks, block)
						}
					} else {
						block := createParagraphBlock(text)
						blocks = append(blocks, block)
					}
				}
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

			// Special handling for Mermaid diagrams - keep as code blocks but ensure proper language
			if language == "mermaid" {
				block := createCodeBlock(text, "mermaid")
				blocks = append(blocks, block)
			} else {
				block := createCodeBlock(text, language)
				blocks = append(blocks, block)
			}
			return ast.WalkSkipChildren, nil

		case east.KindTable:
			table := n.(*east.Table)
			tableBlocks := c.convertTableToBlocks(table, source)
			blocks = append(blocks, tableBlocks...)
			return ast.WalkSkipChildren, nil

		case ast.KindBlockquote:
			blockquote := n.(*ast.Blockquote)
			text := extractTextFromNode(blockquote, source)
			block := createCalloutBlock(text)
			blocks = append(blocks, block)
			return ast.WalkSkipChildren, nil

		case ast.KindThematicBreak:
			block := createDividerBlock()
			blocks = append(blocks, block)
			return ast.WalkSkipChildren, nil

		case ast.KindHTMLBlock:
			htmlBlock := n.(*ast.HTMLBlock)
			if toggleBlock := c.extractToggleFromHTML(htmlBlock, source); toggleBlock != nil {
				blocks = append(blocks, toggleBlock)
				return ast.WalkSkipChildren, nil
			}

		default:
			// Check for math blocks (display math)
			if n.Kind().String() == "MathBlock" {
				text := string(n.Text(source))
				// Remove $$ delimiters if present
				text = strings.TrimPrefix(text, "$$")
				text = strings.TrimSuffix(text, "$$")
				text = strings.TrimSpace(text)
				block := createEquationBlock(text)
				blocks = append(blocks, block)
				return ast.WalkSkipChildren, nil
			}
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

	// Track table state
	tableState := &tableTracker{
		rows: [][]string{},
	}

	for i, block := range blocks {
		switch block.Type {
		case "heading_1", "heading_2", "heading_3":
			c.writeHeading(&md, &block)

		case "paragraph":
			c.writeParagraph(&md, &block)

		case "bulleted_list_item":
			c.writeBulletedListItem(&md, &block)

		case "numbered_list_item":
			c.writeNumberedListItem(&md, &block)

		case "code":
			c.writeCodeBlock(&md, &block)

		case "quote":
			c.writeQuote(&md, &block)

		case "divider":
			md.WriteString("---\n\n")

		case "table":
			c.startTable(tableState, &block)

		case "table_row":
			c.processTableRow(tableState, &block, i, blocks, &md)

		case "image":
			c.writeImage(&md, &block)

		case "callout":
			c.writeCallout(&md, &block)

		case "toggle":
			c.writeToggle(&md, &block)

		case "bookmark":
			c.writeBookmark(&md, &block)

		case "equation":
			c.writeEquation(&md, &block)
		}
	}

	return strings.TrimSpace(md.String()), nil
}

type tableTracker struct {
	inTable   bool
	rows      [][]string
	hasHeader bool
}

func (c *converter) writeHeading(md *strings.Builder, block *notion.Block) {
	var text string
	var prefix string

	switch block.Type {
	case "heading_1":
		if block.Heading1 != nil {
			text = extractPlainTextFromRichText(block.Heading1.RichText)
			prefix = "# "
		}
	case "heading_2":
		if block.Heading2 != nil {
			text = extractPlainTextFromRichText(block.Heading2.RichText)
			prefix = "## "
		}
	case "heading_3":
		if block.Heading3 != nil {
			text = extractPlainTextFromRichText(block.Heading3.RichText)
			prefix = "### "
		}
	}

	if text != "" {
		md.WriteString(prefix + text + "\n\n")
	}
}

func (c *converter) writeParagraph(md *strings.Builder, block *notion.Block) {
	if block.Paragraph != nil {
		text := extractPlainTextFromRichText(block.Paragraph.RichText)
		if strings.TrimSpace(text) != "" {
			md.WriteString(text + "\n\n")
		}
	}
}

func (c *converter) writeBulletedListItem(md *strings.Builder, block *notion.Block) {
	if block.BulletedListItem != nil {
		text := extractPlainTextFromRichText(block.BulletedListItem.RichText)
		md.WriteString("- " + text + "\n")
	}
}

func (c *converter) writeNumberedListItem(md *strings.Builder, block *notion.Block) {
	if block.NumberedListItem != nil {
		text := extractPlainTextFromRichText(block.NumberedListItem.RichText)
		md.WriteString("1. " + text + "\n")
	}
}

func (c *converter) writeCodeBlock(md *strings.Builder, block *notion.Block) {
	if block.Code != nil {
		code := extractPlainTextFromRichText(block.Code.RichText)
		language := block.Code.Language
		md.WriteString("```" + language + "\n" + code + "\n```\n\n")
	}
}

func (c *converter) writeQuote(md *strings.Builder, block *notion.Block) {
	if block.Quote != nil {
		text := extractPlainTextFromRichText(block.Quote.RichText)
		md.WriteString("> " + text + "\n\n")
	}
}

func (c *converter) startTable(state *tableTracker, block *notion.Block) {
	state.inTable = true
	state.rows = [][]string{}
	state.hasHeader = false
	if block.Table != nil {
		state.hasHeader = block.Table.HasColumnHeader
	}
}

func (c *converter) processTableRow(state *tableTracker, block *notion.Block, index int, blocks []notion.Block, md *strings.Builder) {
	if state.inTable && block.TableRow != nil {
		var row []string
		for _, cell := range block.TableRow.Cells {
			cellText := extractPlainTextFromRichText(cell)
			row = append(row, cellText)
		}
		state.rows = append(state.rows, row)
	}

	// Check if this is the last table row
	isLastTableRow := index == len(blocks)-1 || (index < len(blocks)-1 && blocks[index+1].Type != "table_row")

	if state.inTable && isLastTableRow && len(state.rows) > 0 {
		// Write the table
		c.writeMarkdownTable(md, state.rows, state.hasHeader)
		state.inTable = false
		state.rows = nil
	}
}

func (c *converter) writeMarkdownTable(md *strings.Builder, rows [][]string, hasHeader bool) {
	if len(rows) == 0 {
		return
	}

	// Determine column count from first row
	columnCount := len(rows[0])

	// Write all rows
	for i, row := range rows {
		md.WriteString("| ")
		for j, cell := range row {
			md.WriteString(cell)
			if j < len(row)-1 {
				md.WriteString(" | ")
			}
		}
		// Pad with empty cells if needed
		for j := len(row); j < columnCount; j++ {
			md.WriteString(" | ")
		}
		md.WriteString(" |\n")

		// Add separator after header row
		if i == 0 && hasHeader {
			md.WriteString("| ")
			for j := 0; j < columnCount; j++ {
				md.WriteString("---")
				if j < columnCount-1 {
					md.WriteString(" | ")
				}
			}
			md.WriteString(" |\n")
		}
	}

	md.WriteString("\n")
}

// Helper functions

func extractTextFromNode(node ast.Node, source []byte) string {
	var buf strings.Builder

	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
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

func createCalloutBlock(text string) map[string]interface{} {
	// Extract emoji if present at the beginning of the text
	emoji := ""
	content := text

	// Simple check for emoji at start (could be enhanced)
	if len(text) > 0 {
		// Check if text starts with common callout indicators
		if strings.HasPrefix(text, "💡 ") || strings.HasPrefix(text, "⚠️ ") ||
			strings.HasPrefix(text, "❗ ") || strings.HasPrefix(text, "📝 ") {
			emoji = text[:strings.Index(text, " ")]
			content = text[len(emoji)+1:]
		}
	}

	calloutBlock := map[string]interface{}{
		"type": "callout",
		"callout": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]interface{}{
						"content": content,
					},
				},
			},
			"color": "gray_background",
		},
	}

	if emoji != "" {
		calloutBlock["callout"].(map[string]interface{})["icon"] = map[string]interface{}{
			"type":  "emoji",
			"emoji": emoji,
		}
	}

	return calloutBlock
}

func createDividerBlock() map[string]interface{} {
	return map[string]interface{}{
		"type":    "divider",
		"divider": map[string]interface{}{},
	}
}

func createImageBlock(url, caption string) map[string]interface{} {
	imageBlock := map[string]interface{}{
		"type": "image",
		"image": map[string]interface{}{
			"type": "external",
			"external": map[string]interface{}{
				"url": url,
			},
		},
	}

	if caption != "" {
		imageBlock["image"].(map[string]interface{})["caption"] = []map[string]interface{}{
			{
				"type": "text",
				"text": map[string]interface{}{
					"content": caption,
				},
			},
		}
	}

	return imageBlock
}

func createToggleBlock(summary string) map[string]interface{} {
	return map[string]interface{}{
		"type": "toggle",
		"toggle": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]interface{}{
						"content": summary,
					},
				},
			},
		},
	}
}

func createEquationBlock(expression string) map[string]interface{} {
	return map[string]interface{}{
		"type": "equation",
		"equation": map[string]interface{}{
			"expression": expression,
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
	return c.convertListToBlocksWithDepth(list, source, 0)
}

func (c *converter) convertListToBlocksWithDepth(list *ast.List, source []byte, depth int) []map[string]interface{} {
	var blocks []map[string]interface{}

	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		if listItem, ok := child.(*ast.ListItem); ok {
			// Extract the direct text content of this list item (excluding nested lists)
			text := c.extractListItemText(listItem, source)

			blockType := "bulleted_list_item"
			if list.IsOrdered() {
				blockType = "numbered_list_item"
			}

			// Create indentation for nested lists by adding spaces
			indent := strings.Repeat("  ", depth)

			block := map[string]interface{}{
				"type": blockType,
				blockType: map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{
							"type": "text",
							"text": map[string]interface{}{
								"content": indent + text,
							},
						},
					},
				},
			}
			blocks = append(blocks, block)

			// Process nested lists
			for nestedChild := listItem.FirstChild(); nestedChild != nil; nestedChild = nestedChild.NextSibling() {
				if nestedList, ok := nestedChild.(*ast.List); ok {
					nestedBlocks := c.convertListToBlocksWithDepth(nestedList, source, depth+1)
					blocks = append(blocks, nestedBlocks...)
				}
			}
		}
	}

	return blocks
}

func (c *converter) extractListItemText(listItem *ast.ListItem, source []byte) string {
	var text strings.Builder

	// Walk through the list item and extract text, but skip nested lists
	_ = ast.Walk(listItem, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		// If we encounter a nested list, skip it entirely
		if n.Kind() == ast.KindList && n != listItem {
			return ast.WalkSkipChildren, nil
		}

		switch n.Kind() {
		case ast.KindText:
			textNode := n.(*ast.Text)
			text.Write(textNode.Segment.Value(source))
		case ast.KindString:
			stringNode := n.(*ast.String)
			text.Write(stringNode.Value)
		}
		return ast.WalkContinue, nil
	})

	return strings.TrimSpace(text.String())
}

func extractPlainTextFromRichText(richTexts []notion.RichText) string {
	var text strings.Builder

	for _, rt := range richTexts {
		text.WriteString(rt.PlainText)
	}

	return text.String()
}

func extractLanguageFromCodeBlock(_ *ast.CodeBlock, _ []byte) string {
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

func (c *converter) convertTableToBlocks(table *east.Table, source []byte) []map[string]interface{} {
	// Count columns from the first row
	var columnCount int
	var hasHeader bool

	// Check if table has a header
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		if _, ok := child.(*east.TableHeader); ok {
			hasHeader = true
			// Count columns from the header row
			break
		}
	}

	// Count columns from the first row (header or body)
	if firstChild := table.FirstChild(); firstChild != nil {
		switch node := firstChild.(type) {
		case *east.TableHeader:
			// Count cells directly in header
			for cell := node.FirstChild(); cell != nil; cell = cell.NextSibling() {
				columnCount++
			}
		case *east.TableRow:
			// Count cells in row
			for cell := node.FirstChild(); cell != nil; cell = cell.NextSibling() {
				columnCount++
			}
		}
	}

	// Collect all table row blocks as children
	var tableRowBlocks []map[string]interface{}

	// Convert header rows first if they exist
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		if tableHeader, ok := child.(*east.TableHeader); ok {
			// TableHeader contains cells directly, not rows
			var cells [][]map[string]interface{}
			for cell := tableHeader.FirstChild(); cell != nil; cell = cell.NextSibling() {
				if tableCell, ok := cell.(*east.TableCell); ok {
					cellText := extractTextFromNode(tableCell, source)
					cellRichText := []map[string]interface{}{
						{
							"type": "text",
							"text": map[string]interface{}{
								"content": strings.TrimSpace(cellText),
							},
						},
					}
					cells = append(cells, cellRichText)
				}
			}

			// Create a row block for the header
			rowBlock := map[string]interface{}{
				"type": "table_row",
				"table_row": map[string]interface{}{
					"cells": cells,
				},
			}
			tableRowBlocks = append(tableRowBlocks, rowBlock)
		}
	}

	// Convert body rows
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		switch node := child.(type) {
		case *east.TableRow:
			// Direct row (no explicit header/body)
			rowBlock := c.convertTableRow(node, source)
			tableRowBlocks = append(tableRowBlocks, rowBlock)
		case *east.TableHeader:
			// Already handled above
			continue
		default:
			// Skip unknown nodes
		}
	}

	// Create the main table block without children (children are sent as separate blocks)
	tableBlock := map[string]interface{}{
		"type": "table",
		"table": map[string]interface{}{
			"table_width":       columnCount,
			"has_column_header": hasHeader,
			"has_row_header":    false, // Markdown tables don't typically have row headers
		},
	}

	// Return the table block followed by all the row blocks
	allBlocks := []map[string]interface{}{tableBlock}
	allBlocks = append(allBlocks, tableRowBlocks...)
	return allBlocks
}

func (c *converter) convertTableRow(row *east.TableRow, source []byte) map[string]interface{} {
	var cells [][]map[string]interface{}
	for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
		if tableCell, ok := cell.(*east.TableCell); ok {
			cellText := extractTextFromNode(tableCell, source)
			cellRichText := []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]interface{}{
						"content": strings.TrimSpace(cellText),
					},
				},
			}
			cells = append(cells, cellRichText)
		}
	}

	return map[string]interface{}{
		"type": "table_row",
		"table_row": map[string]interface{}{
			"cells": cells,
		},
	}
}

func (c *converter) writeImage(md *strings.Builder, block *notion.Block) {
	if block.Image != nil {
		var url string
		if block.Image.External != nil {
			url = block.Image.External.URL
		} else if block.Image.File != nil {
			url = block.Image.File.URL
		}

		caption := extractPlainTextFromRichText(block.Image.Caption)
		if caption != "" {
			fmt.Fprintf(md, "![%s](%s)\n\n", caption, url)
		} else {
			fmt.Fprintf(md, "![](%s)\n\n", url)
		}
	}
}

func (c *converter) writeCallout(md *strings.Builder, block *notion.Block) {
	if block.Callout != nil {
		text := extractPlainTextFromRichText(block.Callout.RichText)
		icon := ""
		if block.Callout.Icon != nil && block.Callout.Icon.Emoji != "" {
			icon = block.Callout.Icon.Emoji + " "
		}

		// Convert callout to blockquote with icon
		fmt.Fprintf(md, "> %s%s\n\n", icon, text)
	}
}

func (c *converter) writeToggle(md *strings.Builder, block *notion.Block) {
	if block.Toggle != nil {
		text := extractPlainTextFromRichText(block.Toggle.RichText)
		// Use HTML details/summary for toggle functionality
		md.WriteString("<details>\n<summary>" + text + "</summary>\n\n")
		// Note: Child blocks would be added here if we supported nested blocks
		md.WriteString("</details>\n\n")
	}
}

func (c *converter) writeBookmark(md *strings.Builder, block *notion.Block) {
	if block.Bookmark != nil {
		caption := extractPlainTextFromRichText(block.Bookmark.Caption)
		if caption != "" {
			fmt.Fprintf(md, "[%s](%s)\n\n", caption, block.Bookmark.URL)
		} else {
			fmt.Fprintf(md, "<%s>\n\n", block.Bookmark.URL)
		}
	}
}

func (c *converter) writeEquation(md *strings.Builder, block *notion.Block) {
	if block.Equation != nil {
		fmt.Fprintf(md, "$$%s$$\n\n", block.Equation.Expression)
	}
}

func (c *converter) extractImageFromParagraph(paragraph *ast.Paragraph, source []byte) map[string]interface{} {
	// Check if paragraph contains only an image
	if paragraph.ChildCount() == 1 {
		if image, ok := paragraph.FirstChild().(*ast.Image); ok {
			url := string(image.Destination)
			caption := string(image.Title)

			// If no title, try to extract alt text
			if caption == "" && image.ChildCount() > 0 {
				caption = extractTextFromNode(image, source)
			}

			return createImageBlock(url, caption)
		}
	}
	return nil
}

func (c *converter) extractToggleFromHTML(htmlBlock *ast.HTMLBlock, source []byte) map[string]interface{} {
	// Extract the HTML content
	var htmlContent strings.Builder
	for i := 0; i < htmlBlock.Lines().Len(); i++ {
		line := htmlBlock.Lines().At(i)
		htmlContent.Write(line.Value(source))
	}

	html := strings.TrimSpace(htmlContent.String())

	// Check if it's a details/summary element
	if strings.HasPrefix(html, "<details>") && strings.Contains(html, "<summary>") {
		// Extract summary text (simple regex approach)
		summaryStart := strings.Index(html, "<summary>") + 9
		summaryEnd := strings.Index(html, "</summary>")

		if summaryEnd > summaryStart {
			summary := html[summaryStart:summaryEnd]
			return createToggleBlock(summary)
		}
	}

	return nil
}

// extractMathBlocks extracts $$...$$ math blocks from content and replaces them with placeholders
func (c *converter) extractMathBlocks(content string) (string, []string) {
	var mathBlocks []string
	result := content

	// Find all $$...$$ blocks
	for {
		start := strings.Index(result, "$$")
		if start == -1 {
			break
		}

		// Find the closing $$
		end := strings.Index(result[start+2:], "$$")
		if end == -1 {
			break
		}
		end += start + 2 + 2 // Adjust for the offset and length of $$

		// Extract the math content (without the $$ delimiters)
		mathContent := result[start+2 : end-2]
		mathBlocks = append(mathBlocks, strings.TrimSpace(mathContent))

		// Replace with placeholder on its own line
		placeholder := fmt.Sprintf("\n\nMATH_BLOCK_%d\n\n", len(mathBlocks)-1)
		result = result[:start] + placeholder + result[end:]
	}

	return result, mathBlocks
}

// parseInt converts string to int, returns -1 if invalid
func parseInt(s string) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return -1
}
