package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

type repoTestBencher interface {
	Helper()
	Cleanup(func())
	Fatalf(string, ...any)
}

func newTestRepo(t repoTestBencher) *Repository {
	t.Helper()
	db, err := OpenMemory(context.Background())
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return NewRepository(db)
}

// BenchmarkUpsertVideos mede o custo do upsert + sincronização FTS5 em massa.
// Evidência para o limite de batelada (500) usado no primeiro sync (~12k
// vídeos no smoke real).
func BenchmarkUpsertVideos(b *testing.B) {
	ctx := context.Background()
	r := newTestRepo(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vids := make([]domain.Video, 10000)
		now := time.Now()
		for j := range vids {
			vids[j] = domain.Video{
				ID:                 btestingID(i, j),
				ChannelID:          "UC1",
				Title:              "Video de benchmark",
				DescriptionExcerpt: "descrição",
				PublishedAt:        now,
				FirstSeenAt:        now,
				LastSeenAt:         now,
			}
		}
		if err := r.UpsertVideos(ctx, vids); err != nil {
			b.Fatalf("UpsertVideos: %v", err)
		}
	}
}

func btestingID(i, j int) string {
	s := [16]byte{}
	for k := 0; k < 8; k++ {
		s[k] = byte((i >> (k * 8)) & 0xff)
	}
	for k := 0; k < 8; k++ {
		s[8+k] = byte((j >> (k * 8)) & 0xff)
	}
	return fmt.Sprintf("v%x%x", s[:8], s[8:])
}

func TestUpsertVideosAndRecent(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	if err := r.UpsertChannels(ctx, []domain.Channel{
		{ID: "c1", Title: "Canal Um"},
		{ID: "c2", Title: "Canal Dois"},
	}); err != nil {
		t.Fatalf("UpsertChannels: %v", err)
	}

	published := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	videos := []domain.Video{
		{
			ID: "v1", ChannelID: "c1", Title: "Primeiro",
			Description: "Descrição completa do primeiro vídeo",
			PublishedAt: published, Duration: 90 * time.Second,
			ThumbnailURL: "http://x/1.jpg",
			Category:     "Technology", LiveStatus: "is_live",
		},
		{
			ID: "v2", ChannelID: "c2", Title: "Segundo",
			PublishedAt: published.Add(time.Hour), Duration: 5 * time.Minute,
			LiveStatus: "was_live",
		},
	}
	if err := r.UpsertVideos(ctx, videos); err != nil {
		t.Fatalf("UpsertVideos: %v", err)
	}

	got, err := r.Recent(ctx, domain.VideoFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "v2" { // mais recente primeiro
		t.Errorf("got[0].ID = %s, want v2", got[0].ID)
	}
	if got[0].ChannelTitle != "Canal Dois" || got[1].ChannelTitle != "Canal Um" {
		t.Errorf("títulos dos canais = %q, %q", got[0].ChannelTitle, got[1].ChannelTitle)
	}
	if got[0].LiveStatus != "was_live" {
		t.Errorf("live_status v2 = %q, want was_live", got[0].LiveStatus)
	}
	if got[1].Duration != 90*time.Second {
		t.Errorf("duration = %v", got[1].Duration)
	}
	if got[1].Category != "Technology" {
		t.Errorf("category = %q, want Technology", got[1].Category)
	}
	if got[1].LiveStatus != "is_live" {
		t.Errorf("live_status v1 = %q, want is_live", got[1].LiveStatus)
	}
	if got[1].Description != "Descrição completa do primeiro vídeo" {
		t.Errorf("description = %q, want descrição completa", got[1].Description)
	}
	if !got[1].PublishedAt.Equal(published) {
		t.Errorf("published = %v", got[1].PublishedAt)
	}

	filtered, err := r.Recent(ctx, domain.VideoFilter{ChannelID: "c1"})
	if err != nil {
		t.Fatalf("Recent filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "v1" {
		t.Errorf("filtered = %+v", filtered)
	}
}

func TestRecentSubscribedPaginationIsStable(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	if err := r.UpsertChannels(ctx, []domain.Channel{
		{ID: "subscribed", Title: "Inscrito", Subscribed: true},
		{ID: "other", Title: "Outro", Subscribed: false},
	}); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	videos := make([]domain.Video, 0, 4)
	for i, id := range []string{"a", "b", "c", "d"} {
		videos = append(videos, domain.Video{
			ID: id, ChannelID: "subscribed", Title: id,
			PublishedAt: when.Add(-time.Duration(i) * time.Minute),
		})
	}
	videos = append(videos, domain.Video{ID: "outside", ChannelID: "other", Title: "outside", PublishedAt: when.Add(time.Hour)})
	if err := r.UpsertVideos(ctx, videos); err != nil {
		t.Fatal(err)
	}
	first, err := r.Recent(ctx, domain.VideoFilter{OnlySubscribed: true, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Recent(ctx, domain.VideoFilter{OnlySubscribed: true, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 2 || first[0].ID != "a" || first[1].ID != "b" || second[0].ID != "c" || second[1].ID != "d" {
		t.Fatalf("páginas instáveis: first=%v second=%v", first, second)
	}
}

func TestUpsertVideosIsUpdatePreservingFirstSeen(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	second := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if err := r.UpsertVideos(ctx, []domain.Video{{ID: "v1", Title: "Antes", FirstSeenAt: first}}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertVideos(ctx, []domain.Video{{ID: "v1", Title: "Depois", LastSeenAt: second}}); err != nil {
		t.Fatal(err)
	}

	got, err := r.Recent(ctx, domain.VideoFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Title != "Depois" {
		t.Errorf("title = %q", got[0].Title)
	}
	if !got[0].FirstSeenAt.Equal(first) {
		t.Errorf("first_seen_at deveria preservar = %v", got[0].FirstSeenAt)
	}
	if !got[0].LastSeenAt.Equal(second) {
		t.Errorf("last_seen_at = %v", got[0].LastSeenAt)
	}
}

func TestUpsertVideosPreservesThumbnailWhenUpdateIsEmpty(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	if err := r.UpsertVideos(ctx, []domain.Video{{ID: "v-thumb", Title: "Video", ThumbnailURL: "https://img/valid.jpg"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertVideos(ctx, []domain.Video{{ID: "v-thumb", Title: "Video atualizado"}}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Recent(ctx, domain.VideoFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ThumbnailURL != "https://img/valid.jpg" {
		t.Fatalf("thumbnail = %+v, URL válida deveria ser preservada", got)
	}
}

func TestMarkProgress(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	p := domain.PlaybackProgress{
		VideoID: "v1", Position: 30 * time.Second,
		Duration: 90 * time.Second, UpdatedAt: time.Now().UTC(),
	}
	if err := r.MarkProgress(ctx, "v1", p); err != nil {
		t.Fatalf("MarkProgress: %v", err)
	}
	p2 := domain.PlaybackProgress{VideoID: "v1", Position: 45 * time.Second, Completed: true}
	if err := r.MarkProgress(ctx, "v1", p2); err != nil {
		t.Fatalf("MarkProgress update: %v", err)
	}

	var posMs, durMs, completed int
	if err := r.db.QueryRow(
		"SELECT position_ms, duration_ms, completed FROM playback_progress WHERE video_id = 'v1'",
	).Scan(&posMs, &durMs, &completed); err != nil {
		t.Fatalf("select: %v", err)
	}
	if posMs != 45000 || durMs != 0 || completed != 1 {
		t.Errorf("got pos=%d dur=%d completed=%d", posMs, durMs, completed)
	}
}

func TestSearchAndReindex(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	if err := r.UpsertVideos(ctx, []domain.Video{
		{ID: "v1", ChannelID: "c1", Title: "GoLang tutorial", DescriptionExcerpt: "goroutines e canais"},
		{ID: "v2", ChannelID: "c2", Title: "Receita de bolo", DescriptionExcerpt: "farinha e açúcar"},
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := r.Search(ctx, "golang", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].VideoID != "v1" {
		t.Errorf("hits = %+v", hits)
	}

	// Injeção de sintaxe FTS5 não deve quebrar nem retornar tudo.
	hits, err = r.Search(ctx, `" OR *`, 10)
	if err != nil {
		t.Fatalf("Search injection: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("hits com injeção = %+v, want 0", hits)
	}

	hits, err = r.Search(ctx, "bolo", 10)
	if err != nil {
		t.Fatalf("Search bolo: %v", err)
	}
	if len(hits) != 1 || hits[0].VideoID != "v2" {
		t.Errorf("hits bolo = %+v", hits)
	}
}

func TestSearchWithChannelName(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	if _, err := r.db.Exec(
		"INSERT INTO channels (id, title) VALUES ('c1', 'Canal Go')"); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertVideos(ctx, []domain.Video{
		{ID: "v1", ChannelID: "c1", Title: "Introdução"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReindexVideos(ctx, []string{"v1"}); err != nil {
		t.Fatalf("ReindexVideos: %v", err)
	}

	hits, err := r.Search(ctx, "canal", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 || hits[0].VideoID != "v1" {
		t.Errorf("hits = %+v", hits)
	}
	if hits[0].ChannelName != "Canal Go" {
		t.Errorf("channel_name = %q", hits[0].ChannelName)
	}

	// Reindex idempotente não deve duplicar.
	if err := r.ReindexVideos(ctx, []string{"v1"}); err != nil {
		t.Fatal(err)
	}
	hits, _ = r.Search(ctx, "canal", 10)
	if len(hits) != 1 {
		t.Errorf("duplicou: %+v", hits)
	}
}
