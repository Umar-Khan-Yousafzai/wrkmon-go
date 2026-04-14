package store

import (
	"context"
	"database/sql"
	"encoding/json"
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

	CREATE TABLE IF NOT EXISTS search_cache (
		query        TEXT NOT NULL,
		results_json TEXT NOT NULL,
		expires_at   INTEGER NOT NULL,
		PRIMARY KEY (query)
	);

	CREATE TABLE IF NOT EXISTS playlists (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL UNIQUE,
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS playlist_items (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
		video_id    TEXT NOT NULL,
		title       TEXT NOT NULL,
		channel     TEXT NOT NULL DEFAULT '',
		duration_ms INTEGER NOT NULL DEFAULT 0,
		position    INTEGER NOT NULL,
		added_at    TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_playlist_items_pid ON playlist_items(playlist_id, position);

	CREATE TABLE IF NOT EXISTS lyrics_cache (
		video_id   TEXT PRIMARY KEY,
		lyrics     TEXT NOT NULL,
		fetched_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS downloads (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		video_id      TEXT NOT NULL,
		title         TEXT NOT NULL,
		channel       TEXT NOT NULL DEFAULT '',
		file_path     TEXT NOT NULL,
		file_size     INTEGER NOT NULL DEFAULT 0,
		downloaded_at TEXT NOT NULL
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

// CacheSearchResults stores search results with a TTL.
func (s *SQLiteStore) CacheSearchResults(ctx context.Context, query string, results []core.SearchResult, ttl time.Duration) error {
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(ttl).Unix()
	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO search_cache (query, results_json, expires_at) VALUES (?, ?, ?)`,
		query, string(data), expiresAt,
	)
	return err
}

// GetCachedSearch returns cached search results if they exist and haven't expired.
func (s *SQLiteStore) GetCachedSearch(ctx context.Context, query string) ([]core.SearchResult, bool, error) {
	var resultsJSON string
	var expiresAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT results_json, expires_at FROM search_cache WHERE query = ?`, query,
	).Scan(&resultsJSON, &expiresAt)
	if err != nil {
		return nil, false, nil // not found = cache miss, not an error
	}
	if time.Now().Unix() > expiresAt {
		// Expired — clean up
		s.db.ExecContext(ctx, `DELETE FROM search_cache WHERE query = ?`, query)
		return nil, false, nil
	}
	var results []core.SearchResult
	if err := json.Unmarshal([]byte(resultsJSON), &results); err != nil {
		return nil, false, nil // corrupted cache = miss
	}
	return results, true, nil
}

// CacheLyrics stores lyrics for a video.
func (s *SQLiteStore) CacheLyrics(ctx context.Context, videoID, lyrics string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO lyrics_cache (video_id, lyrics, fetched_at) VALUES (?, ?, ?)`,
		videoID, lyrics, time.Now().Format(time.RFC3339),
	)
	return err
}

// GetCachedLyrics returns cached lyrics for a video ID.
func (s *SQLiteStore) GetCachedLyrics(ctx context.Context, videoID string) (string, bool, error) {
	var lyrics string
	err := s.db.QueryRowContext(ctx,
		`SELECT lyrics FROM lyrics_cache WHERE video_id = ?`, videoID,
	).Scan(&lyrics)
	if err != nil {
		return "", false, nil
	}
	return lyrics, true, nil
}

// SaveDownload records a completed download.
func (s *SQLiteStore) SaveDownload(ctx context.Context, dl core.Download) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO downloads (video_id, title, channel, file_path, file_size, downloaded_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		dl.VideoID, dl.Title, dl.Channel, dl.FilePath, dl.FileSize,
		dl.DownloadedAt.Format(time.RFC3339),
	)
	return err
}

// ListDownloads returns recent downloads.
func (s *SQLiteStore) ListDownloads(ctx context.Context, limit int) ([]core.Download, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, video_id, title, channel, file_path, file_size, downloaded_at
		 FROM downloads ORDER BY downloaded_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var downloads []core.Download
	for rows.Next() {
		var d core.Download
		var dlAt string
		if err := rows.Scan(&d.ID, &d.VideoID, &d.Title, &d.Channel, &d.FilePath, &d.FileSize, &dlAt); err != nil {
			return nil, err
		}
		d.DownloadedAt, _ = time.Parse(time.RFC3339, dlAt)
		downloads = append(downloads, d)
	}
	return downloads, rows.Err()
}

// CreatePlaylist creates a new playlist. Returns error if name already exists.
func (s *SQLiteStore) CreatePlaylist(ctx context.Context, name string) (core.Playlist, error) {
	now := time.Now()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO playlists (name, created_at) VALUES (?, ?)`,
		name, now.Format(time.RFC3339),
	)
	if err != nil {
		return core.Playlist{}, fmt.Errorf("create playlist: %w", err)
	}
	id, _ := res.LastInsertId()
	return core.Playlist{ID: int(id), Name: name, CreatedAt: now}, nil
}

// ListPlaylists returns all playlists with track counts.
func (s *SQLiteStore) ListPlaylists(ctx context.Context) ([]core.Playlist, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, created_at FROM playlists ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []core.Playlist
	for rows.Next() {
		var p core.Playlist
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Name, &createdAt); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		playlists = append(playlists, p)
	}
	return playlists, rows.Err()
}

// GetPlaylist returns a playlist with all its tracks.
func (s *SQLiteStore) GetPlaylist(ctx context.Context, id int) (core.Playlist, error) {
	var p core.Playlist
	var createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, created_at FROM playlists WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &createdAt)
	if err != nil {
		return core.Playlist{}, fmt.Errorf("playlist not found: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	// Load tracks
	rows, err := s.db.QueryContext(ctx,
		`SELECT video_id, title, channel, duration_ms, added_at
		 FROM playlist_items WHERE playlist_id = ? ORDER BY position`, id)
	if err != nil {
		return p, err
	}
	defer rows.Close()

	for rows.Next() {
		var t core.Track
		var durationMs int64
		var addedAt string
		if err := rows.Scan(&t.VideoID, &t.Title, &t.Channel, &durationMs, &addedAt); err != nil {
			return p, err
		}
		t.Duration = time.Duration(durationMs) * time.Millisecond
		t.AddedAt, _ = time.Parse(time.RFC3339, addedAt)
		p.Tracks = append(p.Tracks, t)
	}
	return p, rows.Err()
}

// DeletePlaylist removes a playlist and all its items.
func (s *SQLiteStore) DeletePlaylist(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM playlists WHERE id = ?`, id)
	return err
}

// AddToPlaylist appends a track to a playlist.
func (s *SQLiteStore) AddToPlaylist(ctx context.Context, playlistID int, track core.Track) error {
	// Get next position
	var maxPos sql.NullInt64
	s.db.QueryRowContext(ctx,
		`SELECT MAX(position) FROM playlist_items WHERE playlist_id = ?`, playlistID,
	).Scan(&maxPos)
	nextPos := 0
	if maxPos.Valid {
		nextPos = int(maxPos.Int64) + 1
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO playlist_items (playlist_id, video_id, title, channel, duration_ms, position, added_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		playlistID, track.VideoID, track.Title, track.Channel,
		track.Duration.Milliseconds(), nextPos, time.Now().Format(time.RFC3339),
	)
	return err
}

// RemoveFromPlaylist removes a track at given position from a playlist.
func (s *SQLiteStore) RemoveFromPlaylist(ctx context.Context, playlistID int, position int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM playlist_items WHERE playlist_id = ? AND position = ?`,
		playlistID, position,
	)
	return err
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
