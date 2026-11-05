package sync

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// Converter converts between Markdown and Notion blocks.
type Converter interface {
	MarkdownToBlocks(content string) ([]map[string]interface{}, error)
	BlocksToMarkdown(blocks []notion.Block) (string, error)
}

type converter struct {
	md goldmark.Markdown
}

func NewConverter() Converter {
	return &converter{
		md: goldmark.New(goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		)),
	}
}

// pushState carries the placeholders found while preprocessing.
type pushState struct {
	ph      placeholders
	anchors []string
	maths   []string
}

// MarkdownToBlocks converts Markdown into Notion block requests. Nested
// content (list items, quotes, toggles, tables) is put in each block's
// "children". The result may contain notion.AnchorBlock placeholders.
func (c *converter) MarkdownToBlocks(content string) ([]map[string]interface{}, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	ph := newPlaceholders()
	content, anchors, maths := preprocessMarkdown(content, ph)

	source := []byte(content)
	doc := c.md.Parser().Parse(text.NewReader(source))

	st := &pushState{ph: ph, anchors: anchors, maths: maths}
	blocks := c.convertChildren(doc, source, st, true)
	if blocks == nil {
		blocks = []map[string]interface{}{}
	}
	return blocks, nil
}

// convertChildren converts the block-level children of node.
func (c *converter) convertChildren(node ast.Node, source []byte, st *pushState, topLevel bool) []map[string]interface{} {
	return c.convertSiblings(node.FirstChild(), source, st, topLevel)
}

func (c *converter) convertSiblings(first ast.Node, source []byte, st *pushState, topLevel bool) []map[string]interface{} {
	return c.convertRange(first, nil, source, st, topLevel)
}

// convertRange converts sibling nodes from first up to (not including) stop.
func (c *converter) convertRange(first, stop ast.Node, source []byte, st *pushState, topLevel bool) []map[string]interface{} {
	var blocks []map[string]interface{}
	for n := first; n != nil && n != stop; n = n.NextSibling() {
		switch v := n.(type) {
		case *ast.Heading:
			blocks = append(blocks, createHeadingBlock(v.Level, inlineRichText(v, source)))

		case *ast.Paragraph, *ast.TextBlock:
			if b := c.convertParagraph(n, source, st, topLevel); b != nil {
				blocks = append(blocks, b)
			}

		case *ast.List:
			blocks = append(blocks, c.convertList(v, source, st)...)

		case *ast.FencedCodeBlock:
			lang := ""
			if v.Info != nil {
				lang = strings.Fields(string(v.Info.Text(source)) + " ")[0]
			}
			blocks = append(blocks, createCodeBlock(codeLines(v, source), lang))

		case *ast.CodeBlock:
			blocks = append(blocks, createCodeBlock(codeLines(v, source), ""))

		case *ast.Blockquote:
			blocks = append(blocks, c.convertBlockquote(v, source, st))

		case *ast.ThematicBreak:
			blocks = append(blocks, createDividerBlock())

		case *east.Table:
			if b := convertTable(v, source); b != nil {
				blocks = append(blocks, b)
			}

		case *ast.HTMLBlock:
			raw := htmlBlockText(v, source)
			if summary, rest, ok := parseDetailsOpen(raw); ok {
				toggle, last := c.convertDetails(v, summary, rest, source, st)
				blocks = append(blocks, toggle)
				n = last
				continue
			}
			if b := htmlAsParagraph(raw); b != nil {
				blocks = append(blocks, b)
			}
		}
	}
	return blocks
}

func (c *converter) convertParagraph(n ast.Node, source []byte, st *pushState, topLevel bool) map[string]interface{} {
	plain := strings.TrimSpace(plainText(n, source))

	if i := placeholderIndex(plain, st.ph.anchor); i >= 0 && i < len(st.anchors) {
		if topLevel {
			return notion.AnchorBlock(st.anchors[i])
		}
		return nil // Kept blocks can only be positioned at the top level.
	}
	if i := placeholderIndex(plain, st.ph.math); i >= 0 && i < len(st.maths) {
		return createEquationBlock(st.maths[i])
	}

	// A paragraph that is only an image becomes an image block.
	if img := onlyChild[*ast.Image](n); img != nil {
		dest := string(img.Destination)
		if allowedLink(dest) != "" {
			caption := plainText(img, source)
			if caption == "" {
				caption = string(img.Title)
			}
			return createImageBlock(dest, caption)
		}
		// Notion only accepts images by URL. Keep a local image reference as
		// text so nothing is lost and the push doesn't fail.
		return createRichTextBlock("paragraph", plainRichText("!["+plainText(img, source)+"]("+dest+")"))
	}

	// A paragraph that is only a bare URL becomes a bookmark.
	if link := onlyChild[*ast.AutoLink](n); link != nil && link.AutoLinkType == ast.AutoLinkURL {
		if u := allowedLink(string(link.URL(source))); u != "" {
			return createBookmarkBlock(u)
		}
	}

	rt := inlineRichText(n, source)
	if len(rt) == 0 {
		return nil
	}
	return createRichTextBlock("paragraph", rt)
}

// onlyChild returns the node's single child if it has type T.
func onlyChild[T ast.Node](n ast.Node) T {
	var zero T
	if n.ChildCount() != 1 {
		return zero
	}
	if v, ok := n.FirstChild().(T); ok {
		return v
	}
	return zero
}

func (c *converter) convertList(list *ast.List, source []byte, st *pushState) []map[string]interface{} {
	var blocks []map[string]interface{}
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		li, ok := item.(*ast.ListItem)
		if !ok {
			continue
		}

		var rt []map[string]interface{}
		checked, isTask := false, false
		rest := li.FirstChild()
		if first := li.FirstChild(); first != nil {
			if _, isPara := first.(*ast.Paragraph); isPara || first.Kind() == ast.KindTextBlock {
				if cb, ok := first.FirstChild().(*east.TaskCheckBox); ok {
					isTask, checked = true, cb.IsChecked
				}
				rt = inlineRichText(first, source)
				rest = first.NextSibling()
			}
		}
		children := c.convertSiblings(rest, source, st, false)

		blockType := "bulleted_list_item"
		switch {
		case isTask:
			blockType = "to_do"
		case list.IsOrdered():
			blockType = "numbered_list_item"
		}
		block := createRichTextBlock(blockType, rt)
		if isTask {
			block["to_do"].(map[string]interface{})["checked"] = checked
		}
		setChildren(block, children)
		blocks = append(blocks, block)
	}
	return blocks
}

func (c *converter) convertBlockquote(bq *ast.Blockquote, source []byte, st *pushState) map[string]interface{} {
	var rt []map[string]interface{}
	rest := bq.FirstChild()
	emoji := ""
	if p, ok := bq.FirstChild().(*ast.Paragraph); ok {
		rt = inlineRichText(p, source)
		rest = p.NextSibling()
		emoji, rt = splitLeadingEmoji(rt)
	}
	children := c.convertSiblings(rest, source, st, false)

	var block map[string]interface{}
	if emoji != "" {
		block = createRichTextBlock("callout", rt)
		block["callout"].(map[string]interface{})["icon"] = map[string]interface{}{
			"type":  "emoji",
			"emoji": emoji,
		}
	} else {
		block = createRichTextBlock("quote", rt)
	}
	setChildren(block, children)
	return block
}

// splitLeadingEmoji removes an emoji followed by a space from the start of
// rich text, which is how callouts are written in Markdown ("> 💡 text").
func splitLeadingEmoji(rt []map[string]interface{}) (string, []map[string]interface{}) {
	if len(rt) == 0 {
		return "", rt
	}
	textObj, _ := rt[0]["text"].(map[string]interface{})
	content, _ := textObj["content"].(string)
	sp := strings.IndexByte(content, ' ')
	if sp <= 0 || sp > 32 {
		return "", rt
	}
	candidate := content[:sp]
	if !isEmoji(candidate) {
		return "", rt
	}
	rest := content[sp+1:]
	newText := map[string]interface{}{}
	for k, v := range textObj {
		newText[k] = v
	}
	newText["content"] = rest
	first := map[string]interface{}{}
	for k, v := range rt[0] {
		first[k] = v
	}
	first["text"] = newText
	out := append([]map[string]interface{}{}, rt...)
	if rest == "" {
		out = out[1:]
	} else {
		out[0] = first
	}
	return candidate, out
}

// isEmoji reports whether s looks like a single emoji (possibly with
// modifiers, variation selectors or zero-width joiners).
func isEmoji(s string) bool {
	if s == "" || utf8.RuneCountInString(s) > 10 {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r >= 0x1F000 || (r >= 0x2190 && r <= 0x2BFF) || unicode.Is(unicode.So, r)
}

var detailsOpenRe = regexp.MustCompile(`(?is)^\s*<details[^>]*>\s*(?:<summary[^>]*>(.*?)</summary>)?(.*)$`)

// parseDetailsOpen recognises an HTML block that opens a <details> toggle.
func parseDetailsOpen(raw string) (summary, rest string, ok bool) {
	m := detailsOpenRe.FindStringSubmatch(raw)
	if m == nil {
		return "", "", false
	}
	return strings.TrimSpace(html.UnescapeString(stripTags(m[1]))), m[2], true
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string {
	return tagRe.ReplaceAllString(s, "")
}

// convertDetails turns <details><summary>…</summary> … </details> into a
// toggle block. The content between the tags (which Markdown parses as
// separate blocks) becomes the toggle's children. It returns the toggle and
// the last node it consumed.
func (c *converter) convertDetails(open *ast.HTMLBlock, summary, rest string, source []byte, st *pushState) (map[string]interface{}, ast.Node) {
	toggle := createRichTextBlock("toggle", plainRichText(summary))

	lower := strings.ToLower(rest)
	depth := 1 + strings.Count(lower, "<details") - strings.Count(lower, "</details>")
	if depth <= 0 {
		// Everything in one HTML block: <details><summary>x</summary>body</details>
		body := strings.TrimSpace(html.UnescapeString(stripTags(rest)))
		if body != "" {
			setChildren(toggle, []map[string]interface{}{createRichTextBlock("paragraph", plainRichText(body))})
		}
		return toggle, open
	}

	// Find the HTML block that closes this toggle. Without one, the toggle
	// runs to the end of its parent.
	var closeNode ast.Node
	last := ast.Node(open)
	for n := open.NextSibling(); n != nil; n = n.NextSibling() {
		last = n
		if hb, ok := n.(*ast.HTMLBlock); ok {
			raw := strings.ToLower(htmlBlockText(hb, source))
			depth += strings.Count(raw, "<details") - strings.Count(raw, "</details>")
			if depth <= 0 {
				closeNode = n
				break
			}
		}
	}

	setChildren(toggle, c.convertRange(open.NextSibling(), closeNode, source, st, false))
	return toggle, last
}

func htmlBlockText(hb *ast.HTMLBlock, source []byte) string {
	var b strings.Builder
	for i := 0; i < hb.Lines().Len(); i++ {
		line := hb.Lines().At(i)
		b.Write(line.Value(source))
	}
	if hb.HasClosure() {
		b.Write(hb.ClosureLine.Value(source))
	}
	return b.String()
}

var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

// htmlAsParagraph keeps raw HTML (other than comments and stray toggle tags)
// as plain text, so it isn't silently dropped.
func htmlAsParagraph(raw string) map[string]interface{} {
	raw = htmlCommentRe.ReplaceAllString(raw, "")
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "</details>") {
		return nil
	}
	return createRichTextBlock("paragraph", plainRichText(trimmed))
}

func codeLines(n ast.Node, source []byte) string {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		b.Write(line.Value(source))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func convertTable(table *east.Table, source []byte) map[string]interface{} {
	var rows [][][]map[string]interface{}
	hasHeader := false
	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		switch child.(type) {
		case *east.TableHeader:
			hasHeader = true
		case *east.TableRow:
		default:
			continue
		}
		var cells [][]map[string]interface{}
		for cell := child.FirstChild(); cell != nil; cell = cell.NextSibling() {
			if _, ok := cell.(*east.TableCell); ok {
				cells = append(cells, inlineRichText(cell, source))
			}
		}
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		return nil
	}

	width := 0
	for _, r := range rows {
		if len(r) > width {
			width = len(r)
		}
	}
	if width == 0 {
		return nil
	}

	rowBlocks := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		// Notion requires every row to have exactly table_width cells.
		for len(r) < width {
			r = append(r, []map[string]interface{}{})
		}
		rowBlocks = append(rowBlocks, map[string]interface{}{
			"type": "table_row",
			"table_row": map[string]interface{}{
				"cells": r,
			},
		})
	}

	return map[string]interface{}{
		"type": "table",
		"table": map[string]interface{}{
			"table_width":       width,
			"has_column_header": hasHeader,
			"has_row_header":    false,
			"children":          rowBlocks,
		},
	}
}

// --- block builders ---

func createRichTextBlock(blockType string, rt []map[string]interface{}) map[string]interface{} {
	if rt == nil {
		rt = []map[string]interface{}{}
	}
	return map[string]interface{}{
		"type": blockType,
		blockType: map[string]interface{}{
			"rich_text": rt,
		},
	}
}

func setChildren(block map[string]interface{}, children []map[string]interface{}) {
	if len(children) == 0 {
		return
	}
	t := block["type"].(string)
	block[t].(map[string]interface{})["children"] = children
}

func createHeadingBlock(level int, rt []map[string]interface{}) map[string]interface{} {
	if level < 1 {
		level = 1
	}
	if level > 3 {
		level = 3
	}
	return createRichTextBlock("heading_"+strconv.Itoa(level), rt)
}

func createCodeBlock(code, language string) map[string]interface{} {
	return map[string]interface{}{
		"type": "code",
		"code": map[string]interface{}{
			"rich_text": codeRichText(code),
			"language":  normalizeNotionLanguage(language),
		},
	}
}

func createDividerBlock() map[string]interface{} {
	return map[string]interface{}{
		"type":    "divider",
		"divider": map[string]interface{}{},
	}
}

func createBookmarkBlock(url string) map[string]interface{} {
	return map[string]interface{}{
		"type": "bookmark",
		"bookmark": map[string]interface{}{
			"url": url,
		},
	}
}

func createImageBlock(url, caption string) map[string]interface{} {
	image := map[string]interface{}{
		"type": "external",
		"external": map[string]interface{}{
			"url": url,
		},
	}
	if caption != "" {
		image["caption"] = plainRichText(caption)
	}
	return map[string]interface{}{
		"type":  "image",
		"image": image,
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
		"jsx":        "javascript",
		"ts":         "typescript",
		"tsx":        "typescript",
		"py":         "python",
		"rb":         "ruby",
		"sh":         "shell",
		"zsh":        "shell",
		"yml":        "yaml",
		"golang":     "go",
		"rs":         "rust",
		"cs":         "c#",
		"cpp":        "c++",
		"kt":         "kotlin",
		"md":         "markdown",
		"text":       "plain text",
		"txt":        "plain text",
		"plaintext":  "plain text",
		"dockerfile": "docker",
		"":           "plain text",
	}

	langLower := strings.ToLower(strings.TrimSpace(lang))
	if mapped, exists := langMap[langLower]; exists {
		return mapped
	}

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
	for _, validLang := range validLanguages {
		if langLower == validLang {
			return validLang
		}
	}
	return "plain text"
}

// parseInt converts string to int, returns -1 if invalid
func parseInt(s string) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return -1
}
