package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Databases are read and written through their first data source, the way
// API version 2025-09-03 and later require.
func TestClient_DatabasesGoThroughDataSources(t *testing.T) {
	var bodies = map[string]map[string]interface{}{}
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies[r.Method+" "+r.URL.Path] = body
		write := func(v interface{}) { _ = json.NewEncoder(w).Encode(v) }
		switch r.Method + " " + r.URL.Path {
		case "GET /databases/db1":
			write(map[string]interface{}{"object": "database", "id": "db1",
				"title":        []map[string]interface{}{{"type": "text", "plain_text": "Tasks"}},
				"data_sources": []map[string]interface{}{{"id": "ds1", "name": "Tasks"}, {"id": "ds2", "name": "Archive"}}})
		case "GET /data_sources/ds1":
			write(map[string]interface{}{"object": "data_source", "id": "ds1",
				"properties": map[string]interface{}{"Name": map[string]interface{}{"id": "title", "type": "title", "title": map[string]interface{}{}}}})
		case "POST /data_sources/ds1/query":
			write(map[string]interface{}{"object": "list", "results": []interface{}{map[string]interface{}{"object": "page", "id": "row1"}}, "has_more": false})
		case "POST /pages":
			write(map[string]interface{}{"object": "page", "id": "row2"})
		case "POST /databases":
			write(map[string]interface{}{"object": "database", "id": "db2",
				"data_sources": []map[string]interface{}{{"id": "ds3", "name": "New"}}})
		default:
			w.WriteHeader(http.StatusBadRequest)
			write(map[string]interface{}{"object": "error", "code": "invalid_request_url", "message": "Invalid request URL."})
		}
	})
	defer server.Close()
	c := &client{httpClient: &http.Client{Timeout: DefaultTimeout}, token: "t", baseURL: server.URL}
	ctx := context.Background()

	db, err := c.GetDatabase(ctx, "db1")
	require.NoError(t, err)
	assert.Contains(t, db.Properties, "Name", "the schema comes from the data source")

	rows, err := c.QueryDatabase(ctx, "db1", &DatabaseQueryRequest{})
	require.NoError(t, err)
	require.Len(t, rows.Results, 1)

	_, err = c.CreateDatabaseRow(ctx, "db1", map[string]PropertyValue{})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"type": "data_source_id", "data_source_id": "ds1"}, bodies["POST /pages"]["parent"])

	created, err := c.CreateDatabase(ctx, &CreateDatabaseRequest{
		Parent:     Parent{Type: "page_id", PageID: "p1"},
		Properties: map[string]Property{"Name": {Type: "title", Title: &TitleProperty{}}},
	})
	require.NoError(t, err)
	assert.Contains(t, bodies["POST /databases"], "initial_data_source")
	assert.NotContains(t, bodies["POST /databases"], "properties", "the schema belongs to the initial data source")
	_, err = c.CreateDatabaseRow(ctx, created.ID, map[string]PropertyValue{})
	require.NoError(t, err)
	assert.Equal(t, "ds3", bodies["POST /pages"]["parent"].(map[string]interface{})["data_source_id"])

	databaseFetches := 0
	for _, r := range server.requests {
		if r.Method == "GET" && r.Path == "/databases/db1" {
			databaseFetches++
		}
	}
	assert.Equal(t, 1, databaseFetches, "the data source of a database is looked up once")
}

func TestClient_RetriesServiceOverload(t *testing.T) {
	calls := 0
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(statusServiceOverload)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"object": "error", "code": "service_overload", "message": "busy"})
			return
		}
		_ = json.NewEncoder(w).Encode(Page{ID: "p"})
	})
	defer server.Close()
	c := &client{httpClient: &http.Client{Timeout: DefaultTimeout}, token: "t", baseURL: server.URL}

	page, err := c.GetPage(context.Background(), "p")
	require.NoError(t, err)
	assert.Equal(t, "p", page.ID)
	assert.Equal(t, 2, calls)
}

func TestClient_SendsCurrentAPIVersion(t *testing.T) {
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Page{ID: "p"})
	})
	defer server.Close()
	c := &client{httpClient: &http.Client{Timeout: DefaultTimeout}, token: "t", baseURL: server.URL}

	_, err := c.GetPage(context.Background(), "p")
	require.NoError(t, err)
	assert.Equal(t, "2026-03-11", server.requests[0].Headers.Get("Notion-Version"))
}
