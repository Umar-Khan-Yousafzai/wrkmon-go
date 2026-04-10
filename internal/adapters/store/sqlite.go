package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/ports"
	_ "modernc.org/sqlite"
)

// Compile-time check that SQLiteStore satisfies ports.Store.
var _ ports.Store = (*SQLiteStore)(nil)

// SQLiteStore implements ports.Store using a local SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) the database at the given path.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Create parent directory if needed.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS history (
		id          TEXT PRIMARY KEY,
		video_id    TEXT NOT NULL,
		title       TEXT NOT NULL,
		channel     TEXT NOT NULL DEFAULT '',
		duration_ms INTEGER NOT NULL DEFAULT 0,
		played_at   TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_history_played_at ON history(played_at DESC);
	CREATE INDEX IF NOT EXISTS idx_history_title ON history(title);

	CREATE TABLE IF NOT EXISTS queue (
		position    INTEGER PRIMARY KEY,
		track_id    TEXT NOT NULL,
		video_id    TEXT NOT NULL,
		title       TEXT NOT NULL,
		channel     TEXT NOT NULL DEFAULT '',
		duration_ms INTEGER NOT NULL DEFAULT 0,
		stream_url  TEXT NOT NULL DEFAULT '',
		added_at    TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS queue_state (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// SaveHistory persists a history entry, replacing any existing entry with the same ID.
func (s *SQLiteStore) SaveHistory(ctx context.Context, entry core.HistoryEntry) error {
	const q = `INSERT OR REPLACE INTO history (id, video_id, title, channel, duration_ms, played_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, q,
		entry.ID,
		entry.Track.VideoID,
		entry.Track.Title,
		entry.Track.Channel,
		entry.Track.Duration.Milliseconds(),
		entry.PlayedAt.Format(time.RFC3339),
	)
	return err
}

// GetHistory returns history entries ordered by most recently played, with pagination.
func (s *SQLiteStore) GetHistory(ctx context.Context, limit int, offset int) ([]core.HistoryEntry, error) {
	const q = `SELECT id, video_id, title, channel, duration_ms, played_at
		FROM history ORDER BY played_at DESC LIMIT ? OFFSET ?`

	rows, err := s.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanHistoryRows(rows)
}

// SearchHistory finds history entries whose title contains the query substring (case-insensitive).
func (s *SQLiteStore) SearchHistory(ctx context.Context, query string, limit int) ([]core.HistoryEntry, error) {
	const q = `SELECT id, video_id, title, channel, duration_ms, played_at
		FROM history WHERE title LIKE ? ORDER BY played_at DESC LIMIT ?`

	pattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, q, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanHistoryRows(rows)
}

// scanHistoryRows is a helper that scans rows into []core.HistoryEntry.
func scanHistoryRows(rows *sql.Rows) ([]core.HistoryEntry, error) {
	var entries []core.HistoryEntry
	for rows.Next() {
		var (
			e          core.HistoryEntry
			durationMs int64
			playedAt   string
		)
		if err := rows.Scan(
			&e.ID,
			&e.Track.VideoID,
			&e.Track.Title,
			&e.Track.Channel,
			&durationMs,
			&playedAt,
		); err != nil {
			return nil, err
		}
		e.Track.Duration = time.Duration(durationMs) * time.Millisecond
		t, err := time.Parse(time.RFC3339, playedAt)
		if err != nil {
			return nil, fmt.Errorf("parse played_at: %w", err)
		}
		e.PlayedAt = t
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// SaveQueue atomically replaces the persisted queue with the given tracks and cursor.
func (s *SQLiteStore) SaveQueue(ctx context.Context, tracks []core.Track, cursor int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Clear existing queue and state.
	if _, err := tx.ExecContext(ctx, "DELETE FROM queue"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM queue_state"); err != nil {
		return err
	}

	// Insert each track with its position index.
	const insertTrack = `INSERT INTO queue (position, track_id, video_id, title, channel, duration_ms, stream_url, added_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	for i, t := range tracks {
		if _, err := tx.ExecContext(ctx, insertTrack,
			i,
			t.ID,
			t.VideoID,
			t.Title,
			t.Channel,
			t.Duration.Milliseconds(),
			t.StreamURL,
			t.AddedAt.Format(time.RFC3339),
		); err != nil {
			return err
		}
	}

	// Store cursor position.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO queue_state (key, value) VALUES ('cursor', ?)`,
		fmt.Sprintf("%d", cursor),
	); err != nil {
		return err
	}

	return tx.Commit()
}

// LoadQueue restores the persisted queue. Returns empty slice and cursor -1 if no queue is saved.
func (s *SQLiteStore) LoadQueue(ctx context.Context) ([]core.Track, int, error) {
	const q = `SELECT track_id, video_id, title, channel, duration_ms, stream_url, added_at
		FROM queue ORDER BY position`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, -1, err
	}
	defer rows.Close()

	var tracks []core.Track
	for rows.Next() {
		var (
			t          core.Track
			durationMs int64
			addedAt    string
		)
		if err := rows.Scan(
			&t.ID,
			&t.VideoID,
			&t.Title,
			&t.Channel,
			&durationMs,
			&t.StreamURL,
			&addedAt,
		); err != nil {
			return nil, -1, err
		}
		t.Duration = time.Duration(durationMs) * time.Millisecond
		parsed, err := time.Parse(time.RFC3339, addedAt)
		if err != nil {
			return nil, -1, fmt.Errorf("parse added_at: %w", err)
		}
		t.AddedAt = parsed
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, -1, err
	}

	// Read cursor from queue_state.
	cursor := -1
	if len(tracks) > 0 {
		var cursorStr string
		err := s.db.QueryRowContext(ctx, `SELECT value FROM queue_state WHERE key = 'cursor'`).Scan(&cursorStr)
		if err == nil {
			fmt.Sscanf(cursorStr, "%d", &cursor)
		}
	}

	return tracks, cursor, nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
