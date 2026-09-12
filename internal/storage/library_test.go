package storage

import (
	"context"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func seedLibrary(t *testing.T, r *Repository) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	videos := []domain.Video{
		{ID: "v1", ChannelID: "UC1", Title: "Kernel Linux", PublishedAt: now.Add(-3 * time.Hour), FirstSeenAt: now, LastSeenAt: now},
		{ID: "v2", ChannelID: "UC1", Title: "Rust na prática", PublishedAt: now.Add(-2 * time.Hour), FirstSeenAt: now, LastSeenAt: now},
		{ID: "v3", ChannelID: "UC2", Title: "GTK3 e Go", PublishedAt: now.Add(-time.Hour), FirstSeenAt: now, LastSeenAt: now},
	}
	if err := r.UpsertVideos(ctx, videos); err != nil {
		t.Fatalf("UpsertVideos: %v", err)
	}
}

func TestHistoryOrdersByLastWatched(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	base := time.Now().UTC().Truncate(time.Second)
	// v1 assistido primeiro, v3 depois: o histórico deve inverter essa ordem.
	if err := r.MarkProgress(ctx, "v1", domain.PlaybackProgress{
		VideoID: "v1", Position: time.Minute, Duration: 10 * time.Minute, UpdatedAt: base.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("MarkProgress v1: %v", err)
	}
	if err := r.MarkProgress(ctx, "v3", domain.PlaybackProgress{
		VideoID: "v3", Position: 2 * time.Minute, Duration: 8 * time.Minute, UpdatedAt: base,
	}); err != nil {
		t.Fatalf("MarkProgress v3: %v", err)
	}

	history, err := r.History(ctx, 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("History devolveu %d vídeos, esperado 2 (só os com progresso)", len(history))
	}
	if history[0].ID != "v3" || history[1].ID != "v1" {
		t.Fatalf("ordem errada: %s, %s (esperado v3, v1)", history[0].ID, history[1].ID)
	}
	if history[0].Title != "GTK3 e Go" {
		t.Fatalf("metadata não veio junto: título %q", history[0].Title)
	}
}

func TestHistoryEmptyWithoutProgress(t *testing.T) {
	r := newTestRepo(t)
	seedLibrary(t, r)

	history, err := r.History(context.Background(), 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("History devolveu %d vídeos sem nenhum progresso gravado", len(history))
	}
}

func TestVideosByIDsPreservesOrderAndSkipsUnknown(t *testing.T) {
	r := newTestRepo(t)
	seedLibrary(t, r)

	videos, err := r.VideosByIDs(context.Background(), []string{"v3", "desconhecido", "v1"})
	if err != nil {
		t.Fatalf("VideosByIDs: %v", err)
	}
	if len(videos) != 2 {
		t.Fatalf("esperado 2 vídeos, veio %d", len(videos))
	}
	if videos[0].ID != "v3" || videos[1].ID != "v1" {
		t.Fatalf("ordem do chamador não preservada: %s, %s", videos[0].ID, videos[1].ID)
	}
}

func TestVideosByIDsEmptyInput(t *testing.T) {
	r := newTestRepo(t)
	videos, err := r.VideosByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("VideosByIDs: %v", err)
	}
	if len(videos) != 0 {
		t.Fatalf("esperado nenhum vídeo, veio %d", len(videos))
	}
}

func TestCountVideos(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)

	n, err := r.CountVideos(ctx)
	if err != nil {
		t.Fatalf("CountVideos: %v", err)
	}
	if n != 0 {
		t.Fatalf("catálogo novo deveria estar vazio, veio %d", n)
	}

	seedLibrary(t, r)
	if n, err = r.CountVideos(ctx); err != nil {
		t.Fatalf("CountVideos: %v", err)
	}
	if n != 3 {
		t.Fatalf("esperado 3 vídeos, veio %d", n)
	}
}
