package config_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/cli/config"
	"github.com/urfave/cli/v3"
)

// clearLLMEnv neutralizes any SHEPHERD_LLM_* values that the test runner
// (zenv, .env.*) may have injected. Without this, tests that expect a flag to
// be absent see env-supplied defaults and produce confusing failures.
func clearLLMEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"SHEPHERD_LLM_PROVIDER",
		"SHEPHERD_LLM_MODEL",
		"SHEPHERD_LLM_OPENAI_API_KEY",
		"SHEPHERD_LLM_CLAUDE_API_KEY",
		"SHEPHERD_LLM_GEMINI_PROJECT_ID",
		"SHEPHERD_LLM_GEMINI_LOCATION",
	} {
		t.Setenv(k, "")
	}
}

func runWithLLMArgs(t *testing.T, args []string) (*config.LLM, error) {
	t.Helper()
	clearLLMEnv(t)
	llm := &config.LLM{}
	cmd := &cli.Command{
		Name:  "test",
		Flags: llm.Flags(),
		Action: func(_ context.Context, _ *cli.Command) error {
			return nil
		},
	}
	err := cmd.Run(context.Background(), append([]string{"test"}, args...))
	return llm, err
}

func TestLLM_Disabled(t *testing.T) {
	llm, err := runWithLLMArgs(t, nil)
	gt.NoError(t, err)
	gt.False(t, llm.IsEnabled())
	client, err := llm.NewClient(context.Background())
	gt.NoError(t, err)
	gt.Nil(t, client)
}

func TestLLM_OpenAIRequiresAPIKey(t *testing.T) {
	llm, err := runWithLLMArgs(t, []string{"--llm-provider", "openai"})
	gt.NoError(t, err)
	gt.True(t, llm.IsEnabled())
	_, err = llm.NewClient(context.Background())
	gt.Error(t, err)
}

func TestLLM_ClaudeRequiresKeyOrGCP(t *testing.T) {
	llm, err := runWithLLMArgs(t, []string{"--llm-provider", "claude"})
	gt.NoError(t, err)
	_, err = llm.NewClient(context.Background())
	gt.Error(t, err)
}

func TestLLM_ClaudeRejectsBothSources(t *testing.T) {
	llm, err := runWithLLMArgs(t, []string{
		"--llm-provider", "claude",
		"--llm-claude-api-key", "ak-test",
		"--llm-gemini-project-id", "proj",
		"--llm-gemini-location", "us-central1",
	})
	gt.NoError(t, err)
	_, err = llm.NewClient(context.Background())
	gt.Error(t, err)
}

func TestLLM_GeminiRequiresProjectAndLocation(t *testing.T) {
	llm, err := runWithLLMArgs(t, []string{
		"--llm-provider", "gemini",
		"--llm-gemini-project-id", "proj",
	})
	gt.NoError(t, err)
	_, err = llm.NewClient(context.Background())
	gt.Error(t, err)
}

func TestLLM_UnknownProvider(t *testing.T) {
	llm, err := runWithLLMArgs(t, []string{"--llm-provider", "bogus"})
	gt.NoError(t, err)
	_, err = llm.NewClient(context.Background())
	gt.Error(t, err)
}
