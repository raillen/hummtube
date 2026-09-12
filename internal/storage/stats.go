// Estatísticas locais e dados pessoais (M9/QOL-04): contadores calculados sob
// demanda, exportação em JSON versionado e limpeza de dados pessoais. Tudo é
// estado local; nada sai da máquina além do que o usuário exporta.
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// LocalStats calcula os contadores de uso a partir do estado local, sob
// demanda (não há cache: a página de configurações chama quando é exibida).
func (r *Repository) LocalStats(ctx context.Context) (domain.LocalStats, error) {
	var s domain.LocalStats
	var watchMS int64

	// Uma única consulta com subselects: barato, sem N+1 e fácil de auditar.
	err := r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM playback_progress WHERE completed = 1),
			(SELECT COALESCE(SUM(
			    CASE WHEN completed = 1 THEN duration_ms ELSE position_ms END), 0)
			 FROM playback_progress),
			(SELECT COUNT(*) FROM playback_progress),
			(SELECT COUNT(*) FROM favorites),
			(SELECT COUNT(*) FROM playlists),
			(SELECT COUNT(*) FROM playlist_items),
			(SELECT COUNT(*) FROM channels WHERE subscribed = 1),
			(SELECT COUNT(*) FROM channel_favorites),
			(SELECT COUNT(*) FROM channel_folders),
			(SELECT COUNT(*) FROM recommendation_feedback)`).Scan(
		&s.VideosWatched, &watchMS, &s.HistoryCount,
		&s.FavoritesCount, &s.PlaylistsCount, &s.PlaylistItems,
		&s.SubscriptionsCount, &s.ChannelFavoritesCount, &s.FoldersCount,
		&s.FeedbackCount)
	if err != nil {
		return domain.LocalStats{}, fmt.Errorf("estatísticas: %w", err)
	}
	s.TotalWatchTime = time.Duration(watchMS) * time.Millisecond
	return s, nil
}

// ExportPersonalData monta o dump de dados pessoais. A gravação em disco fica
// com quem chamou (a UI escolhe o arquivo); aqui só existe a forma dos dados.
func (r *Repository) ExportPersonalData(ctx context.Context) (domain.PersonalData, error) {
	out := domain.PersonalData{
		Format:     "nanotube-personal-data",
		Version:    1,
		ExportedAt: time.Now(),
	}

	history, err := r.historyExport(ctx)
	if err != nil {
		return out, err
	}
	out.History = history

	favorites, err := r.favoritesExport(ctx)
	if err != nil {
		return out, err
	}
	out.Favorites = favorites

	playlists, err := r.playlistsExport(ctx)
	if err != nil {
		return out, err
	}
	out.Playlists = playlists

	channelFavorites, err := r.channelFavoritesExport(ctx)
	if err != nil {
		return out, err
	}
	out.ChannelFavorites = channelFavorites

	folders, err := r.foldersExport(ctx)
	if err != nil {
		return out, err
	}
	out.Folders = folders

	feedback, err := r.feedbackExport(ctx)
	if err != nil {
		return out, err
	}
	out.Feedback = feedback

	return out, nil
}

// historyExport devolve o histórico de reprodução com a metadata do vídeo.
func (r *Repository) historyExport(ctx context.Context) ([]domain.HistoryEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pp.video_id, COALESCE(v.title, ''), COALESCE(v.channel_id, ''),
		       pp.position_ms, pp.duration_ms, pp.completed, pp.updated_at
		FROM playback_progress pp
		LEFT JOIN videos v ON v.id = pp.video_id
		ORDER BY pp.updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("exportar histórico: %w", err)
	}
	defer rows.Close()

	var out []domain.HistoryEntry
	for rows.Next() {
		var e domain.HistoryEntry
		var completed int
		var updated string
		if err := rows.Scan(&e.VideoID, &e.Title, &e.ChannelID,
			&e.Position, &e.Duration, &completed, &updated); err != nil {
			return nil, fmt.Errorf("exportar histórico scan: %w", err)
		}
		e.Completed = completed == 1
		e.UpdatedAt = parseTime(updated)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("exportar histórico rows: %w", err)
	}
	return out, nil
}

func (r *Repository) favoritesExport(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT video_id FROM favorites ORDER BY video_id`)
	if err != nil {
		return nil, fmt.Errorf("exportar favoritos: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("exportar favoritos scan: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("exportar favoritos rows: %w", err)
	}
	return out, nil
}

func (r *Repository) playlistsExport(ctx context.Context) ([]domain.PlaylistExport, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, color, created_at
		FROM playlists ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("exportar playlists: %w", err)
	}
	defer rows.Close()

	var out []domain.PlaylistExport
	indexMap := make(map[string]int)
	for rows.Next() {
		var p domain.PlaylistExport
		var created string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Color, &created); err != nil {
			return nil, fmt.Errorf("exportar playlists scan: %w", err)
		}
		p.CreatedAt = parseTime(created)
		indexMap[p.ID] = len(out)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("exportar playlists rows: %w", err)
	}

	if len(out) > 0 {
		itemRows, err := r.db.QueryContext(ctx, `
			SELECT playlist_id, video_id FROM playlist_items
			ORDER BY playlist_id, position`)
		if err != nil {
			return nil, fmt.Errorf("exportar itens da playlist: %w", err)
		}
		defer itemRows.Close()

		for itemRows.Next() {
			var plID, videoID string
			if err := itemRows.Scan(&plID, &videoID); err != nil {
				return nil, fmt.Errorf("exportar itens scan: %w", err)
			}
			if idx, ok := indexMap[plID]; ok {
				out[idx].VideoIDs = append(out[idx].VideoIDs, videoID)
			}
		}
		if err := itemRows.Err(); err != nil {
			return nil, fmt.Errorf("exportar itens rows: %w", err)
		}
	}
	return out, nil
}

func (r *Repository) channelFavoritesExport(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT channel_id FROM channel_favorites ORDER BY channel_id`)
	if err != nil {
		return nil, fmt.Errorf("exportar canais favoritos: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("exportar canais favoritos scan: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("exportar canais favoritos rows: %w", err)
	}
	return out, nil
}

func (r *Repository) foldersExport(ctx context.Context) ([]domain.FolderExport, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, created_at FROM channel_folders ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("exportar pastas: %w", err)
	}
	defer rows.Close()

	var out []domain.FolderExport
	indexMap := make(map[string]int)
	for rows.Next() {
		var f domain.FolderExport
		var created string
		if err := rows.Scan(&f.ID, &f.Name, &created); err != nil {
			return nil, fmt.Errorf("exportar pastas scan: %w", err)
		}
		f.CreatedAt = parseTime(created)
		indexMap[f.ID] = len(out)
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("exportar pastas rows: %w", err)
	}

	if len(out) > 0 {
		itemRows, err := r.db.QueryContext(ctx, `
			SELECT folder_id, channel_id FROM channel_folder_items
			ORDER BY folder_id, channel_id`)
		if err != nil {
			return nil, fmt.Errorf("exportar itens da pasta: %w", err)
		}
		defer itemRows.Close()

		for itemRows.Next() {
			var folderID, channelID string
			if err := itemRows.Scan(&folderID, &channelID); err != nil {
				return nil, fmt.Errorf("exportar itens da pasta scan: %w", err)
			}
			if idx, ok := indexMap[folderID]; ok {
				out[idx].ChannelIDs = append(out[idx].ChannelIDs, channelID)
			}
		}
		if err := itemRows.Err(); err != nil {
			return nil, fmt.Errorf("exportar itens da pasta rows: %w", err)
		}
	}
	return out, nil
}

func (r *Repository) feedbackExport(ctx context.Context) ([]domain.RecommendationFeedback, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(video_id, ''), COALESCE(channel_id, ''), COALESCE(topic, ''),
		       COALESCE(action, '')
		FROM recommendation_feedback ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("exportar feedback: %w", err)
	}
	defer rows.Close()

	var out []domain.RecommendationFeedback
	for rows.Next() {
		var fb domain.RecommendationFeedback
		if err := rows.Scan(&fb.VideoID, &fb.ChannelID, &fb.Topic, &fb.Action); err != nil {
			return nil, fmt.Errorf("exportar feedback scan: %w", err)
		}
		out = append(out, fb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("exportar feedback rows: %w", err)
	}
	return out, nil
}

// ClearPersonalData apaga os dados pessoais locais em uma transação: histórico,
// favoritos, playlists, pastas de canais, feedback e perfil de interesses.
// O catálogo (vídeos/canais) e as preferências permanecem.
func (r *Repository) ClearPersonalData(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("limpar dados: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []string{
		`DELETE FROM playback_progress`,
		`DELETE FROM favorites`,
		`DELETE FROM playlist_items`,
		`DELETE FROM playlists`,
		`DELETE FROM channel_folder_items`,
		`DELETE FROM channel_folders`,
		`DELETE FROM channel_favorites`,
		`DELETE FROM recommendation_feedback`,
		`DELETE FROM interest_topics`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("limpar dados: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("limpar dados: %w", err)
	}
	return nil
}
