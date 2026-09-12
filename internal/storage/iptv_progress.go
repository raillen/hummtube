// Progresso de reprodução IPTV: retomada local para VOD sem persistir URL
// de stream. Documento canônico: docs/03-implementation/NANOIPTV.md.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/nanotube/nanotube-web/internal/iptv"
)

// GetIPTVProgress returns the saved position for one item, or ok=false when
// the item has no progress yet.
func (r *Repository) GetIPTVProgress(ctx context.Context, itemID string) (iptv.PlaybackPosition, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT item_id, position_ms, duration_ms, completed, updated_at
		FROM iptv_playback_progress
		WHERE item_id = ?`, itemID)
	var (
		pos           iptv.PlaybackPosition
		positionMS    int64
		durationMS    int64
		completed     int
		updatedAtText string
	)
	if err := row.Scan(&pos.ItemID, &positionMS, &durationMS, &completed, &updatedAtText); err != nil {
		if err == sql.ErrNoRows {
			return iptv.PlaybackPosition{}, false, nil
		}
		return iptv.PlaybackPosition{}, false, fmt.Errorf("get iptv progress %s: %w", itemID, err)
	}
	pos.Position = time.Duration(positionMS) * time.Millisecond
	pos.Duration = time.Duration(durationMS) * time.Millisecond
	pos.Completed = completed != 0
	pos.UpdatedAt = parseTime(updatedAtText)
	return pos, true, nil
}

// SetIPTVProgress upserts the local playback position for one VOD item.
func (r *Repository) SetIPTVProgress(ctx context.Context, pos iptv.PlaybackPosition) error {
	if pos.ItemID == "" {
		return fmt.Errorf("set iptv progress: item_id vazio")
	}
	if pos.UpdatedAt.IsZero() {
		pos.UpdatedAt = time.Now()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO iptv_playback_progress (item_id, position_ms, duration_ms, completed, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(item_id) DO UPDATE SET
			position_ms = excluded.position_ms,
			duration_ms = excluded.duration_ms,
			completed = excluded.completed,
			updated_at = excluded.updated_at`,
		pos.ItemID,
		pos.Position.Milliseconds(),
		pos.Duration.Milliseconds(),
		boolInt(pos.Completed),
		fmtTime(pos.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("set iptv progress %s: %w", pos.ItemID, err)
	}
	return nil
}

// DeleteIPTVProgress removes the saved position (resetar progresso).
func (r *Repository) DeleteIPTVProgress(ctx context.Context, itemID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM iptv_playback_progress WHERE item_id = ?`, itemID)
	if err != nil {
		return fmt.Errorf("delete iptv progress %s: %w", itemID, err)
	}
	return nil
}

// ListIPTVResumeItems devolve os VODs em andamento mais recentes, com o item
// do catálogo embutido, para a linha "Continuar assistindo" da Home.
func (r *Repository) ListIPTVResumeItems(ctx context.Context, limit int) ([]iptv.ResumeEntry, error) {
	if limit <= 0 || limit > maxIPTVItemLimit {
		limit = 24
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, i.source_id, s.name, i.kind, i.classification, i.title,
		       i.raw_title, i.group_name, i.logo_url, i.epg_id, i.channel_number,
		       i.language, i.country, i.season, i.episode, i.has_season,
		       i.has_episode,
		       p.position_ms, p.duration_ms, p.updated_at
		FROM iptv_playback_progress p
		JOIN iptv_items i ON i.id = p.item_id
		JOIN iptv_sources s ON s.id = i.source_id
		WHERE p.completed = 0 AND p.duration_ms > 0 AND p.position_ms > 0
		ORDER BY p.updated_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("listar continuar assistindo: %w", err)
	}
	defer rows.Close()

	out := make([]iptv.ResumeEntry, 0)
	for rows.Next() {
		var (
			entry                                  iptv.ResumeEntry
			kindValue, classificationValue         string
			season, episode, hasSeason, hasEpisode int
			positionMS, durationMS                 int64
			updatedAtText                          string
		)
		if err := rows.Scan(
			&entry.Item.ID, &entry.Item.SourceID, &entry.Item.SourceName,
			&kindValue, &classificationValue,
			&entry.Item.Title, &entry.Item.RawTitle, &entry.Item.Group, &entry.Item.LogoURL,
			&entry.Item.EPGID, &entry.Item.ChannelNumber,
			&entry.Item.Language, &entry.Item.Country, &season, &episode,
			&hasSeason, &hasEpisode,
			&positionMS, &durationMS, &updatedAtText,
		); err != nil {
			return nil, fmt.Errorf("listar continuar assistindo scan: %w", err)
		}
		entry.Item.Kind = iptv.ContentKind(kindValue)
		entry.Item.Classification = iptv.ClassificationSource(classificationValue)
		entry.Item.Episode = iptv.EpisodeRef{
			Season: season, Episode: episode,
			HasSeason: hasSeason != 0, HasEpisode: hasEpisode != 0,
		}
		entry.Position = time.Duration(positionMS) * time.Millisecond
		entry.Duration = time.Duration(durationMS) * time.Millisecond
		entry.UpdatedAt = parseTime(updatedAtText)
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listar continuar assistindo rows: %w", err)
	}
	return out, nil
}
