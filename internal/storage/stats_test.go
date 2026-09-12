package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// seedPersonalData povoa o estado local com histórico, favoritos, playlists,
// canais favoritados, pastas e feedback para os testes de estatísticas.
func seedPersonalData(t *testing.T, r *Repository) {
	t.Helper()
	ctx := context.Background()

	seedLibrary(t, r)
	if err := r.UpsertChannels(ctx, []domain.Channel{
		{ID: "UC1", Title: "Canal Um", Subscribed: true},
		{ID: "UC2", Title: "Canal Dois", Subscribed: true},
	}); err != nil {
		t.Fatalf("UpsertChannels: %v", err)
	}

	// Histórico: v1 e v3 completos, v2 em andamento.
	for _, p := range []domain.PlaybackProgress{
		{VideoID: "v1", Position: 10 * time.Minute, Duration: 10 * time.Minute, Completed: true},
		{VideoID: "v2", Position: 3 * time.Minute, Duration: 6 * time.Minute},
		{VideoID: "v3", Position: 4 * time.Minute, Duration: 4 * time.Minute, Completed: true},
	} {
		if err := r.MarkProgress(ctx, p.VideoID, p); err != nil {
			t.Fatalf("MarkProgress %s: %v", p.VideoID, err)
		}
	}

	if err := r.AddFavorite(ctx, "v1"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}

	p, err := r.CreatePlaylist(ctx, "Playlist", "desc", "#e91e63")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if err := r.AddPlaylistItem(ctx, p.ID, "v1"); err != nil {
		t.Fatalf("AddPlaylistItem: %v", err)
	}

	if err := r.AddChannelFavorite(ctx, "UC1"); err != nil {
		t.Fatalf("AddChannelFavorite: %v", err)
	}
	f, err := r.CreateFolder(ctx, "Tecnologia")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if err := r.AddChannelToFolder(ctx, f.ID, "UC1"); err != nil {
		t.Fatalf("AddChannelToFolder: %v", err)
	}

	if err := r.RecordFeedback(ctx, domain.RecommendationFeedback{
		VideoID: "v1", Action: domain.FeedbackDontRecommend,
	}); err != nil {
		t.Fatalf("RecordFeedback: %v", err)
	}
}

func TestLocalStats(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPersonalData(t, r)

	stats, err := r.LocalStats(ctx)
	if err != nil {
		t.Fatalf("LocalStats: %v", err)
	}

	if stats.VideosWatched != 2 {
		t.Errorf("VideosWatched = %d, esperado 2", stats.VideosWatched)
	}
	// v1 (10min) + v2 (posição 3min) + v3 (4min) = 17min.
	if stats.TotalWatchTime != 17*time.Minute {
		t.Errorf("TotalWatchTime = %v, esperado 17min", stats.TotalWatchTime)
	}
	if stats.HistoryCount != 3 {
		t.Errorf("HistoryCount = %d, esperado 3", stats.HistoryCount)
	}
	if stats.FavoritesCount != 1 {
		t.Errorf("FavoritesCount = %d, esperado 1", stats.FavoritesCount)
	}
	if stats.PlaylistsCount != 1 || stats.PlaylistItems != 1 {
		t.Errorf("Playlists = %d/%d, esperado 1/1", stats.PlaylistsCount, stats.PlaylistItems)
	}
	if stats.SubscriptionsCount != 2 {
		t.Errorf("SubscriptionsCount = %d, esperado 2", stats.SubscriptionsCount)
	}
	if stats.ChannelFavoritesCount != 1 {
		t.Errorf("ChannelFavoritesCount = %d, esperado 1", stats.ChannelFavoritesCount)
	}
	if stats.FoldersCount != 1 {
		t.Errorf("FoldersCount = %d, esperado 1", stats.FoldersCount)
	}
	if stats.FeedbackCount != 1 {
		t.Errorf("FeedbackCount = %d, esperado 1", stats.FeedbackCount)
	}
}

func TestLocalStatsEmpty(t *testing.T) {
	r := openTestRepo(t)
	stats, err := r.LocalStats(context.Background())
	if err != nil {
		t.Fatalf("LocalStats: %v", err)
	}
	if stats.VideosWatched != 0 || stats.TotalWatchTime != 0 ||
		stats.FavoritesCount != 0 || stats.PlaylistsCount != 0 {
		t.Errorf("estatísticas de banco vazio = %+v", stats)
	}
}

func TestExportPersonalDataRoundTrip(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPersonalData(t, r)

	data, err := r.ExportPersonalData(ctx)
	if err != nil {
		t.Fatalf("ExportPersonalData: %v", err)
	}
	if data.Format != "nanotube-personal-data" || data.Version != 1 {
		t.Errorf("formato = %q v%d", data.Format, data.Version)
	}

	if len(data.History) != 3 {
		t.Errorf("History = %d, esperado 3", len(data.History))
	}
	// Mais recente primeiro: v3 e v1 completos, v2 em andamento.
	found := map[string]domain.HistoryEntry{}
	for _, h := range data.History {
		found[h.VideoID] = h
	}
	if e := found["v1"]; !e.Completed || e.Title != "Kernel Linux" || e.Duration != int64(10*time.Minute/time.Millisecond) {
		t.Errorf("history v1 = %+v", e)
	}
	if e := found["v2"]; e.Completed {
		t.Errorf("history v2 não deveria estar completo: %+v", e)
	}

	if len(data.Favorites) != 1 || data.Favorites[0] != "v1" {
		t.Errorf("Favorites = %v", data.Favorites)
	}
	if len(data.Playlists) != 1 || len(data.Playlists[0].VideoIDs) != 1 {
		t.Errorf("Playlists = %+v", data.Playlists)
	}
	if len(data.ChannelFavorites) != 1 || data.ChannelFavorites[0] != "UC1" {
		t.Errorf("ChannelFavorites = %v", data.ChannelFavorites)
	}
	if len(data.Folders) != 1 || data.Folders[0].Name != "Tecnologia" ||
		len(data.Folders[0].ChannelIDs) != 1 {
		t.Errorf("Folders = %+v", data.Folders)
	}
	if len(data.Feedback) != 1 || data.Feedback[0].VideoID != "v1" {
		t.Errorf("Feedback = %+v", data.Feedback)
	}

	// O dump precisa serializar em JSON sem erro (o que a UI grava no disco).
	if _, err := json.Marshal(data); err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
}

func TestExportPersonalDataEmpty(t *testing.T) {
	r := openTestRepo(t)
	data, err := r.ExportPersonalData(context.Background())
	if err != nil {
		t.Fatalf("ExportPersonalData: %v", err)
	}
	if len(data.History) != 0 || len(data.Favorites) != 0 || len(data.Playlists) != 0 {
		t.Errorf("dump de banco vazio = %+v", data)
	}
}

func TestClearPersonalData(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPersonalData(t, r)

	if err := r.ClearPersonalData(ctx); err != nil {
		t.Fatalf("ClearPersonalData: %v", err)
	}

	for table, want := range map[string]int{
		"playback_progress":       0,
		"favorites":               0,
		"playlists":               0,
		"playlist_items":          0,
		"channel_favorites":       0,
		"channel_folders":         0,
		"channel_folder_items":    0,
		"recommendation_feedback": 0,
		"interest_topics":         0,
	} {
		var n int
		if err := r.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
			t.Fatalf("contar %s: %v", table, err)
		}
		if n != want {
			t.Errorf("%s = %d linhas, esperado %d", table, n, want)
		}
	}

	// O catálogo (vídeos e canais) permanece: dados locais de metadados não são
	// dados pessoais.
	n, err := r.CountVideos(ctx)
	if err != nil {
		t.Fatalf("CountVideos: %v", err)
	}
	if n != 3 {
		t.Errorf("vídeos após limpar = %d, esperado 3", n)
	}
	if subs := countSubscribed(t, r); subs != 2 {
		t.Errorf("inscrições após limpar = %d, esperado 2", subs)
	}

	stats, err := r.LocalStats(ctx)
	if err != nil {
		t.Fatalf("LocalStats: %v", err)
	}
	if stats.HistoryCount != 0 || stats.FavoritesCount != 0 || stats.PlaylistsCount != 0 {
		t.Errorf("estatísticas após limpar = %+v", stats)
	}
}

func countSubscribed(t *testing.T, r *Repository) int {
	t.Helper()
	var n int
	if err := r.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM channels WHERE subscribed = 1`).Scan(&n); err != nil {
		t.Fatalf("count subscribed: %v", err)
	}
	return n
}
