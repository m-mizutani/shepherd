package prompt_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
)

func newUC(t *testing.T) *prompt.UseCase {
	t.Helper()
	return prompt.New(memory.New().Prompt())
}

func TestUseCase_EffectiveReturnsEmptyWhenNoOverride(t *testing.T) {
	uc := newUC(t)
	got, version, err := uc.Effective(context.Background(), "ws-1", model.PromptIDTriage)
	gt.NoError(t, err)
	gt.Equal(t, version, 0)
	gt.Equal(t, got, "")
}

func TestUseCase_SaveThenEffective(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")
	const newContent = "Always escalate production outages to the on-call engineer."

	v, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, newContent, prompt.Author{Name: "alice"})
	gt.NoError(t, err)
	gt.Equal(t, v.Version, 1)

	got, version, err := uc.Effective(ctx, ws, model.PromptIDTriage)
	gt.NoError(t, err)
	gt.Equal(t, version, 1)
	gt.Equal(t, got, newContent)
}

func TestUseCase_SaveAcceptsAnyText(t *testing.T) {
	// User content is no longer parsed as a Go template, so previously
	// invalid syntax like an unclosed action or an unknown field reference
	// is now stored verbatim.
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	for i, content := range []string{
		"unclosed {{ action",
		"hi {{ .NonExistent }}",
		"{{ range .Title }}x{{ end }}",
		"# Workspace policy\n\nLeading H1 is fine.",
		"",
	} {
		_, err := uc.Save(ctx, ws, model.PromptIDTriage, i+1, content, prompt.Author{Name: "alice"})
		gt.NoError(t, err)
	}
}

func TestUseCase_SaveRejectsStaleVersion(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, "first guidance", prompt.Author{Name: "alice"})
	gt.NoError(t, err)
	_, err = uc.Save(ctx, ws, model.PromptIDTriage, 2, "second guidance", prompt.Author{Name: "alice"})
	gt.NoError(t, err)

	// Stale writer thinks the next version is still 2.
	_, err = uc.Save(ctx, ws, model.PromptIDTriage, 2, "stale guidance", prompt.Author{Name: "bob"})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, interfaces.ErrPromptVersionConflict))
}

func TestUseCase_HistoryAscending(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	for i, c := range []string{"a guidance", "b guidance", "c guidance"} {
		_, err := uc.Save(ctx, ws, model.PromptIDTriage, i+1, c, prompt.Author{Name: "alice"})
		gt.NoError(t, err)
	}

	hist, err := uc.History(ctx, ws, model.PromptIDTriage)
	gt.NoError(t, err)
	gt.Equal(t, len(hist), 3)
	gt.Equal(t, hist[0].Version, 1)
	gt.S(t, hist[0].Content).Contains("a ")
	gt.Equal(t, hist[2].Version, 3)
	gt.S(t, hist[2].Content).Contains("c ")
}

func TestUseCase_RestoreCopiesContentIntoNewVersion(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, "v1 guidance", prompt.Author{Name: "alice"})
	gt.NoError(t, err)
	_, err = uc.Save(ctx, ws, model.PromptIDTriage, 2, "v2 guidance", prompt.Author{Name: "alice"})
	gt.NoError(t, err)

	// Restore v1 — caller asks to write v3 (the next available version).
	v3, err := uc.Restore(ctx, ws, model.PromptIDTriage, 3, 1, prompt.Author{Name: "bob"})
	gt.NoError(t, err)
	gt.Equal(t, v3.Version, 3)
	gt.S(t, v3.Content).Contains("v1 ")

	got, version, err := uc.Effective(ctx, ws, model.PromptIDTriage)
	gt.NoError(t, err)
	gt.Equal(t, version, 3)
	gt.S(t, got).Contains("v1 ")
}

func TestUseCase_RestoreConflict(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, "v1 guidance", prompt.Author{Name: "alice"})
	gt.NoError(t, err)

	// Stale: caller thinks the next version is still 1, but current is already 1.
	_, err = uc.Restore(ctx, ws, model.PromptIDTriage, 1, 1, prompt.Author{Name: "bob"})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, interfaces.ErrPromptVersionConflict))
}

func TestUseCase_RestoreNotFound(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, "v1 guidance", prompt.Author{Name: "alice"})
	gt.NoError(t, err)

	_, err = uc.Restore(ctx, ws, model.PromptIDTriage, 2, 99, prompt.Author{Name: "bob"})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, interfaces.ErrPromptVersionNotFound))
}

func TestUseCase_RenderTriagePlanRendersBaseWithoutOverride(t *testing.T) {
	uc := newUC(t)
	got, err := uc.RenderTriagePlan(context.Background(), "ws-1", prompt.TriagePlanInput{
		Title: "Login fails", Description: "blank page", InitialMessage: "see thread", Reporter: "U1",
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("Login fails")
	gt.S(t, got).Contains("blank page")
	gt.S(t, got).Contains("U1")
	if strings.Contains(got, "\n---\n") {
		t.Errorf("expected no UserGuidance separator when no override exists, got:\n%s", got)
	}
}

func TestUseCase_RenderTriagePlanEmbedsOverride(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	const guidance = "Always escalate production outages to the on-call engineer."
	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, guidance, prompt.Author{Name: "alice"})
	gt.NoError(t, err)

	got, err := uc.RenderTriagePlan(ctx, ws, prompt.TriagePlanInput{Title: "the title"})
	gt.NoError(t, err)
	gt.S(t, got).Contains("the title")
	gt.S(t, got).Contains(guidance)

	idxSep := strings.Index(got, "\n---\n")
	idxGuidance := strings.Index(got, guidance)
	if idxSep < 0 || idxSep > idxGuidance {
		t.Errorf("expected separator before guidance, got:\n%s", got)
	}
}

func TestUseCase_RenderTriagePlanContinuesOnRepoFailure(t *testing.T) {
	// A flapping repository must not take triage down — RenderTriagePlan
	// should log the failure and render the bare base prompt.
	uc := prompt.New(&boomPromptRepo{})

	got, err := uc.RenderTriagePlan(context.Background(), "ws-1", prompt.TriagePlanInput{Title: "boom test"})
	gt.NoError(t, err)
	gt.S(t, got).Contains("boom test")
	if strings.Contains(got, "\n---\n") {
		t.Errorf("expected bare base prompt on repo failure, got:\n%s", got)
	}
}

// boomPromptRepo is a PromptRepository that always errors on reads. It only
// implements GetCurrent (the path RenderTriagePlan exercises); the other
// methods would never be hit by this test.
type boomPromptRepo struct{}

func (b *boomPromptRepo) Append(ctx context.Context, ws types.WorkspaceID, id model.PromptID, draft *model.PromptVersion) (*model.PromptVersion, error) {
	return nil, errBoom
}
func (b *boomPromptRepo) GetCurrent(ctx context.Context, ws types.WorkspaceID, id model.PromptID) (*model.PromptVersion, error) {
	return nil, errBoom
}
func (b *boomPromptRepo) GetVersion(ctx context.Context, ws types.WorkspaceID, id model.PromptID, version int) (*model.PromptVersion, error) {
	return nil, errBoom
}
func (b *boomPromptRepo) List(ctx context.Context, ws types.WorkspaceID, id model.PromptID) ([]*model.PromptVersion, error) {
	return nil, errBoom
}

var errBoom = errors.New("boom")
