package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/bus"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/config"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/db"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/model"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/realtime"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg, err := config.Load()
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

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

	hub := realtime.NewHub(log)
	server := &realtime.Server{Store: store, Hub: hub, Log: log}
	srv := &http.Server{Addr: cfg.RealtimeAddr, Handler: server.Routes(), ReadHeaderTimeout: 10 * time.Second}

	go consumePatches(ctx, log, eventBus, hub)
	go func() {
		log.Info("realtime api listening", "addr", cfg.RealtimeAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	realtime.ShutdownWithTimeout(srv)
}

func consumePatches(ctx context.Context, log *slog.Logger, eventBus *bus.Bus, hub *realtime.Hub) {
	sub, err := eventBus.SubscribePull(bus.SubjectModelPatch, "realtime-api")
	if err != nil {
		log.Error("subscribe model patches", "error", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msgs, err := sub.Fetch(20, nats.MaxWait(2*time.Second))
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			log.Warn("fetch model patches", "error", err)
			continue
		}
		for _, msg := range msgs {
			patch, err := bus.DecodeJSONMsg[model.VizPatch](msg)
			if err != nil {
				log.Error("decode patch", "error", err)
				_ = msg.Term()
				continue
			}
			hub.Broadcast(patch)
			_ = msg.Ack()
		}
	}
}
