package realtime

import (
	"context"
	"log/slog"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/model"
)

type Hub struct {
	log     *slog.Logger
	mu      sync.RWMutex
	clients map[int64]map[*Client]struct{}
}

type Client struct {
	RepoID int64
	Conn   *websocket.Conn
	Send   chan model.VizPatch
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{log: log, clients: make(map[int64]map[*Client]struct{})}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.RepoID] == nil {
		h.clients[c.RepoID] = make(map[*Client]struct{})
	}
	h.clients[c.RepoID][c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.clients[c.RepoID]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, c.RepoID)
		}
	}
	close(c.Send)
}

func (h *Hub) Broadcast(patch model.VizPatch) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[patch.RepoID] {
		select {
		case c.Send <- patch:
		default:
			h.log.Warn("dropping patch for slow websocket client", "repo_id", patch.RepoID)
		}
	}
}

func (c *Client) WriteLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case patch, ok := <-c.Send:
			if !ok {
				return nil
			}
			if err := wsjson.Write(ctx, c.Conn, patch); err != nil {
				return err
			}
		}
	}
}
