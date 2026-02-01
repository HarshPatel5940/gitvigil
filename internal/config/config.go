package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port    string
	BaseURL string

	// GitHub App
	AppID          int64
	ClientID       string
	ClientSecret   string
	WebhookSecret  string
	PrivateKeyPath string
	PrivateKey     []byte

	// Database
	DatabaseURL string

	// Detection thresholds
	BackdateSuspiciousHours int
	BackdateCriticalHours   int
	StreakInactivityHours   int
}

func Load() (*Config, error) {
	// Load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		Port:                    getEnv("PORT", "8080"),
		BaseURL:                 getEnv("BASE_URL", "http://localhost:8080"),
		ClientID:                os.Getenv("GITHUB_APP_CLIENT_ID"),
		ClientSecret:            os.Getenv("GITHUB_APP_CLIENT_SECRET"),
		WebhookSecret:           getEnv("GITHUB_WEBHOOK_SECRET", ""),
		PrivateKeyPath:          getEnv("GITHUB_PRIVATE_KEY_PATH", ""),
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		BackdateSuspiciousHours: getEnvInt("BACKDATE_SUSPICIOUS_HOURS", 24),
		BackdateCriticalHours:   getEnvInt("BACKDATE_CRITICAL_HOURS", 72),
		StreakInactivityHours:   getEnvInt("STREAK_INACTIVITY_HOURS", 72),
	}

	// Parse App ID
	appIDStr := os.Getenv("GITHUB_APP_ID")
	if appIDStr == "" {
		return nil, fmt.Errorf("GITHUB_APP_ID is required")
	}
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_ID: %w", err)
	}
	cfg.AppID = appID

	// Load private key if path is specified
	if cfg.PrivateKeyPath != "" {
		// Check if file exists first
		if _, err := os.Stat(cfg.PrivateKeyPath); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("private key file not found: %s (generate one from your GitHub App settings)", cfg.PrivateKeyPath)
			}
			return nil, fmt.Errorf("cannot access private key file: %w", err)
		}

		key, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key: %w", err)
		}
		cfg.PrivateKey = key
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
