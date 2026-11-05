package sync

import (
	"fmt"
	"html"
	"strings"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/byvfx/go-notion-md-sync/pkg/util"
)

// BlocksToMarkdown converts a page's blocks (with nested Children, as
// returned by GetPageBlocks) into Markdown.
//
// Sub-pages and databases become links to their local files, and every block
// that can't be round-tripped is written as a marker (see markers.go) so a
// later push leaves it untouched in Notion.
func (c *converter) BlocksToMarkdown(blocks []notion.Block) (string, error) {
	r := &mdRenderer{}
	var parts []mdPart
	for i := range blocks {
		b := &blocks[i]
		if notion.ShouldPreserveTree(b) {
			parts = append(parts, mdPart{text: r.preserved(b)})
			continue
		}
		parts = append(parts, r.block(b, false))
	}
	return strings.TrimSpace(joinParts(parts)), nil
}

// mdPart is a rendered block. List items are joined without a blank line so
// they stay one list.
type mdPart struct {
	text     string
	listKind string // "bullet", "number" or "" for non-list blocks
}

func joinParts(parts []mdPart) string {
	var b strings.Builder
	prevKind := ""
	first := true
	for _, p := range parts {
		if p.text == "" {
			continue
		}
		if !first {
			if p.listKind != "" && p.listKind == prevKind {
				b.WriteString("\n")
			} else {
				b.WriteString("\n\n")
			}
		}
		b.WriteString(p.text)
		prevKind = p.listKind
		first = false
	}
	return b.String()
}

type mdRenderer struct{}

// children renders nested blocks. readOnly is set inside a preserved region,
// where kept blocks are shown as plain links rather than markers.
func (r *mdRenderer) children(blocks []notion.Block, readOnly bool) string {
	parts := make([]mdPart, 0, len(blocks))
	for i := range blocks {
		parts = append(parts, r.block(&blocks[i], readOnly))
	}
	return joinParts(parts)
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}

func quoteLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + l
		}
	}
	return strings.Join(lines, "\n")
}

func (r *mdRenderer) block(b *notion.Block, readOnly bool) mdPart {
	switch b.Type {
	case "paragraph":
		text := ""
		if b.Paragraph != nil {
			text = richTextToMarkdown(b.Paragraph.RichText)
		}
		if kids := r.children(b.Children, readOnly); kids != "" {
			if text == "" {
				return mdPart{text: kids}
			}
			return mdPart{text: text + "\n\n" + kids}
		}
		return mdPart{text: text}

	case "heading_1", "heading_2", "heading_3":
		var rt *notion.RichTextBlock
		prefix := "# "
		switch b.Type {
		case "heading_1":
			rt = b.Heading1
		case "heading_2":
			rt, prefix = b.Heading2, "## "
		case "heading_3":
			rt, prefix = b.Heading3, "### "
		}
		text := ""
		if rt != nil {
			text = richTextToMarkdown(rt.RichText)
		}
		out := ""
		if strings.TrimSpace(text) != "" {
			// Headings are one line.
			out = prefix + strings.ReplaceAll(text, "\\\n", " ")
		}
		// Toggleable headings: show their content after the heading.
		if kids := r.children(b.Children, readOnly); kids != "" {
			if out == "" {
				out = kids
			} else {
				out += "\n\n" + kids
			}
		}
		return mdPart{text: out}

	case "bulleted_list_item":
		text := ""
		if b.BulletedListItem != nil {
			text = richTextToMarkdown(b.BulletedListItem.RichText)
		}
		return mdPart{text: r.listItem("- ", text, b.Children, readOnly), listKind: "bullet"}

	case "numbered_list_item":
		text := ""
		if b.NumberedListItem != nil {
			text = richTextToMarkdown(b.NumberedListItem.RichText)
		}
		return mdPart{text: r.listItem("1. ", text, b.Children, readOnly), listKind: "number"}

	case "to_do":
		text, box := "", "[ ] "
		if b.ToDo != nil {
			text = richTextToMarkdown(b.ToDo.RichText)
			if b.ToDo.Checked {
				box = "[x] "
			}
		}
		return mdPart{text: r.listItem("- ", box+text, b.Children, readOnly), listKind: "bullet"}

	case "code":
		if b.Code == nil {
			return mdPart{}
		}
		code := plainFromRichText(b.Code.RichText)
		lang := b.Code.Language
		if lang == "plain text" {
			lang = ""
		}
		fence := codeFence(code)
		return mdPart{text: fence + lang + "\n" + code + "\n" + fence}

	case "quote", "callout":
		text := ""
		if b.Type == "quote" && b.Quote != nil {
			text = richTextToMarkdown(b.Quote.RichText)
		}
		if b.Type == "callout" && b.Callout != nil {
			text = richTextToMarkdown(b.Callout.RichText)
			if b.Callout.Icon != nil && b.Callout.Icon.Emoji != "" {
				text = b.Callout.Icon.Emoji + " " + text
			}
		}
		body := text
		if kids := r.children(b.Children, readOnly); kids != "" {
			body += "\n\n" + kids
		}
		return mdPart{text: quoteLines(body)}

	case "divider":
		return mdPart{text: "---"}

	case "table":
		return mdPart{text: r.table(b)}

	case "image":
		if b.Image == nil {
			return mdPart{}
		}
		url := ""
		if b.Image.External != nil {
			url = b.Image.External.URL
		} else if b.Image.File != nil {
			url = b.Image.File.URL
		}
		return mdPart{text: "![" + escapeMarkdown(plainFromRichText(b.Image.Caption), false) + "](" + linkDestination(url) + ")"}

	case "toggle":
		summary := ""
		if b.Toggle != nil {
			summary = plainFromRichText(b.Toggle.RichText)
		}
		out := "<details>\n<summary>" + html.EscapeString(summary) + "</summary>"
		if kids := r.children(b.Children, readOnly); kids != "" {
			out += "\n\n" + kids
		}
		return mdPart{text: out + "\n\n</details>"}

	case "bookmark":
		if b.Bookmark == nil {
			return mdPart{}
		}
		if caption := plainFromRichText(b.Bookmark.Caption); caption != "" {
			return mdPart{text: "[" + escapeMarkdown(caption, false) + "](" + linkDestination(b.Bookmark.URL) + ")"}
		}
		return mdPart{text: "<" + b.Bookmark.URL + ">"}

	case "equation":
		if b.Equation == nil {
			return mdPart{}
		}
		return mdPart{text: "$$\n" + b.Equation.Expression + "\n$$"}
	}

	// Kept (non round-trip) block nested inside a region.
	if isLayoutContainer(b.Type) {
		return mdPart{text: r.children(b.Children, true)}
	}
	text := r.describe(b, true)
	if kids := r.children(b.Children, true); kids != "" {
		text += "\n\n" + kids
	}
	return mdPart{text: text}
}

// isLayoutContainer reports whether a block only groups other blocks, so
// showing its children is enough.
func isLayoutContainer(blockType string) bool {
	switch blockType {
	case "column_list", "column", "synced_block", "template":
		return true
	}
	return false
}

func (r *mdRenderer) listItem(marker, text string, children []notion.Block, readOnly bool) string {
	pad := strings.Repeat(" ", len(marker))
	// Continuation lines (after hard breaks) belong to the item.
	out := marker + strings.ReplaceAll(text, "\n", "\n"+pad)
	if kids := r.children(children, readOnly); kids != "" {
		out += "\n" + indentLines(kids, strings.Repeat(" ", len(marker)))
	}
	return out
}

func (r *mdRenderer) table(b *notion.Block) string {
	var rows [][]string
	width := 0
	if b.Table != nil {
		width = b.Table.TableWidth
	}
	for _, child := range b.Children {
		if child.Type != "table_row" || child.TableRow == nil {
			continue
		}
		var row []string
		for _, cell := range child.TableRow.Cells {
			text := richTextToMarkdown(cell)
			text = strings.ReplaceAll(text, "|", `\|`)
			text = strings.ReplaceAll(text, "\\\n", "<br>")
			text = strings.ReplaceAll(text, "\n", "<br>")
			row = append(row, text)
		}
		if len(row) > width {
			width = len(row)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 || width == 0 {
		return ""
	}

	var sb strings.Builder
	writeRow := func(row []string) {
		sb.WriteString("|")
		for i := 0; i < width; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			sb.WriteString(" " + cell + " |")
		}
		sb.WriteString("\n")
	}
	// Markdown tables always have a header row; the first Notion row is used.
	writeRow(rows[0])
	sb.WriteString("|")
	for i := 0; i < width; i++ {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")
	for _, row := range rows[1:] {
		writeRow(row)
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// preserved renders a top-level block that is kept as-is in Notion.
func (r *mdRenderer) preserved(b *notion.Block) string {
	// A one-line stand-in, unless the block has content worth showing (its
	// children, or its own text when it is a normally supported type kept
	// for something like a mention).
	if b.Type == "child_page" || b.Type == "child_database" ||
		(len(b.Children) == 0 && (!notion.IsRoundTripType(b.Type) || b.Type == "image")) {
		return r.describe(b, false) + " " + markerComment(b.Type, b.ID)
	}
	var inner string
	if notion.IsPreservedBlock(b) {
		inner = r.block(b, true).text
	} else {
		inner = r.block(b, true).text
	}
	return regionBegin(b.Type, b.ID) + "\n\n" + inner + "\n\n" + regionEnd(b.ID)
}

// describe renders a short, human-readable stand-in for a kept block.
func (r *mdRenderer) describe(b *notion.Block, nested bool) string {
	switch b.Type {
	case "child_page":
		title := "Untitled"
		if b.ChildPage != nil && b.ChildPage.Title != "" {
			title = b.ChildPage.Title
		}
		return "[📄 " + escapeMarkdown(title, false) + "](" + linkDestination(childPageLink(title)) + ")"
	case "child_database":
		title := ""
		if b.ChildDatabase != nil {
			title = b.ChildDatabase.Title
		}
		label := title
		if label == "" {
			label = "Database"
		}
		return "[🗃️ " + escapeMarkdown(label, false) + "](" + linkDestination("./"+databaseCSVFileName(title, b.ID)) + ")"
	case "image":
		return r.block(&notion.Block{Type: "image", Image: b.Image}, nested).text
	case "link_to_page":
		id := ""
		if b.LinkToPage != nil {
			id = b.LinkToPage.PageID
			if id == "" {
				id = b.LinkToPage.DatabaseID
			}
		}
		return "[↗ Linked page](https://www.notion.so/" + strings.ReplaceAll(id, "-", "") + ")"
	case "embed", "video", "audio", "file", "pdf", "link_preview":
		m := mediaOf(b)
		u := m.MediaURL()
		label := plainFromRichText(captionOf(m))
		if label == "" && m != nil {
			label = m.Name
		}
		if label == "" {
			label = u
		}
		if u == "" {
			return fmt.Sprintf("*Notion %s block (kept in Notion)*", b.Type)
		}
		return "[🔗 " + escapeMarkdown(label, false) + "](" + linkDestination(u) + ")"
	}
	return fmt.Sprintf("*Notion %s block (kept in Notion)*", strings.ReplaceAll(b.Type, "_", " "))
}

func mediaOf(b *notion.Block) *notion.MediaBlock {
	switch b.Type {
	case "embed":
		return b.Embed
	case "video":
		return b.Video
	case "audio":
		return b.Audio
	case "file":
		return b.File
	case "pdf":
		return b.PDF
	case "link_preview":
		return b.LinkPreview
	}
	return nil
}

func captionOf(m *notion.MediaBlock) []notion.RichText {
	if m == nil {
		return nil
	}
	return m.Caption
}

// childPageLink is the relative link from a page's file to a sub-page's file,
// following the pull layout (Page/Page.md with sub-pages in sub-folders).
func childPageLink(title string) string {
	name := util.SanitizeFileName(title)
	return "./" + name + "/" + name + ".md"
}

// databaseCSVFileName is the file name used for an exported database.
func databaseCSVFileName(title, id string) string {
	if strings.TrimSpace(title) == "" {
		short := strings.ReplaceAll(id, "-", "")
		if len(short) > 8 {
			short = short[:8]
		}
		return "Database_" + short + ".csv"
	}
	return sanitizeCSVName(title) + ".csv"
}

// sanitizeCSVName makes a database title safe for a file name.
func sanitizeCSVName(name string) string {
	name = util.SanitizeFileName(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "untitled"
	}
	return name
}

func plainFromRichText(rts []notion.RichText) string {
	var b strings.Builder
	for _, rt := range rts {
		b.WriteString(richTextContent(rt))
	}
	return b.String()
}

func codeFence(code string) string {
	longest, run := 0, 0
	for _, r := range code {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	if longest < 3 {
		return "```"
	}
	return strings.Repeat("`", longest+1)
}
