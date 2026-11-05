package sync

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Pulled Markdown marks content that stands for a Notion block which is kept
// as-is in Notion (a sub-page, a database, an embed, an unsupported block...)
// with an HTML comment:
//
//	[📄 Child](<./Child/Child.md>) <!-- notion:child_page 0123... -->
//
// or, when the kept block has content worth showing, with a region:
//
//	<!-- notion:column_list 0123... begin -->
//	...read-only copy of the content...
//	<!-- notion:end 0123... -->
//
// On push, a marker line or region is not uploaded. It only records where the
// kept block sits, so the content around it stays in place.

const notionIDPattern = `[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}`

// A marker only counts at the end of a top-level line (indented at most three
// spaces, so not inside indented code), optionally after the stand-in text
// pull writes, in which backticks only appear escaped. Markers quoted in
// inline code, escaped with a backslash or written mid-line are left as
// ordinary text.
var (
	markerPrefix = "(?:(?:[^`\\\\\n]|\\\\.)*[^\\s`\\\\]\\s+)?"
	markerRe     = regexp.MustCompile(`^ {0,3}` + markerPrefix + `<!--\s*notion:([a-z0-9_]+)\s+(` + notionIDPattern + `)(\s+begin)?\s*-->\s*$`)
	endMarkerRe  = regexp.MustCompile(`^ {0,3}<!--\s*notion:end\s+(` + notionIDPattern + `)\s*-->\s*$`)
)

// Placeholder words stand in for anchors and equations while the Markdown is
// parsed. Each conversion uses a fresh random token in them, so text a user
// wrote can't be mistaken for one.
type placeholders struct {
	anchor, math string
}

func newPlaceholders() placeholders {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to a fixed but unusual token.
		copy(b[:], "nmsfixed")
	}
	token := hex.EncodeToString(b[:])
	return placeholders{
		anchor: "NOTIONANCHOR" + token + "N",
		math:   "NOTIONMATH" + token + "N",
	}
}

func markerComment(blockType, id string) string {
	return fmt.Sprintf("<!-- notion:%s %s -->", blockType, id)
}

func regionBegin(blockType, id string) string {
	return fmt.Sprintf("<!-- notion:%s %s begin -->", blockType, id)
}

func regionEnd(id string) string {
	return fmt.Sprintf("<!-- notion:end %s -->", id)
}

// fenceTracker follows fenced code blocks line by line so markers and math
// inside code are left alone.
type fenceTracker struct {
	char byte
	n    int
}

// update consumes a line and reports whether it is inside (or delimits) a
// fenced code block. Fences may be indented (inside list items) or follow a
// list marker or quote marker on the same line.
func (f *fenceTracker) update(line string) bool {
	trimmed := stripContainerPrefix(line)
	if f.n == 0 {
		if c, n := fenceRun(trimmed); n >= 3 {
			if c == '`' && strings.Contains(trimmed[n:], "`") {
				return false // not a fence: backticks in the info string
			}
			f.char, f.n = c, n
			return true
		}
		return false
	}
	if c, n := fenceRun(trimmed); c == f.char && n >= f.n && strings.TrimSpace(trimmed[n:]) == "" {
		f.n = 0
	}
	return true
}

var containerPrefixRe = regexp.MustCompile(`^(?:\s*(?:>|[-*+]|[0-9]{1,9}[.)])(?:\s+|$))*\s*`)

// stripContainerPrefix removes indentation and list or quote markers from
// the start of a line.
func stripContainerPrefix(line string) string {
	return line[len(containerPrefixRe.FindString(line)):]
}

func fenceRun(s string) (byte, int) {
	if s == "" || (s[0] != '`' && s[0] != '~') {
		return 0, 0
	}
	c := s[0]
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	return c, n
}

// preprocessMarkdown replaces marker lines and regions with anchor
// placeholders and display-math blocks ($$ on their own lines) with math
// placeholders. It returns the new content, the anchor block IDs and the math
// expressions, indexed by placeholder number.
func preprocessMarkdown(content string, ph placeholders) (string, []string, []string) {
	lines := strings.Split(content, "\n")

	// Find region ends first, so an unclosed region is treated as a single
	// marker instead of swallowing the rest of the file.
	ends := map[string]int{}
	{
		var ft fenceTracker
		for i, line := range lines {
			if ft.update(line) {
				continue
			}
			if m := endMarkerRe.FindStringSubmatch(line); m != nil {
				ends[normalizeID(m[1])] = i
			}
		}
	}

	var out []string
	var anchors, maths []string
	var ft fenceTracker
	skipUntil := -1
	inList := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if i <= skipUntil {
			continue
		}
		if ft.update(line) {
			out = append(out, line)
			continue
		}
		if m := markerRe.FindStringSubmatch(line); m != nil {
			id := m[2]
			if m[3] != "" {
				if end, ok := ends[normalizeID(id)]; ok && end > i {
					skipUntil = end
				}
			}
			out = append(out, "", fmt.Sprintf("%s%d", ph.anchor, len(anchors)), "")
			anchors = append(anchors, id)
			continue
		}
		if endMarkerRe.MatchString(line) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !isBlank(line) && !isIndentedCode(line, inList) {
			inList = listItemRe.MatchString(line) || (inList && leadingSpaces(line) > 0)
		}
		if strings.HasPrefix(trimmed, "$$") && !isIndentedCode(line, inList) {
			if expr, last, ok := collectMath(lines, i); ok {
				// Keep the indentation so math inside a list item stays there.
				indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				out = append(out, "", indent+fmt.Sprintf("%s%d", ph.math, len(maths)), "")
				maths = append(maths, expr)
				i = last
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), anchors, maths
}

// collectMath reads a $$ ... $$ block starting at line i. It returns the
// expression and the index of the closing line.
func collectMath(lines []string, i int) (string, int, bool) {
	first := strings.TrimSpace(lines[i])
	rest := strings.TrimPrefix(first, "$$")
	if strings.HasSuffix(rest, "$$") {
		expr := strings.TrimSpace(strings.TrimSuffix(rest, "$$"))
		if expr == "" {
			return "", i, false
		}
		return expr, i, true
	}
	var parts []string
	if strings.TrimSpace(rest) != "" {
		parts = append(parts, rest)
	}
	for j := i + 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if strings.HasSuffix(t, "$$") {
			if before := strings.TrimSuffix(t, "$$"); strings.TrimSpace(before) != "" {
				parts = append(parts, before)
			}
			return strings.TrimSpace(strings.Join(parts, "\n")), j, true
		}
		if t == "" {
			return "", i, false // a blank line ends the paragraph; not math
		}
		parts = append(parts, lines[j])
	}
	return "", i, false
}

var listItemRe = regexp.MustCompile(`^ {0,3}([-*+]|[0-9]{1,9}[.)])(\s|$)`)

func isBlank(line string) bool {
	return strings.TrimSpace(line) == ""
}

func leadingSpaces(line string) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

// isIndentedCode reports whether a line is (probably) part of an indented
// code block: indented four or more spaces outside a list. Inside a list the
// indentation belongs to the list item.
func isIndentedCode(line string, inList bool) bool {
	return !inList && leadingSpaces(line) >= 4
}

func normalizeID(id string) string {
	return strings.ToLower(strings.ReplaceAll(id, "-", ""))
}

// placeholderIndex returns n for text "<prefix><n>", or -1.
func placeholderIndex(text, prefix string) int {
	if !strings.HasPrefix(text, prefix) {
		return -1
	}
	return parseInt(strings.TrimPrefix(text, prefix))
}
