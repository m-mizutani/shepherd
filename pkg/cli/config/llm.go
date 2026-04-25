package config

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/urfave/cli/v3"
)

// LLM holds CLI configuration for LLM clients backed by gollem.
type LLM struct {
	provider         string
	model            string
	openaiAPIKey     string
	claudeAPIKey     string
	geminiProjectID  string
	geminiLocation   string
}

func (x *LLM) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "llm-provider",
			Usage:       "LLM provider: openai, claude, or gemini (empty disables LLM features)",
			Sources:     cli.EnvVars("SHEPHERD_LLM_PROVIDER"),
			Destination: &x.provider,
		},
		&cli.StringFlag{
			Name:        "llm-model",
			Usage:       "LLM model name (provider default if empty)",
			Sources:     cli.EnvVars("SHEPHERD_LLM_MODEL"),
			Destination: &x.model,
		},
		&cli.StringFlag{
			Name:        "llm-openai-api-key",
			Usage:       "OpenAI API key (required when --llm-provider=openai)",
			Sources:     cli.EnvVars("SHEPHERD_LLM_OPENAI_API_KEY"),
			Destination: &x.openaiAPIKey,
		},
		&cli.StringFlag{
			Name:        "llm-claude-api-key",
			Usage:       "Anthropic Claude API key (used when --llm-provider=claude with direct Anthropic access)",
			Sources:     cli.EnvVars("SHEPHERD_LLM_CLAUDE_API_KEY"),
			Destination: &x.claudeAPIKey,
		},
		&cli.StringFlag{
			Name:        "llm-gemini-project-id",
			Usage:       "Google Cloud project ID (Gemini, or Claude via Gemini Enterprise Agent Platform)",
			Sources:     cli.EnvVars("SHEPHERD_LLM_GEMINI_PROJECT_ID"),
			Destination: &x.geminiProjectID,
		},
		&cli.StringFlag{
			Name:        "llm-gemini-location",
			Usage:       "Google Cloud location for Gemini / Claude on Google Cloud (e.g. us-central1)",
			Sources:     cli.EnvVars("SHEPHERD_LLM_GEMINI_LOCATION"),
			Destination: &x.geminiLocation,
		},
	}
}

// IsEnabled reports whether an LLM provider has been configured.
func (x *LLM) IsEnabled() bool { return x.provider != "" }

// NewClient builds a gollem.LLMClient for the configured provider. Returns nil
// when the LLM feature is disabled.
func (x *LLM) NewClient(ctx context.Context) (gollem.LLMClient, error) {
	if !x.IsEnabled() {
		return nil, nil
	}

	switch x.provider {
	case "openai":
		if x.openaiAPIKey == "" {
			return nil, goerr.New("--llm-openai-api-key is required when --llm-provider=openai")
		}
		var opts []openai.Option
		if x.model != "" {
			opts = append(opts, openai.WithModel(x.model))
		}
		client, err := openai.New(ctx, x.openaiAPIKey, opts...)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to create OpenAI client")
		}
		return client, nil

	case "claude":
		hasAPIKey := x.claudeAPIKey != ""
		hasGCP := x.geminiProjectID != "" || x.geminiLocation != ""
		if hasAPIKey && hasGCP {
			return nil, goerr.New("--llm-claude-api-key and --llm-gemini-project-id are mutually exclusive when --llm-provider=claude")
		}
		switch {
		case hasAPIKey:
			var opts []claude.Option
			if x.model != "" {
				opts = append(opts, claude.WithModel(x.model))
			}
			client, err := claude.New(ctx, x.claudeAPIKey, opts...)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to create Claude client")
			}
			return client, nil
		case x.geminiProjectID != "" && x.geminiLocation != "":
			var opts []claude.VertexOption
			if x.model != "" {
				opts = append(opts, claude.WithVertexModel(x.model))
			}
			client, err := claude.NewWithVertex(ctx, x.geminiLocation, x.geminiProjectID, opts...)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to create Claude (Google Cloud) client")
			}
			return client, nil
		default:
			return nil, goerr.New("--llm-provider=claude requires either --llm-claude-api-key or both --llm-gemini-project-id and --llm-gemini-location")
		}

	case "gemini":
		if x.geminiProjectID == "" || x.geminiLocation == "" {
			return nil, goerr.New("--llm-provider=gemini requires both --llm-gemini-project-id and --llm-gemini-location")
		}
		var opts []gemini.Option
		if x.model != "" {
			opts = append(opts, gemini.WithModel(x.model))
		}
		client, err := gemini.New(ctx, x.geminiProjectID, x.geminiLocation, opts...)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to create Gemini client")
		}
		return client, nil

	default:
		return nil, goerr.New("unsupported --llm-provider value", goerr.V("provider", x.provider))
	}
}
