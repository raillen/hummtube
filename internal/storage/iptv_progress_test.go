package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/iptv"
)

func TestIPTVProgressRoundTrip(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	seedIPTVCatalog(t, r, "movie-1")

	if _, found, err := r.GetIPTVProgress(ctx, "movie-1"); err != nil || found {
		t.Fatalf("progresso inicial = (%v, %v), want (zero, false)", found, err)
	}

	saved := iptv.PlaybackPosition{
		ItemID:    "movie-1",
		Position:  7 * time.Minute,
		Duration:  90 * time.Minute,
		UpdatedAt: time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC),
	}
	if err := r.SetIPTVProgress(ctx, saved); err != nil {
		t.Fatalf("SetIPTVProgress: %v", err)
	}
	got, found, err := r.GetIPTVProgress(ctx, "movie-1")
	if err != nil || !found {
		t.Fatalf("GetIPTVProgress após salvar = (%v, %v), want (true, nil)", found, err)
	}
	if got.Position != saved.Position || got.Duration != saved.Duration {
		t.Fatalf("posição/duração = %v/%v, want %v/%v", got.Position, got.Duration, saved.Position, saved.Duration)
	}
	if got.Completed {
		t.Fatal("completed = true, want false")
	}

	// Upsert atualiza posição e marca concluído.
	saved.Position = 89 * time.Minute
	saved.Completed = true
	if err := r.SetIPTVProgress(ctx, saved); err != nil {
		t.Fatalf("SetIPTVProgress (update): %v", err)
	}
	got, _, _ = r.GetIPTVProgress(ctx, "movie-1")
	if !got.Completed || got.Position != saved.Position {
		t.Fatalf("após update = %+v, want concluído em %v", got, saved.Position)
	}

	if err := r.DeleteIPTVProgress(ctx, "movie-1"); err != nil {
		t.Fatalf("DeleteIPTVProgress: %v", err)
	}
	if _, found, _ := r.GetIPTVProgress(ctx, "movie-1"); found {
		t.Fatal("progresso ainda presente após DeleteIPTVProgress")
	}
}

func TestSetIPTVProgressRejectsEmptyItemID(t *testing.T) {
	r := newTestRepo(t)
	if err := r.SetIPTVProgress(context.Background(), iptv.PlaybackPosition{}); err == nil {
		t.Fatal("aceitou item_id vazio")
	}
}

// seedIPTVCatalog cria fonte + item para satisfazer a FK de progresso.
func seedIPTVCatalog(t *testing.T, r *Repository, itemID string) {
	t.Helper()
	source := iptv.SourceConfig{ID: "fixture", Name: "Fixture", Format: iptv.SourceFormatM3U,
		PlaylistURL: "https://example.invalid/playlist.m3u", Enabled: true}
	playlist := iptv.Playlist{Items: []iptv.Item{{
		ID: itemID, SourceID: "fixture", Kind: iptv.ContentKindMovie,
		Classification: iptv.ClassificationGroup, Title: "Filme fixture",
	}}}
	if err := r.ReplaceIPTVCatalog(context.Background(), source, playlist,
		time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("ReplaceIPTVCatalog (seed): %v", err)
	}
}

func TestIPTVProgressCascadesOnCatalogReplace(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	source := iptv.SourceConfig{ID: "fixture", Name: "Fixture", Format: iptv.SourceFormatM3U,
		PlaylistURL: "https://example.invalid/playlist.m3u", Enabled: true}
	playlist := iptv.Playlist{Items: []iptv.Item{{
		ID: "movie-1", SourceID: "fixture", Kind: iptv.ContentKindMovie,
		Classification: iptv.ClassificationGroup, Title: "Filme fixture",
	}}}
	when := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	if err := r.ReplaceIPTVCatalog(ctx, source, playlist, when); err != nil {
		t.Fatalf("ReplaceIPTVCatalog: %v", err)
	}
	if err := r.SetIPTVProgress(ctx, iptv.PlaybackPosition{ItemID: "movie-1", Position: time.Minute}); err != nil {
		t.Fatalf("SetIPTVProgress: %v", err)
	}

	// Sincronização nova remove o item; o progresso órfão não sobrevive.
	playlist.Items = nil
	if err := r.ReplaceIPTVCatalog(ctx, source, playlist, when.Add(time.Minute)); err != nil {
		t.Fatalf("ReplaceIPTVCatalog (vazio): %v", err)
	}
	if _, found, _ := r.GetIPTVProgress(ctx, "movie-1"); found {
		t.Fatal("progresso órfão sobreviveu à substituição do catálogo")
	}
}

func TestListIPTVGuideCurrentWindowReturnsOnlyCoveringPrograms(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	sourceID := "fixture"
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	// Canais de guia têm FK para a fonte; semeia o catálogo primeiro.
	seedIPTVCatalog(t, r, "movie-1")

	channels := []iptv.GuideChannel{{ID: "ch.1", Name: "Canal Um"}}
	programs := []iptv.GuideProgram{
		// Em curso agora.
		{ChannelID: "ch.1", Title: "Jornal do Meio-Dia",
			Start: now.Add(-30 * time.Minute), End: now.Add(30 * time.Minute)},
		// Já encerrado.
		{ChannelID: "ch.1", Title: "Manhã",
			Start: now.Add(-3 * time.Hour), End: now.Add(-1 * time.Hour)},
		// Futuro.
		{ChannelID: "ch.1", Title: "Sessão da Noite",
			Start: now.Add(2 * time.Hour), End: now.Add(4 * time.Hour)},
	}
	if err := r.ReplaceIPTVGuide(ctx, sourceID, channels, programs); err != nil {
		t.Fatalf("ReplaceIPTVGuide: %v", err)
	}

	current, err := r.ListIPTVGuide(ctx, sourceID, now, now, 10)
	if err != nil {
		t.Fatalf("ListIPTVGuide (agora): %v", err)
	}
	if len(current) != 1 || current[0].Program.Title != "Jornal do Meio-Dia" {
		t.Fatalf("programas em curso = %+v, want apenas Jornal do Meio-Dia", current)
	}
}

func TestImportIPTVCatalogReplacesAcrossBatches(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	source := iptv.SourceConfig{ID: "fixture", Name: "Fixture", Format: iptv.SourceFormatM3U,
		PlaylistURL: "https://example.invalid/playlist.m3u", Enabled: true}
	when := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	// Sincronização 1: dois lotes com três itens.
	if err := r.ImportIPTVCatalog(ctx, source, when, func(yield func([]iptv.Item) error) error {
		if err := yield([]iptv.Item{{ID: "movie-a", Kind: iptv.ContentKindMovie, Title: "A"}}); err != nil {
			return err
		}
		return yield([]iptv.Item{
			{ID: "movie-b", Kind: iptv.ContentKindMovie, Title: "B"},
			{ID: "tv-1", Kind: iptv.ContentKindTV, Title: "Canal"},
		})
	}); err != nil {
		t.Fatalf("ImportIPTVCatalog 1: %v", err)
	}
	items, _ := r.ListAllIPTVItems(ctx, "", 100)
	if len(items) != 3 {
		t.Fatalf("após sync 1 = %d itens, want 3", len(items))
	}

	// Sincronização 2: item removido some, novo entra — substituição atômica.
	if err := r.ImportIPTVCatalog(ctx, source, when.Add(time.Minute), func(yield func([]iptv.Item) error) error {
		return yield([]iptv.Item{
			{ID: "movie-a", Kind: iptv.ContentKindMovie, Title: "A"},
			{ID: "movie-c", Kind: iptv.ContentKindMovie, Title: "C"},
		})
	}); err != nil {
		t.Fatalf("ImportIPTVCatalog 2: %v", err)
	}
	items, _ = r.ListAllIPTVItems(ctx, "", 100)
	if len(items) != 2 {
		t.Fatalf("após sync 2 = %d itens, want 2", len(items))
	}
	for _, it := range items {
		if it.ID == "movie-b" || it.ID == "tv-1" {
			t.Fatalf("item obsoleto sobreviveu: %s", it.ID)
		}
	}

	// Erro no meio do stream aborta tudo: snapshot anterior permanece.
	err := r.ImportIPTVCatalog(ctx, source, when.Add(2*time.Minute), func(yield func([]iptv.Item) error) error {
		if err := yield([]iptv.Item{{ID: "movie-x", Kind: iptv.ContentKindMovie, Title: "X"}}); err != nil {
			return err
		}
		return fmt.Errorf("falha simulada no meio do stream")
	})
	if err == nil {
		t.Fatal("erro simulado deveria propagar")
	}
	items, _ = r.ListAllIPTVItems(ctx, "", 100)
	if len(items) != 2 {
		t.Fatalf("após aborto = %d itens, want snapshot intacto (2)", len(items))
	}
}

func TestListIPTVResumeItemsOrdersAndFilters(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	items := []iptv.Item{
		{ID: "movie-a", SourceID: "fixture", Kind: iptv.ContentKindMovie, Title: "Filme A"},
		{ID: "movie-b", SourceID: "fixture", Kind: iptv.ContentKindMovie, Title: "Filme B"},
		{ID: "movie-c", SourceID: "fixture", Kind: iptv.ContentKindMovie, Title: "Filme C"},
	}
	source := iptv.SourceConfig{ID: "fixture", Name: "Fixture", Format: iptv.SourceFormatM3U,
		PlaylistURL: "https://example.invalid/playlist.m3u", Enabled: true}
	if err := r.ReplaceIPTVCatalog(ctx, source, iptv.Playlist{Items: items},
		time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("ReplaceIPTVCatalog: %v", err)
	}

	ninety := 90 * time.Minute
	progress := []iptv.PlaybackPosition{
		{ItemID: "movie-a", Position: 10 * time.Minute, Duration: ninety,
			UpdatedAt: time.Date(2026, 8, 22, 13, 0, 0, 0, time.UTC)},
		// B foi concluído: não aparece em Continuar assistindo.
		{ItemID: "movie-b", Position: 89 * time.Minute, Duration: ninety, Completed: true,
			UpdatedAt: time.Date(2026, 8, 22, 14, 0, 0, 0, time.UTC)},
		// C é mais recente que A: deve vir primeiro.
		{ItemID: "movie-c", Position: 40 * time.Minute, Duration: ninety,
			UpdatedAt: time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)},
	}
	for _, p := range progress {
		if err := r.SetIPTVProgress(ctx, p); err != nil {
			t.Fatalf("SetIPTVProgress(%s): %v", p.ItemID, err)
		}
	}

	got, err := r.ListIPTVResumeItems(ctx, 10)
	if err != nil {
		t.Fatalf("ListIPTVResumeItems: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("entradas = %d, want 2 (concluído excluído)", len(got))
	}
	if got[0].Item.ID != "movie-c" || got[1].Item.ID != "movie-a" {
		t.Fatalf("ordem = [%s %s], want [movie-c movie-a]", got[0].Item.ID, got[1].Item.ID)
	}
	if got[0].Position != 40*time.Minute || got[0].Duration != ninety {
		t.Fatalf("posição/duração = %v/%v, want 40m/90m", got[0].Position, got[0].Duration)
	}
	if got[0].Item.Title != "Filme C" || got[0].Item.SourceName != "Fixture" {
		t.Fatalf("item embutido = %+v", got[0].Item)
	}
}
