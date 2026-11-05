package notion

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const page = "page-1"

// Issue #7: a sub-page's content must not be fetched as part of its parent.
func TestGetPageBlocks_DoesNotDescendIntoSubPages(t *testing.T) {
	f, c := newFakeNotion(t)
	f.add(page, "paragraph", rtField("intro"))
	sub := f.add(page, "child_page", map[string]interface{}{"title": "Sub"})
	f.add(sub, "paragraph", rtField("sub page content"))
	toggle := f.add(page, "toggle", rtField("t"))
	f.add(toggle, "paragraph", rtField("inside toggle"))

	blocks, err := c.GetPageBlocks(context.Background(), page)
	require.NoError(t, err)
	require.Len(t, blocks, 3)
	assert.Equal(t, "child_page", blocks[1].Type)
	assert.Empty(t, blocks[1].Children, "sub-page content must not be pulled into the parent")
	require.Len(t, blocks[2].Children, 1, "toggle content is nested, not flattened")
	for _, req := range f.requests {
		assert.NotContains(t, req, "/blocks/"+sub+"/children")
	}
}

func TestGetPageBlocks_Paginates(t *testing.T) {
	f, c := newFakeNotion(t)
	for i := 0; i < 250; i++ {
		f.add(page, "paragraph", rtField(fmt.Sprintf("p%d", i)))
	}
	blocks, err := c.GetPageBlocks(context.Background(), page)
	require.NoError(t, err)
	assert.Len(t, blocks, 250)
}

func TestUpdatePageBlocks_KeepsSubPagesAndDatabases(t *testing.T) {
	f, c := newFakeNotion(t)
	f.add(page, "paragraph", rtField("old text"))
	sub := f.add(page, "child_page", map[string]interface{}{"title": "Sub"})
	f.add(sub, "paragraph", rtField("sub content"))
	db := f.add(page, "child_database", map[string]interface{}{"title": "DB"})
	toggle := f.add(page, "toggle", rtField("holds a page"))
	f.add(toggle, "child_page", map[string]interface{}{"title": "Nested"})
	f.add(page, "synced_block", map[string]interface{}{})

	err := c.UpdatePageBlocks(context.Background(), page, []map[string]interface{}{para("new text")})
	require.NoError(t, err)

	for _, id := range f.deleted {
		assert.NotEqual(t, sub, id, "sub-page block deleted (that trashes the page)")
		assert.NotEqual(t, db, id, "database block deleted")
		assert.NotEqual(t, toggle, id, "toggle containing a sub-page deleted")
	}
	out := f.outline(page)
	assert.Contains(t, out, "page:Sub")
	assert.Contains(t, out, "  paragraph:sub content")
	assert.Contains(t, out, "paragraph:new text")
	assert.NotContains(t, out, "paragraph:old text")
}

func TestUpdatePageBlocks_AnchorsKeepOrder(t *testing.T) {
	f, c := newFakeNotion(t)
	f.add(page, "paragraph", rtField("old 1"))
	sub := f.add(page, "child_page", map[string]interface{}{"title": "Sub"})
	f.add(page, "paragraph", rtField("old 2"))

	for _, echoAll := range []bool{false, true} {
		f.echoAll = echoAll
		err := c.UpdatePageBlocks(context.Background(), page, []map[string]interface{}{
			para("before"),
			AnchorBlock(sub),
			para("after 1"),
			para("after 2"),
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"paragraph:before", "page:Sub", "paragraph:after 1", "paragraph:after 2"}, f.outline(page), "echoAll=%v", echoAll)
	}
}

func TestUpdatePageBlocks_FailureLeavesPageUnchanged(t *testing.T) {
	f, c := newFakeNotion(t)
	f.add(page, "paragraph", rtField("keep me"))

	blocks := make([]map[string]interface{}, 150)
	for i := range blocks {
		blocks[i] = para(fmt.Sprintf("new %d", i))
	}
	f.failPatchAt = 2 // the second chunk fails

	err := c.UpdatePageBlocks(context.Background(), page, blocks)
	require.Error(t, err)
	assert.Equal(t, []string{"paragraph:keep me"}, f.outline(page), "old content must survive a failed push")
}

func TestUpdatePageBlocks_LargeAndDeepContent(t *testing.T) {
	f, c := newFakeNotion(t)
	f.add(page, "paragraph", rtField("old"))

	// 250 top-level blocks, one with 150 children, and nesting 4 levels deep.
	var blocks []map[string]interface{}
	for i := 0; i < 250; i++ {
		blocks = append(blocks, para(fmt.Sprintf("p%d", i)))
	}
	var many []map[string]interface{}
	for i := 0; i < 150; i++ {
		many = append(many, bullet(fmt.Sprintf("c%d", i)))
	}
	blocks = append(blocks, bullet("many", many...))
	blocks = append(blocks, bullet("l1", bullet("l2", bullet("l3", bullet("l4")))))

	require.NoError(t, c.UpdatePageBlocks(context.Background(), page, blocks))
	out := f.outline(page)
	assert.Equal(t, "paragraph:p0", out[0])
	assert.NotContains(t, out, "paragraph:old")
	assert.Contains(t, out, "  bulleted_list_item:c149")
	assert.Contains(t, out, "      bulleted_list_item:l4")
	top := 0
	for _, l := range out {
		if !strings.HasPrefix(l, " ") {
			top++
		}
	}
	assert.Equal(t, 252, top)
}

func TestDoRequest_RetriesRateLimits(t *testing.T) {
	f, c := newFakeNotion(t)
	f.add(page, "paragraph", rtField("x"))
	f.rateLimitLeft = 2
	blocks, err := c.GetPageBlocks(context.Background(), page)
	require.NoError(t, err)
	assert.Len(t, blocks, 1)
}

func TestDoRequest_ParsesNotionErrors(t *testing.T) {
	_, c := newFakeNotion(t)
	_, err := c.CreatePage(context.Background(), "x", nil)
	require.NoError(t, err) // the fake accepts any page
	err = c.doJSON(context.Background(), "GET", "/nope", nil, nil)
	var apiErr *NotionAPIError
	require.True(t, errors.As(err, &apiErr), "error should be a *NotionAPIError, got %T: %v", err, err)
	assert.Equal(t, 404, apiErr.Code)
	assert.Equal(t, "validation_error", apiErr.ErrorCode)
}

func TestRecreatePageWithBlocks(t *testing.T) {
	f, c := newFakeNotion(t)
	p, err := c.RecreatePageWithBlocks(context.Background(), "parent", map[string]interface{}{}, []map[string]interface{}{para("a"), bullet("b", bullet("c"))})
	require.NoError(t, err)
	assert.Equal(t, []string{"paragraph:a", "bulleted_list_item:b", "  bulleted_list_item:c"}, f.outline(p.ID))
}

func TestPrepareBlock_LimitsNesting(t *testing.T) {
	b := bullet("a", bullet("b", bullet("c")))
	top, pending, count := prepareBlock(b)
	assert.Equal(t, 2, count)
	require.Len(t, pending, 1)
	assert.Equal(t, 0, pending[0].childIndex)
	child := blockChildren(top)[0]
	assert.Empty(t, blockChildren(child), "grandchildren are sent in a later request")
	assert.NotEmpty(t, blockChildren(b), "the original block is not modified")
}
