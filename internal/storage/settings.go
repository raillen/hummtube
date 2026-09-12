// Preferências locais e fila de reprodução, ambas sobre tabelas já criadas
// pela migration 00002 (docs/03-implementation/STORAGE.md).
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// Chaves de preferência conhecidas. Ficam aqui para não virarem string solta
// espalhada pela UI.
const (
	SettingMaxHeight        = "playback.max_height"
	SettingDensity          = "ui.density"
	SettingColorScheme      = "ui.color_scheme"
	SettingRemoteEJS        = "playback.remote_ejs"
	SettingPlayerClient     = "playback.player_client"
	SettingCookiesFrom      = "playback.cookies_browser"
	SettingCookiesFile      = "playback.cookies_file"
	SettingShortcuts        = "ui.shortcuts"
	SettingSpeed            = "playback.speed"
	SettingGain             = "playback.gain_db"
	SettingActiveProfile    = "ui.active_profile"
	SettingRefreshOnStartup = "sync.refresh_on_startup"
	SettingTrayEnabled      = "ui.tray_enabled"
	SettingTrayHideOnClose  = "ui.tray_hide_on_close"
	SettingTrayRefresh      = "sync.tray_background_refresh" // legado; migrado para SettingPeriodicRefresh
	// SettingPeriodicRefresh é o refresh periódico independente da bandeja
	// (v0.6.1). Na migração, sync.tray_background_refresh=1 vira este.
	SettingPeriodicRefresh   = "sync.periodic_refresh"
	SettingTVMode            = "ui.tv_mode"
	SettingPlaybackProvider  = "playback.provider"
	SettingInvidiousInstance = "playback.invidious_instance"
	SettingAudioDevice       = "playback.audio_device"
	SettingAudioNorm         = "playback.audio_normalization"
	SettingAudioChannels     = "playback.audio_channels"
)

// ProfileSetting reads a preference that belongs to one local profile. These
// settings are deliberately separate from the global settings table: a
// browser cookie source is an authentication boundary and must not leak when
// the active profile changes.
func (r *Repository) ProfileSetting(ctx context.Context, profileID, key string) (string, bool, error) {
	profileID = strings.TrimSpace(profileID)
	key = strings.TrimSpace(key)
	if profileID == "" || key == "" {
		return "", false, fmt.Errorf("perfil setting: perfil e chave são obrigatórios")
	}
	var value string
	err := r.db.QueryRowContext(ctx, `
		SELECT value FROM profile_settings WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("profile setting %s/%s: %w", profileID, key, err)
	}
	return value, true, nil
}

// SetProfileSetting persists a preference for one local profile. The foreign
// key is intentionally enforced by SQLite so an unknown profile cannot create
// an orphaned authentication preference.
func (r *Repository) SetProfileSetting(ctx context.Context, profileID, key, value string) error {
	profileID = strings.TrimSpace(profileID)
	key = strings.TrimSpace(key)
	if profileID == "" || key == "" {
		return fmt.Errorf("perfil setting: perfil e chave são obrigatórios")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO profile_settings (profile_id, key, value) VALUES (?, ?, ?)
		ON CONFLICT(profile_id, key) DO UPDATE SET value = excluded.value`,
		profileID, key, value)
	if err != nil {
		return fmt.Errorf("gravar profile setting %s/%s: %w", profileID, key, err)
	}
	return nil
}

// DeleteProfileSetting removes a profile preference. It is a no-op when the
// key is absent, which makes the "forget cookies" action idempotent.
func (r *Repository) DeleteProfileSetting(ctx context.Context, profileID, key string) error {
	profileID = strings.TrimSpace(profileID)
	key = strings.TrimSpace(key)
	if profileID == "" || key == "" {
		return fmt.Errorf("perfil setting: perfil e chave são obrigatórios")
	}
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM profile_settings WHERE profile_id = ? AND key = ?`, profileID, key); err != nil {
		return fmt.Errorf("remover profile setting %s/%s: %w", profileID, key, err)
	}
	return nil
}

// ProfileSettings returns all preferences for one profile. Values are
// internal configuration and callers must keep sensitive keys out of public
// diagnostics or RPC responses.
func (r *Repository) ProfileSettings(ctx context.Context, profileID string) (map[string]string, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, fmt.Errorf("perfil setting: perfil obrigatório")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT key, value FROM profile_settings WHERE profile_id = ?`, profileID)
	if err != nil {
		return nil, fmt.Errorf("profile settings %s: %w", profileID, err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan profile setting: %w", err)
		}
		settings[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("profile settings rows %s: %w", profileID, err)
	}
	return settings, nil
}

// Setting reads a preference. The bool reports whether the key exists.
func (r *Repository) Setting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("setting %s: %w", key, err)
	}
	return value, true, nil
}

// SearchRejections returns the explicit video/channel exclusions used by the
// local search filter. Only rejection actions are included; positive feedback
// remains a ranking signal.
func (r *Repository) SearchRejections(ctx context.Context) (map[string]bool, map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(video_id, ''), COALESCE(channel_id, '')
		FROM recommendation_feedback
		WHERE action IN (?, ?)`, string(domain.FeedbackDontRecommend), string(domain.FeedbackIgnoreChannel))
	if err != nil {
		return nil, nil, fmt.Errorf("search rejections: %w", err)
	}
	defer rows.Close()
	videos, channels := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var videoID, channelID string
		if err := rows.Scan(&videoID, &channelID); err != nil {
			return nil, nil, fmt.Errorf("search rejections scan: %w", err)
		}
		if videoID != "" {
			videos[videoID] = true
		}
		if channelID != "" {
			channels[channelID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("search rejections rows: %w", err)
	}
	return videos, channels, nil
}

// SetSetting persists a preference, overwriting any previous value.
func (r *Repository) SetSetting(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("gravar setting %s: %w", key, err)
	}
	return nil
}

// ------------------------------------------------------------------- fila

// Enqueue adiciona um vídeo ao fim da fila. Reenfileirar um vídeo já presente
// não duplica: a fila é um conjunto ordenado, não um histórico.
func (r *Repository) Enqueue(ctx context.Context, videoID string) error {
	if videoID == "" {
		return errors.New("fila: video id vazio")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM queue_items WHERE video_id = ?`, videoID).Scan(&exists); err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	if exists > 0 {
		return tx.Commit()
	}

	var next sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT MAX(order_index) FROM queue_items`).Scan(&next); err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	order := int64(0)
	if next.Valid {
		order = next.Int64 + 1
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO queue_items (video_id, position_ms, order_index, opened_at)
		VALUES (?, 0, ?, ?)`, videoID, order, fmtTime(time.Now())); err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	return tx.Commit()
}

// Dequeue remove um vídeo da fila.
func (r *Repository) Dequeue(ctx context.Context, videoID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM queue_items WHERE video_id = ?`, videoID); err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	return nil
}

// ClearQueue remove todos os vídeos da fila.
func (r *Repository) ClearQueue(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM queue_items`); err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	return nil
}

// EnqueuePlaylist adiciona todos os vídeos de uma playlist ao fim da fila, na
// ordem da playlist, ignorando os que já estão na fila (conjunto ordenado).
func (r *Repository) EnqueuePlaylist(ctx context.Context, playlistID string) error {
	if playlistID == "" {
		return errors.New("fila: playlist id vazio")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT i.video_id FROM playlist_items i
		WHERE i.playlist_id = ? ORDER BY i.position`, playlistID)
	if err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("fila: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("fila: %w", err)
	}

	existingRows, err := tx.QueryContext(ctx, `SELECT video_id FROM queue_items`)
	if err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	queued := make(map[string]bool)
	for existingRows.Next() {
		var vid string
		if err := existingRows.Scan(&vid); err == nil {
			queued[vid] = true
		}
	}
	existingRows.Close()

	var next sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(order_index) FROM queue_items`).Scan(&next); err != nil {
		return fmt.Errorf("fila: %w", err)
	}
	nextOrder := int64(0)
	if next.Valid {
		nextOrder = next.Int64 + 1
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO queue_items (video_id, position_ms, order_index, opened_at)
		VALUES (?, 0, ?, ?)`)
	if err != nil {
		return fmt.Errorf("fila: preparar insercao: %w", err)
	}
	defer stmt.Close()

	now := fmtTime(time.Now())
	for _, id := range ids {
		if queued[id] {
			continue
		}
		if _, err := stmt.ExecContext(ctx, id, nextOrder, now); err != nil {
			return fmt.Errorf("fila: %w", err)
		}
		queued[id] = true
		nextOrder++
	}
	return tx.Commit()
}

// QueuedVideos lista a fila na ordem em que foi montada, já com a metadata dos
// vídeos. Itens cujo vídeo sumiu do catálogo são ignorados.
func (r *Repository) QueuedVideos(ctx context.Context) ([]domain.Video, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+videoColumns+`
		FROM queue_items q
		JOIN videos v ON v.id = q.video_id
		ORDER BY q.order_index, q.id`)
	if err != nil {
		return nil, fmt.Errorf("fila: %w", err)
	}
	defer rows.Close()
	return scanVideoRows(rows, "fila")
}

// QueuedIDs lista só os ids, na ordem — é o que os atalhos de navegação
// (Ctrl+Tab / Ctrl+W) precisam saber.
func (r *Repository) QueuedIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT video_id FROM queue_items ORDER BY order_index, id`)
	if err != nil {
		return nil, fmt.Errorf("fila: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("fila scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fila rows: %w", err)
	}
	return ids, nil
}

// ------------------------------------------------- feedback de recomendação

// RecordFeedback guarda uma opinião explícita do usuário sobre um vídeo, canal
// ou tema. `BuildProfile` já lê esta tabela para excluir o que foi rejeitado;
// ações de tema também ajustam `interest_topics` na mesma transação, para o
// efeito existir de verdade no ranking (docs/03-implementation/RECOMMENDATIONS.md).
func (r *Repository) RecordFeedback(ctx context.Context, fb domain.RecommendationFeedback) error {
	if fb.Action == "" {
		return errors.New("feedback: ação vazia")
	}
	delta := 0
	switch fb.Action {
	case domain.FeedbackMoreTopic:
		delta = 1
	case domain.FeedbackLessTopic:
		delta = -1
	}
	if delta != 0 && fb.Topic == "" {
		return errors.New("feedback: tema vazio")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO recommendation_feedback (video_id, channel_id, topic, action, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		nullable(fb.VideoID), nullable(fb.ChannelID), nullable(fb.Topic),
		string(fb.Action), fmtTime(time.Now())); err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	if delta != 0 {
		if err := applyTopicScoreTx(ctx, tx, fb.Topic, delta); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	return nil
}

// nullable mantém colunas opcionais como NULL em vez de string vazia, que é o
// que as consultas de exclusão esperam.
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// AllSettings recupera todas as chaves e valores da tabela settings.
func (r *Repository) AllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("all settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		settings[k] = v
	}
	return settings, nil
}
