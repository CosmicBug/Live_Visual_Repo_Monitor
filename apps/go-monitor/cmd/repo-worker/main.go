package main

import (
	"context"
	"errors"
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
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/githubapp"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/model"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/modeler"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/snapshot"
)

type worker struct {
	store   *db.Store
	bus     *bus.Bus
	builder snapshot.Builder
	modeler *modeler.Client
	log     *slog.Logger
}

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

	w := &worker{
		store: store,
		bus:   eventBus,
		builder: snapshot.Builder{
			GitHubFactory:         githubapp.Factory{AppID: cfg.GitHubAppID, PrivateKeyPath: cfg.GitHubPrivateKeyPath, BaseURL: cfg.GitHubAPIBaseURL},
			MaxSourceFilesToParse: cfg.MaxSourceFilesToParse,
			MaxBlobBytes:          cfg.MaxBlobBytes,
		},
		modeler: modeler.New(cfg.JuliaModelerURL),
		log:     log,
	}

	go serveHealth(log, cfg.WorkerMetricsAddr)

	sub, err := eventBus.SubscribePull(bus.SubjectGitHub, "repo-worker")
	if err != nil {
		log.Error("subscribe", "error", err)
		os.Exit(1)
	}

	log.Info("repo worker started")
	for {
		select {
		case <-ctx.Done():
			log.Info("repo worker stopping")
			return
		default:
		}
		msgs, err := sub.Fetch(10, nats.MaxWait(2*time.Second))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			log.Warn("fetch messages", "error", err)
			continue
		}
		for _, msg := range msgs {
			ev, err := bus.DecodeJSONMsg[model.NormalizedEvent](msg)
			if err != nil {
				log.Error("decode event", "error", err)
				_ = msg.Term()
				continue
			}
			if err := w.process(ctx, ev); err != nil {
				log.Error("process event", "error", err, "event", ev.Event, "delivery", ev.DeliveryID)
				_ = store.MarkDeliveryFailed(ctx, ev.DeliveryID, err)
				_ = msg.Nak()
				continue
			}
			_ = store.MarkDeliveryProcessed(ctx, ev.DeliveryID)
			_ = msg.Ack()
		}
	}
}

func (w *worker) process(ctx context.Context, ev model.NormalizedEvent) error {
	w.log.Info("processing event", "event", ev.Event, "action", ev.Action, "repo", ev.Repository.FullName)
	prevModel, err := w.store.LatestVizModel(ctx, ev.Repository.ID)
	if err != nil {
		return err
	}
	version, err := w.store.NextModelVersion(ctx, ev.Repository.ID)
	if err != nil {
		return err
	}

	var snapshotID string
	var req model.CompileRequest
	req.PreviousModel = prevModel
	req.Event = &ev
	req.Context = map[string]any{"model_version": version}

	if ev.Event == "push" && !ev.Deleted {
		prevSnapRecord, err := w.store.LatestSnapshot(ctx, ev.Repository.ID)
		if err != nil {
			return err
		}
		eventsWindow, err := w.store.RecentEvents(ctx, ev.Repository.ID, 50)
		if err != nil {
			return err
		}
		snap, err := w.builder.Build(ctx, ev, eventsWindow)
		if err != nil {
			return err
		}
		var prevSnap *model.RepoSnapshot
		beforeSnapshotID := ""
		if prevSnapRecord != nil {
			prevSnap = &prevSnapRecord.Snapshot
			beforeSnapshotID = prevSnapRecord.ID
		}
		diff := snapshot.Diff(prevSnap, snap)
		snapshotID, err = w.store.InsertSnapshot(ctx, snap)
		if err != nil {
			return err
		}
		if _, err := w.store.InsertDiff(ctx, ev.Repository.ID, beforeSnapshotID, snapshotID, diff); err != nil {
			return err
		}
		req.Snapshot = &snap
		req.Diff = &diff
	}

	resp, err := w.modeler.Compile(ctx, req)
	if err != nil {
		return err
	}
	if resp.ModelVersion == 0 {
		resp.ModelVersion = version
	}
	resp.VizModel.ModelVersion = resp.ModelVersion
	resp.Patch.ModelVersion = resp.ModelVersion
	resp.Patch.ToVersion = resp.ModelVersion
	resp.Patch.RepoID = ev.Repository.ID
	if resp.Patch.CreatedAt.IsZero() {
		resp.Patch.CreatedAt = time.Now().UTC()
	}

	if _, err := w.store.InsertAlgebraicModel(ctx, ev.Repository.ID, snapshotID, resp); err != nil {
		return err
	}
	if _, err := w.store.InsertVizPatch(ctx, resp.Patch); err != nil {
		return err
	}
	if err := w.bus.PublishJSON(ctx, bus.SubjectModelPatch, resp.Patch); err != nil {
		return err
	}
	return nil
}

func serveHealth(log *slog.Logger, addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	log.Info("worker health server listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Warn("health server stopped", "error", err)
	}
}
