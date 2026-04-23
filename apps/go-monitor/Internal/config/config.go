package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL           string
	NATSURL               string
	LogLevel              string
	GitHubAppID           int64
	GitHubPrivateKeyPath  string
	GitHubWebhookSecret   string
	GitHubAPIBaseURL      string
	WebhookAddr           string
	WorkerMetricsAddr     string
	RealtimeAddr          string
	JuliaModelerURL       string
	MaxSourceFilesToParse int
	MaxBlobBytes          int
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:           getenv("DATABASE_URL", "postgres://repo_monitor:repo_monitor@localhost:5432/repo_monitor?sslmode=disable"),
		NATSURL:               getenv("NATS_URL", "nats://localhost:4222"),
		LogLevel:              getenv("LOG_LEVEL", "info"),
		GitHubPrivateKeyPath:  getenv("GITHUB_PRIVATE_KEY_PATH", ""),
		GitHubWebhookSecret:   getenv("GITHUB_WEBHOOK_SECRET", ""),
		GitHubAPIBaseURL:      getenv("GITHUB_API_BASE_URL", "https://api.github.com/"),
		WebhookAddr:           getenv("WEBHOOK_ADDR", ":8080"),
		WorkerMetricsAddr:     getenv("WORKER_METRICS_ADDR", ":8081"),
		RealtimeAddr:          getenv("REALTIME_ADDR", ":8082"),
		JuliaModelerURL:       getenv("JULIA_MODELER_URL", "http://localhost:8090"),
		MaxSourceFilesToParse: getenvInt("MAX_SOURCE_FILES_TO_PARSE", 200),
		MaxBlobBytes:          getenvInt("MAX_BLOB_BYTES", 262144),
	}

	appID, err := strconv.ParseInt(getenv("GITHUB_APP_ID", "0"), 10, 64)
	if err != nil {
		return cfg, fmt.Errorf("GITHUB_APP_ID must be an integer: %w", err)
	}
	cfg.GitHubAppID = appID
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
