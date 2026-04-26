package notion

import "net/http"

// NewWithBaseForTest constructs a client whose base URL is overridden — used
// by package-external tests to point at httptest servers without exposing the
// production constructor's internals.
func NewWithBaseForTest(base, token string, httpClient *http.Client) *Client {
	c := New(token, httpClient)
	c.base = base
	return c
}
