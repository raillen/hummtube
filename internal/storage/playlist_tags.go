package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// PlaylistTags lista as tags de uma playlist em ordem alfabética (NOCASE).
func (r *Repository) PlaylistTags(ctx context.Context, playlistID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT tag FROM playlist_tags WHERE playlist_id = ? ORDER BY tag COLLATE NOCASE`, playlistID)
	if err != nil {
		return nil, fmt.Errorf("tags: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

// SetPlaylistTags substitui as tags da playlist. Tags vazias são ignoradas;
// duplicatas por caixa são deduplicadas mantendo a primeira.
func (r *Repository) SetPlaylistTags(ctx context.Context, playlistID string, tags []string) error {
	seen := map[string]bool{}
	var clean []string
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		clean = append(clean, t)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM playlist_tags WHERE playlist_id = ?`, playlistID); err != nil {
		return err
	}
	for _, t := range clean {
		if _, err := tx.ExecContext(ctx, `INSERT INTO playlist_tags (playlist_id, tag) VALUES (?, ?)`, playlistID, t); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeduplicatePlaylist remove duplicatas exatas (por PK não deveria haver, mas
// cobre re-imports com race) e recompacta positions 0..n-1. Retorna removidos.
func (r *Repository) DeduplicatePlaylist(ctx context.Context, playlistID string) (int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT video_id, position, added_at FROM playlist_items WHERE playlist_id = ? ORDER BY position`, playlistID)
	if err != nil {
		return 0, err
	}
	type row struct {
		id      string
		pos     int
		addedAt string
	}
	var all []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.id, &rr.pos, &rr.addedAt); err != nil {
			rows.Close()
			return 0, err
		}
		all = append(all, rr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	var keep []row
	dups := 0
	for _, rr := range all {
		if seen[rr.id] {
			dups++
			continue
		}
		seen[rr.id] = true
		keep = append(keep, rr)
	}
	if dups == 0 {
		// Ainda recompacta se houver buracos.
		needsCompact := false
		for i, rr := range keep {
			if rr.pos != i {
				needsCompact = true
				break
			}
		}
		if !needsCompact {
			return 0, nil
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM playlist_items WHERE playlist_id = ?`, playlistID); err != nil {
		return 0, err
	}
	for i, rr := range keep {
		at := rr.addedAt
		if at == "" {
			at = fmtTime(time.Now())
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO playlist_items (playlist_id, video_id, position, added_at) VALUES (?, ?, ?, ?)`, playlistID, rr.id, i, at); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return dups, nil
}
