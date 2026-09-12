// Consultas de leitura usadas pelas telas de biblioteca (Histórico e
// resultados de busca). A UI não emite SQL: docs/03-implementation/STORAGE.md.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const videoColumns = `v.id, v.channel_id, v.title, v.description, v.description_excerpt, v.published_at,
	v.duration, v.category, v.thumbnail_url, v.live_status, v.first_seen_at, v.last_seen_at`

// History lists videos with playback progress, most recently watched first.
// It is the source of "Histórico" and feeds "Continuar assistindo".
func (r *Repository) History(ctx context.Context, limit int) ([]domain.Video, error) {
	return r.HistoryPaged(ctx, limit, 0)
}

// HistoryPaged é History com offset estável para "Carregar mais".
func (r *Repository) HistoryPaged(ctx context.Context, limit, offset int) ([]domain.Video, error) {
	if limit <= 0 {
		limit = defaultRecentLimit
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+videoColumns+`
		FROM videos v
		JOIN playback_progress p ON p.video_id = v.id
		ORDER BY p.updated_at DESC, v.id
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	defer rows.Close()
	return scanVideoRows(rows, "history")
}

// CountVideos returns how many videos the local catalog knows. It is what
// lets a refresh report "N vídeos novos" instead of a paged guess.
func (r *Repository) CountVideos(ctx context.Context) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM videos`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count videos: %w", err)
	}
	return n, nil
}

// VideosByIDs resolves full video metadata for ids, preserving the caller's
// order. Ids without a local row are skipped.
func (r *Repository) VideosByIDs(ctx context.Context, ids []string) ([]domain.Video, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+videoColumns+`
		FROM videos v
		WHERE v.id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("videos by ids: %w", err)
	}
	defer rows.Close()

	found, err := scanVideoRows(rows, "videos by ids")
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.Video, len(found))
	for _, v := range found {
		byID[v.ID] = v
	}
	out := make([]domain.Video, 0, len(ids))
	for _, id := range ids {
		if v, ok := byID[id]; ok {
			out = append(out, v)
		}
	}
	return out, nil
}

// scanVideoRows decodes rows selected with videoColumns.
func scanVideoRows(rows *sql.Rows, what string) ([]domain.Video, error) {
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
			&durationS, &v.Category, &v.ThumbnailURL, &v.LiveStatus, &firstSeen, &lastSeen,
		); err != nil {
			return nil, fmt.Errorf("%s scan: %w", what, err)
		}
		v.PublishedAt = parseTime(published)
		v.Duration = time.Duration(durationS) * time.Second
		v.FirstSeenAt = parseTime(firstSeen)
		v.LastSeenAt = parseTime(lastSeen)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s rows: %w", what, err)
	}
	return out, nil
}
