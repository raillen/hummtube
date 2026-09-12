package search

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/playback"
)

func TestYtDlpSearchExecutesFakeProcess(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-yt-dlp")
	marker := filepath.Join(dir, "nao-deve-executar")
	body := "#!/bin/sh\nprintf '%s' '{\"entries\":[{\"id\":\"vid\",\"title\":\"Resultado\",\"duration\":60}]}'\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	videos, err := (YtDlp{Binary: script}).Search(context.Background(), domain.SearchOptions{
		Query: "linux $(touch " + marker + ")", MaxResults: 60,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(videos) != 1 || videos[0].ID != "vid" {
		t.Fatalf("videos = %+v", videos)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("consulta foi interpretada pelo shell")
	}
}

func TestYtDlpSearchAppliesBoundedPublicPageToken(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-yt-dlp")
	body := "#!/bin/sh\nprintf '%s' '{\"entries\":[{\"id\":\"one\",\"title\":\"One\"},{\"id\":\"two\",\"title\":\"Two\"},{\"id\":\"three\",\"title\":\"Three\"}]}'\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	videos, err := (YtDlp{Binary: script}).Search(context.Background(), domain.SearchOptions{
		Query: "linux", MaxResults: 2, PageToken: "1",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(videos) != 2 || videos[0].ID != "two" || videos[1].ID != "three" {
		t.Fatalf("página = %+v", videos)
	}

	_, err = (YtDlp{Binary: script}).Search(context.Background(), domain.SearchOptions{
		Query: "linux", MaxResults: 2, PageToken: "501",
	})
	if err == nil || !strings.Contains(err.Error(), "token de página") {
		t.Fatalf("token excessivo aceito: %v", err)
	}
}

func TestYtDlpSearchHonorsTimeout(t *testing.T) {
	script := filepath.Join(t.TempDir(), "slow-yt-dlp")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 10\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err := (YtDlp{Binary: script, Timeout: 30 * time.Millisecond}).Search(context.Background(), domain.SearchOptions{Query: "linux"})
	if err == nil || !strings.Contains(err.Error(), "tempo esgotado") {
		t.Fatalf("erro = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("cancelamento demorou %v", elapsed)
	}
}

func TestYtDlpPlaylistItemsUsesExplicitPageRange(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-yt-dlp")
	body := "#!/bin/sh\nprintf '%s' '{\"entries\":[{\"id\":\"vid\",\"title\":\"Playlist item\"}]}'\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	videos, next, err := (YtDlp{Binary: script}).PlaylistItems(context.Background(), "PL_test", domain.PageOptions{MaxResults: 1})
	if err != nil {
		t.Fatalf("PlaylistItems: %v", err)
	}
	if len(videos) != 1 || videos[0].ID != "vid" || next != "1" {
		t.Fatalf("videos = %+v, next = %q", videos, next)
	}
}

func TestYtDlpChannelVideosUsesChannelURLAndPages(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-yt-dlp")
	body := "#!/bin/sh\nprintf '%s' '{\"entries\":[{\"id\":\"vid\",\"title\":\"Upload\"}]}'\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	videos, next, err := (YtDlp{Binary: script}).ChannelVideos(context.Background(), "UC_test", domain.PageOptions{MaxResults: 1})
	if err != nil {
		t.Fatalf("ChannelVideos: %v", err)
	}
	if len(videos) != 1 || videos[0].ID != "vid" || next != "1" {
		t.Fatalf("videos = %+v, next = %q", videos, next)
	}
}

func TestYtDlpChannelVideosRejectsInvalidToken(t *testing.T) {
	_, _, err := (YtDlp{Binary: "/bin/true"}).ChannelVideos(context.Background(), "UC_test", domain.PageOptions{PageToken: "abc"})
	if err == nil || !strings.Contains(err.Error(), "token inválido") {
		t.Fatalf("erro = %v", err)
	}
}

func TestRemoteListingsRejectInvalidIDsAndExcessiveOffsets(t *testing.T) {
	ytdlp := YtDlp{Binary: "/bin/true"}
	for _, playlistID := range []string{"", "id com espaço", strings.Repeat("x", 129)} {
		if _, _, err := ytdlp.PlaylistItems(context.Background(), playlistID, domain.PageOptions{}); err == nil {
			t.Fatalf("playlist ID %q foi aceito", playlistID)
		}
	}
	if _, _, err := ytdlp.PlaylistItems(context.Background(), "PL_ok", domain.PageOptions{PageToken: "5001"}); err == nil {
		t.Fatal("offset excessivo de playlist foi aceito")
	}
	if _, _, err := ytdlp.ChannelVideos(context.Background(), "canal com espaço", domain.PageOptions{}); err == nil {
		t.Fatal("channel ID inválido foi aceito")
	}
}

func TestPublicSearchConfigDropsPlaybackCredentialsAndRemoteCode(t *testing.T) {
	in := playback.YtdlConfig{
		JSRuntime:    playback.JSRuntime{Name: "deno", Path: "/usr/bin/deno"},
		PlayerClient: "mweb", POTProvider: "/secret/provider", POTMode: "script",
		CookiesFromBrowser: "firefox", CookiesFile: "/secret/cookies.txt",
		AllowRemoteComponents: true, MaxHeight: 720,
	}
	got := publicSearchConfig(in)
	if got.JSRuntime != in.JSRuntime {
		t.Fatalf("runtime foi descartado: %+v", got.JSRuntime)
	}
	if got.PlayerClient != playback.AutoPlayerClient || got.POTProvider != "" || got.POTMode != "" ||
		got.CookiesFromBrowser != "" || got.CookiesFile != "" || got.AllowRemoteComponents || got.MaxHeight != 0 {
		t.Fatalf("perfil público reteve opção sensível: %+v", got)
	}
	args := got.CommandArgs()
	for _, arg := range args {
		if arg == "firefox" || arg == "/secret/cookies.txt" || arg == "/secret/provider" || arg == "ejs:github" {
			t.Fatalf("argv contém valor sensível: %q em %#v", arg, args)
		}
	}
}

func TestParseFlatPlaylist(t *testing.T) {
	rawJSON := `{
		"_type": "playlist",
		"entries": [
			{
				"id": "vid123",
				"title": "Vídeo Teste 1",
				"channel": "Canal Legal",
				"channel_id": "UC12345",
				"duration": 125,
				"description": "Descrição",
				"view_count": 12345,
				"live_status": "not_live",
				"timestamp": 1704164645,
				"url": "https://www.youtube.com/watch?v=vid123",
				"thumbnails": [
					{"url": "https://i.ytimg.com/vi/vid123/default.jpg", "width": 120, "height": 90},
					{"url": "https://i.ytimg.com/vi/vid123/hqdefault.jpg", "width": 480, "height": 360},
					{"url": "https://i.ytimg.com/vi/vid123/maxresdefault.jpg", "width": 1280, "height": 720}
				]
			},
			{
				"id": "vid456",
				"title": "Vídeo Teste 2",
				"uploader": "Canal Uploader",
				"channel_id": "UC67890",
				"duration": 3600,
				"thumbnails": []
			},
			{
				"id": ""
			}
		]
	}`

	videos, err := parseFlatPlaylist([]byte(rawJSON))
	if err != nil {
		t.Fatalf("parseFlatPlaylist failed: %v", err)
	}

	if len(videos) != 2 {
		t.Fatalf("expected 2 videos, got %d", len(videos))
	}

	v1 := videos[0]
	if v1.ID != "vid123" || v1.Title != "Vídeo Teste 1" || v1.ChannelTitle != "Canal Legal" || v1.ChannelID != "UC12345" {
		t.Errorf("unexpected video 1 fields: %+v", v1)
	}
	if v1.Duration != 125*time.Second {
		t.Errorf("expected 125s, got %v", v1.Duration)
	}
	if v1.ViewCount != 12345 || v1.DescriptionExcerpt != "Descrição" || v1.ExternalURL == "" {
		t.Errorf("metadata flat incompleta: %+v", v1)
	}
	if v1.PublishedAt.IsZero() {
		t.Error("timestamp não foi convertido")
	}
	if v1.ThumbnailURL != "https://i.ytimg.com/vi/vid123/hqdefault.jpg" {
		t.Errorf("expected hqdefault thumb, got %s", v1.ThumbnailURL)
	}

	v2 := videos[1]
	if v2.ID != "vid456" || v2.Title != "Vídeo Teste 2" || v2.ChannelTitle != "Canal Uploader" {
		t.Errorf("unexpected video 2 fields: %+v", v2)
	}
	if v2.ThumbnailURL != "https://i.ytimg.com/vi/vid456/mqdefault.jpg" {
		t.Errorf("expected default fallback thumb, got %s", v2.ThumbnailURL)
	}
}

func TestParseFlatPlaylist_InvalidJSON(t *testing.T) {
	_, err := parseFlatPlaylist([]byte("invalid json"))
	if err == nil {
		t.Fatal("expected error on invalid json, got nil")
	}
}
