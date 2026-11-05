package notion

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeNotion is a small in-memory Notion API: pages and blocks form a tree,
// and it implements the endpoints the client uses for content. It follows the
// API's limits (100 children per array, two levels of nesting, pagination of
// 100) so tests notice when the client breaks them.
type fakeNotion struct {
	t        *testing.T
	mu       sync.Mutex
	children map[string][]string // parent ID -> child block IDs in order
	blocks   map[string]map[string]interface{}
	nextID   int
	pageSize int

	failPatchAt   int // fail the Nth PATCH request (1-based); 0 = never
	patchCount    int
	rateLimitLeft int // respond 429 to this many requests first
	deleted       []string
	requests      []string
	// echoAll makes append responses list all children of the parent (as
	// Notion does when "after" is used) instead of just the new ones.
	echoAll bool
}

func newFakeNotion(t *testing.T) (*fakeNotion, *client) {
	f := &fakeNotion{
		t:        t,
		children: map[string][]string{},
		blocks:   map[string]map[string]interface{}{},
		pageSize: 100,
	}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	c := &client{httpClient: srv.Client(), token: "test", baseURL: srv.URL}
	return f, c
}

// add appends an existing block to a parent and returns its ID.
func (f *fakeNotion) add(parent, blockType string, fields map[string]interface{}) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := fmt.Sprintf("00000000-0000-0000-0000-%012d", f.nextID)
	b := map[string]interface{}{"id": id, "object": "block", "type": blockType, blockType: fields}
	f.blocks[id] = b
	f.children[parent] = append(f.children[parent], id)
	return id
}

func (f *fakeNotion) text(id string) string {
	b := f.blocks[id]
	t := b["type"].(string)
	inner, _ := b[t].(map[string]interface{})
	if t == "child_page" {
		return "page:" + inner["title"].(string)
	}
	rts, _ := inner["rich_text"].([]interface{})
	var sb strings.Builder
	for _, rt := range rts {
		m := rt.(map[string]interface{})
		if tx, ok := m["text"].(map[string]interface{}); ok {
			sb.WriteString(tx["content"].(string))
		}
	}
	return t + ":" + sb.String()
}

// outline returns the tree under parent as "type:text" lines, indented.
func (f *fakeNotion) outline(parent string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		for _, c := range f.children[id] {
			out = append(out, strings.Repeat("  ", depth)+f.text(c))
			walk(c, depth+1)
		}
	}
	walk(parent, 0)
	return out
}

func (f *fakeNotion) insert(parent string, raw []interface{}, after string, depth int) ([]string, error) {
	if len(raw) > 100 {
		return nil, fmt.Errorf("children length %d > 100", len(raw))
	}
	if depth > 2 {
		return nil, fmt.Errorf("nesting too deep")
	}
	var ids []string
	for _, r := range raw {
		b := r.(map[string]interface{})
		t := b["type"].(string)
		if t == AnchorBlockType {
			return nil, fmt.Errorf("anchor placeholder sent to the API")
		}
		inner, _ := b[t].(map[string]interface{})
		kids, _ := inner["children"].([]interface{})
		delete(inner, "children")
		f.nextID++
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", f.nextID)
		f.blocks[id] = map[string]interface{}{"id": id, "object": "block", "type": t, t: inner}
		ids = append(ids, id)
		if len(kids) > 0 {
			if _, err := f.insert(id, kids, "", depth+1); err != nil {
				return nil, err
			}
		}
	}
	list := f.children[parent]
	pos := len(list)
	if after != "" {
		pos = -1
		for i, c := range list {
			if c == after {
				pos = i + 1
			}
		}
		if pos < 0 {
			return nil, fmt.Errorf("after block %s not found", after)
		}
	}
	newList := append([]string{}, list[:pos]...)
	newList = append(newList, ids...)
	newList = append(newList, list[pos:]...)
	f.children[parent] = newList
	return ids, nil
}

func (f *fakeNotion) blockJSON(id string) map[string]interface{} {
	b := map[string]interface{}{}
	for k, v := range f.blocks[id] {
		b[k] = v
	}
	b["has_children"] = len(f.children[id]) > 0 || b["type"] == "child_page"
	return b
}

func (f *fakeNotion) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)

	if f.rateLimitLeft > 0 {
		f.rateLimitLeft--
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"object":"error","status":429,"code":"rate_limited","message":"slow down"}`))
		return
	}

	fail := func(status int, msg string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"object": "error", "status": status, "code": "validation_error", "message": msg})
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.Method == "GET" && len(parts) == 3 && parts[0] == "blocks" && parts[2] == "children":
		list := f.children[parts[1]]
		start := 0
		if c := r.URL.Query().Get("start_cursor"); c != "" {
			start, _ = strconv.Atoi(c)
		}
		end := start + f.pageSize
		if end > len(list) {
			end = len(list)
		}
		var results []map[string]interface{}
		for _, id := range list[start:end] {
			results = append(results, f.blockJSON(id))
		}
		resp := map[string]interface{}{"object": "list", "results": results, "has_more": end < len(list)}
		if end < len(list) {
			resp["next_cursor"] = strconv.Itoa(end)
		}
		_ = json.NewEncoder(w).Encode(resp)

	case r.Method == "PATCH" && len(parts) == 3 && parts[0] == "blocks" && parts[2] == "children":
		f.patchCount++
		if f.failPatchAt > 0 && f.patchCount == f.failPatchAt {
			fail(400, "simulated failure")
			return
		}
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if _, ok := req["after"]; ok {
			fail(400, "body.after is not supported in this API version; use body.position")
			return
		}
		after := ""
		if pos, ok := req["position"].(map[string]interface{}); ok && pos["type"] == "after_block" {
			ab, _ := pos["after_block"].(map[string]interface{})
			after, _ = ab["id"].(string)
		}
		kids, _ := req["children"].([]interface{})
		total := 0
		var count func([]interface{})
		count = func(bs []interface{}) {
			for _, b := range bs {
				total++
				m := b.(map[string]interface{})
				inner, _ := m[m["type"].(string)].(map[string]interface{})
				if k, ok := inner["children"].([]interface{}); ok {
					count(k)
				}
			}
		}
		count(kids)
		if total > 1000 {
			fail(400, "too many blocks in one request")
			return
		}
		ids, err := f.insert(parts[1], kids, after, 1)
		if err != nil {
			fail(400, err.Error())
			return
		}
		var results []map[string]interface{}
		list := ids
		if f.echoAll {
			list = f.children[parts[1]]
		}
		for _, id := range list {
			results = append(results, f.blockJSON(id))
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"object": "list", "results": results})

	case r.Method == "DELETE" && len(parts) == 2 && parts[0] == "blocks":
		id := parts[1]
		f.deleted = append(f.deleted, id)
		for p, list := range f.children {
			for i, c := range list {
				if c == id {
					f.children[p] = append(list[:i:i], list[i+1:]...)
				}
			}
		}
		_ = json.NewEncoder(w).Encode(f.blocks[id])

	case r.Method == "POST" && len(parts) == 1 && parts[0] == "pages":
		f.nextID++
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", f.nextID)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"object": "page", "id": id, "created_time": time.Now().Format(time.RFC3339)})

	case r.Method == "PATCH" && len(parts) == 2 && parts[0] == "pages":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"object": "page", "id": parts[1]})

	default:
		fail(404, "unexpected request "+r.Method+" "+r.URL.Path)
	}
}

func para(text string) map[string]interface{} {
	return map[string]interface{}{
		"type": "paragraph",
		"paragraph": map[string]interface{}{
			"rich_text": []map[string]interface{}{{"type": "text", "text": map[string]interface{}{"content": text}}},
		},
	}
}

func bullet(text string, children ...map[string]interface{}) map[string]interface{} {
	b := map[string]interface{}{
		"type": "bulleted_list_item",
		"bulleted_list_item": map[string]interface{}{
			"rich_text": []map[string]interface{}{{"type": "text", "text": map[string]interface{}{"content": text}}},
		},
	}
	if len(children) > 0 {
		b["bulleted_list_item"].(map[string]interface{})["children"] = children
	}
	return b
}

func rtField(text string) map[string]interface{} {
	return map[string]interface{}{"rich_text": []interface{}{map[string]interface{}{"type": "text", "text": map[string]interface{}{"content": text}}}}
}
