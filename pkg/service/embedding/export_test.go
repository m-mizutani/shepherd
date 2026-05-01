package embedding

import "context"

// FakeClientForTest is a stub embeddingClient used by tests in this package.
type FakeClientForTest struct {
	GenerateFn func(ctx context.Context, dimension int, input []string) ([][]float64, error)
}

func (f *FakeClientForTest) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	return f.GenerateFn(ctx, dimension, input)
}

// NewWithClientForTest builds a Service that bypasses the real gemini.New
// constructor (and the self-test against the live API). Tests substitute a
// fake embeddingClient here.
func NewWithClientForTest(client embeddingClient, modelID string, dim int) *Service {
	return &Service{client: client, modelID: modelID, dim: dim}
}
