package http

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/m-mizutani/shepherd/frontend"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/safe"
)

type Server struct {
	mux      *chi.Mux
	slackCfg *SlackConfig
}

type ServerOption func(*Server)

type SlackConfig struct {
	SigningSecret string
	SlackUC       *usecase.SlackUseCase
	Notifier      usecase.StatusChangeNotifier
}

func WithSlack(cfg SlackConfig) ServerOption {
	return func(s *Server) {
		s.slackCfg = &cfg
	}
}

func New(registry *model.WorkspaceRegistry, repo interfaces.Repository, authUC usecase.AuthUseCaseInterface, opts ...ServerOption) *Server {
	s := &Server{
		mux: chi.NewRouter(),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.mux.Use(middleware.Recoverer)
	s.mux.Use(middleware.RealIP)
	s.mux.Use(httpLogger)

	// Auth endpoints (no auth middleware)
	s.mux.Route("/api/auth", func(r chi.Router) {
		r.Get("/login", authLoginHandler(authUC))
		r.Get("/callback", authCallbackHandler(authUC))
		r.Post("/logout", authLogoutHandler(authUC))
		r.Get("/me", authMeHandler(authUC))
	})

	// API endpoints (auth required)
	var notifier usecase.StatusChangeNotifier
	var slackUC *usecase.SlackUseCase
	if s.slackCfg != nil {
		notifier = s.slackCfg.Notifier
		slackUC = s.slackCfg.SlackUC
	}
	apiHandler := NewAPIHandler(registry, repo, notifier, slackUC)
	s.mux.Group(func(r chi.Router) {
		r.Use(authMiddleware(authUC))
		HandlerFromMux(apiHandler, r)
	})

	// Slack webhook (optional)
	if s.slackCfg != nil {
		s.mux.Route("/hooks/slack", func(r chi.Router) {
			r.Use(slackSignatureMiddleware(s.slackCfg.SigningSecret))
			r.Post("/event", slackEventHandler(s.slackCfg.SlackUC))
		})
	}

	// SPA handler
	staticFS, _ := fs.Sub(frontend.StaticFiles, "dist")
	s.mux.Handle("/*", spaHandler(staticFS))

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func spaHandler(staticFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(staticFS))

	return func(w http.ResponseWriter, r *http.Request) {
		urlPath := strings.TrimPrefix(r.URL.Path, "/")
		if urlPath == "" {
			urlPath = "index.html"
		}

		if file, err := staticFS.Open(urlPath); err != nil {
			if !strings.Contains(urlPath, ".") {
				if indexFile, err := staticFS.Open("index.html"); err == nil {
					defer safe.Close(r.Context(), indexFile)
					w.Header().Set("Content-Type", "text/html")
					safe.Copy(r.Context(), w, indexFile)
					return
				}
			}
			http.NotFound(w, r)
			return
		} else {
			safe.Close(r.Context(), file)
		}

		fileServer.ServeHTTP(w, r)
	}
}
