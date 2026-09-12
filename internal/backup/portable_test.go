package backup

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/storage"
)

func TestExportImportJSONRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	repo := storage.NewRepository(db)

	// Seed data
	pl, err := repo.CreatePlaylist(ctx, "Minha Playlist", "Desc", "#ff0000")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if err := repo.AddPlaylistItem(ctx, pl.ID, "vid-1"); err != nil {
		t.Fatalf("AddPlaylistItem: %v", err)
	}
	if err := repo.AddFavorite(ctx, "vid-fav-1"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}
	if err := repo.AddChannelFavorite(ctx, "ch-fav-1"); err != nil {
		t.Fatalf("AddChannelFavorite: %v", err)
	}

	// Export
	exported, err := ExportJSON(ctx, repo)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	var pd domain.PersonalData
	if err := json.Unmarshal(exported, &pd); err != nil {
		t.Fatalf("Unmarshal exported: %v", err)
	}
	if len(pd.Playlists) != 1 || len(pd.Favorites) != 1 || len(pd.ChannelFavorites) != 1 {
		t.Fatalf("Dados exportados incompletos: %+v", pd)
	}

	// Test StrategyReplace on a new database
	db2, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatalf("OpenMemory 2: %v", err)
	}
	defer db2.Close()
	if err := storage.Migrate(db2); err != nil {
		t.Fatalf("Migrate 2: %v", err)
	}
	repo2 := storage.NewRepository(db2)

	if err := ImportJSON(ctx, repo2, exported, StrategyReplace); err != nil {
		t.Fatalf("ImportJSON replace: %v", err)
	}

	pls, err := repo2.Playlists(ctx)
	if err != nil || len(pls) != 1 {
		t.Fatalf("Playlists após replace: %v (%d)", err, len(pls))
	}
	favs, err := repo2.FavoriteIDs(ctx)
	if err != nil || len(favs) != 1 {
		t.Fatalf("Favorites após replace: %v (%d)", err, len(favs))
	}

	// Test StrategyDuplicate on repo2
	if err := ImportJSON(ctx, repo2, exported, StrategyDuplicate); err != nil {
		t.Fatalf("ImportJSON duplicate: %v", err)
	}
	plsDuplicated, err := repo2.Playlists(ctx)
	if err != nil || len(plsDuplicated) != 2 {
		t.Fatalf("Playlists após duplicate esperava 2, obteve %d (%v)", len(plsDuplicated), err)
	}
}
