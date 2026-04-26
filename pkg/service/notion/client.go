package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

const (
	apiBase = "https://api.notion.com"

	// notionVersionStable is used for non-Markdown endpoints.
	notionVersionStable = "2022-06-28"

	// notionVersionMarkdown unlocks GET /v1/pages/{id}/markdown.
	notionVersionMarkdown = "2026-03-11"
)

var (
	// ErrUnauthorized signals a 401 from the Notion API (token rejected).
	ErrUnauthorized = errors.New("notion: unauthorized")
	// ErrForbidden signals a 403 (integration not invited / capability missing).
	ErrForbidden = errors.New("notion: forbidden")
	// ErrNotFound signals a 404 (object missing or not visible to the integration).
	ErrNotFound = errors.New("notion: not found")
	// ErrMarkdownUnsupported is returned when the Markdown endpoint refuses
	// the object (e.g. database row pages on certain plans). Callers can fall
	// back to other strategies.
	ErrMarkdownUnsupported = errors.New("notion: markdown endpoint unsupported for this object")
)

// Client is a thin Notion HTTP client. Construct once, share across goroutines.
type Client struct {
	token string
	http  *http.Client
	base  string
}

// New constructs a client. httpClient may be nil — http.DefaultClient is used.
func New(token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{token: token, http: httpClient, base: apiBase}
}

// PageMeta is the metadata returned for a page (no body).
type PageMeta struct {
	ID         string
	Title      string
	URL        string
	ParentType string
	ParentID   string
	Archived   bool
}

// DatabaseMeta is the metadata returned for a database.
type DatabaseMeta struct {
	ID         string
	Title      string
	URL        string
	ParentType string
	ParentID   string
}

// PageMarkdown is the result of GetPageMarkdown.
type PageMarkdown struct {
	Markdown         string
	ChildPageIDs     []string
	ChildDatabaseIDs []string
}

// SearchHit is a flattened representation of a /v1/search result item.
type SearchHit struct {
	ID         string
	ObjectType types.NotionObjectType
	Title      string
	URL        string
}

// SearchOptions narrows the search.
type SearchOptions struct {
	// ObjectType is "page", "database", or "" (any).
	ObjectType string
	PageSize   int
}

// QueryDatabaseOptions is the body forwarded to /v1/databases/{id}/query.
type QueryDatabaseOptions struct {
	Filter    map[string]any
	Sorts     []map[string]any
	PageSize  int
	StartCursor string
}

// QueryDatabaseResult mirrors the Notion query response shape we care about.
type QueryDatabaseResult struct {
	Pages      []*PageMeta
	HasMore    bool
	NextCursor string
}

// RetrievePage hits GET /v1/pages/{id}.
func (c *Client) RetrievePage(ctx context.Context, pageID string) (*PageMeta, error) {
	id, err := NormalizeID(pageID)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/pages/"+id, notionVersionStable, nil, &raw); err != nil {
		return nil, err
	}
	return parsePageMeta(raw), nil
}

// RetrieveDatabase hits GET /v1/databases/{id}.
func (c *Client) RetrieveDatabase(ctx context.Context, dbID string) (*DatabaseMeta, error) {
	id, err := NormalizeID(dbID)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.do(ctx, http.MethodGet, "/v1/databases/"+id, notionVersionStable, nil, &raw); err != nil {
		return nil, err
	}
	return parseDatabaseMeta(raw), nil
}

// GetPageMarkdown hits GET /v1/pages/{id}/markdown (Notion-Version 2026-03-11).
func (c *Client) GetPageMarkdown(ctx context.Context, pageID string) (*PageMarkdown, error) {
	id, err := NormalizeID(pageID)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Markdown string `json:"markdown"`
	}
	err = c.do(ctx, http.MethodGet, "/v1/pages/"+id+"/markdown", notionVersionMarkdown, nil, &raw)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && (apiErr.Status == http.StatusBadRequest || apiErr.Status == http.StatusNotImplemented) {
			return nil, goerr.Wrap(ErrMarkdownUnsupported, "markdown endpoint declined object",
				goerr.V("status", apiErr.Status), goerr.V("page_id", id))
		}
		return nil, err
	}
	pm := &PageMarkdown{Markdown: raw.Markdown}
	pm.ChildPageIDs, pm.ChildDatabaseIDs = extractChildRefs(raw.Markdown)
	return pm, nil
}

// Search hits POST /v1/search.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) ([]*SearchHit, error) {
	body := map[string]any{
		"query": query,
	}
	if opts.ObjectType == "page" || opts.ObjectType == "database" {
		body["filter"] = map[string]any{
			"property": "object",
			"value":    opts.ObjectType,
		}
	}
	if opts.PageSize > 0 {
		body["page_size"] = opts.PageSize
	}
	var raw struct {
		Results []map[string]any `json:"results"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/search", notionVersionStable, body, &raw); err != nil {
		return nil, err
	}
	hits := make([]*SearchHit, 0, len(raw.Results))
	for _, r := range raw.Results {
		hits = append(hits, parseSearchHit(r))
	}
	return hits, nil
}

// QueryDatabase hits POST /v1/databases/{id}/query.
func (c *Client) QueryDatabase(ctx context.Context, dbID string, opts QueryDatabaseOptions) (*QueryDatabaseResult, error) {
	id, err := NormalizeID(dbID)
	if err != nil {
		return nil, err
	}
	body := map[string]any{}
	if opts.Filter != nil {
		body["filter"] = opts.Filter
	}
	if opts.Sorts != nil {
		body["sorts"] = opts.Sorts
	}
	if opts.PageSize > 0 {
		body["page_size"] = opts.PageSize
	}
	if opts.StartCursor != "" {
		body["start_cursor"] = opts.StartCursor
	}
	var raw struct {
		Results    []map[string]any `json:"results"`
		HasMore    bool             `json:"has_more"`
		NextCursor string           `json:"next_cursor"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/databases/"+id+"/query", notionVersionStable, body, &raw); err != nil {
		return nil, err
	}
	out := &QueryDatabaseResult{HasMore: raw.HasMore, NextCursor: raw.NextCursor}
	for _, r := range raw.Results {
		out.Pages = append(out.Pages, parsePageMeta(r))
	}
	return out, nil
}

type apiError struct {
	Status int
	Body   string
}

func (e *apiError) Error() string {
	return "notion api error: status=" + strconv.Itoa(e.Status) + " body=" + e.Body
}

func (c *Client) do(ctx context.Context, method, path, version string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return goerr.Wrap(err, "failed to marshal notion request body")
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return goerr.Wrap(err, "failed to build notion request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Notion-Version", version)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return goerr.Wrap(err, "notion http call failed",
			goerr.V("method", method), goerr.V("path", path))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		ae := &apiError{Status: resp.StatusCode, Body: string(raw)}
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return goerr.Wrap(ErrUnauthorized, ae.Error(),
				goerr.V("status", resp.StatusCode), goerr.V("path", path))
		case http.StatusForbidden:
			return goerr.Wrap(ErrForbidden, ae.Error(),
				goerr.V("status", resp.StatusCode), goerr.V("path", path))
		case http.StatusNotFound:
			return goerr.Wrap(ErrNotFound, ae.Error(),
				goerr.V("status", resp.StatusCode), goerr.V("path", path))
		}
		return ae
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return goerr.Wrap(err, "failed to decode notion response",
			goerr.V("path", path))
	}
	return nil
}

// parsePageMeta extracts the bits we care about from a Notion page object.
func parsePageMeta(raw map[string]any) *PageMeta {
	pm := &PageMeta{}
	pm.ID = strOr(raw, "id")
	pm.URL = strOr(raw, "url")
	if a, ok := raw["archived"].(bool); ok {
		pm.Archived = a
	}
	if parent, ok := raw["parent"].(map[string]any); ok {
		pt := strOr(parent, "type")
		pm.ParentType = pt
		if pt != "" {
			pm.ParentID = strOr(parent, pt)
		}
	}
	pm.Title = pageTitle(raw)
	return pm
}

func parseDatabaseMeta(raw map[string]any) *DatabaseMeta {
	dm := &DatabaseMeta{}
	dm.ID = strOr(raw, "id")
	dm.URL = strOr(raw, "url")
	if parent, ok := raw["parent"].(map[string]any); ok {
		pt := strOr(parent, "type")
		dm.ParentType = pt
		if pt != "" {
			dm.ParentID = strOr(parent, pt)
		}
	}
	if titleArr, ok := raw["title"].([]any); ok {
		dm.Title = joinRichText(titleArr)
	}
	return dm
}

func parseSearchHit(raw map[string]any) *SearchHit {
	hit := &SearchHit{}
	hit.ID = strOr(raw, "id")
	hit.URL = strOr(raw, "url")
	switch strOr(raw, "object") {
	case "page":
		hit.ObjectType = types.NotionObjectPage
		hit.Title = pageTitle(raw)
	case "database":
		hit.ObjectType = types.NotionObjectDatabase
		if t, ok := raw["title"].([]any); ok {
			hit.Title = joinRichText(t)
		}
	}
	return hit
}

// pageTitle digs through a page's properties to find the title-typed property.
func pageTitle(raw map[string]any) string {
	props, ok := raw["properties"].(map[string]any)
	if !ok {
		return ""
	}
	for _, v := range props {
		p, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if strOr(p, "type") != "title" {
			continue
		}
		if t, ok := p["title"].([]any); ok {
			return joinRichText(t)
		}
	}
	return ""
}

func joinRichText(items []any) string {
	var b strings.Builder
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			b.WriteString(strOr(m, "plain_text"))
		}
	}
	return b.String()
}

func strOr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// childPagePattern picks Notion-style links pointing at a child page in
// rendered Markdown. Child databases use the same scheme.
var childPagePattern = regexp.MustCompile(`(?i)\(\s*([^)\s]*notion\.so[^)\s]*)\s*\)`)

// extractChildRefs scans the rendered markdown for outbound links to other
// notion.so objects and returns their canonical IDs. The caller is responsible
// for distinguishing pages vs databases via subsequent API calls.
func extractChildRefs(md string) (pages, databases []string) {
	if md == "" {
		return nil, nil
	}
	seen := make(map[string]struct{})
	matches := childPagePattern.FindAllStringSubmatch(md, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := m[1]
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		segs := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(segs) == 0 {
			continue
		}
		id, ok := extractIDFromSegment(segs[len(segs)-1])
		if !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if u.Query().Get("v") != "" {
			databases = append(databases, id)
		} else {
			pages = append(pages, id)
		}
	}
	return pages, databases
}
