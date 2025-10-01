package http

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
)

// config holds internal HTTP server configuration
type config struct {
	addr          string
	webhookSecret string
}

// Option is a functional option for Server configuration
type Option func(*config)

// WithAddr sets the server address
func WithAddr(addr string) Option {
	return func(c *config) {
		c.addr = addr
	}
}

// WithWebhookSecret sets the webhook secret
func WithWebhookSecret(secret string) Option {
	return func(c *config) {
		c.webhookSecret = secret
	}
}

// Server represents the HTTP server
type Server struct {
	*http.Server
}

// NewServer creates a new HTTP server
func NewServer(
	ctx context.Context,
	webhookUC interfaces.WebhookUseCase,
	opts ...Option,
) (*Server, error) {
	// Default configuration
	cfg := &config{
		addr: "localhost:8080",
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	router := chi.NewRouter()

	// Global middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(LoggingMiddleware(ctx))
	router.Use(middleware.Recoverer)

	// Health check
	router.Get("/health", handleHealth)

	// Webhook endpoint
	webhookHandler := NewWebhookHandler(cfg.webhookSecret, webhookUC)
	router.Post("/hooks/github/app", webhookHandler.Handle)

	server := &Server{
		Server: &http.Server{
			Addr:              cfg.addr,
			Handler:           router,
			ReadHeaderTimeout: 15 * time.Second,
		},
	}

	return server, nil
}
