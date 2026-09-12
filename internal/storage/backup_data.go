// Importação de dados pessoais e regras para o contrato .ntbackup
// (M6 BCK-01/02, PLY-06). A estratégia é replace: restaurar um backup volta o
// estado para o momento do backup — merge fino fica no import JSON de
// playlists (PLY-06), que tem UI própria.
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// ImportPersonalData substitui os dados pessoais locais pelo conteúdo do
// backup, em uma transação. Histórico sem metadata de vídeo continua válido:
// playback_progress não referencia videos.
func (r *Repository) ImportPersonalData(ctx context.Context, data domain.PersonalData) error {
	if data.Format != "" && data.Format != "nanotube-personal-data" {
		return fmt.Errorf("importar dados: formato inesperado %q", data.Format)
	}
	if data.Version > 1 {
		return fmt.Errorf("importar dados: versão futura %d", data.Version)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("importar dados: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []string{
		`DELETE FROM playback_progress`,
		`DELETE FROM favorites`,
		`DELETE FROM playlist_items`,
		`DELETE FROM playlist_rules`,
		`DELETE FROM playlists`,
		`DELETE FROM channel_folder_items`,
		`DELETE FROM channel_folders`,
		`DELETE FROM channel_favorites`,
		`DELETE FROM recommendation_feedback`,
		`DELETE FROM interest_topics`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("importar dados: %w", err)
		}
	}

	for _, e := range data.History {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO playback_progress (video_id, position_ms, duration_ms, updated_at, completed)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(video_id) DO UPDATE SET position_ms = excluded.position_ms,
				duration_ms = excluded.duration_ms, updated_at = excluded.updated_at,
				completed = excluded.completed`,
			e.VideoID, e.Position, e.Duration,
			fmtTime(e.UpdatedAt), boolInt(e.Completed)); err != nil {
			return fmt.Errorf("importar histórico: %w", err)
		}
	}
	for _, id := range data.Favorites {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO favorites (video_id, created_at) VALUES (?, ?)`,
			id, fmtTime(time.Now())); err != nil {
			return fmt.Errorf("importar favoritos: %w", err)
		}
	}
	for _, p := range data.Playlists {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO playlists (id, name, description, color, created_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			p.ID, p.Name, p.Description, p.Color, fmtTime(p.CreatedAt)); err != nil {
			return fmt.Errorf("importar playlists: %w", err)
		}
		for pos, videoID := range p.VideoIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO playlist_items (playlist_id, video_id, position, added_at)
				VALUES (?, ?, ?, ?)`,
				p.ID, videoID, pos, fmtTime(time.Now())); err != nil {
				return fmt.Errorf("importar itens da playlist: %w", err)
			}
		}
	}
	for _, id := range data.ChannelFavorites {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO channel_favorites (channel_id, created_at) VALUES (?, ?)`,
			id, fmtTime(time.Now())); err != nil {
			return fmt.Errorf("importar canais favoritos: %w", err)
		}
	}
	for _, f := range data.Folders {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO channel_folders (id, name, created_at) VALUES (?, ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			f.ID, f.Name, fmtTime(f.CreatedAt)); err != nil {
			return fmt.Errorf("importar pastas: %w", err)
		}
		for _, channelID := range f.ChannelIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO channel_folder_items (folder_id, channel_id) VALUES (?, ?)`,
				f.ID, channelID); err != nil {
				return fmt.Errorf("importar itens da pasta: %w", err)
			}
		}
	}
	for _, fb := range data.Feedback {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO recommendation_feedback (video_id, channel_id, topic, action, created_at)
			VALUES (?, ?, ?, ?, ?)`,
			nullable(fb.VideoID), nullable(fb.ChannelID), nullable(fb.Topic),
			string(fb.Action), fmtTime(data.ExportedAt)); err != nil {
			return fmt.Errorf("importar feedback: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("importar dados: %w", err)
	}
	return nil
}

// ExportSmartRules monta a seção "rules" do backup: regras por playlist e
// presets, com o rule_json original preservado.
func (r *Repository) ExportSmartRules(ctx context.Context) (domain.SmartRulesBackup, error) {
	out := domain.SmartRulesBackup{Format: "nanotube-smart-rules", Version: 1}

	rows, err := r.db.QueryContext(ctx, `
		SELECT playlist_id, version, rule_json, updated_at FROM playlist_rules ORDER BY playlist_id`)
	if err != nil {
		return out, fmt.Errorf("exportar regras: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row domain.SmartRuleRow
		var updated string
		if err := rows.Scan(&row.PlaylistID, &row.Version, &row.Rule, &updated); err != nil {
			return out, fmt.Errorf("exportar regras scan: %w", err)
		}
		row.UpdatedAt = parseTime(updated)
		out.Rules = append(out.Rules, row)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("exportar regras rows: %w", err)
	}

	presets, err := r.db.QueryContext(ctx, `
		SELECT id, name, version, rule_json, created_at, updated_at
		FROM smart_presets ORDER BY created_at, id`)
	if err != nil {
		return out, fmt.Errorf("exportar presets: %w", err)
	}
	defer presets.Close()
	for presets.Next() {
		var p domain.SmartPresetRow
		var created, updated string
		if err := presets.Scan(&p.ID, &p.Name, &p.Version, &p.Rule, &created, &updated); err != nil {
			return out, fmt.Errorf("exportar presets scan: %w", err)
		}
		p.CreatedAt, p.UpdatedAt = parseTime(created), parseTime(updated)
		out.Presets = append(out.Presets, p)
	}
	return out, presets.Err()
}

// ImportSmartRules substitui regras e presets pelos do backup. Regras cuja
// playlist não existe mais são descartadas (FK cascade já garantiu isso no
// export; aqui é defesa contra backup editado).
func (r *Repository) ImportSmartRules(ctx context.Context, backup domain.SmartRulesBackup) error {
	if backup.Format != "" && backup.Format != "nanotube-smart-rules" {
		return fmt.Errorf("importar regras: formato inesperado %q", backup.Format)
	}
	if backup.Version > 1 {
		return fmt.Errorf("importar regras: versão futura %d", backup.Version)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("importar regras: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM playlist_rules`); err != nil {
		return fmt.Errorf("importar regras: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM smart_presets`); err != nil {
		return fmt.Errorf("importar regras: %w", err)
	}
	for _, row := range backup.Rules {
		// Valida o conteúdo antes de gravar; versão futura aborta o restore.
		if _, err := domain.UpgradeSmartRule([]byte(row.Rule), row.Version); err != nil {
			return fmt.Errorf("importar regras: %w", err)
		}
		var n int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM playlists WHERE id = ?`, row.PlaylistID).Scan(&n); err != nil {
			return fmt.Errorf("importar regras: %w", err)
		}
		if n == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO playlist_rules (playlist_id, version, rule_json, updated_at)
			VALUES (?, ?, ?, ?)`,
			row.PlaylistID, row.Version, row.Rule, fmtTime(row.UpdatedAt)); err != nil {
			return fmt.Errorf("importar regras: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE playlists SET is_smart = 1 WHERE id = ? AND is_smart = 0`, row.PlaylistID); err != nil {
			return fmt.Errorf("importar regras: %w", err)
		}
	}
	for _, p := range backup.Presets {
		if _, err := domain.UpgradeSmartRule([]byte(p.Rule), p.Version); err != nil {
			return fmt.Errorf("importar regras: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO smart_presets (id, name, version, rule_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			p.ID, p.Name, p.Version, p.Rule, fmtTime(p.CreatedAt), fmtTime(p.UpdatedAt)); err != nil {
			return fmt.Errorf("importar presets: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("importar regras: %w", err)
	}
	return nil
}
