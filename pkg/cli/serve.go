package cli

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/cli/config"
	httpController "github.com/m-mizutani/shepherd/pkg/controller/http"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/m-mizutani/shepherd/pkg/tool/meta"
	tnotion "github.com/m-mizutani/shepherd/pkg/tool/notion"
	tslack "github.com/m-mizutani/shepherd/pkg/tool/slack"
	"github.com/m-mizutani/shepherd/pkg/tool/ticket"
	usecaseroot "github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/usecase/source"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	"github.com/urfave/cli/v3"
)

// slackToolerOrNil returns the Slack tool client when a bot token is
// configured, or nil so the slack Factory reports Available()=false.
func slackToolerOrNil(cfg config.Slack) tslack.SlackTooler {
	if cfg.BotToken() == "" {
		return nil
	}
	return cfg.NewSlackClient()
}

func cmdServe() *cli.Command {
	var (
		addr    string
		baseURL string
		lang    string

		workspaceCfg    config.WorkspaceFiles
		repoCfg         config.Repository
		slackCfg        config.Slack
		sentryCfg       config.Sentry
		llmCfg          config.LLM
		agentStorageCfg config.AgentStorage

		triageIterationCap int

		// Tool factories own their own --flags via Flags() and are constructed
		// up-front so the CLI flag list can be aggregated without pkg/cli
		// having to learn anything about the inner provider deps.
		notionFactory = tnotion.New(nil, nil) // deps wired after repo is built
	)

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:        "addr",
			Usage:       "Listen address",
			Sources:     cli.EnvVars("SHEPHERD_ADDR"),
			Value:       "localhost:8080",
			Destination: &addr,
		},
		&cli.StringFlag{
			Name:        "base-url",
			Usage:       "External base URL (required for Slack OAuth callback)",
			Sources:     cli.EnvVars("SHEPHERD_BASE_URL"),
			Destination: &baseURL,
		},
		&cli.StringFlag{
			Name:        "lang",
			Usage:       "Backend message language (en, ja)",
			Sources:     cli.EnvVars("SHEPHERD_LANG"),
			Value:       "en",
			Destination: &lang,
		},
		&cli.IntFlag{
			Name:        "triage-iteration-cap",
			Usage:       "Maximum number of triage planner turns per ticket before aborting",
			Sources:     cli.EnvVars("SHEPHERD_TRIAGE_ITERATION_CAP"),
			Value:       10,
			Destination: &triageIterationCap,
		},
	}
	flags = append(flags, workspaceCfg.Flags()...)
	flags = append(flags, repoCfg.Flags()...)
	flags = append(flags, slackCfg.Flags()...)
	flags = append(flags, sentryCfg.Flags()...)
	flags = append(flags, llmCfg.Flags()...)
	flags = append(flags, agentStorageCfg.Flags()...)
	flags = append(flags, notionFactory.Flags()...)

	return &cli.Command{
		Name:  "serve",
		Usage: "Start HTTP server",
		Flags: flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			logger := logging.Default()

			translator, err := i18n.NewTranslator(i18n.Lang(lang))
			if err != nil {
				return goerr.Wrap(err, "invalid --lang value")
			}
			ctx = i18n.With(ctx, translator)

			sentryCleanup, err := sentryCfg.Configure()
			if err != nil {
				return err
			}
			defer sentryCleanup()

			workspaceConfigs, err := workspaceCfg.Configure()
			if err != nil {
				return goerr.Wrap(err, "failed to load workspace configs")
			}

			var channelResolver config.ChannelResolver
			if slackCfg.BotToken() != "" {
				slackClient := slackCfg.NewSlackClient()
				channelResolver = slackClient.ResolveChannelName
			}

			registry, err := config.BuildRegistry(ctx, workspaceConfigs, channelResolver)
			if err != nil {
				return goerr.Wrap(err, "failed to build workspace registry")
			}

			repo, err := repoCfg.Configure(ctx)
			if err != nil {
				return goerr.Wrap(err, "failed to configure repository")
			}
			defer func() {
				if err := repo.Close(); err != nil {
					errutil.Handle(ctx, goerr.Wrap(err, "failed to close repository"))
				}
			}()

			authUC, err := slackCfg.ConfigureAuth(ctx, repo, baseURL)
			if err != nil {
				return err
			}

			if !llmCfg.IsEnabled() {
				return goerr.New("--llm-provider is required (openai, claude, or gemini)")
			}
			llmClient, err := llmCfg.NewClient(ctx)
			if err != nil {
				return goerr.Wrap(err, "failed to configure LLM client")
			}
			logger.Info("LLM integration enabled")

			historyRepo, traceRepo, agentBackend, err := agentStorageCfg.Configure(ctx)
			if err != nil {
				return goerr.Wrap(err, "failed to configure agent storage")
			}
			defer func() {
				if err := agentBackend.Close(); err != nil {
					errutil.Handle(ctx, goerr.Wrap(err, "failed to close agent storage backend"))
				}
			}()
			logger.Info("Agent storage configured", "agent_storage", &agentStorageCfg)

			var serverOpts []httpController.ServerOption
			var slackUC *usecaseroot.SlackUseCase
			var slackClient *slackService.Client
			if slackCfg.IsWebhookConfigured() {
				slackClient = slackCfg.NewSlackClient()
				slackUC = slackCfg.NewSlackUseCase(repo, registry, baseURL, llmClient, historyRepo, traceRepo)
				logger.Info("Slack integration enabled")
			}
			// Build the tool catalog: meta/ticket/slack are inert without
			// per-workspace data; notion gets its repo-derived deps + Init
			// here, after the repo is constructed.
			httpClient := &http.Client{Timeout: 30 * time.Second}
			notionFactory.SetDeps(repo.Source(), httpClient)

			factories := []tool.ToolFactory{
				meta.New(registry, time.Now),
				ticket.New(repo),
				tslack.New(slackToolerOrNil(slackCfg)),
				notionFactory,
			}
			for _, f := range factories {
				if err := f.Init(ctx); err != nil {
					return goerr.Wrap(err, "tool factory init failed", goerr.V("provider", string(f.ID())))
				}
			}
			if notionFactory.Available() {
				logger.Info("Notion integration enabled")
			} else {
				logger.Info("Notion integration disabled (set --notion-token to enable)")
			}

			catalog := tool.NewCatalog(factories, repo.ToolSettings()).
				WithGate(tool.ProviderNotion, func(ctx context.Context, ws types.WorkspaceID) (bool, error) {
					srcs, err := repo.Source().ListByProvider(ctx, ws, types.SourceProviderNotion)
					if err != nil {
						return false, err
					}
					return len(srcs) > 0, nil
				})

			// Build the triage usecase once Slack + catalog are ready, and
			// register it with both the SlackUseCase (Entry-1: ticket
			// creation trigger) and the HTTP interactions endpoint (Entry-2:
			// reporter submit click).
			var triageUC *triage.UseCase
			if slackUC != nil {
				triageExec := triage.NewPlanExecutor(
					repo, historyRepo, llmClient, slackClient, catalog,
					triage.Config{IterationCap: triageIterationCap},
				)
				triageUC = triage.NewUseCase(triageExec, &triage.RegistryResolver{Registry: registry})
				slackUC.SetTriageTrigger(triageUC)
				logger.Info("Triage agent enabled",
					"iteration_cap", triageIterationCap,
				)
			}

			if slackUC != nil {
				serverOpts = append(serverOpts, httpController.WithSlack(httpController.SlackConfig{
					SigningSecret: slackCfg.SignSecret(),
					SlackUC:       slackUC,
					Notifier:      slackClient,
					TriageUC:      triageUC,
				}))
			}

			sourceUC := source.New(repo.Source(), notionFactory.Client(), time.Now)
			serverOpts = append(serverOpts, httpController.WithSource(sourceUC, catalog))

			httpServer := httpController.New(registry, repo, authUC, serverOpts...)

			server := &http.Server{
				Addr:              addr,
				Handler:           httpServer,
				ReadHeaderTimeout: 10 * time.Second,
				BaseContext:       func(_ net.Listener) context.Context { return ctx },
			}

			errCh := make(chan error, 1)
			go func() {
				logger.Info("Starting server", "addr", addr)
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
				close(errCh)
			}()

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			select {
			case <-quit:
				logger.Info("Shutting down server...")
			case err := <-errCh:
				errutil.Handle(ctx, goerr.Wrap(err, "server error"))
				return err
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				errutil.Handle(ctx, goerr.Wrap(err, "server shutdown error"))
				return err
			}

			async.Wait()
			logger.Info("Server stopped")
			return nil
		},
	}
}
