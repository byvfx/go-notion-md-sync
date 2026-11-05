package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	gosync "sync"
	"time"
)

const (
	BaseURL        = "https://api.notion.com/v1"
	NotionVersion  = "2026-03-11"
	DefaultTimeout = 30 * time.Second
)

type Client interface {
	GetPage(ctx context.Context, pageID string) (*Page, error)
	GetPageBlocks(ctx context.Context, pageID string) ([]Block, error)
	CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*Page, error)
	UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error
	UpdatePageProperties(ctx context.Context, pageID string, properties map[string]interface{}) (*Page, error)
	DeletePage(ctx context.Context, pageID string) error
	RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*Page, error)
	SearchPages(ctx context.Context, query string) ([]Page, error)
	GetChildPages(ctx context.Context, parentID string) ([]Page, error)
	GetAllDescendantPages(ctx context.Context, parentID string) ([]Page, error)

	// Streaming methods for large operations
	StreamDescendantPages(ctx context.Context, parentID string) *PageStream
	StreamDatabaseRows(ctx context.Context, databaseID string) *DatabaseRowStream

	// Database methods
	GetDatabase(ctx context.Context, databaseID string) (*Database, error)
	QueryDatabase(ctx context.Context, databaseID string, request *DatabaseQueryRequest) (*DatabaseQueryResponse, error)
	CreateDatabase(ctx context.Context, request *CreateDatabaseRequest) (*Database, error)
	CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]PropertyValue) (*DatabaseRow, error)
	UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]PropertyValue) (*DatabaseRow, error)
}

type client struct {
	httpClient *http.Client
	token      string
	baseURL    string
	// retryBaseDelay is the first backoff delay for rate-limited or failed
	// requests. Zero means retry immediately (used by tests).
	retryBaseDelay time.Duration
	// deleteDelay is the pause between block deletions.
	deleteDelay time.Duration
	// dataSources caches the data source used for each database ID.
	dataSources gosync.Map
}

// maxRetries is how many times a request is retried after a 429 or a
// temporary server error.
const maxRetries = 5

// maxRetryAfter caps how long we honor a Retry-After header. Notion's
// workspace-wide rate limit can ask for more than a minute.
const maxRetryAfter = 5 * time.Minute

// statusServiceOverload is Notion's "service_overload" status.
const statusServiceOverload = 529

// NotionAPIError is an error response from the Notion API.
type NotionAPIError struct {
	// Code is the HTTP status code.
	Code int `json:"-"`
	// ErrorCode is Notion's error code, for example "validation_error".
	ErrorCode string `json:"code"`
	Message   string `json:"message"`
	PageID    string `json:"-"`
}

func (e *NotionAPIError) Error() string {
	code := ""
	if e.ErrorCode != "" {
		code = " (" + e.ErrorCode + ")"
	}
	if e.PageID != "" {
		return fmt.Sprintf("notion api error %d%s: %s (page: %s)", e.Code, code, e.Message, e.PageID)
	}
	return fmt.Sprintf("notion api error %d%s: %s", e.Code, code, e.Message)
}

// APIURLEnv names an environment variable that overrides the Notion API
// address, for testing against a local server or going through a gateway.
const APIURLEnv = "NOTION_MD_SYNC_API_URL"

// apiBaseURL returns the Notion API address to use.
func apiBaseURL() string {
	if u := strings.TrimRight(os.Getenv(APIURLEnv), "/"); u != "" {
		return u
	}
	return BaseURL
}

func NewClient(token string) Client {
	return &client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		token:          token,
		baseURL:        apiBaseURL(),
		retryBaseDelay: 500 * time.Millisecond,
		deleteDelay:    50 * time.Millisecond,
	}
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusConflict, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout, statusServiceOverload:
		return true
	}
	return false
}

func (c *client) retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if s := resp.Header.Get("Retry-After"); s != "" {
			if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
				d := time.Duration(secs) * time.Second
				if d > maxRetryAfter {
					d = maxRetryAfter
				}
				return d
			}
		}
	}
	return c.retryBaseDelay * time.Duration(1<<attempt)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// doRequest sends a request to the Notion API, retrying on rate limits and
// temporary server errors. On success the caller must close the response body.
func (c *client) doRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		var reqBody io.Reader
		if jsonBody != nil {
			reqBody = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Notion-Version", NotionVersion)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode < 400 {
			return resp, nil
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		closeBody(resp)

		if isRetryableStatus(resp.StatusCode) && attempt < maxRetries {
			if err := sleepCtx(ctx, c.retryDelay(resp, attempt)); err != nil {
				return nil, fmt.Errorf("request cancelled while waiting to retry: %w", err)
			}
			continue
		}

		var apiErr NotionAPIError
		if err := json.Unmarshal(bodyBytes, &apiErr); err != nil || (apiErr.Message == "" && apiErr.ErrorCode == "") {
			return nil, fmt.Errorf("http error %d: %s", resp.StatusCode, string(bodyBytes))
		}
		apiErr.Code = resp.StatusCode
		return nil, &apiErr
	}
}

// closeBody closes a response body, logging (but otherwise ignoring) errors.
func closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	if err := resp.Body.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to close response body: %v\n", err)
	}
}

// doJSON sends a request and decodes the JSON response into out (if non-nil).
func (c *client) doJSON(ctx context.Context, method, endpoint string, body, out interface{}) error {
	resp, err := c.doRequest(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	defer closeBody(resp)
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

func withPageID(err error, pageID string) error {
	var apiErr *NotionAPIError
	if errors.As(err, &apiErr) && apiErr.PageID == "" {
		apiErr.PageID = pageID
	}
	return err
}

func (c *client) GetPage(ctx context.Context, pageID string) (*Page, error) {
	var page Page
	if err := c.doJSON(ctx, "GET", "/pages/"+pageID, nil, &page); err != nil {
		return nil, fmt.Errorf("failed to get page %s: %w", pageID, withPageID(err, pageID))
	}
	return &page, nil
}

// listChildren returns all direct children of a block, following pagination.
func (c *client) listChildren(ctx context.Context, blockID string) ([]Block, error) {
	var all []Block
	cursor := ""
	for {
		endpoint := "/blocks/" + blockID + "/children?page_size=100"
		if cursor != "" {
			endpoint += "&start_cursor=" + url.QueryEscape(cursor)
		}
		var resp BlocksResponse
		if err := c.doJSON(ctx, "GET", endpoint, nil, &resp); err != nil {
			return nil, withPageID(err, blockID)
		}
		all = append(all, resp.Results...)
		if !resp.HasMore || resp.NextCursor == nil || *resp.NextCursor == "" {
			return all, nil
		}
		cursor = *resp.NextCursor
	}
}

// isSeparatePage reports whether a block's children belong to another page.
func isSeparatePage(blockType string) bool {
	return blockType == "child_page" || blockType == "child_database"
}

// GetPageBlocks returns the page's top-level blocks, with nested blocks in
// each block's Children. It does not descend into sub-pages or databases.
func (c *client) GetPageBlocks(ctx context.Context, pageID string) ([]Block, error) {
	blocks, err := c.getBlockTree(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocks for page %s: %w", pageID, err)
	}
	return blocks, nil
}

func (c *client) getBlockTree(ctx context.Context, blockID string) ([]Block, error) {
	blocks, err := c.listChildren(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocks for %s: %w", blockID, err)
	}
	for i := range blocks {
		b := &blocks[i]
		if !b.HasChildren || isSeparatePage(b.Type) {
			continue
		}
		children, err := c.getBlockTree(ctx, b.ID)
		if err != nil {
			return nil, err
		}
		b.Children = children
	}
	return blocks, nil
}

func (c *client) CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*Page, error) {
	createReq := CreatePageRequest{
		Parent: Parent{
			Type:   "page_id",
			PageID: parentID,
		},
		Properties: properties,
	}

	var page Page
	if err := c.doJSON(ctx, "POST", "/pages", createReq, &page); err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	return &page, nil
}

// UpdatePageProperties updates a page's properties (for example its title).
func (c *client) UpdatePageProperties(ctx context.Context, pageID string, properties map[string]interface{}) (*Page, error) {
	var page Page
	req := map[string]interface{}{"properties": properties}
	if err := c.doJSON(ctx, "PATCH", "/pages/"+pageID, req, &page); err != nil {
		return nil, fmt.Errorf("failed to update page %s: %w", pageID, withPageID(err, pageID))
	}
	return &page, nil
}

// UpdatePageBlocks makes a page's content match blocks.
//
// Existing blocks that blocks still contains are left alone, and edited
// blocks are changed in place when their type allows it, so they keep their
// IDs (and the comments and links attached to them); only what is new is
// added and only what is gone is deleted. See blockdiff.go.
//
// It is designed not to lose data:
//   - Sub-pages, databases, Notion-hosted images, block types that can't be
//     round-tripped through Markdown, and any block that contains one of those
//     are never deleted or changed.
//   - New blocks are added first, then blocks are changed in place, and old
//     blocks are deleted only after all of that succeeded. If a step fails,
//     the added blocks are removed and changed blocks are written back, so
//     the page is left as it was.
//
// blocks may contain AnchorBlock placeholders; the blocks after an anchor are
// placed right below that existing block, which keeps text in its place
// around sub-pages and databases.
func (c *client) UpdatePageBlocks(ctx context.Context, pageID string, blocks []map[string]interface{}) error {
	existing, err := c.getBlockTree(ctx, pageID)
	if err != nil {
		return fmt.Errorf("failed to get existing blocks: %w", withPageID(err, pageID))
	}

	var plan pushPlan
	if err := c.syncChildren(ctx, pageID, existing, blocks, &plan); err != nil {
		c.deleteBlocks(ctx, plan.created)
		return fmt.Errorf("failed to update blocks for page %s: %w", pageID, withPageID(err, pageID))
	}
	for i, u := range plan.updates {
		if err := c.doJSON(ctx, "PATCH", "/blocks/"+u.id, u.body, nil); err != nil {
			for j := i - 1; j >= 0; j-- {
				if rerr := c.doJSON(ctx, "PATCH", "/blocks/"+plan.updates[j].id, plan.updates[j].revert, nil); rerr != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not restore block %s: %v\n", plan.updates[j].id, rerr)
				}
			}
			c.deleteBlocks(ctx, plan.created)
			return fmt.Errorf("failed to update block %s on page %s: %w", u.id, pageID, withPageID(err, pageID))
		}
	}
	c.deleteBlocks(ctx, plan.deletes)
	return nil
}

// pushPlan collects what a push did and still has to do.
type pushPlan struct {
	created []string      // blocks added so far (removed again on failure)
	updates []blockUpdate // in-place changes, applied after all additions
	deletes []string      // blocks to delete once everything else worked
}

// blockUpdate is an in-place change of one block, with the body that
// restores it.
type blockUpdate struct {
	id           string
	body, revert map[string]interface{}
}

// syncChildren makes the children of parentID (currently existing) match
// desired. Blocks are added right away; in-place updates and deletions are
// recorded in plan.
func (c *client) syncChildren(ctx context.Context, parentID string, existing []Block, desired []map[string]interface{}, plan *pushPlan) error {
	eForms := make([]blockForm, len(existing))
	eKeys := make([]string, len(existing))
	keptIDs := map[string]string{} // normalized ID -> ID of kept blocks
	for i := range existing {
		b := &existing[i]
		if ShouldPreserveTree(b) {
			keptIDs[normalizeBlockID(b.ID)] = b.ID
			eKeys[i] = anchorKey(b.ID)
			continue
		}
		eForms[i] = formOfExisting(b)
		eKeys[i] = fingerprint(eForms[i])
	}
	dForms := make([]blockForm, len(desired))
	dKeys := make([]string, len(desired))
	for j, d := range desired {
		if t, _ := d["type"].(string); t == AnchorBlockType {
			id, _ := d[AnchorBlockType].(string)
			dKeys[j] = anchorKey(id)
			continue
		}
		dForms[j] = formOfNew(d)
		dKeys[j] = fingerprint(dForms[j])
	}
	match := matchSequences(eKeys, dKeys)

	// prev is where the next new block goes: after this block, or at the
	// start of an empty position.
	prev := positionStart
	if len(existing) == 0 {
		prev = ""
	}
	var pending []map[string]interface{}
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		ids, err := c.appendBlocks(ctx, parentID, pending, prev)
		plan.created = append(plan.created, ids...)
		pending = nil
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			prev = ids[len(ids)-1]
		}
		return nil
	}

	// Walk the desired blocks from one matched block to the next. The
	// unmatched existing blocks in between are reused for edited blocks of
	// the same type, or deleted.
	lastE := -1
	for j := 0; j <= len(desired); {
		next := j
		for next < len(desired) && match[next] < 0 {
			next++
		}
		nextE := len(existing)
		if next < len(desired) {
			nextE = match[next]
		}
		var gap []int
		for i := lastE + 1; i < nextE; i++ {
			if eForms[i] != nil {
				gap = append(gap, i)
			}
		}
		// When as many blocks replace as many old ones, they are edits of
		// those, in order. Otherwise a block only reuses an old one whose
		// text it resembles.
		content := 0
		for k := j; k < next; k++ {
			if dForms[k] != nil {
				content++
			}
		}
		oneToOne := content == len(gap)
		g := 0
		for k := j; k < next; k++ {
			if dForms[k] == nil {
				// An anchor whose block is out of order: put what follows
				// right below it, if it still exists.
				id, _ := desired[k][AnchorBlockType].(string)
				if kept, ok := keptIDs[normalizeBlockID(id)]; ok {
					if err := flush(); err != nil {
						return err
					}
					prev = kept
				}
				continue
			}
			found := -1
			for x := g; x < len(gap); x++ {
				if canUpdateInPlace(eForms[gap[x]], dForms[k]) && (oneToOne || similarText(eForms[gap[x]], dForms[k])) {
					found = x
					break
				}
			}
			if found < 0 {
				pending = append(pending, desired[k])
				continue
			}
			if err := flush(); err != nil {
				return err
			}
			for x := g; x < found; x++ {
				plan.deletes = append(plan.deletes, existing[gap[x]].ID)
			}
			e := &existing[gap[found]]
			if ownFingerprint(eForms[gap[found]]) != ownFingerprint(dForms[k]) {
				plan.updates = append(plan.updates, blockUpdate{id: e.ID, body: updateBody(dForms[k]), revert: updateBody(eForms[gap[found]])})
			}
			if err := c.syncChildren(ctx, e.ID, e.Children, blockChildren(desired[k]), plan); err != nil {
				return err
			}
			prev = e.ID
			g = found + 1
		}
		if err := flush(); err != nil {
			return err
		}
		for x := g; x < len(gap); x++ {
			plan.deletes = append(plan.deletes, existing[gap[x]].ID)
		}
		if next < len(desired) {
			prev = existing[nextE].ID
			lastE = nextE
		}
		j = next + 1
	}
	return nil
}

func anchorKey(id string) string {
	return "\x00anchor:" + normalizeBlockID(id)
}

func normalizeBlockID(id string) string {
	return strings.ToLower(strings.ReplaceAll(id, "-", ""))
}

const (
	// maxChildrenPerRequest is Notion's limit on the length of any children
	// array in one request.
	maxChildrenPerRequest = 100
	// maxBlocksPerRequest is Notion's limit on the total number of blocks in
	// one request.
	maxBlocksPerRequest = 1000
)

// blockChildren returns the nested children of a block map built by the
// converter, if any.
func blockChildren(b map[string]interface{}) []map[string]interface{} {
	t, _ := b["type"].(string)
	inner, ok := b[t].(map[string]interface{})
	if !ok {
		return nil
	}
	children, _ := inner["children"].([]map[string]interface{})
	return children
}

// withChildren returns a shallow copy of b with its children replaced.
func withChildren(b map[string]interface{}, children []map[string]interface{}) map[string]interface{} {
	t, _ := b["type"].(string)
	inner, ok := b[t].(map[string]interface{})
	if !ok {
		return b
	}
	innerCopy := make(map[string]interface{}, len(inner))
	for k, v := range inner {
		innerCopy[k] = v
	}
	if len(children) == 0 {
		delete(innerCopy, "children")
	} else {
		innerCopy["children"] = children
	}
	out := make(map[string]interface{}, len(b))
	for k, v := range b {
		out[k] = v
	}
	out[t] = innerCopy
	return out
}

// pendingChildren records children that could not be sent in the request that
// created their parent (Notion allows two levels of nesting and 100 children
// per array). They are appended once the parent exists.
type pendingChildren struct {
	topIndex   int // index of the top-level block in the request
	childIndex int // -1 for the top-level block itself, else index of its child
	blocks     []map[string]interface{}
}

// positionStart, passed as appendBlocks' after, inserts at the start.
const positionStart = "\x00start"

// appendBlocks appends blocks to parentID (after the block "after", at the
// start when after is positionStart, or at the end when after is ""). It
// returns the IDs of the created top-level blocks, including those created
// before an error.
func (c *client) appendBlocks(ctx context.Context, parentID string, blocks []map[string]interface{}, after string) ([]string, error) {
	var created []string
	for start := 0; start < len(blocks); {
		// Build one request: up to 100 top-level blocks and 1000 blocks total.
		var batch []map[string]interface{}
		var pending []pendingChildren
		total := 0
		end := start
		for end < len(blocks) && len(batch) < maxChildrenPerRequest {
			top, topPending, count := prepareBlock(blocks[end])
			if len(batch) > 0 && total+count > maxBlocksPerRequest {
				break
			}
			for _, p := range topPending {
				p.topIndex = len(batch)
				pending = append(pending, p)
			}
			batch = append(batch, top)
			total += count
			end++
		}

		req := map[string]interface{}{"children": batch}
		switch after {
		case "":
		case positionStart:
			req["position"] = map[string]interface{}{"type": "start"}
		default:
			req["position"] = afterBlock(after)
		}
		var resp BlocksResponse
		if err := c.doJSON(ctx, "PATCH", "/blocks/"+parentID+"/children", req, &resp); err != nil {
			return created, fmt.Errorf("blocks %d-%d: %w", start+1, end, err)
		}

		newIDs, ok := locateCreated(resp.Results, after, len(batch))
		if !ok {
			// Fall back to reading the parent's children. Never guess: these
			// IDs are deleted again on rollback.
			all, err := c.listChildren(ctx, parentID)
			if err == nil {
				newIDs, ok = locateCreated(all, after, len(batch))
			}
			if !ok {
				return created, fmt.Errorf("blocks %d-%d: could not identify the blocks Notion created", start+1, end)
			}
		}
		created = append(created, newIDs...)

		for _, p := range pending {
			target := newIDs[p.topIndex]
			if p.childIndex >= 0 {
				kids, err := c.listChildren(ctx, target)
				if err != nil {
					return created, err
				}
				if p.childIndex >= len(kids) {
					return created, fmt.Errorf("nested block %d not found under %s", p.childIndex, target)
				}
				target = kids[p.childIndex].ID
			}
			if _, err := c.appendBlocks(ctx, target, p.blocks, ""); err != nil {
				return created, err
			}
		}

		if len(newIDs) > 0 {
			after = newIDs[len(newIDs)-1]
		}
		start = end
		if start < len(blocks) {
			_ = sleepCtx(ctx, c.deleteDelay)
		}
	}
	return created, nil
}

// afterBlock is the append position that inserts right below block id.
func afterBlock(id string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "after_block",
		"after_block": map[string]interface{}{"id": id},
	}
}

// locateCreated finds the IDs of n newly created blocks in a list of blocks:
// the n blocks right after the block "after", the first n blocks when after
// is positionStart, or the last n blocks when after is "". Notion's append
// response sometimes lists only the new blocks and
// sometimes all children of the parent, so both shapes are handled.
func locateCreated(results []Block, after string, n int) ([]string, bool) {
	ids := func(bs []Block) []string {
		out := make([]string, 0, len(bs))
		for _, b := range bs {
			out = append(out, b.ID)
		}
		return out
	}
	if len(results) == n {
		return ids(results), true
	}
	if len(results) < n {
		return nil, false
	}
	switch after {
	case "":
		return ids(results[len(results)-n:]), true
	case positionStart:
		return ids(results[:n]), true
	}
	for i, b := range results {
		if b.ID == after {
			if i+1+n > len(results) {
				return nil, false
			}
			return ids(results[i+1 : i+1+n]), true
		}
	}
	return nil, false
}

// prepareBlock trims a block to what Notion accepts in one request (two levels
// of nesting, 100 children per array) and returns the trimmed block, the
// children that must be appended later, and the number of blocks it sends.
func prepareBlock(b map[string]interface{}) (map[string]interface{}, []pendingChildren, int) {
	children := blockChildren(b)
	if len(children) == 0 {
		return b, nil, 1
	}
	var pending []pendingChildren
	sent := children
	if len(sent) > maxChildrenPerRequest {
		pending = append(pending, pendingChildren{childIndex: -1, blocks: children[maxChildrenPerRequest:]})
		sent = children[:maxChildrenPerRequest]
	}
	count := 1
	trimmed := make([]map[string]interface{}, len(sent))
	for i, child := range sent {
		count++
		grandchildren := blockChildren(child)
		if len(grandchildren) == 0 {
			trimmed[i] = child
			continue
		}
		pending = append(pending, pendingChildren{childIndex: i, blocks: grandchildren})
		trimmed[i] = withChildren(child, nil)
	}
	return withChildren(b, trimmed), pending, count
}

// deleteBlocks deletes blocks one by one, logging failures.
func (c *client) deleteBlocks(ctx context.Context, ids []string) {
	for i, id := range ids {
		if err := c.doJSON(ctx, "DELETE", "/blocks/"+id, nil, nil); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete block %s: %v\n", id, err)
			continue
		}
		if i < len(ids)-1 {
			_ = sleepCtx(ctx, c.deleteDelay)
		}
	}
}

func (c *client) DeletePage(ctx context.Context, pageID string) error {
	// Move the page to the trash (Notion doesn't allow true deletion).
	updateReq := map[string]interface{}{
		"in_trash": true,
	}
	if err := c.doJSON(ctx, "PATCH", "/pages/"+pageID, updateReq, nil); err != nil {
		return fmt.Errorf("failed to delete page %s: %w", pageID, withPageID(err, pageID))
	}
	return nil
}

// RecreatePageWithBlocks creates a new page under parentID with the given
// content. Large or deeply nested content is added in several requests.
func (c *client) RecreatePageWithBlocks(ctx context.Context, parentID string, properties map[string]interface{}, blocks []map[string]interface{}) (*Page, error) {
	page, err := c.CreatePage(ctx, parentID, properties)
	if err != nil {
		return nil, fmt.Errorf("failed to recreate page: %w", err)
	}
	if _, err := c.appendBlocks(ctx, page.ID, blocks, ""); err != nil {
		_ = c.DeletePage(ctx, page.ID)
		return nil, fmt.Errorf("failed to add content to recreated page: %w", err)
	}
	return page, nil
}

func (c *client) SearchPages(ctx context.Context, query string) ([]Page, error) {
	searchReq := SearchRequest{
		Query: query,
		Filter: Filter{
			Value:    "page",
			Property: "object",
		},
	}

	var all []Page
	for {
		var searchResp SearchResponse
		if err := c.doJSON(ctx, "POST", "/search", searchReq, &searchResp); err != nil {
			return nil, fmt.Errorf("failed to search pages: %w", err)
		}
		all = append(all, searchResp.Results...)
		if !searchResp.HasMore || searchResp.NextCursor == nil || *searchResp.NextCursor == "" {
			return all, nil
		}
		searchReq.StartCursor = searchResp.NextCursor
	}
}

// GetChildPages returns the sub-pages that are direct children of parentID.
// Sub-pages the integration can't read are skipped with a warning.
func (c *client) GetChildPages(ctx context.Context, parentID string) ([]Page, error) {
	blocks, err := c.listChildren(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get child pages: %w", err)
	}

	var pages []Page
	for _, block := range blocks {
		if block.Type != "child_page" {
			continue
		}
		page, err := c.GetPage(ctx, block.ID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			fmt.Fprintf(os.Stderr, "Warning: skipping sub-page %s: %v\n", block.ID, err)
			continue
		}
		if page.InTrash {
			continue
		}
		pages = append(pages, *page)
	}

	return pages, nil
}

// GetAllDescendantPages returns every sub-page below parentID, parents before
// their children. An error in any branch fails the whole call, so callers
// never mistake a partial tree for the full one.
func (c *client) GetAllDescendantPages(ctx context.Context, parentID string) ([]Page, error) {
	visited := map[string]bool{parentID: true}
	var walk func(id string) ([]Page, error)
	walk = func(id string) ([]Page, error) {
		children, err := c.GetChildPages(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get child pages of %s: %w", id, err)
		}
		var all []Page
		for _, page := range children {
			if visited[page.ID] {
				continue
			}
			visited[page.ID] = true
			all = append(all, page)
			descendants, err := walk(page.ID)
			if err != nil {
				return nil, err
			}
			all = append(all, descendants...)
		}
		return all, nil
	}
	return walk(parentID)
}

// Database methods

//
// The methods take database IDs, which is what child_database blocks and
// links carry. Rows and the schema live in the database's data source, so
// each database is resolved to its first data source; a database with more
// than one is synced through its first one, with a warning.

// GetDatabase returns a database with the schema of its data source in
// Properties.
func (c *client) GetDatabase(ctx context.Context, databaseID string) (*Database, error) {
	database, err := c.getDatabase(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	dsID, err := c.dataSourceID(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	var ds DataSource
	if err := c.doJSON(ctx, "GET", "/data_sources/"+dsID, nil, &ds); err != nil {
		return nil, fmt.Errorf("failed to get data source of database %s: %w", databaseID, err)
	}
	database.Properties = ds.Properties
	return database, nil
}

func (c *client) getDatabase(ctx context.Context, databaseID string) (*Database, error) {
	var database Database
	if err := c.doJSON(ctx, "GET", "/databases/"+databaseID, nil, &database); err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}
	c.rememberDataSource(&database)
	return &database, nil
}

// rememberDataSource caches the data source a database is synced through.
func (c *client) rememberDataSource(database *Database) {
	if len(database.DataSources) == 0 {
		return
	}
	if _, loaded := c.dataSources.LoadOrStore(database.ID, database.DataSources[0].ID); !loaded && len(database.DataSources) > 1 {
		fmt.Fprintf(os.Stderr, "Warning: database %s has %d data sources; only the first (%q) is synced\n",
			database.ID, len(database.DataSources), database.DataSources[0].Name)
	}
}

// dataSourceID returns the data source used for a database.
func (c *client) dataSourceID(ctx context.Context, databaseID string) (string, error) {
	if id, ok := c.dataSources.Load(databaseID); ok {
		return id.(string), nil
	}
	database, err := c.getDatabase(ctx, databaseID)
	if err != nil {
		return "", err
	}
	id, ok := c.dataSources.Load(database.ID)
	if !ok {
		return "", fmt.Errorf("database %s has no data source", databaseID)
	}
	if database.ID != databaseID {
		c.dataSources.Store(databaseID, id)
	}
	return id.(string), nil
}

func (c *client) QueryDatabase(ctx context.Context, databaseID string, request *DatabaseQueryRequest) (*DatabaseQueryResponse, error) {
	dsID, err := c.dataSourceID(ctx, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}
	var queryResp DatabaseQueryResponse
	if err := c.doJSON(ctx, "POST", "/data_sources/"+dsID+"/query", request, &queryResp); err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}
	return &queryResp, nil
}

func (c *client) CreateDatabase(ctx context.Context, request *CreateDatabaseRequest) (*Database, error) {
	// The schema goes to the database's first data source.
	payload := map[string]interface{}{
		"parent":              request.Parent,
		"title":               request.Title,
		"initial_data_source": map[string]interface{}{"properties": request.Properties},
	}
	var database Database
	if err := c.doJSON(ctx, "POST", "/databases", payload, &database); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}
	c.rememberDataSource(&database)
	return &database, nil
}

func (c *client) CreateDatabaseRow(ctx context.Context, databaseID string, properties map[string]PropertyValue) (*DatabaseRow, error) {
	dsID, err := c.dataSourceID(ctx, databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to create database row: %w", err)
	}
	payload := map[string]interface{}{
		"parent": map[string]interface{}{
			"type":           "data_source_id",
			"data_source_id": dsID,
		},
		"properties": properties,
	}

	var row DatabaseRow
	if err := c.doJSON(ctx, "POST", "/pages", payload, &row); err != nil {
		return nil, fmt.Errorf("failed to create database row: %w", err)
	}
	return &row, nil
}

func (c *client) UpdateDatabaseRow(ctx context.Context, pageID string, properties map[string]PropertyValue) (*DatabaseRow, error) {
	payload := map[string]interface{}{
		"properties": properties,
	}

	var row DatabaseRow
	if err := c.doJSON(ctx, "PATCH", "/pages/"+pageID, payload, &row); err != nil {
		return nil, fmt.Errorf("failed to update database row: %w", err)
	}
	return &row, nil
}
