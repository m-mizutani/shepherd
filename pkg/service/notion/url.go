// Package notion contains a thin Notion HTTP client and URL parser used to
// resolve user-supplied Notion URLs into canonical object IDs and to read
// pages/databases via the official Markdown Content API.
package notion

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// hex32Pattern matches a 32-character lowercase hex string (no hyphens).
var hex32Pattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// uuidPattern matches a hyphenated UUID (8-4-4-4-12).
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// trailingIDPattern picks the trailing 32-hex chunk out of a "{slug}-{id}" segment.
var trailingIDPattern = regexp.MustCompile(`([0-9a-fA-F]{32})$`)

// ParseURL turns a Notion URL or raw object ID into (objectType, normalizedID).
// Database URLs are detected by the presence of a `?v=` view parameter; URLs
// without it default to a page. Raw IDs default to page (callers performing
// access verification should override based on what the API returns).
func ParseURL(raw string) (types.NotionObjectType, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", goerr.New("notion url/id is empty")
	}

	// Bare ID (with or without hyphens).
	if id, ok := normalizeBareID(raw); ok {
		return types.NotionObjectPage, id, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", "", goerr.Wrap(err, "invalid notion url", goerr.V("input", raw))
	}
	if u.Host != "" && !strings.HasSuffix(strings.ToLower(u.Host), "notion.so") {
		return "", "", goerr.New("not a notion.so URL", goerr.V("host", u.Host))
	}

	// Pull the last path segment, then extract the trailing 32-hex chunk.
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segs) == 0 {
		return "", "", goerr.New("notion url has no path", goerr.V("input", raw))
	}
	last := segs[len(segs)-1]

	id, ok := extractIDFromSegment(last)
	if !ok {
		return "", "", goerr.New("could not extract notion object id from URL", goerr.V("input", raw))
	}

	objType := types.NotionObjectPage
	if u.Query().Get("v") != "" {
		objType = types.NotionObjectDatabase
	}
	return objType, id, nil
}

// NormalizeID returns the 32-hex (no hyphen) form of any accepted Notion id.
func NormalizeID(s string) (string, error) {
	if id, ok := normalizeBareID(s); ok {
		return id, nil
	}
	return "", goerr.New("not a notion object id", goerr.V("input", s))
}

func normalizeBareID(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if hex32Pattern.MatchString(strings.ToLower(s)) {
		return strings.ToLower(s), true
	}
	if uuidPattern.MatchString(s) {
		return strings.ToLower(strings.ReplaceAll(s, "-", "")), true
	}
	return "", false
}

func extractIDFromSegment(seg string) (string, bool) {
	if id, ok := normalizeBareID(seg); ok {
		return id, true
	}
	m := trailingIDPattern.FindString(seg)
	if m == "" {
		return "", false
	}
	return strings.ToLower(m), true
}
