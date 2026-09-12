package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func presetID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "pr_" + hex.EncodeToString(buf), nil
}

func (r *Repository) SaveSmartRule(ctx context.Context, playlistID string, rule domain.SmartRule) error {
	rule.Version = domain.CurrentSmartRuleVersion
	if err := rule.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := fmtTime(time.Now())
	_, err = tx.ExecContext(ctx, `
		INSERT INTO playlist_rules (playlist_id, version, rule_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(playlist_id) DO UPDATE SET version=excluded.version, rule_json=excluded.rule_json, updated_at=excluded.updated_at`,
		playlistID, rule.Version, string(data), now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE playlists SET is_smart = 1, updated_at = ? WHERE id = ?`, now, playlistID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) GetSmartRule(ctx context.Context, playlistID string) (domain.SmartRule, error) {
	var version int
	var raw string
	err := r.db.QueryRowContext(ctx, `SELECT version, rule_json FROM playlist_rules WHERE playlist_id = ?`, playlistID).Scan(&version, &raw)
	if err != nil {
		return domain.SmartRule{}, err
	}
	// O modelo versionado (D-064): a coluna espelha a versão do JSON e o app
	// rejeita o que não sabe ler — nunca interpreta regra futura como atual.
	if version > domain.CurrentSmartRuleVersion {
		return domain.SmartRule{}, fmt.Errorf("regra versão futura %d (suportado até %d)", version, domain.CurrentSmartRuleVersion)
	}
	rule, err := domain.UpgradeSmartRule([]byte(raw), version)
	if err != nil {
		return domain.SmartRule{}, fmt.Errorf("regra inválida: %w", err)
	}
	return rule, nil
}

func (r *Repository) IsSmartPlaylist(ctx context.Context, playlistID string) (bool, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM playlist_rules WHERE playlist_id = ?`, playlistID).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// SmartPlaylistVideos avalia a regra sobre o catálogo local (sem playlist_items).
func (r *Repository) SmartPlaylistVideos(ctx context.Context, rule domain.SmartRule) ([]domain.Video, error) {
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	query, args := smartVideosQuery(rule)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("smart: %w", err)
	}
	defer rows.Close()
	return scanVideoRows(rows, "smart")
}

func smartVideosQuery(rule domain.SmartRule) (string, []any) {
	conds := []string{"1=1"}
	var args []any
	joins := ""

	if rule.SubscribedOnly {
		joins += " JOIN channels c ON c.id = v.channel_id AND c.subscribed = 1"
	}
	if rule.Term != "" {
		conds = append(conds, `(v.title LIKE ? ESCAPE '\' COLLATE NOCASE OR v.description_excerpt LIKE ? ESCAPE '\' COLLATE NOCASE)`)
		like := escapeLikePattern(rule.Term)
		args = append(args, like, like)
	}
	f := rule.Filter
	if f.Channel != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM channels ch WHERE ch.id = v.channel_id AND ch.title LIKE ? ESCAPE '\' COLLATE NOCASE)`)
		args = append(args, escapeLikePattern(f.Channel))
	}
	switch f.Duration {
	case domain.SearchDurationShort:
		conds = append(conds, `v.duration > 0 AND v.duration < 4*60`)
	case domain.SearchDurationMedium:
		conds = append(conds, `v.duration >= 4*60 AND v.duration <= 20*60`)
	case domain.SearchDurationLong:
		conds = append(conds, `v.duration > 20*60`)
	case domain.SearchDurationCustom:
		if f.MinDuration > 0 {
			conds = append(conds, `v.duration >= ?`)
			args = append(args, int64(f.MinDuration/time.Second))
		}
		if f.MaxDuration > 0 {
			conds = append(conds, `v.duration <= ?`)
			args = append(args, int64(f.MaxDuration/time.Second))
		}
	}
	switch f.Age {
	case domain.FeedAgeToday:
		conds = append(conds, `v.published_at >= ?`)
		args = append(args, fmtTime(time.Now().Add(-24*time.Hour)))
	case domain.FeedAgeWeek:
		conds = append(conds, `v.published_at >= ?`)
		args = append(args, fmtTime(time.Now().Add(-7*24*time.Hour)))
	case domain.FeedAgeMonth:
		conds = append(conds, `v.published_at >= ?`)
		args = append(args, fmtTime(time.Now().Add(-30*24*time.Hour)))
	case domain.FeedAgeYear:
		conds = append(conds, `v.published_at >= ?`)
		args = append(args, fmtTime(time.Now().Add(-365*24*time.Hour)))
	}
	if cat := strings.TrimSpace(f.Category); cat != "" {
		conds = append(conds, `v.category = ? COLLATE NOCASE`)
		args = append(args, cat)
	}
	switch f.Watched {
	case domain.SearchWatchWatched:
		joins += " LEFT JOIN playback_progress pp ON pp.video_id = v.id"
		conds = append(conds, `(pp.completed = 1 OR pp.position_ms >= 15000)`)
	case domain.SearchWatchUnwatched:
		joins += " LEFT JOIN playback_progress pp ON pp.video_id = v.id"
		conds = append(conds, `(pp.video_id IS NULL OR (pp.completed = 0 AND pp.position_ms < 15000))`)
	case domain.SearchWatchContinue:
		joins += " LEFT JOIN playback_progress pp ON pp.video_id = v.id"
		conds = append(conds, `pp.video_id IS NOT NULL AND pp.completed = 0 AND pp.position_ms > 0 AND pp.duration_ms > 0 AND (pp.duration_ms - pp.position_ms) > 30000`)
	}
	if f.Favorite {
		conds = append(conds, `EXISTS (SELECT 1 FROM favorites fav WHERE fav.video_id = v.id)`)
	}
	switch f.Content {
	case domain.FeedRegular:
		conds = append(conds, `NOT (`+liveKindCondition("v.live_status")+`)`)
	case domain.FeedLive:
		conds = append(conds, liveKindCondition("v.live_status"))
	}
	if f.ShortsOnly {
		conds = append(conds, `v.duration > 0 AND v.duration <= 3*60`)
	}
	orderBy := "v.published_at DESC, v.id"
	switch rule.Sort {
	case domain.PlaylistSortTitle:
		orderBy = "v.title COLLATE NOCASE, v.id"
	case domain.PlaylistSortChannel:
		if !strings.Contains(joins, "JOIN channels c ") {
			joins += " LEFT JOIN channels c2 ON c2.id = v.channel_id"
			orderBy = "c2.title COLLATE NOCASE, v.id"
		} else {
			orderBy = "c.title COLLATE NOCASE, v.id"
		}
	case domain.PlaylistSortDuration:
		orderBy = "v.duration, v.id"
	case domain.PlaylistSortAdded:
		orderBy = "v.first_seen_at DESC, v.id"
	case domain.PlaylistSortPublished:
		orderBy = "v.published_at DESC, v.id"
	}
	query := `SELECT ` + videoColumns + ` FROM videos v ` + joins + ` WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY ` + orderBy + ` LIMIT ?`
	args = append(args, domain.PlaylistFilterLimit)
	return query, args
}

// Presets
func (r *Repository) SavePreset(ctx context.Context, id, name string, rule domain.SmartRule) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("nome vazio")
	}
	rule.Version = domain.CurrentSmartRuleVersion
	if err := rule.Validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(rule)
	if err != nil {
		return "", fmt.Errorf("serializar smart rule: %w", err)
	}
	now := fmtTime(time.Now())
	if id == "" {
		var err error
		id, err = presetID()
		if err != nil {
			return "", err
		}
		_, err = r.db.ExecContext(ctx, `INSERT INTO smart_presets (id, name, rule_json, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			id, strings.TrimSpace(name), string(data), rule.Version, now, now)
		return id, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO smart_presets (id, name, rule_json, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, rule_json=excluded.rule_json, updated_at=excluded.updated_at`,
		id, strings.TrimSpace(name), string(data), rule.Version, now, now)
	return id, err
}

func (r *Repository) RenamePreset(ctx context.Context, id, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("nome vazio")
	}
	res, err := r.db.ExecContext(ctx, `UPDATE smart_presets SET name = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(name), fmtTime(time.Now()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("preset não encontrado")
	}
	return nil
}

func (r *Repository) DeletePreset(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM smart_presets WHERE id = ?`, id)
	return err
}

func (r *Repository) ListPresets(ctx context.Context) ([]struct {
	ID   string
	Name string
	Rule domain.SmartRule
}, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, rule_json FROM smart_presets ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID   string
		Name string
		Rule domain.SmartRule
	}
	for rows.Next() {
		var id, name, raw string
		if err := rows.Scan(&id, &name, &raw); err != nil {
			return nil, err
		}
		rule, err := domain.UnmarshalSmartRule([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf("deserializar preset %s: %w", id, err)
		}
		out = append(out, struct {
			ID   string
			Name string
			Rule domain.SmartRule
		}{ID: id, Name: name, Rule: rule})
	}
	return out, rows.Err()
}
