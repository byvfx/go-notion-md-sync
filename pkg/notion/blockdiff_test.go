package notion

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func heading(text string) map[string]interface{} {
	return map[string]interface{}{
		"type":      "heading_2",
		"heading_2": map[string]interface{}{"rich_text": []map[string]interface{}{{"type": "text", "text": map[string]interface{}{"content": text}}}},
	}
}

// ids returns the block IDs under parent, in order.
func (f *fakeNotion) ids(parent string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.children[parent]...)
}

// seed adds paragraphs a, b, c... to the page and returns their IDs.
func seed(f *fakeNotion, texts ...string) []string {
	var ids []string
	for _, t := range texts {
		ids = append(ids, f.add(page, "paragraph", rtField(t)))
	}
	return ids
}

func TestUpdatePageBlocks_UnchangedPageSendsNothing(t *testing.T) {
	f, c := newFakeNotion(t)
	ids := seed(f, "a", "b", "c")
	require.NoError(t, c.UpdatePageBlocks(context.Background(), page, []map[string]interface{}{para("a"), para("b"), para("c")}))
	assert.Equal(t, ids, f.ids(page))
	assert.Empty(t, f.updates)
	assert.Empty(t, f.deleted)
	assert.Zero(t, f.appended)
}

func TestUpdatePageBlocks_EditKeepsBlockIDs(t *testing.T) {
	tests := []struct {
		name         string
		desired      []map[string]interface{}
		wantOutline  []string
		keep         []int // indexes of seeded blocks that must keep their ID, in final order
		wantUpdates  int
		wantAppended int
		wantDeleted  int
	}{
		{
			name:        "edited paragraph is updated in place",
			desired:     []map[string]interface{}{para("a"), para("B edited"), para("c")},
			wantOutline: []string{"paragraph:a", "paragraph:B edited", "paragraph:c"},
			keep:        []int{0, 1, 2}, wantUpdates: 1,
		},
		{
			name:        "inserted paragraph is the only new block",
			desired:     []map[string]interface{}{para("a"), para("b"), para("new"), para("c")},
			wantOutline: []string{"paragraph:a", "paragraph:b", "paragraph:new", "paragraph:c"},
			keep:        []int{0, 1, -1, 2}, wantAppended: 1,
		},
		{
			name:        "removed paragraph is the only deleted block",
			desired:     []map[string]interface{}{para("a"), para("c")},
			wantOutline: []string{"paragraph:a", "paragraph:c"},
			keep:        []int{0, 2}, wantDeleted: 1,
		},
		{
			name:        "type change replaces just that block",
			desired:     []map[string]interface{}{para("a"), heading("b"), para("c")},
			wantOutline: []string{"paragraph:a", "heading_2:b", "paragraph:c"},
			keep:        []int{0, -1, 2}, wantAppended: 1, wantDeleted: 1,
		},
		{
			name:        "new first block goes to the start",
			desired:     []map[string]interface{}{heading("top"), para("a"), para("b"), para("c")},
			wantOutline: []string{"heading_2:top", "paragraph:a", "paragraph:b", "paragraph:c"},
			keep:        []int{-1, 0, 1, 2}, wantAppended: 1,
		},
		{
			name:        "edits pair with the old block they resemble",
			desired:     []map[string]interface{}{para("a edited"), para("brand new"), para("b edited"), para("c")},
			wantOutline: []string{"paragraph:a edited", "paragraph:brand new", "paragraph:b edited", "paragraph:c"},
			keep:        []int{0, -1, 1, 2}, wantUpdates: 2, wantAppended: 1,
		},
		{
			name:        "rewritten block in place of the old one is updated",
			desired:     []map[string]interface{}{para("a"), para("something else entirely"), para("c")},
			wantOutline: []string{"paragraph:a", "paragraph:something else entirely", "paragraph:c"},
			keep:        []int{0, 1, 2}, wantUpdates: 1,
		},
		{
			name:        "moved block keeps the others",
			desired:     []map[string]interface{}{para("b"), para("c"), para("a")},
			wantOutline: []string{"paragraph:b", "paragraph:c", "paragraph:a"},
			keep:        []int{1, 2, -1}, wantAppended: 1, wantDeleted: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, c := newFakeNotion(t)
			seeded := seed(f, "a", "b", "c")
			require.NoError(t, c.UpdatePageBlocks(context.Background(), page, tt.desired))
			assert.Equal(t, tt.wantOutline, f.outline(page))
			got := f.ids(page)
			require.Len(t, got, len(tt.keep))
			for i, k := range tt.keep {
				if k >= 0 {
					assert.Equal(t, seeded[k], got[i], "block %d should keep its ID", i)
				}
			}
			assert.Len(t, f.updates, tt.wantUpdates, "updates")
			assert.Equal(t, tt.wantAppended, f.appended, "appended")
			assert.Len(t, f.deleted, tt.wantDeleted, "deleted")
		})
	}
}

func TestUpdatePageBlocks_NestedEditKeepsParentAndSiblings(t *testing.T) {
	f, c := newFakeNotion(t)
	parent := f.add(page, "bulleted_list_item", rtField("parent"))
	one := f.add(parent, "bulleted_list_item", rtField("one"))
	two := f.add(parent, "bulleted_list_item", rtField("two"))

	require.NoError(t, c.UpdatePageBlocks(context.Background(), page, []map[string]interface{}{
		bullet("parent", bullet("one"), bullet("two edited"), bullet("three")),
	}))
	assert.Equal(t, []string{
		"bulleted_list_item:parent",
		"  bulleted_list_item:one",
		"  bulleted_list_item:two edited",
		"  bulleted_list_item:three",
	}, f.outline(page))
	assert.Equal(t, []string{parent}, f.ids(page))
	kids := f.ids(parent)
	assert.Equal(t, one, kids[0])
	assert.Equal(t, two, kids[1], "edited nested item keeps its ID")
	assert.Equal(t, []string{two}, f.updates)
	assert.Empty(t, f.deleted)
}

func TestUpdatePageBlocks_ContentAboveLeadingSubPage(t *testing.T) {
	f, c := newFakeNotion(t)
	sub := f.add(page, "child_page", map[string]interface{}{"title": "Sub"})
	f.add(page, "paragraph", rtField("below"))

	require.NoError(t, c.UpdatePageBlocks(context.Background(), page, []map[string]interface{}{
		para("above"), AnchorBlock(sub), para("below"),
	}))
	assert.Equal(t, []string{"paragraph:above", "page:Sub", "paragraph:below"}, f.outline(page))
}

func TestUpdatePageBlocks_FailedUpdateRestoresPage(t *testing.T) {
	f, c := newFakeNotion(t)
	ids := seed(f, "a", "b", "c")
	f.failUpdateAt = 2 // the second in-place update fails

	err := c.UpdatePageBlocks(context.Background(), page, []map[string]interface{}{
		para("a edited"), para("new"), para("b edited"), para("c"),
	})
	require.Error(t, err)
	assert.Equal(t, []string{"paragraph:a", "paragraph:b", "paragraph:c"}, f.outline(page), "the first update is written back, the new block removed")
	assert.Equal(t, ids, f.ids(page))
}

func TestUpdatePageBlocks_TrailingSpacesAreNotAnEdit(t *testing.T) {
	f, c := newFakeNotion(t)
	f.add(page, "bulleted_list_item", rtField("item with a trailing space "))
	require.NoError(t, c.UpdatePageBlocks(context.Background(), page, []map[string]interface{}{bullet("item with a trailing space")}))
	assert.Empty(t, f.updates)
	assert.Zero(t, f.appended)
}

func TestFingerprint_IgnoresWhatMarkdownCannotExpress(t *testing.T) {
	existing := &Block{Type: "code", Code: &CodeBlock{Language: "go", RichText: []RichText{
		{Type: "text", Text: &TextContent{Content: "a()"}, PlainText: "a()", Annotations: &Annotations{Color: "default"}},
		{Type: "text", Text: &TextContent{Content: "\n"}, PlainText: "\n", Annotations: &Annotations{Color: "default"}},
	}}}
	same := map[string]interface{}{"type": "code", "code": map[string]interface{}{
		"language":  "go",
		"rich_text": []map[string]interface{}{{"type": "text", "text": map[string]interface{}{"content": "a()\n"}}},
	}}
	trimmed := map[string]interface{}{"type": "code", "code": map[string]interface{}{
		"language":  "go",
		"rich_text": []map[string]interface{}{{"type": "text", "text": map[string]interface{}{"content": "a()"}}},
	}}
	assert.Equal(t, fingerprint(formOfExisting(existing)), fingerprint(formOfNew(same)), "split runs and default annotations don't matter")
	assert.NotEqual(t, fingerprint(formOfExisting(existing)), fingerprint(formOfNew(trimmed)), "whitespace in code does")
}

func TestMatchSequences(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want []int
	}{
		{"same", []string{"x", "y"}, []string{"x", "y"}, []int{0, 1}},
		{"insert", []string{"x", "z"}, []string{"x", "y", "z"}, []int{0, -1, 1}},
		{"delete", []string{"x", "y", "z"}, []string{"x", "z"}, []int{0, 2}},
		{"replace", []string{"x", "y", "z"}, []string{"x", "q", "z"}, []int{0, -1, 2}},
		{"reorder", []string{"a", "b", "c"}, []string{"c", "a", "b"}, []int{-1, 0, 1}},
		{"duplicates", []string{"d", "d"}, []string{"d", "x", "d"}, []int{0, -1, 1}},
		{"empty", nil, []string{"a"}, []int{-1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchSequences(tt.a, tt.b))
		})
	}
}
