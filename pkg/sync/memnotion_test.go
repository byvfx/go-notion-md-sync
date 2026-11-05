package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	gosync "sync"
	"time"

	"github.com/byvfx/go-notion-md-sync/pkg/notion"
)

// memNotion is an in-memory Notion workspace for engine tests. Pages form a
// tree; each page's content is a list of blocks, and sub-pages appear in
// their parent's content as child_page blocks, as in the real API.
type memNotion struct {
	mu      gosync.Mutex
	pages   map[string]*notion.Page
	content map[string][]notion.Block
	order   []string
	nextID  int
	now     time.Time

	updateCalls int
	createCalls int
	failUpdate  bool
}

func newMemNotion() *memNotion {
	return &memNotion{
		pages:   map[string]*notion.Page{},
		content: map[string][]notion.Block{},
		now:     time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
}

func (m *memNotion) newID() string {
	m.nextID++
	return fmt.Sprintf("%08d-0000-0000-0000-000000000000", m.nextID)
}

func titleProps(title string) map[string]interface{} {
	return map[string]interface{}{
		"title": map[string]interface{}{
			"type":  "title",
			"title": []interface{}{map[string]interface{}{"plain_text": title}},
		},
	}
}

// addPage creates a page (and its child_page block in the parent).
func (m *memNotion) addPage(parent, title string, blocks ...notion.Block) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addPageLocked(parent, title, blocks...)
}

func (m *memNotion) addPageLocked(parent, title string, blocks ...notion.Block) string {
	id := m.newID()
	m.now = m.now.Add(time.Minute)
	p := &notion.Page{ID: id, Properties: titleProps(title), CreatedTime: m.now, LastEditedTime: m.now}
	if parent != "" {
		p.Parent = notion.Parent{Type: "page_id", PageID: parent}
		m.content[parent] = append(m.content[parent], notion.Block{
			ID: id, Type: "child_page", HasChildren: true, ChildPage: &notion.ChildPageBlock{Title: title},
		})
	}
	m.pages[id] = p
	m.order = append(m.order, id)
	m.content[id] = append(m.content[id], blocks...)
	return id
}

func paraBlock(text string) notion.Block {
	return notion.Block{ID: fmt.Sprintf("b-%s", text), Type: "paragraph", Paragraph: &notion.RichTextBlock{
		RichText: []notion.RichText{{Type: "text", PlainText: text, Text: &notion.TextContent{Content: text}}},
	}}
}

func (m *memNotion) title(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return (&engine{}).extractTitleFromPage(m.pages[id])
}

func (m *memNotion) childrenOf(parent string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, id := range m.order {
		p := m.pages[id]
		if p.Parent.PageID == parent && !p.Archived {
			out = append(out, id)
		}
	}
	return out
}

func (m *memNotion) GetPage(ctx context.Context, pageID string) (*notion.Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pages[pageID]
	if !ok {
		return nil, &notion.NotionAPIError{Code: 404, Message: "not found", PageID: pageID}
	}
	cp := *p
	return &cp, nil
}

func (m *memNotion) GetPageBlocks(ctx context.Context, pageID string) ([]notion.Block, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]notion.Block(nil), m.content[pageID]...), nil
}

func (m *memNotion) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*notion.Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls++
	title := ""
	if t, ok := properties["title"].(map[string]interface{}); ok {
		if rts, ok := t["title"].([]notion.RichText); ok && len(rts) > 0 {
			title = rts[0].PlainText
		}
	}
	id := m.addPageLocked(parentID, title)
	cp := *m.pages[id]
	return &cp, nil
}

func (m *memNotion) UpdatePageProperties(ctx context.Context, pageID string, properties map[string]interface{}) (*notion.Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := properties["title"].(map[string]interface{}); ok {
		if rts, ok := t["title"].([]notion.RichText); ok && len(rts) > 0 {
			m.pages[pageID].Properties = titleProps(rts[0].PlainText)
		}
	}
	return m.pages[pageID], nil
}

// UpdatePageBlocks mimics the real client: kept blocks stay, everything else
// is replaced, and anchors position new content after kept blocks.
func (m *memNotion) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls++
	if m.failUpdate {
		return fmt.Errorf("simulated failure")
	}
	kept := map[string]notion.Block{}
	var keptOrder []string
	for _, b := range m.content[pageID] {
		if notion.ShouldPreserveTree(&b) {
			kept[normalizeID(b.ID)] = b
			keptOrder = append(keptOrder, normalizeID(b.ID))
		}
	}
	var out []notion.Block
	used := map[string]bool{}
	for _, raw := range blocks {
		if t, _ := raw["type"].(string); t == notion.AnchorBlockType {
			id := normalizeID(raw[notion.AnchorBlockType].(string))
			if b, ok := kept[id]; ok && !used[id] {
				out = append(out, b)
				used[id] = true
			}
			continue
		}
		out = append(out, blocksFromMaps([]map[string]interface{}{raw})...)
	}
	// Kept blocks without an anchor stay at the top, as with the real client.
	var head []notion.Block
	for _, id := range keptOrder {
		if !used[id] {
			head = append(head, kept[id])
		}
	}
	m.content[pageID] = append(head, out...)
	m.now = m.now.Add(time.Minute)
	m.pages[pageID].LastEditedTime = m.now
	return nil
}

func (m *memNotion) DeletePage(ctx context.Context, pageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.pages[pageID]
	p.Archived = true
	parent := p.Parent.PageID
	var kept []notion.Block
	for _, b := range m.content[parent] {
		if b.ID != pageID {
			kept = append(kept, b)
		}
	}
	m.content[parent] = kept
	return nil
}

func (m *memNotion) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*notion.Page, error) {
	return nil, fmt.Errorf("not used")
}

func (m *memNotion) SearchPages(ctx context.Context, query string) ([]notion.Page, error) {
	return nil, nil
}

func (m *memNotion) GetChildPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	var out []notion.Page
	for _, id := range m.childrenOf(parentID) {
		p, _ := m.GetPage(ctx, id)
		out = append(out, *p)
	}
	return out, nil
}

func (m *memNotion) GetAllDescendantPages(ctx context.Context, parentID string) ([]notion.Page, error) {
	var out []notion.Page
	var walk func(id string)
	walk = func(id string) {
		for _, c := range m.childrenOf(id) {
			p, _ := m.GetPage(ctx, c)
			out = append(out, *p)
			walk(c)
		}
	}
	walk(parentID)
	return out, nil
}

func (m *memNotion) StreamDescendantPages(ctx context.Context, parentID string) *notion.PageStream {
	return notion.NewPageStream()
}

func (m *memNotion) StreamDatabaseRows(ctx context.Context, databaseID string) *notion.DatabaseRowStream {
	return notion.NewDatabaseRowStream()
}

func (m *memNotion) GetDatabase(ctx context.Context, databaseID string) (*notion.Database, error) {
	return &notion.Database{ID: databaseID, Properties: map[string]notion.Property{"Name": {Type: "title"}}}, nil
}

func (m *memNotion) QueryDatabase(ctx context.Context, databaseID string, request *notion.DatabaseQueryRequest) (*notion.DatabaseQueryResponse, error) {
	return &notion.DatabaseQueryResponse{}, nil
}

func (m *memNotion) CreateDatabase(ctx context.Context, request *notion.CreateDatabaseRequest) (*notion.Database, error) {
	return nil, fmt.Errorf("not used")
}

func (m *memNotion) CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return nil, fmt.Errorf("not used")
}

func (m *memNotion) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]notion.PropertyValue) (*notion.DatabaseRow, error) {
	return nil, fmt.Errorf("not used")
}

// blocksFromMaps turns block requests into notion.Block values, as if they
// had been created in Notion and read back.
func blocksFromMaps(in []map[string]interface{}) []notion.Block {
	var out []notion.Block
	for _, m := range in {
		t, _ := m["type"].(string)
		if t == notion.AnchorBlockType {
			continue
		}
		var kids []map[string]interface{}
		if inner, ok := m[t].(map[string]interface{}); ok {
			kids, _ = inner["children"].([]map[string]interface{})
		}
		j, _ := json.Marshal(m)
		var b notion.Block
		_ = json.Unmarshal(j, &b)
		fillPlainText(&b)
		b.Children = blocksFromMaps(kids)
		b.HasChildren = len(b.Children) > 0
		out = append(out, b)
	}
	return out
}

func fillRichText(rts []notion.RichText) {
	for i := range rts {
		if rts[i].Text != nil {
			rts[i].PlainText = rts[i].Text.Content
			if rts[i].Text.Link != nil {
				rts[i].Href = rts[i].Text.Link.URL
			}
		}
	}
}

func fillPlainText(b *notion.Block) {
	for _, r := range []*notion.RichTextBlock{b.Paragraph, b.Heading1, b.Heading2, b.Heading3, b.BulletedListItem, b.NumberedListItem, b.Quote} {
		if r != nil {
			fillRichText(r.RichText)
		}
	}
	if b.ToDo != nil {
		fillRichText(b.ToDo.RichText)
	}
	if b.Code != nil {
		fillRichText(b.Code.RichText)
	}
	if b.Callout != nil {
		fillRichText(b.Callout.RichText)
	}
	if b.Toggle != nil {
		fillRichText(b.Toggle.RichText)
	}
	if b.TableRow != nil {
		for _, c := range b.TableRow.Cells {
			fillRichText(c)
		}
	}
	if b.Image != nil {
		fillRichText(b.Image.Caption)
	}
}

func sortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
