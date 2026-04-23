package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/db"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/model"
)

type Server struct {
	Store *db.Store
	Hub   *Hub
	Log   *slog.Logger
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/model/latest", s.withCORS(s.latestModel))
	mux.HandleFunc("/api/patches", s.withCORS(s.recentPatches))
	mux.HandleFunc("/ws", s.withCORS(s.ws))
	return mux
}

func (s *Server) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *Server) latestModel(w http.ResponseWriter, r *http.Request) {
	repoID, ok := parseRepoID(w, r)
	if !ok {
		return
	}
	model, err := s.Store.LatestVizModel(r.Context(), repoID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if model == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"model": nil})
		return
	}
	_ = json.NewEncoder(w).Encode(model)
}

func (s *Server) recentPatches(w http.ResponseWriter, r *http.Request) {
	repoID, ok := parseRepoID(w, r)
	if !ok {
		return
	}
	since := int64(0)
	if v := r.URL.Query().Get("since_version"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			since = n
		}
	}
	patches, err := s.Store.RecentPatches(r.Context(), repoID, since, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(patches)
}

func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	repoID, ok := parseRepoID(w, r)
	if !ok {
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		s.Log.Warn("websocket accept", "error", err)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	client := &Client{RepoID: repoID, Conn: conn, Send: make(chan model.VizPatch, 128)}
	s.Hub.Register(client)
	defer func() {
		s.Hub.Unregister(client)
		cancel()
		_ = conn.Close(websocket.StatusNormalClosure, "closed")
	}()

	if v := r.URL.Query().Get("since_version"); v != "" {
		since, _ := strconv.ParseInt(v, 10, 64)
		patches, err := s.Store.RecentPatches(ctx, repoID, since, 200)
		if err == nil {
			for _, patch := range patches {
				_ = wsjson.Write(ctx, conn, patch)
			}
		}
	}

	go func() {
		for {
			_, _, err := conn.Read(ctx)
			if err != nil {
				cancel()
				return
			}
		}
	}()

	writeCtx, writeCancel := context.WithCancel(ctx)
	defer writeCancel()
	if err := client.WriteLoop(writeCtx); err != nil {
		s.Log.Debug("websocket write loop stopped", "error", err)
	}
}

func parseRepoID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	v := r.URL.Query().Get("repo_id")
	if v == "" {
		http.Error(w, "repo_id is required", http.StatusBadRequest)
		return 0, false
	}
	repoID, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		http.Error(w, "repo_id must be an integer", http.StatusBadRequest)
		return 0, false
	}
	return repoID, true
}

func ShutdownWithTimeout(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
