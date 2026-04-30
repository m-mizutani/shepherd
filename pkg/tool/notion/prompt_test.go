package notion_test

import (
	"strings"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	tnotion "github.com/m-mizutani/shepherd/pkg/tool/notion"
)

func TestRenderPromptFromSources_NoSources(t *testing.T) {
	got, err := tnotion.RenderPromptFromSourcesForTest(nil)
	gt.NoError(t, err)
	gt.S(t, got).Contains("No Notion sources are currently registered")
	gt.S(t, got).NotContains("Registered Notion sources")
}

func TestRenderPromptFromSources_WithSources(t *testing.T) {
	srcs := []*model.Source{
		{
			ID:          types.SourceID("src-engineering"),
			Provider:    types.SourceProviderNotion,
			Description: "Engineering team's main wiki",
			Notion: &model.NotionSource{
				ObjectType: types.NotionObjectType("database"),
				Title:      "Engineering Wiki",
			},
		},
		{
			ID:       types.SourceID("src-postmortem"),
			Provider: types.SourceProviderNotion,
			// Description intentionally empty to verify the conditional rendering.
			Notion: &model.NotionSource{
				ObjectType: types.NotionObjectType("page"),
				Title:      "Q3 Postmortems",
			},
		},
	}
	got, err := tnotion.RenderPromptFromSourcesForTest(srcs)
	gt.NoError(t, err)
	gt.S(t, got).Contains("Registered Notion sources")
	gt.S(t, got).Contains("`src-engineering`")
	gt.S(t, got).Contains("Engineering Wiki")
	gt.S(t, got).Contains("(database)")
	gt.S(t, got).Contains("Engineering team's main wiki")
	gt.S(t, got).Contains("`src-postmortem`")
	gt.S(t, got).Contains("Q3 Postmortems")
	gt.S(t, got).Contains("(page)")
	// Confirm the empty Description does not produce an "—" tail with no text.
	if strings.Contains(got, "Q3 Postmortems (page) — \n") {
		t.Fatalf("expected no trailing em-dash for source without description, got: %q", got)
	}
	gt.S(t, got).NotContains("No Notion sources are currently registered")
}

func TestRenderPromptFromSources_SkipsSourcesWithoutNotionPayload(t *testing.T) {
	srcs := []*model.Source{
		{
			ID:       types.SourceID("src-broken"),
			Provider: types.SourceProviderNotion,
			// Notion field intentionally nil — should be skipped silently.
		},
	}
	got, err := tnotion.RenderPromptFromSourcesForTest(srcs)
	gt.NoError(t, err)
	// With every source skipped the output collapses to the "no sources" branch.
	gt.S(t, got).Contains("No Notion sources are currently registered")
}
