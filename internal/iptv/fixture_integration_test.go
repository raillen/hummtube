package iptv_test

import (
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/iptv"
	"github.com/nanotube/nanotube-web/internal/storage"
)

//go:embed testdata/fixture.m3u
var fixtureM3U []byte

func TestFixtureM3UEndToEndCatalogSync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = writer.Write(fixtureM3U)
	}))
	defer server.Close()

	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repository := storage.NewRepository(db)
	source := iptv.SourceConfig{
		ID: "fixture", Name: "Fixture local", Format: iptv.SourceFormatM3U,
		PlaylistURL: server.URL + "/fixture.m3u", Enabled: true,
	}

	result, err := (iptv.CatalogSync{
		Source: iptv.HTTPM3USource{Client: server.Client()},
		Store:  repository,
		Now:    func() time.Time { return time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC) },
	}).SyncM3U(ctx, source)
	if err != nil {
		t.Fatalf("SyncM3U: %v", err)
	}
	// Caminho de importação em lotes: o retorno carrega apenas avisos;
	// a persistência é verificada pelo catálogo gravado.
	if len(result.Warnings) != 0 {
		t.Fatalf("warnings = %+v, want nenhum", result.Warnings)
	}

	items, err := repository.ListAllIPTVItems(ctx, "", 10)
	if err != nil {
		t.Fatalf("ListAllIPTVItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("persisted items=%d, want 3", len(items))
	}
	if items[0].StreamURL != "" {
		t.Fatalf("StreamURL persistida: %q", items[0].StreamURL)
	}
	var seriesItem iptv.Item
	for _, item := range items {
		if item.Kind == iptv.ContentKindSeries {
			seriesItem = item
			break
		}
	}
	if seriesItem.Episode.Season != 1 || seriesItem.Episode.Episode != 2 {
		t.Fatalf("episódio não normalizado: %+v", seriesItem.Episode)
	}
}
