package usecase_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

type panickingLLM struct{}

func (panickingLLM) NewSession(ctx context.Context, opts ...gollem.SessionOption) (gollem.Session, error) {
	panic("LLM should not be invoked for this test")
}

func (panickingLLM) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	panic("embedding should not be invoked for this test")
}

func TestHandleAppMention_NoLLM(t *testing.T) {
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })
	registry := model.NewWorkspaceRegistry()

	uc := usecase.NewSlackUseCase(repo, registry, nil, "https://example.com", nil)

	err := uc.HandleAppMention(context.Background(), "C123", "U123", "<@UBOT> hello", "1.0", "1.0")
	gt.NoError(t, err)
}

func TestHandleAppMention_UnknownChannel(t *testing.T) {
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })
	registry := model.NewWorkspaceRegistry()

	// llm is non-nil but should not be invoked because the channel is unmapped.
	// We pass a sentinel that would panic if called; never reached here.
	uc := usecase.NewSlackUseCase(repo, registry, nil, "https://example.com", panickingLLM{})

	err := uc.HandleAppMention(context.Background(), "C-not-mapped", "U123", "hi", "1.0", "1.0")
	gt.NoError(t, err)
}
