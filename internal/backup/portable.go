package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/storage"
)

// ExportJSON retorna JSON versionado de dados pessoais (sem credenciais).
func ExportJSON(ctx context.Context, repo *storage.Repository) ([]byte, error) {
	data, err := repo.ExportPersonalData(ctx)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(data, "", "  ")
}

// ImportStrategy define como conflitos de dados são tratados na importação.
type ImportStrategy string

const (
	StrategyMerge     ImportStrategy = "merge"
	StrategyReplace   ImportStrategy = "replace"
	StrategyDuplicate ImportStrategy = "duplicate"
)

// ImportJSON importa PersonalData segundo a estratégia (merge = upsert, replace = clear+insert, duplicate = novos IDs).
func ImportJSON(ctx context.Context, repo *storage.Repository, data []byte, strategy ImportStrategy) error {
	var pd domain.PersonalData
	if err := json.Unmarshal(data, &pd); err != nil {
		return fmt.Errorf("json inválido: %w", err)
	}

	if strategy == StrategyReplace {
		return repo.ImportPersonalData(ctx, pd)
	}

	// Playlists
	for _, pl := range pd.Playlists {
		targetID := pl.ID
		if strategy == StrategyDuplicate {
			created, err := repo.CreatePlaylist(ctx, pl.Name, pl.Description, pl.Color)
			if err != nil {
				return fmt.Errorf("criar playlist duplicada: %w", err)
			}
			targetID = created.ID
		} else {
			if _, err := repo.CreatePlaylist(ctx, pl.Name, pl.Description, pl.Color); err != nil {
				_ = repo.UpdatePlaylist(ctx, targetID, pl.Name, pl.Description, pl.Color)
			}
		}
		for _, vid := range pl.VideoIDs {
			if err := repo.AddPlaylistItem(ctx, targetID, vid); err != nil {
				return fmt.Errorf("adicionar item na playlist: %w", err)
			}
		}
	}

	// Favoritos de vídeos
	for _, id := range pd.Favorites {
		if err := repo.AddFavorite(ctx, id); err != nil {
			return fmt.Errorf("importar favorito: %w", err)
		}
	}

	// Histórico de reprodução
	for _, e := range pd.History {
		if err := repo.MarkProgress(ctx, e.VideoID, domain.PlaybackProgress{
			Position:  time.Duration(e.Position) * time.Millisecond,
			Duration:  time.Duration(e.Duration) * time.Millisecond,
			UpdatedAt: e.UpdatedAt,
			Completed: e.Completed,
		}); err != nil {
			return fmt.Errorf("importar progresso: %w", err)
		}
	}

	// Canais favoritos
	for _, chID := range pd.ChannelFavorites {
		if err := repo.AddChannelFavorite(ctx, chID); err != nil {
			return fmt.Errorf("importar canal favorito: %w", err)
		}
	}

	// Pastas de canais
	for _, f := range pd.Folders {
		folderID := f.ID
		if created, err := repo.CreateFolder(ctx, f.Name); err == nil {
			folderID = created.ID
		}
		for _, chID := range f.ChannelIDs {
			_ = repo.AddChannelToFolder(ctx, folderID, chID)
		}
	}

	return nil
}
