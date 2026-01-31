package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/harshpatel5940/gitvigil/internal/api"
	"github.com/harshpatel5940/gitvigil/internal/auth"
	"github.com/harshpatel5940/gitvigil/internal/config"
	"github.com/harshpatel5940/gitvigil/internal/database"
	"github.com/harshpatel5940/gitvigil/internal/github"
	"github.com/harshpatel5940/gitvigil/internal/scorecard"
	"github.com/harshpatel5940/gitvigil/internal/webhook"
	"github.com/rs/zerolog"
)

type Server struct {
	cfg    *config.Config
	db     *database.DB
	gh     *github.AppClient
	router *chi.Mux
	logger zerolog.Logger
}

func New(cfg *config.Config, db *database.DB, gh *github.AppClient, logger zerolog.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		db:     db,
		gh:     gh,
		router: chi.NewRouter(),
		logger: logger,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))
	s.router.Use(s.loggingMiddleware)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			s.logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Dur("duration", time.Since(start)).
				Msg("request completed")
		}()

		next.ServeHTTP(ww, r)
	})
}

func (s *Server) setupRoutes() {
	s.router.Get("/health", s.handleHealth)

	// Webhook endpoint
	webhookHandler := webhook.NewHandler(s.cfg, s.db, s.gh, s.logger)
	s.router.Post("/webhook", webhookHandler.ServeHTTP)

	// Scorecard endpoint
	scorecardHandler := scorecard.NewHandler(s.db, s.logger)
	s.router.Get("/scorecard", scorecardHandler.ServeHTTP)

	// Auth endpoint
	authHandler := auth.NewHandler(s.cfg, s.logger)
	s.router.Get("/auth/github/callback", authHandler.HandleCallback)

	// API v1 endpoints
	apiHandler := api.NewHandler(s.db, s.logger)
	s.router.Mount("/api/v1", apiHandler.Router())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := s.db.Health(ctx); err != nil {
		s.logger.Error().Err(err).Msg("database health check failed")
		http.Error(w, "database unhealthy", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

func (s *Server) Router() *chi.Mux {
	return s.router
}

func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:         ":" + s.cfg.Port,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info().Str("port", s.cfg.Port).Msg("starting server")

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info().Msg("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
