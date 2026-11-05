package notion

// AnchorBlockType is the type of the placeholder block that the converter puts
// in a block list to say "the following blocks go right after this existing
// block". UpdatePageBlocks consumes anchors; they are never sent to Notion.
const AnchorBlockType = "__anchor__"

// AnchorBlock returns a placeholder that positions the blocks after it below
// the existing block blockID (a sub-page, a database, or another block that is
// kept as-is during a push).
func AnchorBlock(blockID string) map[string]interface{} {
	return map[string]interface{}{
		"type":          AnchorBlockType,
		AnchorBlockType: blockID,
	}
}

// roundTripBlockTypes are the block types that the converter can turn into
// Markdown and back without losing anything. Every other block type is kept
// untouched in Notion when a page is pushed.
var roundTripBlockTypes = map[string]bool{
	"paragraph":          true,
	"heading_1":          true,
	"heading_2":          true,
	"heading_3":          true,
	"bulleted_list_item": true,
	"numbered_list_item": true,
	"to_do":              true,
	"code":               true,
	"quote":              true,
	"callout":            true,
	"divider":            true,
	"table":              true,
	"table_row":          true,
	"image":              true,
	"toggle":             true,
	"bookmark":           true,
	"equation":           true,
}

// IsPreservedBlock reports whether a block, looked at on its own, must be left
// in place when a page's content is replaced: sub-pages and databases (deleting
// their block would move them to the trash), images hosted by Notion (their
// URLs expire, so they can't be re-uploaded from Markdown), and every block
// type the converter cannot round-trip.
func IsPreservedBlock(b *Block) bool {
	if !roundTripBlockTypes[b.Type] {
		return true
	}
	if b.Type == "image" && b.Image != nil && b.Image.File != nil {
		return true
	}
	// Toggleable headings have no Markdown form; re-creating them would make
	// them plain headings.
	for _, h := range []*RichTextBlock{b.Heading1, b.Heading2, b.Heading3} {
		if h != nil && h.IsToggleable {
			return true
		}
	}
	// Mentions (pages, people, dates) and inline equations would come back
	// as plain text.
	for _, rts := range richTextsOf(b) {
		for _, rt := range rts {
			if rt.Type != "" && rt.Type != "text" {
				return true
			}
		}
	}
	return false
}

// IsRoundTripType reports whether blocks of this type can normally be
// converted to Markdown and back.
func IsRoundTripType(blockType string) bool {
	return roundTripBlockTypes[blockType]
}

// richTextsOf returns every rich text array in a block (text, table cells,
// captions).
func richTextsOf(b *Block) [][]RichText {
	var out [][]RichText
	for _, r := range []*RichTextBlock{b.Paragraph, b.Heading1, b.Heading2, b.Heading3, b.BulletedListItem, b.NumberedListItem, b.Quote} {
		if r != nil {
			out = append(out, r.RichText)
		}
	}
	if b.ToDo != nil {
		out = append(out, b.ToDo.RichText)
	}
	if b.Code != nil {
		out = append(out, b.Code.RichText)
	}
	if b.Callout != nil {
		out = append(out, b.Callout.RichText)
	}
	if b.Toggle != nil {
		out = append(out, b.Toggle.RichText)
	}
	if b.TableRow != nil {
		out = append(out, b.TableRow.Cells...)
	}
	if b.Image != nil {
		out = append(out, b.Image.Caption)
	}
	if b.Bookmark != nil {
		out = append(out, b.Bookmark.Caption)
	}
	return out
}

// ShouldPreserveTree reports whether a top-level block must be left in place,
// either because it is preserved itself or because something preserved is
// nested inside it (for example a sub-page inside a toggle, which would be
// trashed along with the toggle).
func ShouldPreserveTree(b *Block) bool {
	if IsPreservedBlock(b) {
		return true
	}
	for i := range b.Children {
		if ShouldPreserveTree(&b.Children[i]) {
			return true
		}
	}
	return false
}
