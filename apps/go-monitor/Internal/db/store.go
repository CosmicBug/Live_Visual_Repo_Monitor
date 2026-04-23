package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shashank/repo-visual-monitor/apps/go-monitor/internal/model"
)

type Store struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func (s *Store) UpsertRepository(ctx context.Context, repo model.Repository) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO repositories(id, owner, name, full_name, default_branch, installation_id, html_url, updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,now())
ON CONFLICT(id) DO UPDATE SET
  owner = EXCLUDED.owner,
  name = EXCLUDED.name,
  full_name = EXCLUDED.full_name,
  default_branch = EXCLUDED.default_branch,
  installation_id = EXCLUDED.installation_id,
  html_url = EXCLUDED.html_url,
  updated_at = now()
`, repo.ID, repo.Owner, repo.Name, repo.FullName, repo.DefaultBranch, repo.InstallationID, repo.HTMLURL)
	return err
}

func (s *Store) InsertDelivery(ctx context.Context, ev model.NormalizedEvent, raw []byte) (bool, error) {
	if err := s.UpsertRepository(ctx, ev.Repository); err != nil {
		return false, err
	}
	normalized, err := json.Marshal(ev)
	if err != nil {
		return false, err
	}
	tag, err := s.pool.Exec(ctx, `
INSERT INTO webhook_deliveries(delivery_id, repo_id, event, action, received_at, payload, normalized, status)
VALUES($1,$2,$3,$4,$5,$6,$7,'received')
ON CONFLICT(delivery_id) DO NOTHING
`, ev.DeliveryID, ev.Repository.ID, ev.Event, ev.Action, ev.ReceivedAt, raw, normalized)
	return tag.RowsAffected() > 0, err
}

func (s *Store) MarkDeliveryProcessed(ctx context.Context, deliveryID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE webhook_deliveries SET status='processed', processed_at=now(), error=NULL WHERE delivery_id=$1`, deliveryID)
	return err
}

func (s *Store) MarkDeliveryFailed(ctx context.Context, deliveryID string, processErr error) error {
	msg := ""
	if processErr != nil {
		msg = processErr.Error()
	}
	_, err := s.pool.Exec(ctx, `UPDATE webhook_deliveries SET status='failed', processed_at=now(), error=$2 WHERE delivery_id=$1`, deliveryID, msg)
	return err
}

func (s *Store) RecentEvents(ctx context.Context, repoID int64, limit int) ([]model.NormalizedEvent, error) {
	rows, err := s.pool.Query(ctx, `
SELECT normalized
FROM webhook_deliveries
WHERE repo_id=$1 AND normalized IS NOT NULL
ORDER BY received_at DESC
LIMIT $2
`, repoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.NormalizedEvent, 0)
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var ev model.NormalizedEvent
		if err := json.Unmarshal(b, &ev); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) LatestSnapshot(ctx context.Context, repoID int64) (*model.StoredSnapshot, error) {
	row := s.pool.QueryRow(ctx, `
SELECT id::text, repo_id, commit_sha, COALESCE(branch,''), COALESCE(tree_sha,''), created_at, snapshot
FROM repo_snapshots
WHERE repo_id=$1
ORDER BY created_at DESC
LIMIT 1
`, repoID)
	var rec model.StoredSnapshot
	var b []byte
	if err := row.Scan(&rec.ID, &rec.RepoID, &rec.CommitSHA, &rec.Branch, &rec.TreeSHA, &rec.CreatedAt, &b); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &rec.Snapshot); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *Store) InsertSnapshot(ctx context.Context, snap model.RepoSnapshot) (string, error) {
	if err := s.UpsertRepository(ctx, snap.Repo); err != nil {
		return "", err
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return "", err
	}
	var id string
	err = s.pool.QueryRow(ctx, `
INSERT INTO repo_snapshots(repo_id, commit_sha, branch, tree_sha, snapshot)
VALUES($1,$2,$3,$4,$5)
ON CONFLICT(repo_id, commit_sha) DO UPDATE SET
  branch = EXCLUDED.branch,
  tree_sha = EXCLUDED.tree_sha,
  snapshot = EXCLUDED.snapshot
RETURNING id::text
`, snap.Repo.ID, snap.Commit.SHA, snap.Commit.Branch, snap.TreeSHA, b).Scan(&id)
	return id, err
}

func (s *Store) InsertDiff(ctx context.Context, repoID int64, beforeSnapshotID string, afterSnapshotID string, diff model.RepoDiff) (string, error) {
	b, err := json.Marshal(diff)
	if err != nil {
		return "", err
	}
	var id string
	err = s.pool.QueryRow(ctx, `
INSERT INTO repo_diffs(repo_id, before_snapshot, after_snapshot, diff)
VALUES($1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, $4)
RETURNING id::text
`, repoID, beforeSnapshotID, afterSnapshotID, b).Scan(&id)
	return id, err
}

func (s *Store) NextModelVersion(ctx context.Context, repoID int64) (int64, error) {
	var version int64
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(model_version),0)+1 FROM algebraic_models WHERE repo_id=$1`, repoID).Scan(&version)
	return version, err
}

func (s *Store) LatestVizModel(ctx context.Context, repoID int64) (*model.VizModel, error) {
	row := s.pool.QueryRow(ctx, `
SELECT viz_model
FROM algebraic_models
WHERE repo_id=$1
ORDER BY model_version DESC
LIMIT 1
`, repoID)
	var b []byte
	if err := row.Scan(&b); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	var viz model.VizModel
	if err := json.Unmarshal(b, &viz); err != nil {
		return nil, err
	}
	return &viz, nil
}

func (s *Store) InsertAlgebraicModel(ctx context.Context, repoID int64, snapshotID string, resp model.CompileResponse) (string, error) {
	catlab, err := json.Marshal(resp.CatlabModel)
	if err != nil {
		return "", err
	}
	petri, err := json.Marshal(resp.PetriModel)
	if err != nil {
		return "", err
	}
	viz, err := json.Marshal(resp.VizModel)
	if err != nil {
		return "", err
	}
	var id string
	err = s.pool.QueryRow(ctx, `
INSERT INTO algebraic_models(repo_id, snapshot_id, model_version, catlab_model, petri_model, viz_model)
VALUES($1, NULLIF($2,'')::uuid, $3, $4, $5, $6)
RETURNING id::text
`, repoID, snapshotID, resp.ModelVersion, catlab, petri, viz).Scan(&id)
	return id, err
}

func (s *Store) InsertVizPatch(ctx context.Context, patch model.VizPatch) (string, error) {
	b, err := json.Marshal(patch)
	if err != nil {
		return "", err
	}
	var id string
	err = s.pool.QueryRow(ctx, `
INSERT INTO viz_patches(repo_id, from_version, to_version, patch)
VALUES($1, NULLIF($2,0), $3, $4)
RETURNING id::text
`, patch.RepoID, patch.FromVersion, patch.ToVersion, b).Scan(&id)
	return id, err
}

func (s *Store) RecentPatches(ctx context.Context, repoID int64, sinceVersion int64, limit int) ([]model.VizPatch, error) {
	rows, err := s.pool.Query(ctx, `
SELECT patch
FROM viz_patches
WHERE repo_id=$1 AND to_version > $2
ORDER BY to_version ASC, created_at ASC
LIMIT $3
`, repoID, sinceVersion, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	patches := make([]model.VizPatch, 0)
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var p model.VizPatch
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, err
		}
		patches = append(patches, p)
	}
	return patches, rows.Err()
}

func NowUTC() time.Time { return time.Now().UTC() }
