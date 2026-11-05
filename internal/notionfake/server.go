// Package notionfake is an in-memory imitation of the parts of the Notion
// API that notion-md-sync uses. It is meant for end-to-end tests: start a
// Server, point the CLI at it with NOTION_MD_SYNC_API_URL, and inspect the
// workspace afterwards.
//
// It enforces the API limits that matter for correctness (pagination of 100,
// at most 100 children per array and 1000 blocks per request, two levels of
// nesting, table rows nested under their table, 2000 characters per rich
// text item, valid link URLs) so tests fail where the real API would.
package notionfake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

// Server is a fake Notion API.
type Server struct {
	*httptest.Server

	mu       sync.Mutex
	objects  map[string]map[string]interface{} // pages, blocks and databases by ID
	children map[string][]string               // parent ID -> ordered child block IDs
	rows     map[string][]string               // database ID -> row page IDs
	nextID   int
	clock    time.Time

	// Writes counts block and page changes (appends, deletes, updates).
	Writes int
	// Requests logs "METHOD /path" for every request.
	Requests []string
}

// New starts a fake Notion API.
func New() *Server {
	s := &Server{
		objects:  map[string]map[string]interface{}{},
		children: map[string][]string{},
		rows:     map[string][]string{},
		clock:    time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

func (s *Server) newIDLocked() string {
	s.nextID++
	return fmt.Sprintf("%08x-0000-4000-8000-%012x", s.nextID, s.nextID)
}

func (s *Server) tickLocked() string {
	s.clock = s.clock.Add(time.Minute)
	return s.clock.Format(time.RFC3339)
}

// Text builds a rich text array with one plain text item.
func Text(content string) []interface{} {
	return []interface{}{map[string]interface{}{
		"type":       "text",
		"text":       map[string]interface{}{"content": content},
		"plain_text": content,
	}}
}

// Block builds a block of the given type with the given fields.
func Block(blockType string, fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		fields = map[string]interface{}{}
	}
	return map[string]interface{}{"type": blockType, blockType: fields}
}

// Para builds a paragraph block.
func Para(text string) map[string]interface{} {
	return Block("paragraph", map[string]interface{}{"rich_text": Text(text)})
}

// AddPage creates a page under parent ("" for a top-level page) with the
// given content and returns its ID.
func (s *Server) AddPage(parent, title string, content ...map[string]interface{}) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.addPageLocked(map[string]interface{}{"type": "page_id", "page_id": parent}, titleProps(title), title)
	if parent == "" {
		s.objects[id]["parent"] = map[string]interface{}{"type": "workspace", "workspace": true}
	}
	if _, err := s.seedLocked(id, toIfaces(content)); err != nil {
		panic(err)
	}
	return id
}

// AddBlocks appends blocks (which may have nested "children") to a page or
// block, as a Notion user would.
func (s *Server) AddBlocks(parent string, blocks ...map[string]interface{}) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids, err := s.seedLocked(parent, toIfaces(blocks))
	if err != nil {
		panic(err)
	}
	s.touchLocked(parent)
	return ids
}

// AddDatabase creates an inline database on a page with the given rows
// (each row maps column name to text; the "Name" column is the title).
func (s *Server) AddDatabase(page, title string, columns []string, rows []map[string]string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.newIDLocked()
	props := map[string]interface{}{"Name": map[string]interface{}{"id": "title", "type": "title", "title": map[string]interface{}{}}}
	for _, c := range columns {
		props[c] = map[string]interface{}{"id": c, "type": "rich_text", "rich_text": map[string]interface{}{}}
	}
	s.objects[id] = map[string]interface{}{
		"object": "database", "id": id, "title": Text(title), "properties": props,
		"parent": map[string]interface{}{"type": "page_id", "page_id": page},
	}
	// The child_database block has the database's ID.
	block := map[string]interface{}{"object": "block", "id": id, "type": "child_database", "child_database": map[string]interface{}{"title": title}}
	s.children[page] = append(s.children[page], id)
	s.objects[id]["__block"] = block
	for _, r := range rows {
		rowID := s.newIDLocked()
		rp := map[string]interface{}{"Name": map[string]interface{}{"type": "title", "title": Text(r["Name"])}}
		for _, c := range columns {
			rp[c] = map[string]interface{}{"type": "rich_text", "rich_text": Text(r[c])}
		}
		s.objects[rowID] = map[string]interface{}{"object": "page", "id": rowID, "properties": rp,
			"parent": map[string]interface{}{"type": "database_id", "database_id": id}}
		s.rows[id] = append(s.rows[id], rowID)
	}
	return id
}

// Rename changes a page's title, as a Notion user would.
func (s *Server) Rename(page, title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[page]["properties"] = titleProps(title)
	s.touchLocked(page)
	for _, list := range s.children {
		for _, c := range list {
			if c == page {
				s.blockLocked(c)["child_page"] = map[string]interface{}{"title": title}
			}
		}
	}
}

// SetText replaces the rich text of a block, as a Notion user would.
func (s *Server) SetText(block, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.objects[block]
	t := b["type"].(string)
	b[t].(map[string]interface{})["rich_text"] = Text(text)
	s.touchLocked(s.pageOfLocked(block))
}

// FindBlock returns the ID of the first block under parent (at any depth,
// not inside sub-pages) whose text contains text, or "".
func (s *Server) FindBlock(parent, text string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var find func(id string) string
	find = func(id string) string {
		for _, c := range s.children[id] {
			b := s.blockLocked(c)
			if b["type"] == "child_page" || b["type"] == "child_database" {
				continue
			}
			if strings.Contains(blockText(b), text) {
				return c
			}
			if found := find(c); found != "" {
				return found
			}
		}
		return ""
	}
	return find(parent)
}

// Title returns a page's title.
func (s *Server) Title(page string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return titleOf(s.objects[page])
}

// Archived reports whether a page is archived (in the trash).
func (s *Server) Archived(page string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, _ := s.objects[page]["archived"].(bool)
	return a
}

// SubPages returns the titles of a page's (non-archived) sub-pages, in order.
func (s *Server) SubPages(page string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	for _, c := range s.children[page] {
		if s.blockLocked(c)["type"] == "child_page" {
			if a, _ := s.objects[c]["archived"].(bool); !a {
				out[titleOf(s.objects[c])] = c
			}
		}
	}
	return out
}

// Outline describes a page's content, one line per block ("type: text"),
// with nested blocks indented. Sub-pages are listed but not expanded.
func (s *Server) Outline(page string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		for _, c := range s.children[id] {
			b := s.blockLocked(c)
			t := b["type"].(string)
			out = append(out, strings.Repeat("  ", depth)+t+": "+blockText(b))
			if t != "child_page" && t != "child_database" {
				walk(c, depth+1)
			}
		}
	}
	walk(page, 0)
	return out
}

func blockText(b map[string]interface{}) string {
	t := b["type"].(string)
	inner, _ := b[t].(map[string]interface{})
	switch t {
	case "child_page", "child_database":
		s, _ := inner["title"].(string)
		return s
	case "table_row":
		var cells []string
		for _, c := range inner["cells"].([]interface{}) {
			cells = append(cells, plain(c))
		}
		return strings.Join(cells, " | ")
	case "equation":
		s, _ := inner["expression"].(string)
		return s
	case "bookmark":
		s, _ := inner["url"].(string)
		return s
	}
	text := plain(inner["rich_text"])
	if t == "to_do" {
		if c, _ := inner["checked"].(bool); c {
			return "[x] " + text
		}
		return "[ ] " + text
	}
	return text
}

func plain(rt interface{}) string {
	items, _ := rt.([]interface{})
	var b strings.Builder
	for _, it := range items {
		m, _ := it.(map[string]interface{})
		if p, ok := m["plain_text"].(string); ok {
			b.WriteString(p)
		} else if tx, ok := m["text"].(map[string]interface{}); ok {
			s, _ := tx["content"].(string)
			b.WriteString(s)
		}
	}
	return b.String()
}

func titleProps(title string) map[string]interface{} {
	return map[string]interface{}{"title": map[string]interface{}{"id": "title", "type": "title", "title": Text(title)}}
}

func titleOf(page map[string]interface{}) string {
	props, _ := page["properties"].(map[string]interface{})
	for _, p := range props {
		m, _ := p.(map[string]interface{})
		if m["type"] == "title" {
			return plain(m["title"])
		}
	}
	return ""
}

func toIfaces(in []map[string]interface{}) []interface{} {
	out := make([]interface{}, len(in))
	for i, b := range in {
		out[i] = b
	}
	return out
}

func (s *Server) blockLocked(id string) map[string]interface{} {
	o := s.objects[id]
	if b, ok := o["__block"].(map[string]interface{}); ok {
		return b
	}
	return o
}

func (s *Server) pageOfLocked(id string) string {
	for i := 0; i < 100; i++ {
		o := s.objects[id]
		if o["object"] == "page" {
			return id
		}
		parent, _ := o["parent"].(map[string]interface{})
		next, _ := parent["block_id"].(string)
		if next == "" {
			next, _ = parent["page_id"].(string)
		}
		if next == "" {
			return id
		}
		id = next
	}
	return id
}

func (s *Server) touchLocked(id string) {
	page := s.pageOfLocked(id)
	if p, ok := s.objects[page]; ok && p["object"] == "page" {
		p["last_edited_time"] = s.tickLocked()
	}
}

func (s *Server) addPageLocked(parent, props map[string]interface{}, title string) string {
	id := s.newIDLocked()
	now := s.tickLocked()
	s.objects[id] = map[string]interface{}{
		"object": "page", "id": id, "created_time": now, "last_edited_time": now,
		"parent": parent, "properties": props, "archived": false,
		"__block": map[string]interface{}{"object": "block", "id": id, "type": "child_page", "child_page": map[string]interface{}{"title": title}},
	}
	if pid, _ := parent["page_id"].(string); pid != "" {
		s.children[pid] = append(s.children[pid], id)
		s.touchLocked(pid)
	}
	return id
}

// validate checks a block request the way Notion does.
func validate(b map[string]interface{}) error {
	t, _ := b["type"].(string)
	inner, ok := b[t].(map[string]interface{})
	if !ok {
		return fmt.Errorf("body.children[].%s should be defined", t)
	}
	check := func(rt interface{}) error {
		items, _ := rt.([]interface{})
		if len(items) > 100 {
			return fmt.Errorf("rich_text length should be <= 100")
		}
		for _, it := range items {
			m, _ := it.(map[string]interface{})
			tx, _ := m["text"].(map[string]interface{})
			content, _ := tx["content"].(string)
			if len(utf16.Encode([]rune(content))) > 2000 {
				return fmt.Errorf("text.content.length should be <= 2000")
			}
			if link, ok := tx["link"].(map[string]interface{}); ok {
				u, _ := link["url"].(string)
				if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "mailto:") {
					return fmt.Errorf("invalid URL for link: %q", u)
				}
			}
		}
		return nil
	}
	if err := check(inner["rich_text"]); err != nil {
		return err
	}
	switch t {
	case "table_row":
		return fmt.Errorf("table_row can only be created inside a table")
	case "table":
		rows, _ := inner["children"].([]interface{})
		if len(rows) == 0 {
			return fmt.Errorf("table must be created with at least one row")
		}
		width, _ := inner["table_width"].(float64)
		for _, r := range rows {
			m := r.(map[string]interface{})
			cells, _ := m["table_row"].(map[string]interface{})["cells"].([]interface{})
			if float64(len(cells)) != width {
				return fmt.Errorf("table row has %d cells, table_width is %v", len(cells), width)
			}
			for _, c := range cells {
				if err := check(c); err != nil {
					return err
				}
			}
		}
	case "image":
		ext, _ := inner["external"].(map[string]interface{})
		u, _ := ext["url"].(string)
		if !strings.HasPrefix(u, "http") {
			return fmt.Errorf("image external url invalid: %q", u)
		}
	case "paragraph", "heading_1", "heading_2", "heading_3", "bulleted_list_item", "numbered_list_item",
		"to_do", "toggle", "quote", "callout", "code", "divider", "equation", "bookmark":
	default:
		return fmt.Errorf("block type %q cannot be created by this integration", t)
	}
	return nil
}

func (s *Server) insertLocked(parent string, raw []interface{}, after string, depth int) ([]string, error) {
	return s.insertCheckedLocked(parent, raw, after, depth, true)
}

// seedLocked adds blocks the way a Notion user can, including types an
// integration can't create (columns, synced blocks...).
func (s *Server) seedLocked(parent string, raw []interface{}) ([]string, error) {
	return s.insertCheckedLocked(parent, raw, "", 1, false)
}

func (s *Server) insertCheckedLocked(parent string, raw []interface{}, after string, depth int, checked bool) ([]string, error) {
	if checked && len(raw) > 100 {
		return nil, fmt.Errorf("body.children.length should be <= 100, instead was %d", len(raw))
	}
	if checked && depth > 3 {
		return nil, fmt.Errorf("nesting too deep: at most two levels of children per request")
	}
	var ids []string
	for _, r := range raw {
		b, _ := r.(map[string]interface{})
		t, _ := b["type"].(string)
		isRow := t == "table_row"
		if checked && !isRow {
			if err := validate(b); err != nil {
				return nil, err
			}
		}
		inner, _ := b[t].(map[string]interface{})
		kids, _ := inner["children"].([]interface{})
		copyInner := map[string]interface{}{}
		for k, v := range inner {
			if k != "children" {
				copyInner[k] = v
			}
		}
		// Notion fills in plain_text.
		if rt, ok := copyInner["rich_text"].([]interface{}); ok {
			copyInner["rich_text"] = withPlainText(rt)
		}
		if cells, ok := copyInner["cells"].([]interface{}); ok {
			for i, c := range cells {
				cells[i] = withPlainText(c.([]interface{}))
			}
		}
		id := s.newIDLocked()
		s.objects[id] = map[string]interface{}{
			"object": "block", "id": id, "type": t, t: copyInner,
			"parent": map[string]interface{}{"type": "block_id", "block_id": parent},
		}
		ids = append(ids, id)
		if len(kids) > 0 {
			if _, err := s.insertCheckedLocked(id, kids, "", depth+1, checked); err != nil {
				return nil, err
			}
		}
	}
	list := s.children[parent]
	pos := len(list)
	if after != "" {
		pos = -1
		for i, c := range list {
			if c == after {
				pos = i + 1
			}
		}
		if pos < 0 {
			return nil, fmt.Errorf("block %s to insert after is not a child of %s", after, parent)
		}
	}
	newList := append([]string{}, list[:pos]...)
	newList = append(newList, ids...)
	s.children[parent] = append(newList, list[pos:]...)
	return ids, nil
}

func withPlainText(rt []interface{}) []interface{} {
	for _, it := range rt {
		m, _ := it.(map[string]interface{})
		if tx, ok := m["text"].(map[string]interface{}); ok {
			m["plain_text"] = tx["content"]
			if link, ok := tx["link"].(map[string]interface{}); ok {
				m["href"] = link["url"]
			}
		}
	}
	return rt
}

func (s *Server) blockJSON(id string) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range s.blockLocked(id) {
		if !strings.HasPrefix(k, "__") {
			out[k] = v
		}
	}
	out["has_children"] = len(s.children[id]) > 0
	return out
}

func (s *Server) objectJSON(id string) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range s.objects[id] {
		if !strings.HasPrefix(k, "__") {
			out[k] = v
		}
	}
	return out
}

func (s *Server) removeLocked(id string) {
	for p, list := range s.children {
		for i, c := range list {
			if c == id {
				s.children[p] = append(list[:i:i], list[i+1:]...)
				s.touchLocked(p)
				return
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func apiError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]interface{}{"object": "error", "status": status, "code": code, "message": msg})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Requests = append(s.Requests, r.Method+" "+r.URL.Path)

	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || r.Header.Get("Notion-Version") == "" {
		apiError(w, 401, "unauthorized", "API token is invalid.")
		return
	}

	var body map[string]interface{}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Accept an optional /v1 prefix.
	if len(parts) > 0 && parts[0] == "v1" {
		parts = parts[1:]
	}
	notFound := func(id string) {
		apiError(w, 404, "object_not_found", "Could not find object with ID: "+id)
	}

	switch {
	case r.Method == "GET" && len(parts) == 2 && parts[0] == "pages":
		o, ok := s.objects[parts[1]]
		if !ok || o["object"] != "page" {
			notFound(parts[1])
			return
		}
		writeJSON(w, 200, s.objectJSON(parts[1]))

	case r.Method == "PATCH" && len(parts) == 2 && parts[0] == "pages":
		o, ok := s.objects[parts[1]]
		if !ok {
			notFound(parts[1])
			return
		}
		s.Writes++
		if a, ok := body["archived"].(bool); ok {
			o["archived"] = a
			if a {
				s.removeLocked(parts[1])
			}
		}
		if props, ok := body["properties"].(map[string]interface{}); ok {
			for k, v := range props {
				vm, _ := v.(map[string]interface{})
				if title, ok := vm["title"].([]interface{}); ok {
					vm = map[string]interface{}{"id": "title", "type": "title", "title": withPlainText(title)}
					if b, ok := o["__block"].(map[string]interface{}); ok {
						b["child_page"] = map[string]interface{}{"title": plain(vm["title"])}
					}
				}
				o["properties"].(map[string]interface{})[k] = vm
			}
		}
		o["last_edited_time"] = s.tickLocked()
		writeJSON(w, 200, s.objectJSON(parts[1]))

	case r.Method == "POST" && len(parts) == 1 && parts[0] == "pages":
		parent, _ := body["parent"].(map[string]interface{})
		props, _ := body["properties"].(map[string]interface{})
		pid, _ := parent["page_id"].(string)
		if pid == "" {
			apiError(w, 400, "validation_error", "parent.page_id should be defined")
			return
		}
		if _, ok := s.objects[pid]; !ok {
			notFound(pid)
			return
		}
		title := ""
		if tp, ok := props["title"].(map[string]interface{}); ok {
			if rt, ok := tp["title"].([]interface{}); ok {
				title = plain(withPlainText(rt))
				props = map[string]interface{}{"title": map[string]interface{}{"id": "title", "type": "title", "title": rt}}
			}
		}
		s.Writes++
		id := s.addPageLocked(map[string]interface{}{"type": "page_id", "page_id": pid}, props, title)
		if kids, ok := body["children"].([]interface{}); ok {
			if _, err := s.insertLocked(id, kids, "", 1); err != nil {
				apiError(w, 400, "validation_error", err.Error())
				return
			}
		}
		writeJSON(w, 200, s.objectJSON(id))

	case r.Method == "GET" && len(parts) == 3 && parts[0] == "blocks" && parts[2] == "children":
		if _, ok := s.objects[parts[1]]; !ok {
			notFound(parts[1])
			return
		}
		list := s.children[parts[1]]
		start := 0
		if c := r.URL.Query().Get("start_cursor"); c != "" {
			start, _ = strconv.Atoi(c)
		}
		size := 100
		if ps := r.URL.Query().Get("page_size"); ps != "" {
			if n, err := strconv.Atoi(ps); err == nil && n > 0 && n < size {
				size = n
			}
		}
		end := start + size
		if end > len(list) {
			end = len(list)
		}
		results := []interface{}{}
		for _, id := range list[start:end] {
			results = append(results, s.blockJSON(id))
		}
		resp := map[string]interface{}{"object": "list", "results": results, "has_more": end < len(list), "next_cursor": nil}
		if end < len(list) {
			resp["next_cursor"] = strconv.Itoa(end)
		}
		writeJSON(w, 200, resp)

	case r.Method == "PATCH" && len(parts) == 3 && parts[0] == "blocks" && parts[2] == "children":
		if _, ok := s.objects[parts[1]]; !ok {
			notFound(parts[1])
			return
		}
		kids, _ := body["children"].([]interface{})
		total := 0
		var count func([]interface{})
		count = func(bs []interface{}) {
			for _, b := range bs {
				total++
				m, _ := b.(map[string]interface{})
				t, _ := m["type"].(string)
				inner, _ := m[t].(map[string]interface{})
				if k, ok := inner["children"].([]interface{}); ok {
					count(k)
				}
			}
		}
		count(kids)
		if total > 1000 {
			apiError(w, 400, "validation_error", "too many blocks in one request")
			return
		}
		after, _ := body["after"].(string)
		ids, err := s.insertLocked(parts[1], kids, after, 1)
		if err != nil {
			apiError(w, 400, "validation_error", err.Error())
			return
		}
		s.Writes++
		s.touchLocked(parts[1])
		results := []interface{}{}
		for _, id := range ids {
			results = append(results, s.blockJSON(id))
		}
		writeJSON(w, 200, map[string]interface{}{"object": "list", "results": results})

	case r.Method == "DELETE" && len(parts) == 2 && parts[0] == "blocks":
		id := parts[1]
		if _, ok := s.objects[id]; !ok {
			notFound(id)
			return
		}
		s.Writes++
		if o := s.objects[id]; o["object"] == "page" {
			o["archived"] = true // deleting a child_page block trashes the page
		}
		s.removeLocked(id)
		writeJSON(w, 200, s.blockJSON(id))

	case r.Method == "GET" && len(parts) == 2 && parts[0] == "databases":
		o, ok := s.objects[parts[1]]
		if !ok || o["object"] != "database" {
			notFound(parts[1])
			return
		}
		writeJSON(w, 200, s.objectJSON(parts[1]))

	case r.Method == "POST" && len(parts) == 3 && parts[0] == "databases" && parts[2] == "query":
		results := []interface{}{}
		for _, id := range s.rows[parts[1]] {
			results = append(results, s.objectJSON(id))
		}
		writeJSON(w, 200, map[string]interface{}{"object": "list", "results": results, "has_more": false, "next_cursor": nil})

	case r.Method == "POST" && len(parts) == 1 && parts[0] == "search":
		var results []interface{}
		var ids []string
		for id, o := range s.objects {
			if o["object"] == "page" {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			results = append(results, s.objectJSON(id))
		}
		writeJSON(w, 200, map[string]interface{}{"object": "list", "results": results, "has_more": false})

	default:
		apiError(w, 400, "invalid_request_url", "Invalid request URL: "+r.Method+" "+r.URL.Path)
	}
}
