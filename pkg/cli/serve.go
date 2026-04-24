package cli

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/cli/config"
	httpController "github.com/m-mizutani/shepherd/pkg/controller/http"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	"github.com/urfave/cli/v3"
)

func cmdServe() *cli.Command {
	var (
		addr    string
		baseURL string

		workspaceCfg config.WorkspaceFiles
		repoCfg      config.Repository
		slackCfg     config.Slack
		sentryCfg    config.Sentry
	)

	flags := []cli.Flag{
		&cli.StringFlag{
			Name:        "addr",
			Usage:       "Listen address",
			Sources:     cli.EnvVars("SHEPHERD_ADDR"),
			Value:       ":8080",
			Destination: &addr,
		},
		&cli.StringFlag{
			Name:        "base-url",
			Usage:       "External base URL (required for Slack OAuth callback)",
			Sources:     cli.EnvVars("SHEPHERD_BASE_URL"),
			Destination: &baseURL,
		},
	}
	flags = append(flags, workspaceCfg.Flags()...)
	flags = append(flags, repoCfg.Flags()...)
	flags = append(flags, slackCfg.Flags()...)
	flags = append(flags, sentryCfg.Flags()...)

	return &cli.Command{
		Name:  "serve",
		Usage: "Start HTTP server",
		Flags: flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			logger := logging.Default()

			sentryCleanup, err := sentryCfg.Configure()
			if err != nil {
				return err
			}
			defer sentryCleanup()

			workspaceConfigs, err := workspaceCfg.Configure()
			if err != nil {
				return goerr.Wrap(err, "failed to load workspace configs")
			}
			registry := config.BuildRegistry(workspaceConfigs)

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

			var serverOpts []httpController.ServerOption
			if slackCfg.IsWebhookConfigured() {
				slackUC := slackCfg.NewSlackUseCase(repo, registry, baseURL)
				serverOpts = append(serverOpts, httpController.WithSlack(httpController.SlackConfig{
					SigningSecret: slackCfg.SignSecret(),
					SlackUC:       slackUC,
				}))
				logger.Info("Slack integration enabled")
			}
			httpServer := httpController.New(registry, repo, authUC, serverOpts...)

			server := &http.Server{
				Addr:    addr,
				Handler: httpServer,
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

			logger.Info("Server stopped")
			return nil
		},
	}
}
