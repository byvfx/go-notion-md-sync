package sync

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toMarkdown(t *testing.T, blocks []notion.Block) string {
	t.Helper()
	md, err := NewConverter().BlocksToMarkdown(blocks)
	require.NoError(t, err)
	return md
}

func toBlocks(t *testing.T, md string) []map[string]interface{} {
	t.Helper()
	blocks, err := NewConverter().MarkdownToBlocks(md)
	require.NoError(t, err)
	return blocks
}

// Markdown produced by a pull must come back unchanged after a push.
func TestConverter_RoundTripIsStable(t *testing.T) {
	docs := map[string]string{
		"inline formatting": "Plain **bold** *italic* ***both*** ~~gone~~ `code` <u>under</u> [link](https://example.com/a)",
		"escaping":          `Stars \*not bold\* and \_under\_ and snake_case and \[brackets\] and AT\&amp;T and 3 \< 4`,
		"line breaks":       "first line\\\nsecond line",
		"start of line":     "\\# not a heading\n\n1\\. not a list\n\n\\- not a bullet",
		"nested lists": "- one\n  - nested\n    1. deep\n- two\n\n" +
			"1. first\n1. second",
		"todos":        "- [ ] open\n- [x] done",
		"quote":        "> just a quote",
		"callout":      "> 💡 an idea\n>\n> more inside",
		"code":         "```go\nfmt.Println(\"$$ not math $$\")\n```",
		"code fences":  "````\nhas ``` inside\n````",
		"table":        "| a | b\\|c |\n| --- | --- |\n| **1** | 2 |",
		"toggle":       "<details>\n<summary>Open me</summary>\n\nhidden\n\n- item\n\n</details>",
		"equation":     "$$\nE = mc^2\n$$",
		"divider":      "above\n\n---\n\nbelow",
		"image":        "![a caption](https://example.com/x.png)",
		"bookmark":     "<https://example.com>",
		"heading":      "# One\n\n## Two **bold**\n\n### Three",
		"long text":    strings.Repeat("abcdefghij", 450),
		"emoji text":   strings.Repeat("😀", 1500),
		"local image":  "!\\[pic\\](./pic.png)",
		"mixed lists":  "- bullet\n\n1. number",
		"empty":        "",
		"html text":    "a \\<b> tag",
		"hard in item": "- line one\\\n  line two",
	}
	for name, md := range docs {
		t.Run(name, func(t *testing.T) {
			blocks := toBlocks(t, md)
			back := toMarkdown(t, blocksFromMaps(blocks))
			assert.Equal(t, md, back)
		})
	}
}

func TestConverter_LongTextIsChunked(t *testing.T) {
	text := strings.Repeat("a", 2500) + strings.Repeat("😀", 1200) // emoji are 2 UTF-16 units
	blocks := toBlocks(t, text)
	require.Len(t, blocks, 1)
	rts := blocks[0]["paragraph"].(map[string]interface{})["rich_text"].([]map[string]interface{})
	require.Greater(t, len(rts), 1)
	var joined strings.Builder
	for _, rt := range rts {
		content := rt["text"].(map[string]interface{})["content"].(string)
		assert.LessOrEqual(t, len(utf16.Encode([]rune(content))), maxRichTextLength)
		joined.WriteString(content)
	}
	assert.Equal(t, text, joined.String())

	code := toBlocks(t, "```\n"+strings.Repeat("x", 4100)+"\n```")
	codeRT := code[0]["code"].(map[string]interface{})["rich_text"].([]map[string]interface{})
	assert.Len(t, codeRT, 3)
}

func TestConverter_TablesNestRows(t *testing.T) {
	blocks := toBlocks(t, "| a | b |\n| --- | --- |\n| 1 |")
	require.Len(t, blocks, 1, "table rows must be children of the table, not siblings")
	table := blocks[0]["table"].(map[string]interface{})
	assert.Equal(t, 2, table["table_width"])
	rows := table["children"].([]map[string]interface{})
	require.Len(t, rows, 2)
	cells := rows[1]["table_row"].(map[string]interface{})["cells"].([][]map[string]interface{})
	assert.Len(t, cells, 2, "short rows are padded to the table width")
}

func TestConverter_MarkersBecomeAnchors(t *testing.T) {
	page := "11111111-2222-3333-4444-555555555555"
	region := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	md := "intro\n\n" +
		"[📄 Sub](./Sub/Sub.md) " + markerComment("child_page", page) + "\n\n" +
		"middle\n\n" +
		regionBegin("column_list", region) + "\n\nread-only copy\n\n" + regionEnd(region) + "\n\n" +
		"end"
	blocks := toBlocks(t, md)
	var got []string
	for _, b := range blocks {
		if b["type"] == notion.AnchorBlockType {
			got = append(got, "anchor:"+b[notion.AnchorBlockType].(string))
			continue
		}
		got = append(got, b["type"].(string))
	}
	assert.Equal(t, []string{"paragraph", "anchor:" + page, "paragraph", "anchor:" + region, "paragraph"}, got)
	assert.NotContains(t, strings.Join(got, ","), "read-only")
}

func TestConverter_UnclosedRegionKeepsFollowingContent(t *testing.T) {
	region := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	blocks := toBlocks(t, regionBegin("column_list", region)+"\n\nstill mine\n\nand this")
	assert.Len(t, blocks, 3, "a region without an end marker must not swallow the rest of the file")
}

func TestConverter_MarkersInCodeAreText(t *testing.T) {
	md := "```\n" + markerComment("child_page", "11111111-2222-3333-4444-555555555555") + "\n$$\nx\n$$\n```"
	blocks := toBlocks(t, md)
	require.Len(t, blocks, 1)
	assert.Equal(t, "code", blocks[0]["type"])
}

func TestConverter_KeptBlocksRenderAsMarkers(t *testing.T) {
	sub := "11111111-2222-3333-4444-555555555555"
	db := "22222222-2222-3333-4444-555555555555"
	cols := "33333333-2222-3333-4444-555555555555"
	toggle := "44444444-2222-3333-4444-555555555555"
	blocks := []notion.Block{
		{ID: sub, Type: "child_page", HasChildren: true, ChildPage: &notion.ChildPageBlock{Title: "My Sub"}},
		{ID: db, Type: "child_database", ChildDatabase: &notion.ChildDatabaseBlock{Title: "Tasks DB"}},
		{ID: cols, Type: "column_list", Children: []notion.Block{{Type: "column", Children: []notion.Block{paraBlock("in a column")}}}},
		{ID: toggle, Type: "toggle", Toggle: &notion.ToggleBlock{}, Children: []notion.Block{
			{ID: "55555555-2222-3333-4444-555555555555", Type: "child_page", ChildPage: &notion.ChildPageBlock{Title: "Hidden"}},
		}},
		{ID: "66666666-2222-3333-4444-555555555555", Type: "image", Image: &notion.ImageBlock{File: &notion.InternalFile{URL: "https://s3/x.png"}}},
	}
	md := toMarkdown(t, blocks)
	assert.Contains(t, md, "[📄 My Sub](<./My Sub/My Sub.md>) "+markerComment("child_page", sub))
	assert.Contains(t, md, "[🗃️ Tasks DB](./Tasks_DB.csv) "+markerComment("child_database", db))
	assert.Contains(t, md, regionBegin("column_list", cols)+"\n\nin a column\n\n"+regionEnd(cols))
	assert.Contains(t, md, regionBegin("toggle", toggle))
	assert.Contains(t, md, "[📄 Hidden](./Hidden/Hidden.md)")
	assert.Contains(t, md, "![](https://s3/x.png) "+markerComment("image", "66666666-2222-3333-4444-555555555555"))

	// And they turn back into anchors, so a push keeps them in place.
	var anchors int
	for _, b := range toBlocks(t, md) {
		if b["type"] == notion.AnchorBlockType {
			anchors++
		}
	}
	assert.Equal(t, len(blocks), anchors)
}

func TestConverter_RelativeLinksAreDroppedButTextKept(t *testing.T) {
	blocks := toBlocks(t, "see [the other page](../Other/Other.md)")
	rts := blocks[0]["paragraph"].(map[string]interface{})["rich_text"].([]map[string]interface{})
	require.Len(t, rts, 1)
	text := rts[0]["text"].(map[string]interface{})
	assert.Equal(t, "see the other page", text["content"])
	assert.NotContains(t, text, "link", "Notion rejects relative links")
}

func TestConverter_SoftBreaksBecomeSpaces(t *testing.T) {
	blocks := toBlocks(t, "wrapped\ntext")
	rts := blocks[0]["paragraph"].(map[string]interface{})["rich_text"].([]map[string]interface{})
	assert.Equal(t, "wrapped text", rts[0]["text"].(map[string]interface{})["content"])
}

func TestConverter_CRLF(t *testing.T) {
	blocks := toBlocks(t, "# Title\r\n\r\ntext\r\n")
	require.Len(t, blocks, 2)
	assert.Equal(t, "heading_1", blocks[0]["type"])
}

func TestConverter_KeepsBlocksWithUnsupportedContent(t *testing.T) {
	headingID := "77777777-2222-3333-4444-555555555555"
	mentionID := "88888888-2222-3333-4444-555555555555"
	blocks := []notion.Block{
		{ID: headingID, Type: "heading_2", Heading2: &notion.RichTextBlock{
			RichText: []notion.RichText{{Type: "text", PlainText: "Toggle heading"}}, IsToggleable: true,
		}, Children: []notion.Block{paraBlock("inside")}},
		{ID: mentionID, Type: "paragraph", Paragraph: &notion.RichTextBlock{RichText: []notion.RichText{
			{Type: "text", PlainText: "Ask "}, {Type: "mention", PlainText: "@Ada"},
		}}},
	}
	md := toMarkdown(t, blocks)
	// Shown read-only, with their text visible.
	assert.Contains(t, md, regionBegin("heading_2", headingID)+"\n\n## Toggle heading\n\ninside\n\n"+regionEnd(headingID))
	assert.Contains(t, md, regionBegin("paragraph", mentionID)+"\n\nAsk @Ada\n\n"+regionEnd(mentionID))

	// On push both stay untouched (anchors only), nothing is uploaded again.
	pushed := toBlocks(t, md)
	require.Len(t, pushed, 2)
	for _, b := range pushed {
		assert.Equal(t, notion.AnchorBlockType, b["type"])
	}
}

func TestConverter_MarkerLookalikesAreText(t *testing.T) {
	id := "11111111-2222-3333-4444-555555555555"
	for name, md := range map[string]string{
		"inline code": "`" + markerComment("child_page", id) + "`",
		"escaped":     `\` + markerComment("child_page", id),
		"mid-line":    "see " + markerComment("child_page", id) + " here",
		"code span":   "use `x` " + markerComment("child_page", id),
	} {
		t.Run(name, func(t *testing.T) {
			for _, b := range toBlocks(t, md) {
				assert.NotEqual(t, notion.AnchorBlockType, b["type"], "%q is not a marker", md)
			}
			assert.NotEmpty(t, toBlocks(t, md), "the text must not disappear")
		})
	}
	// Generated stand-ins with escaped backticks still count.
	line := "[📄 a\\`b](./a_b/a_b.md) " + markerComment("child_page", id)
	assert.Equal(t, notion.AnchorBlockType, toBlocks(t, line)[0]["type"])
}

func TestConverter_KeepsSettingsMarkdownCannotExpress(t *testing.T) {
	row := notion.Block{Type: "table_row", TableRow: &notion.TableRowBlock{Cells: [][]notion.RichText{{{Type: "text", PlainText: "a"}}}}}
	colored := notion.RichText{Type: "text", PlainText: "red", Annotations: &notion.Annotations{Color: "red"}}
	cases := map[string]notion.Block{
		"table without header":  {Type: "table", Table: &notion.TableBlock{TableWidth: 1, HasColumnHeader: false}, Children: []notion.Block{row}},
		"table with row header": {Type: "table", Table: &notion.TableBlock{TableWidth: 1, HasColumnHeader: true, HasRowHeader: true}, Children: []notion.Block{row}},
		"captioned bookmark":    {Type: "bookmark", Bookmark: &notion.BookmarkBlock{URL: "https://x.com", Caption: []notion.RichText{{Type: "text", PlainText: "cap"}}}},
		"colored text":          {Type: "paragraph", Paragraph: &notion.RichTextBlock{RichText: []notion.RichText{colored}}},
		"colored block":         {Type: "paragraph", Paragraph: &notion.RichTextBlock{RichText: []notion.RichText{{Type: "text", PlainText: "x"}}, Color: "blue_background"}},
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			assert.True(t, notion.ShouldPreserveTree(&b))
		})
	}
	plain := notion.Block{Type: "table", Table: &notion.TableBlock{TableWidth: 1, HasColumnHeader: true}, Children: []notion.Block{row}}
	assert.False(t, notion.ShouldPreserveTree(&plain), "ordinary tables still round-trip")
	callout := notion.Block{Type: "callout", Callout: &notion.CalloutBlock{Color: "gray_background", Icon: &notion.CalloutIcon{Type: "emoji", Emoji: "💡"}}}
	assert.False(t, notion.ShouldPreserveTree(&callout), "the default callout color is not a custom color")
}

func TestConverter_PlaceholderLookalikesStayText(t *testing.T) {
	id := "11111111-2222-3333-4444-555555555555"
	md := "NOTION_ANCHOR_0\n\nNOTION_MATH_0\n\n[📄 A](./A/A.md) " + markerComment("child_page", id) + "\n\n$$\nx\n$$"
	var types []string
	for _, b := range toBlocks(t, md) {
		types = append(types, b["type"].(string))
	}
	assert.Equal(t, []string{"paragraph", "paragraph", notion.AnchorBlockType, "equation"}, types)
}

func TestConverter_MathInIndentedCodeIsCode(t *testing.T) {
	blocks := toBlocks(t, "text\n\n    $$\n    literal\n    $$")
	require.Len(t, blocks, 2)
	assert.Equal(t, "code", blocks[1]["type"])

	// Inside a list the indentation belongs to the item, so it is math.
	nested := toBlocks(t, "- item\n  - sub\n\n    $$\n    x^2\n    $$")
	require.Len(t, nested, 1)
	assert.Contains(t, fmt.Sprint(nested), "equation")
}

func TestConverter_KeepsBlocksWhoseMarkdownWouldChangeThem(t *testing.T) {
	styled := []notion.RichText{{Type: "text", PlainText: "bold", Annotations: &notion.Annotations{Bold: true}}}
	cases := map[string]notion.Block{
		"quote starting with emoji":  {Type: "quote", Quote: &notion.RichTextBlock{RichText: []notion.RichText{{Type: "text", PlainText: "💡 Remember this"}}}},
		"callout with external icon": {Type: "callout", Callout: &notion.CalloutBlock{Icon: &notion.CalloutIcon{Type: "external"}}},
		"callout without icon":       {Type: "callout", Callout: &notion.CalloutBlock{}},
		"styled toggle summary":      {Type: "toggle", Toggle: &notion.ToggleBlock{RichText: styled}},
		"styled image caption":       {Type: "image", Image: &notion.ImageBlock{External: &notion.ExternalFile{URL: "https://x/y.png"}, Caption: styled}},
		"paragraph with children":    {Type: "paragraph", Paragraph: &notion.RichTextBlock{}, Children: []notion.Block{paraBlock("nested")}},
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			assert.True(t, notion.ShouldPreserveTree(&b))
		})
	}
	quote := notion.Block{Type: "quote", Quote: &notion.RichTextBlock{RichText: []notion.RichText{{Type: "text", PlainText: "plain quote"}}}}
	assert.False(t, notion.ShouldPreserveTree(&quote))
}

func TestConverter_FencesInsideListItems(t *testing.T) {
	id := "11111111-2222-3333-4444-555555555555"
	for name, md := range map[string]string{
		"fence on the marker line": "- ```\n  " + markerComment("child_page", id) + "\n  $$\n  x\n  $$\n  ```",
		"deeply indented fence":    "- a\n  - b\n\n        ```\n        " + markerComment("child_page", id) + "\n        ```",
	} {
		t.Run(name, func(t *testing.T) {
			out := fmt.Sprint(toBlocks(t, md))
			assert.NotContains(t, out, notion.AnchorBlockType)
			assert.NotContains(t, out, "equation")
			assert.Contains(t, out, "notion:child_page", "the code keeps its text")
		})
	}
}

func TestConverter_QuotedFenceLineInsideCodeIsCode(t *testing.T) {
	id := "11111111-2222-3333-4444-555555555555"
	md := "```\n> ```\n" + markerComment("child_page", id) + "\n```"
	blocks := toBlocks(t, md)
	require.Len(t, blocks, 1)
	assert.Equal(t, "code", blocks[0]["type"])
	assert.Contains(t, fmt.Sprint(blocks), "notion:child_page", "the marker text stays in the code")
}
