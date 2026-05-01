package embedding_test

import (
	"context"
	"errors"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/service/embedding"
)

func TestGenerate_Success(t *testing.T) {
	const dim = 4
	want := []float64{0.1, 0.2, 0.3, 0.4}
	fake := &embedding.FakeClientForTest{
		GenerateFn: func(ctx context.Context, dimension int, input []string) ([][]float64, error) {
			gt.Equal(t, dim, dimension)
			gt.A(t, input).Length(1).At(0, func(t testing.TB, v string) {
				gt.Equal(t, "hello", v)
			})
			return [][]float64{want}, nil
		},
	}
	svc := embedding.NewWithClientForTest(fake, "gemini:test-model", dim)

	vec, id, err := svc.Generate(context.Background(), "hello")
	gt.NoError(t, err)
	gt.Equal(t, "gemini:test-model", id)
	gt.A(t, vec).Length(dim)
	for i := range want {
		gt.Equal(t, float32(want[i]), vec[i])
	}
}

func TestGenerate_EmptyInput(t *testing.T) {
	fake := &embedding.FakeClientForTest{
		GenerateFn: func(ctx context.Context, dimension int, input []string) ([][]float64, error) {
			t.Fatalf("client must not be called for empty input")
			return nil, nil
		},
	}
	svc := embedding.NewWithClientForTest(fake, "gemini:test-model", 4)
	_, _, err := svc.Generate(context.Background(), "")
	gt.Error(t, err)
}

func TestGenerate_DimensionMismatch(t *testing.T) {
	fake := &embedding.FakeClientForTest{
		GenerateFn: func(ctx context.Context, dimension int, input []string) ([][]float64, error) {
			return [][]float64{{0.1, 0.2}}, nil
		},
	}
	svc := embedding.NewWithClientForTest(fake, "gemini:test-model", 4)
	_, _, err := svc.Generate(context.Background(), "hello")
	gt.Error(t, err)
}

func TestGenerate_APIError(t *testing.T) {
	apiErr := errors.New("upstream boom")
	fake := &embedding.FakeClientForTest{
		GenerateFn: func(ctx context.Context, dimension int, input []string) ([][]float64, error) {
			return nil, apiErr
		},
	}
	svc := embedding.NewWithClientForTest(fake, "gemini:test-model", 4)
	_, _, err := svc.Generate(context.Background(), "hello")
	gt.Error(t, err)
}

// TestGenerate_LiveAPI exercises the real Vertex AI Gemini embedding endpoint.
// Skipped unless TEST_EMBEDDING_GEMINI_PROJECT is set, mirroring the
// TEST_FIRESTORE_PROJECT_ID convention used by repository tests.
//
// Optional knobs:
//   - TEST_EMBEDDING_GEMINI_LOCATION (default: "global")
//   - TEST_EMBEDDING_GEMINI_MODEL    (default: "gemini-embedding-2")
//   - TEST_EMBEDDING_DIM             (default: 768)
func TestGenerate_LiveAPI(t *testing.T) {
	projectID := os.Getenv("TEST_EMBEDDING_GEMINI_PROJECT")
	if projectID == "" {
		t.Skip("TEST_EMBEDDING_GEMINI_PROJECT not set")
	}
	location := os.Getenv("TEST_EMBEDDING_GEMINI_LOCATION")
	if location == "" {
		location = "global"
	}
	model := os.Getenv("TEST_EMBEDDING_GEMINI_MODEL")
	if model == "" {
		model = "gemini-embedding-2"
	}
	dim := 768
	if s := os.Getenv("TEST_EMBEDDING_DIM"); s != "" {
		parsed, err := strconv.Atoi(s)
		gt.NoError(t, err)
		dim = parsed
	}

	ctx := context.Background()
	svc, err := embedding.New(ctx, projectID, location, model, dim)
	gt.NoError(t, err)
	gt.NotNil(t, svc)

	vec, modelID, err := svc.Generate(ctx, "hello world")
	gt.NoError(t, err)
	gt.Equal(t, "gemini:"+model, modelID)
	gt.A(t, vec).Length(dim)

	// Stability: same input twice should be near-identical (cosine ≈ 1).
	vec2, _, err := svc.Generate(ctx, "hello world")
	gt.NoError(t, err)
	if sim := cosine(vec, vec2); sim < 0.999 {
		t.Fatalf("expected near-identical vectors for the same input, got cosine=%v", sim)
	}

	// Semantic discrimination: similar pair > unrelated pair.
	catA, _, err := svc.Generate(ctx, "a cat sleeps on the sofa")
	gt.NoError(t, err)
	catB, _, err := svc.Generate(ctx, "the cat is napping on the couch")
	gt.NoError(t, err)
	dbErr, _, err := svc.Generate(ctx, "a database query timed out due to lock contention")
	gt.NoError(t, err)

	simSim := cosine(catA, catB)
	simUnrelated := cosine(catA, dbErr)
	if !(simSim > simUnrelated+0.05) {
		t.Fatalf("expected similar-pair cosine (%v) to clearly exceed unrelated-pair cosine (%v)", simSim, simUnrelated)
	}
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
