package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/bus"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/config"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/db"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/webhook"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg, err := config.Load()
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	store, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	eventBus, err := bus.Connect(cfg.NATSURL)
	if err != nil {
		log.Error("connect nats", "error", err)
		os.Exit(1)
	}
	defer eventBus.Close()

	mux := http.NewServeMux()
	mux.Handle("/webhooks/github", &webhook.Handler{Secret: cfg.GitHubWebhookSecret, Store: store, Bus: eventBus, Log: log})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: cfg.WebhookAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Info("webhook gateway listening", "addr", cfg.WebhookAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
