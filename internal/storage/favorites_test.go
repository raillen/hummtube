package storage

import (
	"context"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestFavoritesRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	if err := r.AddFavorite(ctx, "v1"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}
	if err := r.AddFavorite(ctx, "v3"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}

	ids, err := r.FavoriteIDs(ctx)
	if err != nil {
		t.Fatalf("FavoriteIDs: %v", err)
	}
	if !ids["v1"] || !ids["v3"] || ids["v2"] {
		t.Fatalf("conjunto de favoritos errado: %v", ids)
	}

	videos, err := r.FavoriteVideos(ctx)
	if err != nil {
		t.Fatalf("FavoriteVideos: %v", err)
	}
	if len(videos) != 2 || videos[0].Title == "" {
		t.Fatalf("favoritos sem metadata: %+v", videos)
	}

	if err := r.RemoveFavorite(ctx, "v1"); err != nil {
		t.Fatalf("RemoveFavorite: %v", err)
	}
	ids, _ = r.FavoriteIDs(ctx)
	if ids["v1"] || !ids["v3"] {
		t.Fatalf("remoção não aplicada: %v", ids)
	}
}

// Salvar duas vezes não pode duplicar nem reordenar a lista.
func TestAddFavoriteIsIdempotent(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	if err := r.AddFavorite(ctx, "v1"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}
	var first string
	if err := r.db.QueryRowContext(ctx,
		`SELECT created_at FROM favorites WHERE video_id = 'v1'`).Scan(&first); err != nil {
		t.Fatalf("leitura: %v", err)
	}

	if err := r.AddFavorite(ctx, "v1"); err != nil {
		t.Fatalf("AddFavorite repetido: %v", err)
	}
	videos, _ := r.FavoriteVideos(ctx)
	if len(videos) != 1 {
		t.Fatalf("favorito duplicado: %d entradas", len(videos))
	}
	var second string
	if err := r.db.QueryRowContext(ctx,
		`SELECT created_at FROM favorites WHERE video_id = 'v1'`).Scan(&second); err != nil {
		t.Fatalf("leitura: %v", err)
	}
	if first != second {
		t.Errorf("resalvar mexeu na data (%q → %q) e reordenaria a lista", first, second)
	}
}

func TestFavoritesEdgeCases(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)

	if err := r.AddFavorite(ctx, ""); err == nil {
		t.Error("id vazio deveria ser recusado")
	}
	if err := r.RemoveFavorite(ctx, "inexistente"); err != nil {
		t.Errorf("remover ausente deveria ser no-op: %v", err)
	}
	ids, err := r.FavoriteIDs(ctx)
	if err != nil || len(ids) != 0 {
		t.Errorf("catálogo novo sem favoritos: %v %v", ids, err)
	}
}

// Favorito cujo vídeo saiu do catálogo não pode quebrar a listagem.
func TestFavoriteVideosSkipsUnknown(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	if err := r.AddFavorite(ctx, "v1"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}
	if err := r.AddFavorite(ctx, "fantasma"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}
	videos, err := r.FavoriteVideos(ctx)
	if err != nil {
		t.Fatalf("FavoriteVideos: %v", err)
	}
	if len(videos) != 1 || videos[0].ID != "v1" {
		t.Fatalf("órfão deveria ser ignorado: %+v", videos)
	}
}

// A migration 00003 precisa subir sobre um banco que já tinha dados.
func TestMigrationV3PreservesData(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	version, err := MigrationVersion(r.db)
	if err != nil {
		t.Fatalf("MigrationVersion: %v", err)
	}
	if version != latestMigration {
		t.Fatalf("versão de migration esperada %d, veio %d", latestMigration, version)
	}
	videos, err := r.Recent(ctx, domain.VideoFilter{Limit: 50})
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(videos) != 3 {
		t.Fatalf("dados anteriores não sobreviveram: %d vídeos", len(videos))
	}
}
