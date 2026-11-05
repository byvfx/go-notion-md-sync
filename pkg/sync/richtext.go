package sync

import (
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	gutil "github.com/yuin/goldmark/util"
)

// maxRichTextLength is Notion's limit for the content of one rich text item,
// counted in UTF-16 code units.
const maxRichTextLength = 2000

// textStyle is the formatting of a run of text.
type textStyle struct {
	bold, italic, strike, code, underline bool
	link                                  string
}

type textRun struct {
	text  string
	style textStyle
}

// inlineRichText converts the inline children of a Markdown node (paragraph,
// heading, table cell...) into Notion rich text.
func inlineRichText(node ast.Node, source []byte) []map[string]interface{} {
	var runs []textRun
	collectInline(node, source, textStyle{}, &runs)
	return buildRichText(runs)
}

func collectInline(node ast.Node, source []byte, style textStyle, runs *[]textRun) {
	for n := node.FirstChild(); n != nil; n = n.NextSibling() {
		switch v := n.(type) {
		case *ast.Text:
			text := unescapeText(v.Segment.Value(source))
			switch {
			case v.HardLineBreak():
				text += "\n"
			case v.SoftLineBreak():
				text += " "
			}
			*runs = append(*runs, textRun{text, style})
		case *ast.String:
			*runs = append(*runs, textRun{string(v.Value), style})
		case *ast.CodeSpan:
			s := style
			s.code = true
			var b strings.Builder
			for c := v.FirstChild(); c != nil; c = c.NextSibling() {
				switch t := c.(type) {
				case *ast.Text:
					b.Write(t.Segment.Value(source))
				case *ast.String:
					b.Write(t.Value)
				}
			}
			*runs = append(*runs, textRun{b.String(), s})
		case *ast.Emphasis:
			s := style
			if v.Level >= 2 {
				s.bold = true
			} else {
				s.italic = true
			}
			collectInline(v, source, s, runs)
		case *east.Strikethrough:
			s := style
			s.strike = true
			collectInline(v, source, s, runs)
		case *ast.Link:
			s := style
			s.link = allowedLink(string(v.Destination))
			collectInline(v, source, s, runs)
		case *ast.AutoLink:
			s := style
			label := string(v.Label(source))
			u := string(v.URL(source))
			if v.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(u, "mailto:") {
				u = "mailto:" + u
			}
			s.link = allowedLink(u)
			*runs = append(*runs, textRun{label, s})
		case *ast.Image:
			s := style
			s.link = allowedLink(string(v.Destination))
			alt := plainText(v, source)
			if alt == "" {
				alt = string(v.Destination)
			}
			*runs = append(*runs, textRun{alt, s})
		case *ast.RawHTML:
			var b strings.Builder
			for i := 0; i < v.Segments.Len(); i++ {
				seg := v.Segments.At(i)
				b.Write(seg.Value(source))
			}
			raw := b.String()
			switch strings.ToLower(strings.ReplaceAll(raw, " ", "")) {
			case "<br>", "<br/>":
				*runs = append(*runs, textRun{"\n", style})
			case "<u>":
				style.underline = true
			case "</u>":
				style.underline = false
			default:
				*runs = append(*runs, textRun{raw, style})
			}
		case *east.TaskCheckBox:
			// The checkbox becomes the to_do block's "checked" flag.
		default:
			collectInline(n, source, style, runs)
		}
	}
}

// unescapeText resolves backslash escapes and entities in raw Markdown text
// in a single pass, so an escaped "\&amp;" stays "&amp;".
func unescapeText(b []byte) string {
	var out strings.Builder
	for i := 0; i < len(b); i++ {
		c := b[i]
		if c == '\\' && i+1 < len(b) && gutil.IsPunct(b[i+1]) {
			out.WriteByte(b[i+1])
			i++
			continue
		}
		if c == '&' {
			if loc := entityRe.FindIndex(b[i:]); loc != nil && loc[0] == 0 {
				entity := string(b[i : i+loc[1]])
				if decoded := html.UnescapeString(entity); decoded != entity {
					out.WriteString(decoded)
					i += loc[1] - 1
					continue
				}
			}
		}
		out.WriteByte(c)
	}
	return out.String()
}

// allowedLink returns the URL if Notion accepts it as a link target, else "".
// Notion rejects relative links, so links between local files are dropped
// (the link text is kept).
func allowedLink(u string) string {
	l := strings.ToLower(u)
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "mailto:") {
		return u
	}
	return ""
}

func buildRichText(runs []textRun) []map[string]interface{} {
	// Merge neighbours with the same style.
	var merged []textRun
	for _, r := range runs {
		if r.text == "" {
			continue
		}
		if n := len(merged); n > 0 && merged[n-1].style == r.style {
			merged[n-1].text += r.text
			continue
		}
		merged = append(merged, r)
	}
	// Trim a trailing line break left by a hard break at the end.
	if n := len(merged); n > 0 {
		merged[n-1].text = strings.TrimRight(merged[n-1].text, "\n")
		if merged[n-1].text == "" {
			merged = merged[:n-1]
		}
	}

	out := []map[string]interface{}{}
	for _, r := range merged {
		for _, chunk := range chunkText(r.text, maxRichTextLength) {
			out = append(out, richTextItem(chunk, r.style))
		}
	}
	return out
}

// plainRichText builds rich text for unformatted text (code blocks, captions).
func plainRichText(text string) []map[string]interface{} {
	return buildRichText([]textRun{{text: text}})
}

func richTextItem(content string, style textStyle) map[string]interface{} {
	text := map[string]interface{}{"content": content}
	if style.link != "" {
		text["link"] = map[string]interface{}{"url": style.link}
	}
	item := map[string]interface{}{
		"type": "text",
		"text": text,
	}
	ann := map[string]interface{}{}
	if style.bold {
		ann["bold"] = true
	}
	if style.italic {
		ann["italic"] = true
	}
	if style.strike {
		ann["strikethrough"] = true
	}
	if style.code {
		ann["code"] = true
	}
	if style.underline {
		ann["underline"] = true
	}
	if len(ann) > 0 {
		item["annotations"] = ann
	}
	return item
}

// chunkText splits text into pieces of at most limit UTF-16 code units,
// without splitting characters.
func chunkText(text string, limit int) []string {
	var chunks []string
	var b strings.Builder
	units := 0
	for _, r := range text {
		n := utf16.RuneLen(r)
		if n < 0 {
			n = 1
		}
		if units+n > limit {
			chunks = append(chunks, b.String())
			b.Reset()
			units = 0
		}
		b.WriteRune(r)
		units += n
	}
	if b.Len() > 0 {
		chunks = append(chunks, b.String())
	}
	return chunks
}

// plainText returns the text content of a node without formatting.
func plainText(node ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Text:
			b.Write(v.Segment.Value(source))
			if v.SoftLineBreak() || v.HardLineBreak() {
				b.WriteString(" ")
			}
		case *ast.String:
			b.Write(v.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}

// --- Notion rich text to Markdown ---

// richTextStyle returns the Markdown-relevant style of a rich text item.
func richTextStyle(rt notion.RichText) textStyle {
	var s textStyle
	if rt.Annotations != nil {
		s.bold = rt.Annotations.Bold
		s.italic = rt.Annotations.Italic
		s.strike = rt.Annotations.Strikethrough
		s.code = rt.Annotations.Code
		s.underline = rt.Annotations.Underline
	}
	switch {
	case rt.Href != "":
		s.link = rt.Href
	case rt.Text != nil && rt.Text.Link != nil:
		s.link = rt.Text.Link.URL
	}
	return s
}

func richTextContent(rt notion.RichText) string {
	if rt.PlainText != "" {
		return rt.PlainText
	}
	if rt.Text != nil {
		return rt.Text.Content
	}
	return ""
}

// richTextToMarkdown renders Notion rich text as inline Markdown.
func richTextToMarkdown(rts []notion.RichText) string {
	var runs []textRun
	for _, rt := range rts {
		text := richTextContent(rt)
		if text == "" {
			continue
		}
		style := richTextStyle(rt)
		if n := len(runs); n > 0 && runs[n-1].style == style {
			runs[n-1].text += text
			continue
		}
		runs = append(runs, textRun{text, style})
	}

	var b strings.Builder
	for i, r := range runs {
		b.WriteString(renderRun(r, i == 0))
	}
	return b.String()
}

func renderRun(r textRun, atStart bool) string {
	text := r.text
	if r.style.code {
		text = codeSpan(strings.ReplaceAll(text, "\n", " "))
	} else {
		text = escapeMarkdown(text, atStart)
	}

	// Emphasis can't start or end with whitespace: keep it outside.
	core := strings.TrimFunc(text, unicode.IsSpace)
	if core == "" {
		return hardBreaks(text)
	}
	lead := text[:strings.Index(text, core)]
	trail := text[len(lead)+len(core):]

	if r.style.strike {
		core = "~~" + core + "~~"
	}
	if r.style.underline {
		core = "<u>" + core + "</u>"
	}
	switch {
	case r.style.bold && r.style.italic:
		core = "***" + core + "***"
	case r.style.bold:
		core = "**" + core + "**"
	case r.style.italic:
		core = "*" + core + "*"
	}
	if r.style.link != "" {
		core = "[" + core + "](" + linkDestination(r.style.link) + ")"
	}
	return hardBreaks(lead) + hardBreaks(core) + hardBreaks(trail)
}

// hardBreaks turns newlines into Markdown hard line breaks.
func hardBreaks(s string) string {
	return strings.ReplaceAll(s, "\n", "\\\n")
}

func codeSpan(s string) string {
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		s = " " + s + " "
	}
	return fence + s + fence
}

// linkDestination formats a URL for a Markdown link.
func linkDestination(u string) string {
	if strings.ContainsAny(u, " ()<>") {
		return "<" + strings.NewReplacer("<", "%3C", ">", "%3E").Replace(u) + ">"
	}
	return u
}

var entityRe = regexp.MustCompile(`&(#[0-9]+|#[xX][0-9a-fA-F]+|[A-Za-z][A-Za-z0-9]*);`)

// escapeMarkdown escapes characters in plain text that Markdown would
// otherwise read as formatting. atStart says whether the text starts a block.
func escapeMarkdown(s string, atStart bool) string {
	// Byte offsets of "&" that start an entity like &amp; (which Markdown
	// would decode).
	entityStarts := map[int]bool{}
	for _, loc := range entityRe.FindAllStringIndex(s, -1) {
		entityStarts[loc[0]] = true
	}

	var b strings.Builder
	runes := []rune(s)
	offsets := make([]int, len(runes))
	{
		i := 0
		for off := range s {
			offsets[i] = off
			i++
		}
	}
	lineStart := atStart
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if lineStart && unicode.IsDigit(r) {
			// "1. text" at the start of a line would become a list.
			j := i
			for j < len(runes) && unicode.IsDigit(runes[j]) {
				j++
			}
			if j < len(runes) && (runes[j] == '.' || runes[j] == ')') {
				b.WriteString(string(runes[i:j]))
				b.WriteRune('\\')
				b.WriteRune(runes[j])
				i = j
				lineStart = false
				continue
			}
		}
		switch r {
		case '\\', '`', '*', '[', ']', '<', '~':
			b.WriteRune('\\')
		case '&':
			if entityStarts[offsets[i]] {
				b.WriteRune('\\')
			}
		case '_':
			prevWord := i > 0 && isWordRune(runes[i-1])
			nextWord := i+1 < len(runes) && isWordRune(runes[i+1])
			if !prevWord || !nextWord {
				b.WriteRune('\\')
			}
		case '#', '>', '+', '-', '=', '|':
			if lineStart {
				b.WriteRune('\\')
			}
		}
		b.WriteRune(r)
		if r == '\n' {
			lineStart = true
		} else if !unicode.IsSpace(r) {
			lineStart = false
		}
	}
	return b.String()
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
