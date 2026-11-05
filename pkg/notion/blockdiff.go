package notion

import (
	"encoding/json"
	"strings"
	"unicode"
)

// Pushing a page compares the blocks Notion has with the blocks the Markdown
// asks for, so that unchanged blocks are left alone and edited ones are
// updated in place. Either way they keep their IDs, and with them comments,
// links to the block and edit history.
//
// Blocks are compared by fingerprint: the parts of a block that Markdown can
// express (type, text and its formatting, links, checked state, code
// language, URLs, table cells, children). Everything else (block IDs,
// timestamps, colors) is ignored. Blocks that carry anything Markdown can't
// express are kept blocks (see ShouldPreserveTree) and never get here.

// blockForm is a block in the shape the converter produces: type, inner
// fields and, for nested blocks, inner "children".
type blockForm = map[string]interface{}

// formOfExisting turns an existing block (with its nested Children) into the
// generic form used for fingerprints and reverts.
func formOfExisting(b *Block) blockForm {
	m := toGeneric(b)
	if len(b.Children) > 0 {
		inner, _ := m[b.Type].(map[string]interface{})
		if inner == nil {
			inner = map[string]interface{}{}
			m[b.Type] = inner
		}
		kids := make([]interface{}, len(b.Children))
		for i := range b.Children {
			kids[i] = formOfExisting(&b.Children[i])
		}
		inner["children"] = kids
	}
	return m
}

// formOfNew normalises a converter block into the same generic form, with
// []interface{} slices and float64 numbers like decoded JSON.
func formOfNew(b map[string]interface{}) blockForm {
	return toGeneric(b)
}

func toGeneric(v interface{}) map[string]interface{} {
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func formType(m blockForm) string {
	t, _ := m["type"].(string)
	return t
}

func formInner(m blockForm) map[string]interface{} {
	inner, _ := m[formType(m)].(map[string]interface{})
	return inner
}

func formChildren(m blockForm) []blockForm {
	raw, _ := formInner(m)["children"].([]interface{})
	out := make([]blockForm, 0, len(raw))
	for _, r := range raw {
		if c, ok := r.(map[string]interface{}); ok {
			out = append(out, c)
		}
	}
	return out
}

// ownFingerprint describes a block without its children.
func ownFingerprint(m blockForm) string {
	t := formType(m)
	out := map[string]interface{}{"type": t}
	for k, v := range formInner(m) {
		switch k {
		case "rich_text":
			// Code keeps its whitespace; elsewhere trailing spaces are
			// meaningless and Markdown drops them.
			if rt := canonRichText(v, t != "code"); rt != nil {
				out[k] = rt
			}
		case "caption":
			if rt := canonRichText(v, true); rt != nil {
				out[k] = rt
			}
		case "cells":
			rows, _ := v.([]interface{})
			cells := make([]interface{}, len(rows))
			for i, c := range rows {
				cells[i] = canonRichText(c, true)
			}
			out[k] = cells
		case "icon":
			if icon, ok := v.(map[string]interface{}); ok && icon["emoji"] != nil {
				out[k] = icon["emoji"]
			}
		case "external":
			if ext, ok := v.(map[string]interface{}); ok {
				out[k] = ext["url"]
			}
		case "checked", "language", "url", "expression", "table_width", "has_column_header", "has_row_header", "is_toggleable":
			if !isZeroValue(v) {
				out[k] = v
			}
		}
	}
	data, _ := json.Marshal(out)
	return string(data)
}

// fingerprint describes a block and everything nested in it.
func fingerprint(m blockForm) string {
	kids := formChildren(m)
	if len(kids) == 0 {
		return ownFingerprint(m)
	}
	parts := make([]string, len(kids))
	for i, k := range kids {
		parts[i] = fingerprint(k)
	}
	return ownFingerprint(m) + "[" + strings.Join(parts, ",") + "]"
}

func isZeroValue(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case string:
		return x == ""
	case float64:
		return x == 0
	}
	return false
}

type canonRun struct {
	Text  string `json:"t"`
	Link  string `json:"l,omitempty"`
	Style string `json:"s,omitempty"`
}

// canonRichText reduces rich text to text runs with their link and
// formatting, merging neighbours that look the same (Notion and the
// converter may split text differently). It returns nil for empty text.
func canonRichText(v interface{}, trimEnd bool) []canonRun {
	items, _ := v.([]interface{})
	var runs []canonRun
	for _, it := range items {
		m, _ := it.(map[string]interface{})
		run := canonRun{}
		if tx, ok := m["text"].(map[string]interface{}); ok {
			run.Text, _ = tx["content"].(string)
			if link, ok := tx["link"].(map[string]interface{}); ok {
				run.Link, _ = link["url"].(string)
			}
		} else {
			// Mentions and inline equations: never equal to plain text.
			pt, _ := m["plain_text"].(string)
			typ, _ := m["type"].(string)
			run.Text = "\x00" + typ + ":" + pt
		}
		if run.Link == "" {
			run.Link, _ = m["href"].(string)
		}
		if a, ok := m["annotations"].(map[string]interface{}); ok {
			for _, f := range []struct{ key, flag string }{{"bold", "b"}, {"italic", "i"}, {"strikethrough", "s"}, {"underline", "u"}, {"code", "c"}} {
				if on, _ := a[f.key].(bool); on {
					run.Style += f.flag
				}
			}
		}
		if n := len(runs); n > 0 && runs[n-1].Link == run.Link && runs[n-1].Style == run.Style {
			runs[n-1].Text += run.Text
			continue
		}
		runs = append(runs, run)
	}
	if trimEnd && len(runs) > 0 {
		last := &runs[len(runs)-1]
		last.Text = strings.TrimRight(last.Text, " \t\n")
		if last.Text == "" {
			runs = runs[:len(runs)-1]
		}
	}
	if len(runs) == 0 {
		return nil
	}
	return runs
}

// updatableTypes are the block types whose own content can be changed with
// PATCH /v1/blocks/{id}, which keeps the block's ID. Children are never
// changed that way; they are compared and updated one by one.
var updatableTypes = map[string]bool{
	"paragraph":          true,
	"heading_1":          true,
	"heading_2":          true,
	"heading_3":          true,
	"bulleted_list_item": true,
	"numbered_list_item": true,
	"to_do":              true,
	"toggle":             true,
	"quote":              true,
	"callout":            true,
	"code":               true,
	"equation":           true,
}

// canUpdateInPlace reports whether existing can be turned into desired
// without replacing it.
func canUpdateInPlace(existing, desired blockForm) bool {
	t := formType(existing)
	if t != formType(desired) || !updatableTypes[t] {
		return false
	}
	// Only plain text can be written back exactly if the update has to be
	// reverted.
	items, _ := formInner(existing)["rich_text"].([]interface{})
	for _, it := range items {
		if m, _ := it.(map[string]interface{}); m["type"] != nil && m["type"] != "text" {
			return false
		}
	}
	return true
}

// similarText reports whether two blocks' texts share at least a third of
// their words, so an edited block is matched with the block it came from
// rather than with whatever block happens to be next.
func similarText(a, b blockForm) bool {
	wa, wb := words(a), words(b)
	if len(wa) == 0 && len(wb) == 0 {
		return true
	}
	common := 0
	for w := range wa {
		if wb[w] {
			common++
		}
	}
	union := len(wa) + len(wb) - common
	return common*3 >= union
}

func words(m blockForm) map[string]bool {
	out := map[string]bool{}
	for _, run := range canonRichText(formInner(m)["rich_text"], true) {
		for _, w := range strings.FieldsFunc(strings.ToLower(run.Text), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}) {
			out[w] = true
		}
	}
	return out
}

// updateBody builds the PATCH /v1/blocks/{id} body that gives a block the
// content of form, without touching its children.
func updateBody(form blockForm) map[string]interface{} {
	t := formType(form)
	inner := map[string]interface{}{}
	for k, v := range formInner(form) {
		switch k {
		case "children":
		case "rich_text", "caption":
			inner[k] = cleanRichText(v)
		default:
			inner[k] = v
		}
	}
	if _, ok := inner["rich_text"]; !ok {
		inner["rich_text"] = []interface{}{}
	}
	return map[string]interface{}{t: inner}
}

// cleanRichText keeps only the request fields of text items, so a block read
// from Notion can be written back.
func cleanRichText(v interface{}) []interface{} {
	items, _ := v.([]interface{})
	out := make([]interface{}, 0, len(items))
	for _, it := range items {
		m, _ := it.(map[string]interface{})
		item := map[string]interface{}{"type": "text"}
		if tx, ok := m["text"]; ok {
			item["text"] = tx
		}
		if a, ok := m["annotations"]; ok {
			item["annotations"] = a
		}
		out = append(out, item)
	}
	return out
}

// maxDiffCells bounds the size of the comparison table. Bigger lists are
// compared by their common start and end only.
const maxDiffCells = 4_000_000

// matchSequences pairs equal keys of a and b in order (a longest common
// subsequence). It returns, for each index of b, the matched index of a or
// -1.
func matchSequences(a, b []string) []int {
	match := make([]int, len(b))
	for i := range match {
		match[i] = -1
	}
	// Common start and end are matched directly.
	lo := 0
	for lo < len(a) && lo < len(b) && a[lo] == b[lo] {
		match[lo] = lo
		lo++
	}
	hiA, hiB := len(a), len(b)
	for hiA > lo && hiB > lo && a[hiA-1] == b[hiB-1] {
		hiA--
		hiB--
		match[hiB] = hiA
	}
	n, m := hiA-lo, hiB-lo
	if n == 0 || m == 0 || n*m > maxDiffCells {
		return match
	}
	// lcs[i][j] is the LCS length of a[lo+i:hiA] and b[lo+j:hiB].
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case a[lo+i] == b[lo+j]:
				lcs[i][j] = lcs[i+1][j+1] + 1
			case lcs[i+1][j] >= lcs[i][j+1]:
				lcs[i][j] = lcs[i+1][j]
			default:
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case a[lo+i] == b[lo+j]:
			match[lo+j] = lo + i
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			i++
		default:
			j++
		}
	}
	return match
}
