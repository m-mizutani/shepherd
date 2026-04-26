package notion_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/service/notion"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*notion.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cli := notion.NewWithBaseForTest(srv.URL, "secret_test", srv.Client())
	return cli, srv
}

func TestRetrievePage_Success(t *testing.T) {
	cli, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gt.Equal(t, r.Method, http.MethodGet)
		gt.Equal(t, r.URL.Path, "/v1/pages/abcdef0123456789abcdef0123456789")
		gt.Equal(t, r.Header.Get("Notion-Version"), "2022-06-28")
		gt.Equal(t, r.Header.Get("Authorization"), "Bearer secret_test")
		_, _ = w.Write([]byte(`{
			"id":"abcdef01-2345-6789-abcd-ef0123456789",
			"url":"https://www.notion.so/x/abcdef0123456789abcdef0123456789",
			"properties":{"Name":{"type":"title","title":[{"plain_text":"Hello"}]}}
		}`))
	})
	pm, err := cli.RetrievePage(context.Background(), "abcdef0123456789abcdef0123456789")
	gt.NoError(t, err)
	gt.Equal(t, pm.Title, "Hello")
}

func TestRetrievePage_403(t *testing.T) {
	cli, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"object":"error","status":403}`))
	})
	_, err := cli.RetrievePage(context.Background(), "abcdef0123456789abcdef0123456789")
	gt.Error(t, err)
	gt.True(t, errors.Is(err, notion.ErrForbidden))
}

func TestRetrievePage_404(t *testing.T) {
	cli, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := cli.RetrievePage(context.Background(), "abcdef0123456789abcdef0123456789")
	gt.True(t, errors.Is(err, notion.ErrNotFound))
}

func TestGetPageMarkdown_Success(t *testing.T) {
	cli, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gt.Equal(t, r.Header.Get("Notion-Version"), "2026-03-11")
		gt.True(t, strings.HasSuffix(r.URL.Path, "/markdown"))
		_, _ = w.Write([]byte(`{"markdown":"# title\n\nbody [child](https://www.notion.so/x/cafe0123456789abcdefcafe01234567)"}`))
	})
	pm, err := cli.GetPageMarkdown(context.Background(), "abcdef0123456789abcdef0123456789")
	gt.NoError(t, err)
	gt.True(t, strings.Contains(pm.Markdown, "# title"))
	gt.Equal(t, len(pm.ChildPageIDs), 1)
	gt.Equal(t, pm.ChildPageIDs[0], "cafe0123456789abcdefcafe01234567")
}

func TestGetPageMarkdown_Unsupported(t *testing.T) {
	cli, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	_, err := cli.GetPageMarkdown(context.Background(), "abcdef0123456789abcdef0123456789")
	gt.True(t, errors.Is(err, notion.ErrMarkdownUnsupported))
}

func TestSearch(t *testing.T) {
	cli, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gt.Equal(t, r.URL.Path, "/v1/search")
		gt.Equal(t, r.Method, http.MethodPost)
		_, _ = w.Write([]byte(`{
			"results":[
				{"object":"page","id":"abcdef0123456789abcdef0123456789","url":"https://www.notion.so/x","properties":{"N":{"type":"title","title":[{"plain_text":"P"}]}}},
				{"object":"database","id":"deadbeefdeadbeefdeadbeefdeadbeef","url":"https://www.notion.so/y","title":[{"plain_text":"DB"}]}
			]
		}`))
	})
	hits, err := cli.Search(context.Background(), "hello", notion.SearchOptions{})
	gt.NoError(t, err)
	gt.Equal(t, len(hits), 2)
	gt.Equal(t, hits[0].ObjectType, types.NotionObjectPage)
	gt.Equal(t, hits[0].Title, "P")
	gt.Equal(t, hits[1].ObjectType, types.NotionObjectDatabase)
	gt.Equal(t, hits[1].Title, "DB")
}

func TestRetrieveDatabase_Unauthorized(t *testing.T) {
	cli, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := cli.RetrieveDatabase(context.Background(), "abcdef0123456789abcdef0123456789")
	gt.True(t, errors.Is(err, notion.ErrUnauthorized))
}
