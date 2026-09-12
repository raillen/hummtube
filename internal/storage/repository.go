// SQLite repository and FTS5 search index
// (docs/03-implementation/STORAGE.md).
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const (
	defaultRecentLimit = 20
	// videoUpsertBatch bounds each upsert transaction. One giant transaction
	// with tens of thousands of FTS5 rows stalls the Celeron-class targets.
	videoUpsertBatch = 500
)

// Repository implements domain.VideoRepository and domain.SearchIndex on SQLite.
type Repository struct {
	db *sql.DB
}

// NewRepository wraps a migrated SQLite connection.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// DB expõe o handle SQLite para operações que pertencem ao arquivo inteiro
// (integrity check e snapshot do backup), não a uma consulta do domínio.
func (r *Repository) DB() *sql.DB { return r.db }

// UpsertVideos inserts or updates video metadata, batching the writes into
// bounded transactions.
func (r *Repository) UpsertVideos(ctx context.Context, videos []domain.Video) error {
	for len(videos) > 0 {
		n := videoUpsertBatch
		if n > len(videos) {
			n = len(videos)
		}
		if err := r.upsertVideosBatch(ctx, videos[:n]); err != nil {
			return err
		}
		videos = videos[n:]
	}
	return nil
}

func (r *Repository) upsertVideosBatch(ctx context.Context, videos []domain.Video) error {
	if len(videos) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO videos
			(id, channel_id, title, description, description_excerpt, published_at, duration,
			 category, thumbnail_url, live_status, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			channel_id = excluded.channel_id,
			title = excluded.title,
			description = CASE WHEN excluded.description <> '' THEN excluded.description ELSE videos.description END,
			description_excerpt = excluded.description_excerpt,
			published_at = excluded.published_at,
			duration = excluded.duration,
			category = excluded.category,
			thumbnail_url = CASE WHEN excluded.thumbnail_url <> '' THEN excluded.thumbnail_url ELSE videos.thumbnail_url END,
			live_status = excluded.live_status,
			last_seen_at = excluded.last_seen_at`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, v := range videos {
		if _, err := stmt.ExecContext(ctx,
			v.ID, v.ChannelID, v.Title, v.Description, v.DescriptionExcerpt,
			fmtTime(v.PublishedAt), int64(v.Duration/time.Second),
			v.Category, v.ThumbnailURL, v.LiveStatus,
			fmtTime(v.FirstSeenAt), fmtTime(v.LastSeenAt),
		); err != nil {
			return fmt.Errorf("upsert video %s: %w", v.ID, err)
		}
	}

	// Sincroniza o FTS5 em massa (uma varredura por batelada). DELETE/INSERT
	// individuais com video_id UNINDEXED varrem o índice FTS inteiro por vídeo,
	// o que é O(n²) com milhares de vídeos.
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(videos)), ",")
	args := make([]any, 0, len(videos))
	for _, v := range videos {
		args = append(args, v.ID)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM videos_fts WHERE video_id IN (`+placeholders+`)`, args...); err != nil {
		return fmt.Errorf("fts delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO videos_fts (video_id, title, description, channel_name)
		SELECT v.id, v.title, COALESCE(NULLIF(v.description, ''), v.description_excerpt), COALESCE(c.title, '')
		FROM videos v LEFT JOIN channels c ON c.id = v.channel_id
		WHERE v.id IN (`+placeholders+`)`, args...); err != nil {
		return fmt.Errorf("fts insert: %w", err)
	}
	return tx.Commit()
}

// MarkProgress upserts the resume state for a video.
func (r *Repository) MarkProgress(ctx context.Context, videoID string, p domain.PlaybackProgress) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO playback_progress (video_id, position_ms, duration_ms, updated_at, completed)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET
			position_ms = excluded.position_ms,
			duration_ms = excluded.duration_ms,
			updated_at = excluded.updated_at,
			completed = excluded.completed`,
		videoID,
		int64(p.Position/time.Millisecond),
		int64(p.Duration/time.Millisecond),
		fmtTime(p.UpdatedAt),
		boolInt(p.Completed),
	)
	if err != nil {
		return fmt.Errorf("mark progress %s: %w", videoID, err)
	}
	return nil
}

// Recent lists videos, optionally filtered by channel or subscribed channels.
func (r *Repository) Recent(ctx context.Context, filter domain.VideoFilter) ([]domain.Video, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultRecentLimit
	}
	if limit > 100 {
		limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	query := `
		SELECT v.id, v.channel_id, v.title, v.description, v.description_excerpt, v.published_at,
		       v.duration, v.category, v.thumbnail_url, v.live_status, v.first_seen_at, v.last_seen_at,
		       COALESCE(c.title, '')
		FROM videos v
		LEFT JOIN channels c ON c.id = v.channel_id`
	args := []any{}
	where := []string{}

	if filter.ChannelID != "" {
		where = append(where, "v.channel_id = ?")
		args = append(args, filter.ChannelID)
	}
	if filter.OnlySubscribed {
		where = append(where, "c.subscribed = 1")
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY v.published_at DESC, v.id LIMIT ? OFFSET ?"
	args = append(args, limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("recent: %w", err)
	}
	defer rows.Close()

	var out []domain.Video
	for rows.Next() {
		var (
			v         domain.Video
			published string
			durationS int64
			firstSeen string
			lastSeen  string
		)
		if err := rows.Scan(
			&v.ID, &v.ChannelID, &v.Title, &v.Description, &v.DescriptionExcerpt, &published,
			&durationS, &v.Category, &v.ThumbnailURL, &v.LiveStatus, &firstSeen, &lastSeen, &v.ChannelTitle,
		); err != nil {
			return nil, fmt.Errorf("recent scan: %w", err)
		}
		v.PublishedAt = parseTime(published)
		v.Duration = time.Duration(durationS) * time.Second
		v.FirstSeenAt = parseTime(firstSeen)
		v.LastSeenAt = parseTime(lastSeen)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recent rows: %w", err)
	}
	return out, nil
}

// Search runs a safe FTS5 MATCH query over videos.
func (r *Repository) Search(ctx context.Context, query string, limit int) ([]domain.SearchHit, error) {
	if limit <= 0 {
		limit = defaultRecentLimit
	}
	if limit > 100 {
		limit = 100
	}
	match := sanitizeFTSPattern(query)
	if match == "" {
		return nil, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT video_id, title, channel_name
		FROM videos_fts
		WHERE videos_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var out []domain.SearchHit
	for rows.Next() {
		var h domain.SearchHit
		if err := rows.Scan(&h.VideoID, &h.Title, &h.ChannelName); err != nil {
			return nil, fmt.Errorf("search scan: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search rows: %w", err)
	}
	return out, nil
}

// ReindexVideos refreshes the FTS5 rows for the given video ids.
func (r *Repository) ReindexVideos(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	del, err := tx.PrepareContext(ctx, `DELETE FROM videos_fts WHERE video_id = ?`)
	if err != nil {
		return fmt.Errorf("prepare delete: %w", err)
	}
	defer del.Close()

	ins, err := tx.PrepareContext(ctx, `
		INSERT INTO videos_fts (video_id, title, description, channel_name)
		SELECT v.id, v.title, COALESCE(NULLIF(v.description, ''), v.description_excerpt), COALESCE(c.title, '')
		FROM videos v LEFT JOIN channels c ON c.id = v.channel_id
		WHERE v.id = ?`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer ins.Close()

	for _, id := range ids {
		if _, err := del.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("delete fts %s: %w", id, err)
		}
		if _, err := ins.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("reindex %s: %w", id, err)
		}
	}
	return tx.Commit()
}

// sanitizeFTSPattern quotes each token so the user query can't inject FTS5
// syntax, and returns "" for empty input.
func sanitizeFTSPattern(query string) string {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		quoted = append(quoted, `"`+strings.ReplaceAll(f, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " AND ")
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
