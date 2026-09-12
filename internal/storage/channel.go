package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// UpsertChannels inserts or updates channel metadata in a transaction.
func (r *Repository) UpsertChannels(ctx context.Context, channels []domain.Channel) error {
	if len(channels) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO channels (id, title, subscribed, uploads_playlist_id, last_sync_at, last_known_video_id, last_error, thumbnail_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			subscribed = excluded.subscribed,
			uploads_playlist_id = CASE
				WHEN excluded.uploads_playlist_id != '' THEN excluded.uploads_playlist_id
				ELSE channels.uploads_playlist_id
			END,
			thumbnail_url = CASE WHEN excluded.thumbnail_url != '' THEN excluded.thumbnail_url ELSE channels.thumbnail_url END`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, ch := range channels {
		if _, err := stmt.ExecContext(ctx,
			ch.ID, ch.Title, boolInt(ch.Subscribed), ch.UploadsPlaylistID,
			fmtTime(ch.LastSyncAt), ch.LastKnownVideoID, ch.LastError, ch.ThumbnailURL,
		); err != nil {
			return fmt.Errorf("upsert channel %s: %w", ch.ID, err)
		}
	}
	return tx.Commit()
}

const channelCols = `id, title, subscribed, uploads_playlist_id, last_sync_at, last_known_video_id, last_error, thumbnail_url`

func scanChannels(rows *sql.Rows) ([]domain.Channel, error) {
	var out []domain.Channel
	for rows.Next() {
		var (
			ch       domain.Channel
			sub      int
			lastSync string
		)
		if err := rows.Scan(
			&ch.ID, &ch.Title, &sub, &ch.UploadsPlaylistID,
			&lastSync, &ch.LastKnownVideoID, &ch.LastError, &ch.ThumbnailURL,
		); err != nil {
			return nil, err
		}
		ch.Subscribed = sub != 0
		ch.LastSyncAt = parseTime(lastSync)
		out = append(out, ch)
	}
	return out, rows.Err()
}

// ChannelsByIDs returns channels matching the given ids (input order).
func (r *Repository) ChannelsByIDs(ctx context.Context, ids []string) ([]domain.Channel, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT ` + channelCols + ` FROM channels WHERE id IN (` +
		strings.Join(placeholders, ",") + `)`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("channels by ids: %w", err)
	}
	defer rows.Close()

	byID := make(map[string]domain.Channel, len(ids))
	got, err := scanChannels(rows)
	if err != nil {
		return nil, fmt.Errorf("channels by ids scan: %w", err)
	}
	for _, ch := range got {
		byID[ch.ID] = ch
	}
	out := make([]domain.Channel, 0, len(ids))
	for _, id := range ids {
		if ch, ok := byID[id]; ok {
			out = append(out, ch)
		}
	}
	return out, nil
}

// SubscribedChannels lists channels flagged as subscribed.
func (r *Repository) SubscribedChannels(ctx context.Context) ([]domain.Channel, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+channelCols+" FROM channels WHERE subscribed = 1 ORDER BY title")
	if err != nil {
		return nil, fmt.Errorf("subscribed channels: %w", err)
	}
	defer rows.Close()
	got, err := scanChannels(rows)
	if err != nil {
		return nil, fmt.Errorf("subscribed channels scan: %w", err)
	}
	return got, nil
}

// SetChannelSync updates the sync markers for a channel.
func (r *Repository) SetChannelSync(ctx context.Context, channelID, lastKnownVideoID, lastError string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE channels
		SET last_sync_at = ?, last_known_video_id = ?, last_error = ?
		WHERE id = ?`,
		fmtTime(time.Now()), lastKnownVideoID, lastError, channelID)
	if err != nil {
		return fmt.Errorf("set channel sync %s: %w", channelID, err)
	}
	return nil
}

// SubscribeChannel sets subscribed = 1 for a channel.
func (r *Repository) SubscribeChannel(ctx context.Context, id, title string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("subscribe channel: id vazio")
	}
	if strings.TrimSpace(title) == "" {
		title = id
	}
	now := fmtTime(time.Now())
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO channels (id, title, subscribed, uploads_playlist_id, last_sync_at, last_known_video_id, last_error, subscribed_at)
		VALUES (?, ?, 1, '', ?, '', '', ?)
		ON CONFLICT(id) DO UPDATE SET
			subscribed = 1,
			subscribed_at = CASE WHEN channels.subscribed = 0 OR channels.subscribed_at = '' THEN excluded.subscribed_at ELSE channels.subscribed_at END,
			title = CASE WHEN excluded.title != '' THEN excluded.title ELSE channels.title END`,
		id, title, now, now)
	if err != nil {
		return fmt.Errorf("subscribe channel %s: %w", id, err)
	}
	return nil
}

// UnsubscribeChannel sets subscribed = 0 for a channel.
func (r *Repository) UnsubscribeChannel(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("unsubscribe channel: id vazio")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE channels SET subscribed = 0 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("unsubscribe channel %s: %w", id, err)
	}
	return nil
}
