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

func TestUseCase_EffectiveReturnsDefaultWhenNoOverride(t *testing.T) {
	uc := newUC(t)
	got, version, err := uc.Effective(context.Background(), "ws-1", model.PromptIDTriage)
	gt.NoError(t, err)
	gt.Equal(t, version, 0)
	if !strings.Contains(got, "{{ .Title }}") {
		t.Fatalf("default content should still contain the {{ .Title }} action; got:\n%s", got)
	}
}

func TestUseCase_SaveThenEffective(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")
	const newContent = "Custom triage prompt for {{ .Title }}"

	v, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, newContent, prompt.Author{Name: "alice"})
	gt.NoError(t, err)
	gt.Equal(t, v.Version, 1)

	got, version, err := uc.Effective(ctx, ws, model.PromptIDTriage)
	gt.NoError(t, err)
	gt.Equal(t, version, 1)
	gt.Equal(t, got, newContent)
}

func TestUseCase_SaveRejectsParseError(t *testing.T) {
	uc := newUC(t)
	_, err := uc.Save(context.Background(), "ws-1", model.PromptIDTriage, 1,
		"broken {{ .Title", prompt.Author{Name: "alice"})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, prompt.ErrInvalidTemplate))
	gt.S(t, prompt.InvalidTemplateReason(err)).Contains("unclosed action")
}

func TestUseCase_SaveRejectsMissingField(t *testing.T) {
	uc := newUC(t)
	// Parse passes, but Execute should fail under missingkey=error because
	// .NonExistent is not a TriagePlanInput field.
	_, err := uc.Save(context.Background(), "ws-1", model.PromptIDTriage, 1,
		"hi {{ .NonExistent }}", prompt.Author{Name: "alice"})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, prompt.ErrInvalidTemplate))
	gt.S(t, prompt.InvalidTemplateReason(err)).Contains("NonExistent")
}

func TestUseCase_SaveRejectsBadRangeTarget(t *testing.T) {
	uc := newUC(t)
	// {{ range .Title }} on a string fails Execute.
	_, err := uc.Save(context.Background(), "ws-1", model.PromptIDTriage, 1,
		"{{ range .Title }}x{{ end }}", prompt.Author{Name: "alice"})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, prompt.ErrInvalidTemplate))
}

func TestUseCase_SaveRejectsStaleVersion(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, "first {{ .Title }}", prompt.Author{Name: "alice"})
	gt.NoError(t, err)
	_, err = uc.Save(ctx, ws, model.PromptIDTriage, 2, "second {{ .Title }}", prompt.Author{Name: "alice"})
	gt.NoError(t, err)

	// Stale writer thinks the next version is still 2.
	_, err = uc.Save(ctx, ws, model.PromptIDTriage, 2, "stale {{ .Title }}", prompt.Author{Name: "bob"})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, interfaces.ErrPromptVersionConflict))
}

func TestUseCase_SaveDoesNotPersistInvalidContent(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, "broken {{", prompt.Author{Name: "alice"})
	gt.Error(t, err)

	_, version, err := uc.Effective(ctx, ws, model.PromptIDTriage)
	gt.NoError(t, err)
	gt.Equal(t, version, 0) // still the default — invalid content was not stored
}

func TestUseCase_HistoryAscending(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	for i, c := range []string{"a {{ .Title }}", "b {{ .Title }}", "c {{ .Title }}"} {
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

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, "v1 {{ .Title }}", prompt.Author{Name: "alice"})
	gt.NoError(t, err)
	_, err = uc.Save(ctx, ws, model.PromptIDTriage, 2, "v2 {{ .Title }}", prompt.Author{Name: "alice"})
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

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, "v1 {{ .Title }}", prompt.Author{Name: "alice"})
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

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1, "v1 {{ .Title }}", prompt.Author{Name: "alice"})
	gt.NoError(t, err)

	_, err = uc.Restore(ctx, ws, model.PromptIDTriage, 2, 99, prompt.Author{Name: "bob"})
	gt.Error(t, err)
	gt.True(t, errors.Is(err, interfaces.ErrPromptVersionNotFound))
}

func TestUseCase_RenderTriagePlanUsesDefault(t *testing.T) {
	uc := newUC(t)
	got, err := uc.RenderTriagePlan(context.Background(), "ws-1", prompt.TriagePlanInput{
		Title: "Login fails", Description: "blank page", InitialMessage: "see thread", Reporter: "U1",
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("Login fails")
	gt.S(t, got).Contains("blank page")
	gt.S(t, got).Contains("U1")
}

func TestUseCase_RenderTriagePlanUsesOverride(t *testing.T) {
	uc := newUC(t)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	_, err := uc.Save(ctx, ws, model.PromptIDTriage, 1,
		"OVERRIDE {{ .Title }}", prompt.Author{Name: "alice"})
	gt.NoError(t, err)

	got, err := uc.RenderTriagePlan(ctx, ws, prompt.TriagePlanInput{Title: "the title"})
	gt.NoError(t, err)
	gt.Equal(t, got, "OVERRIDE the title")
}

func TestUseCase_RenderTriagePlanFallsBackOnRepoFailure(t *testing.T) {
	// A flapping repository must not take triage down — RenderTriagePlan
	// should log the failure and fall back to the embedded default.
	uc := prompt.New(&boomPromptRepo{})

	got, err := uc.RenderTriagePlan(context.Background(), "ws-1", prompt.TriagePlanInput{Title: "boom test"})
	gt.NoError(t, err)
	gt.S(t, got).Contains("boom test")
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

func TestUseCase_RenderTriagePlanFallsBackOnBrokenOverride(t *testing.T) {
	// Inject a broken override directly through the repo so we bypass Save's
	// validation — this simulates an override that worked at save time but
	// regressed (e.g. slotDefs probe inputs evolved).
	repo := memory.New().Prompt()
	uc := prompt.New(repo)
	ctx := context.Background()
	ws := types.WorkspaceID("ws-1")

	_, err := repo.Append(ctx, ws, model.PromptIDTriage, &model.PromptVersion{
		Version: 1,
		Content: "{{ .Bogus }}",
	})
	gt.NoError(t, err)

	// RenderTriagePlan should swallow the error and fall back to the default.
	got, err := uc.RenderTriagePlan(ctx, ws, prompt.TriagePlanInput{Title: "fallback test"})
	gt.NoError(t, err)
	gt.S(t, got).Contains("fallback test")
}
