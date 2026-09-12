package iptv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCatalogSyncFetchesParsesAndPersists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("#EXTM3U\n#EXTINF:-1 group-title=\"TV\",Fixture\nhttps://example.invalid/live.m3u8\n"))
	}))
	defer server.Close()

	when := time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC)
	store := &recordingCatalogStore{}
	sync := CatalogSync{
		Source: HTTPM3USource{Client: server.Client()},
		Store:  store,
		Now:    func() time.Time { return when },
	}
	playlist, err := sync.SyncM3U(context.Background(), SourceConfig{
		ID: "fixture", Name: "Fixture", PlaylistURL: server.URL,
	})
	if err != nil {
		t.Fatalf("SyncM3U() error = %v", err)
	}
	if len(playlist.Items) != 1 || len(store.playlist.Items) != 1 {
		t.Fatalf("playlist=%+v store=%+v", playlist, store.playlist)
	}
	if store.syncedAt != when || store.source.ID != "fixture" {
		t.Fatalf("store recebeu source=%+v syncedAt=%v", store.source, store.syncedAt)
	}
}

type recordingCatalogStore struct {
	source   SourceConfig
	playlist Playlist
	syncedAt time.Time
}

func (store *recordingCatalogStore) ReplaceIPTVCatalog(_ context.Context, source SourceConfig, playlist Playlist, syncedAt time.Time) error {
	store.source = source
	store.playlist = playlist
	store.syncedAt = syncedAt
	return nil
}
