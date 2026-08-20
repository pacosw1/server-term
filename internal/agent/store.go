package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/metrics"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`PRAGMA journal_mode=WAL`, `PRAGMA synchronous=NORMAL`, `PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS samples (ts_ms INTEGER PRIMARY KEY, payload BLOB NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS samples_ts ON samples(ts_ms)`,
	} {
		if _, err := db.Exec(q); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize sqlite: %w", err)
		}
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("protect sqlite database: %w", err)
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Insert(ctx context.Context, w metrics.WireSample) error {
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR REPLACE INTO samples(ts_ms,payload) VALUES(?,?)`, w.Sample.At.UnixMilli(), b)
	return err
}
func (s *Store) History(ctx context.Context, since time.Time, limit int) ([]metrics.WireSample, error) {
	if limit < 1 || limit > 10000 {
		limit = 3600
	}
	span := time.Since(since).Milliseconds()
	bucket := span / int64(limit)
	if bucket < 1 {
		bucket = 1
	}
	rows, err := s.db.QueryContext(ctx, `WITH ranked AS (
        SELECT payload, ts_ms, ROW_NUMBER() OVER (PARTITION BY ((ts_ms - ?) / ?) ORDER BY ts_ms DESC) AS rn
        FROM samples WHERE ts_ms >= ?
    ) SELECT payload FROM ranked WHERE rn = 1 ORDER BY ts_ms ASC LIMIT ?`, since.UnixMilli(), bucket, since.UnixMilli(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []metrics.WireSample{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var w metrics.WireSample
		if err := json.Unmarshal(b, &w); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func (s *Store) Prune(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM samples WHERE ts_ms < ?`, before.UnixMilli())
	return err
}

// Compact maintains a sliding window in one table: recent samples stay dense
// while older ranges retain one representative sample per progressively larger bucket.
func (s *Store) Compact(ctx context.Context, now time.Time) error {
	tiers := []struct {
		newer, older time.Duration
		bucketMS     int64
	}{
		{time.Hour, 24 * time.Hour, 60_000},
		{24 * time.Hour, 7 * 24 * time.Hour, 300_000},
		{7 * 24 * time.Hour, 30 * 24 * time.Hour, 1_800_000},
	}
	for _, tier := range tiers {
		_, err := s.db.ExecContext(ctx, `DELETE FROM samples WHERE ts_ms IN (
            SELECT ts_ms FROM (
              SELECT ts_ms, ROW_NUMBER() OVER (PARTITION BY (ts_ms / ?) ORDER BY ts_ms DESC) rn
              FROM samples WHERE ts_ms < ? AND ts_ms >= ?
            ) WHERE rn > 1
          )`, tier.bucketMS, now.Add(-tier.newer).UnixMilli(), now.Add(-tier.older).UnixMilli())
		if err != nil {
			return err
		}
	}
	return s.Prune(ctx, now.Add(-30*24*time.Hour))
}
