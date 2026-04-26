package notion_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	tnotion "github.com/m-mizutani/shepherd/pkg/tool/notion"
)

func TestFactory_AvailableFollowsToken(t *testing.T) {
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	t.Run("no token leaves factory inert", func(t *testing.T) {
		f := tnotion.New(repo.Source(), http.DefaultClient)
		gt.NoError(t, f.Init(context.Background()))
		gt.False(t, f.Available())
		gt.Equal(t, len(f.Tools()), 0)
		gt.Nil(t, f.Client())
	})

	t.Run("token enables tools", func(t *testing.T) {
		f := tnotion.New(repo.Source(), http.DefaultClient)
		// Bind the token field that --notion-token would normally fill.
		tnotion.SetTokenForTest(f, "secret_xx")
		gt.NoError(t, f.Init(context.Background()))
		gt.True(t, f.Available())
		gt.Equal(t, len(f.Tools()), 4) // search / get_page / query_database / list_sources
		gt.NotNil(t, f.Client())
	})
}
