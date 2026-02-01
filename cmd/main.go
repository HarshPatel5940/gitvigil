package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/harshpatel5940/gitvigil/internal/config"
	"github.com/harshpatel5940/gitvigil/internal/database"
	"github.com/harshpatel5940/gitvigil/internal/github"
	"github.com/harshpatel5940/gitvigil/internal/server"
	"github.com/rs/zerolog"
)

func main() {
	// Setup logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Caller().
		Logger()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load configuration")
	}

	logger.Info().
		Int64("app_id", cfg.AppID).
		Str("port", cfg.Port).
		Msg("configuration loaded")

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info().Str("signal", sig.String()).Msg("received shutdown signal")
		cancel()
	}()

	// Connect to database
	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()
	logger.Info().Msg("connected to database")

	// Run migrations
	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		logger.Fatal().Err(err).Msg("failed to run migrations")
	}
	logger.Info().Msg("migrations completed")

	// Create GitHub App client (optional - webhooks won't work without it)
	var gh *github.AppClient
	if len(cfg.PrivateKey) > 0 {
		gh, err = github.NewAppClient(cfg.AppID, cfg.PrivateKey)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to create GitHub App client")
		}
		logger.Info().Int64("app_id", gh.AppID()).Msg("GitHub App client created")
	} else {
		logger.Warn().Msg("no private key configured - GitHub App features disabled (webhooks, license checks)")
	}

	// Create and start server
	srv := server.New(cfg, db, gh, logger)

	if err := srv.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("server error")
	}

	logger.Info().Msg("server stopped gracefully")
}
