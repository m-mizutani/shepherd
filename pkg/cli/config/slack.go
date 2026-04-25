package config

import (
	"context"
	"log/slog"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/trace"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	"github.com/urfave/cli/v3"
)

type Slack struct {
	clientID     string
	clientSecret string
	botToken     string
	signSecret   string
	noAuthn      string
}

func (x *Slack) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "slack-client-id",
			Usage:       "Slack OAuth Client ID",
			Sources:     cli.EnvVars("SHEPHERD_SLACK_CLIENT_ID"),
			Destination: &x.clientID,
		},
		&cli.StringFlag{
			Name:        "slack-client-secret",
			Usage:       "Slack OAuth Client Secret",
			Sources:     cli.EnvVars("SHEPHERD_SLACK_CLIENT_SECRET"),
			Destination: &x.clientSecret,
		},
		&cli.StringFlag{
			Name:        "slack-bot-token",
			Usage:       "Slack Bot Token",
			Sources:     cli.EnvVars("SHEPHERD_SLACK_BOT_TOKEN"),
			Destination: &x.botToken,
		},
		&cli.StringFlag{
			Name:        "slack-signing-secret",
			Usage:       "Slack Signing Secret",
			Sources:     cli.EnvVars("SHEPHERD_SLACK_SIGNING_SECRET"),
			Destination: &x.signSecret,
		},
		&cli.StringFlag{
			Name:        "no-authn",
			Usage:       "NoAuthn mode: always authenticate as this Slack User ID",
			Sources:     cli.EnvVars("SHEPHERD_NO_AUTHN"),
			Destination: &x.noAuthn,
		},
	}
}

func (x *Slack) ConfigureAuth(ctx context.Context, repo interfaces.Repository, baseURL string) (usecase.AuthUseCaseInterface, error) {
	logger := logging.Default()

	if x.noAuthn != "" {
		logger.Warn("NoAuthn mode enabled", slog.String("user_id", x.noAuthn))

		email := x.noAuthn + "@noauthn.local"
		name := x.noAuthn
		if x.botToken != "" {
			client := x.NewSlackClient()
			info, err := client.GetUserInfo(ctx, x.noAuthn)
			if err != nil {
				logger.Warn("Failed to fetch Slack user info, using user ID as name",
					slog.String("user_id", x.noAuthn), slog.Any("error", err))
			} else {
				email = info.Email
				name = info.Name
				logger.Info("NoAuthn user resolved", slog.String("name", name), slog.String("email", email))
			}
		}
		return usecase.NewNoAuthnUseCase(x.noAuthn, email, name), nil
	}

	if x.clientID != "" && x.clientSecret != "" {
		if baseURL == "" {
			return nil, goerr.New("--base-url is required when Slack OAuth is enabled (--slack-client-id and --slack-client-secret are set)")
		}
		callbackURL := baseURL + "/api/auth/callback"
		return usecase.NewAuthUseCase(repo, x.clientID, x.clientSecret, callbackURL), nil
	}

	logger.Warn("No auth configured, using NoAuthn with default user")
	return usecase.NewNoAuthnUseCase("U_DEFAULT", "default@example.com", "Default User"), nil
}

func (x *Slack) IsWebhookConfigured() bool {
	return x.botToken != "" && x.signSecret != ""
}

func (x *Slack) BotToken() string    { return x.botToken }
func (x *Slack) SignSecret() string  { return x.signSecret }

func (x *Slack) NewSlackClient() *slackService.Client {
	return slackService.NewClient(x.botToken)
}

func (x *Slack) NewSlackUseCase(repo interfaces.Repository, registry *model.WorkspaceRegistry, baseURL string, llm gollem.LLMClient, history gollem.HistoryRepository, traceRepo trace.Repository) *usecase.SlackUseCase {
	client := x.NewSlackClient()
	return usecase.NewSlackUseCase(repo, registry, client, baseURL, llm, history, traceRepo)
}
