package cli

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/go-github/v75/github"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/shepherd/pkg/cli/config"
	controller "github.com/m-mizutani/shepherd/pkg/controller/http"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/urfave/cli/v3"
)

func cmdServe() *cli.Command {
	var (
		serverCfg config.Server
		githubCfg config.GitHub
		geminiCfg config.Gemini
	)

	flags := append(append(serverCfg.Flags(), githubCfg.Flags()...), geminiCfg.Flags()...)

	return &cli.Command{
		Name:    "serve",
		Aliases: []string{"s"},
		Usage:   "Start HTTP server",
		Flags:   flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			logger := ctxlog.From(ctx)

			logger.Info("Starting shepherd server",
				slog.String("addr", serverCfg.Addr),
			)

			// Create Gemini client with ADC
			geminiClient, err := gemini.New(ctx, geminiCfg.Location, geminiCfg.ProjectID,
				gemini.WithModel(geminiCfg.Model),
			)
			if err != nil {
				return goerr.Wrap(err, "failed to create gemini client")
			}

			// Create GitHub client for commenting
			githubToken := os.Getenv("GITHUB_TOKEN")
			githubClient := github.NewClient(nil).WithAuthToken(githubToken)

			// Create use cases
			webhookUC := usecase.NewWebhook()

			// Create package detector use case
			pkgDetectorUC, err := usecase.NewPackageDetector(geminiClient, githubClient)
			if err != nil {
				return goerr.Wrap(err, "failed to create package detector use case")
			}

			// Create HTTP server with options
			server, err := controller.NewServer(
				ctx,
				webhookUC,
				pkgDetectorUC,
				controller.WithAddr(serverCfg.Addr),
				controller.WithWebhookSecret(githubCfg.WebhookSecret),
			)
			if err != nil {
				return goerr.Wrap(err, "failed to create HTTP server")
			}

			// Start server in goroutine
			go func() {
				logger.Info("HTTP server starting", slog.String("addr", serverCfg.Addr))
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Error("HTTP server error", slog.Any("error", err))
				}
			}()

			// Wait for interrupt signal
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			select {
			case <-ctx.Done():
				logger.Info("Context cancelled, shutting down...")
			case sig := <-sigChan:
				logger.Info("Signal received, shutting down...", slog.Any("signal", sig))
			}

			// Graceful shutdown
			shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			if err := server.Shutdown(shutdownCtx); err != nil {
				return goerr.Wrap(err, "failed to shutdown server gracefully")
			}

			logger.Info("Server shutdown complete")
			return nil
		},
	}
}
