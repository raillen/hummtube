// Favoritos / assistir depois. É estado puramente local: nada é enviado ao
// YouTube (docs/03-implementation/STORAGE.md).
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// AddFavorite salva um vídeo. Salvar de novo não duplica nem atualiza a data,
// para a ordem da lista não mudar sozinha.
func (r *Repository) AddFavorite(ctx context.Context, videoID string) error {
	if videoID == "" {
		return errors.New("favoritos: video id vazio")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO favorites (video_id, created_at) VALUES (?, ?)
		ON CONFLICT(video_id) DO NOTHING`, videoID, fmtTime(time.Now()))
	if err != nil {
		return fmt.Errorf("favoritos: %w", err)
	}
	return nil
}

// RemoveFavorite tira um vídeo dos favoritos. Remover o que não está salvo não
// é erro.
func (r *Repository) RemoveFavorite(ctx context.Context, videoID string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM favorites WHERE video_id = ?`, videoID); err != nil {
		return fmt.Errorf("favoritos: %w", err)
	}
	return nil
}

// FavoriteVideos lista os favoritos, mais recentes primeiro, já com a metadata
// do catálogo. Favoritos cujo vídeo sumiu do catálogo são ignorados.
func (r *Repository) FavoriteVideos(ctx context.Context) ([]domain.Video, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+videoColumns+`
		FROM favorites f
		JOIN videos v ON v.id = f.video_id
		ORDER BY f.created_at DESC, v.id`)
	if err != nil {
		return nil, fmt.Errorf("favoritos: %w", err)
	}
	defer rows.Close()
	return scanVideoRows(rows, "favoritos")
}

// FavoriteIDs devolve o conjunto de favoritos. A UI usa isso para decidir se o
// menu de contexto oferece "salvar" ou "remover".
func (r *Repository) FavoriteIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT video_id FROM favorites`)
	if err != nil {
		return nil, fmt.Errorf("favoritos: %w", err)
	}
	defer rows.Close()

	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("favoritos scan: %w", err)
		}
		ids[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("favoritos rows: %w", err)
	}
	return ids, nil
}
