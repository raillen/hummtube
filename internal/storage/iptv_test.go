package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/iptv"
)

func TestReplaceIPTVCatalogPersistsMetadataWithoutStreamURL(t *testing.T) {
	r := newTestRepo(t)
	source := iptv.SourceConfig{
		ID:          "fixture",
		Name:        "Fixture",
		Format:      iptv.SourceFormatM3U,
		PlaylistURL: "https://example.invalid/playlist.m3u?type=m3u_plus",
		Enabled:     true,
	}
	playlist := iptv.Playlist{Items: []iptv.Item{{
		ID: "movie-1", SourceID: "fixture", Kind: iptv.ContentKindMovie,
		Classification: iptv.ClassificationGroup, Title: "Filme fixture",
		RawTitle: "Filme fixture", Group: "Filmes",
		StreamURL: "https://user:secret@example.invalid/movie.mp4",
	}}}
	when := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	if err := r.ReplaceIPTVCatalog(context.Background(), source, playlist, when); err != nil {
		t.Fatalf("ReplaceIPTVCatalog: %v", err)
	}

	items, err := r.ListIPTVItems(context.Background(), "fixture", iptv.ContentKindMovie, 10)
	if err != nil {
		t.Fatalf("ListIPTVItems: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Filme fixture" {
		t.Fatalf("items = %+v", items)
	}
	if items[0].StreamURL != "" {
		t.Fatalf("StreamURL persistida: %q", items[0].StreamURL)
	}
	var endpoint string
	if err := r.DB().QueryRow(`SELECT playlist_endpoint FROM iptv_sources WHERE id = ?`, "fixture").Scan(&endpoint); err != nil {
		t.Fatalf("source endpoint: %v", err)
	}
	if endpoint == "" {
		t.Fatal("endpoint público não foi persistido")
	}
}

func TestImportIPTVCatalogUpsertsDuplicateIDsAcrossBatches(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	source := iptv.SourceConfig{ID: "fixture", Name: "Fixture", Enabled: true}
	err := r.ImportIPTVCatalog(ctx, source, time.Now(), func(yield func([]iptv.Item) error) error {
		if err := yield([]iptv.Item{{ID: "duplicate", SourceID: source.ID, Kind: iptv.ContentKindUnknown, Title: "Primeiro"}}); err != nil {
			return err
		}
		return yield([]iptv.Item{{ID: "duplicate", SourceID: source.ID, Kind: iptv.ContentKindMovie, Title: "Último"}})
	})
	if err != nil {
		t.Fatalf("ImportIPTVCatalog: %v", err)
	}
	items, err := r.ListIPTVItems(ctx, source.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Último" || items[0].Kind != iptv.ContentKindMovie {
		t.Fatalf("catálogo deduplicado = %+v", items)
	}
}

func TestReplaceIPTVCatalogRemovesObsoleteItems(t *testing.T) {
	r := newTestRepo(t)
	source := iptv.SourceConfig{ID: "fixture", Name: "Fixture", Enabled: true}
	first := iptv.Playlist{Items: []iptv.Item{
		{ID: "one", SourceID: "fixture", Kind: iptv.ContentKindTV, Title: "Um"},
		{ID: "two", SourceID: "fixture", Kind: iptv.ContentKindTV, Title: "Dois"},
	}}
	second := iptv.Playlist{Items: []iptv.Item{{ID: "two", SourceID: "fixture", Kind: iptv.ContentKindTV, Title: "Dois atualizado"}}}
	if err := r.ReplaceIPTVCatalog(context.Background(), source, first, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceIPTVCatalog(context.Background(), source, second, time.Now()); err != nil {
		t.Fatal(err)
	}
	items, err := r.ListIPTVItems(context.Background(), "fixture", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "two" || items[0].Title != "Dois atualizado" {
		t.Fatalf("items = %+v", items)
	}
}

func TestReplaceIPTVCatalogRejectsInlineCredentials(t *testing.T) {
	r := newTestRepo(t)
	err := r.ReplaceIPTVCatalog(context.Background(), iptv.SourceConfig{
		ID: "fixture", Name: "Fixture", PlaylistURL: "https://example.invalid/get?username=u&password=p",
	}, iptv.Playlist{}, time.Now())
	if !errors.Is(err, ErrSensitiveEndpoint) {
		t.Fatalf("erro = %v, want ErrSensitiveEndpoint", err)
	}
	var count int
	if err := r.DB().QueryRow(`SELECT COUNT(*) FROM iptv_sources`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("fonte parcialmente persistida: %d", count)
	}
}

func TestReplaceIPTVGuideAndCurrentProgram(t *testing.T) {
	r := newTestRepo(t)
	source := iptv.SourceConfig{ID: "fixture", Name: "Fixture", Enabled: true}
	if err := r.ReplaceIPTVCatalog(context.Background(), source, iptv.Playlist{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	channels := []iptv.GuideChannel{{ID: "news", Name: "Notícias"}}
	programs := []iptv.GuideProgram{{ChannelID: "news", Title: "Jornal", Start: base.Add(-time.Minute), End: base.Add(time.Hour)}}
	if err := r.ReplaceIPTVGuide(context.Background(), "fixture", channels, programs); err != nil {
		t.Fatalf("ReplaceIPTVGuide: %v", err)
	}
	got, err := r.CurrentIPTVPrograms(context.Background(), "fixture", "news", base)
	if err != nil {
		t.Fatalf("CurrentIPTVPrograms: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Jornal" {
		t.Fatalf("programs = %+v", got)
	}
}

func TestIPTVSourceLifecycleAndCatalogSearch(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	source := iptv.SourceConfig{
		ID: "fixture", Name: "Fonte Fixture", Format: iptv.SourceFormatM3U,
		PlaylistURL: "https://example.invalid/playlist.m3u", Enabled: true,
	}
	if err := r.SaveIPTVSource(ctx, source); err != nil {
		t.Fatalf("SaveIPTVSource: %v", err)
	}
	if err := r.ReplaceIPTVCatalog(ctx, source, iptv.Playlist{Items: []iptv.Item{
		{ID: "movie", SourceID: "fixture", Kind: iptv.ContentKindMovie, Title: "O Filme Fixture", Group: "Filmes"},
		{ID: "news", SourceID: "fixture", Kind: iptv.ContentKindTV, Title: "Notícias Fixture", Group: "TV"},
	}}, time.Now()); err != nil {
		t.Fatalf("ReplaceIPTVCatalog: %v", err)
	}
	sources, err := r.ListIPTVSources(ctx)
	if err != nil || len(sources) != 1 || sources[0].Config.Name != source.Name {
		t.Fatalf("sources=%+v err=%v", sources, err)
	}
	items, err := r.SearchIPTVItems(ctx, "notícias", iptv.ContentKindTV, 10)
	if err != nil || len(items) != 1 || items[0].ID != "news" {
		t.Fatalf("search=%+v err=%v", items, err)
	}
	if err := r.SetIPTVSourceError(ctx, "fixture", "falha em https://user:secret@example.invalid/x"); err != nil {
		t.Fatalf("SetIPTVSourceError: %v", err)
	}
	var lastError string
	if err := r.DB().QueryRow(`SELECT last_error FROM iptv_sources WHERE id = 'fixture'`).Scan(&lastError); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(lastError, "https://") || strings.Contains(lastError, "secret") {
		t.Fatalf("erro sensível persistido: %q", lastError)
	}
	if err := r.DeleteIPTVSource(ctx, "fixture"); err != nil {
		t.Fatalf("DeleteIPTVSource: %v", err)
	}
	items, err = r.ListAllIPTVItems(ctx, "", 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("itens após delete=%+v err=%v", items, err)
	}
}

func TestIPTVSavedItemsStayLocalAndJoinCurrentCatalog(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	source := iptv.SourceConfig{ID: "fixture", Name: "Fixture", Enabled: true}
	if err := r.ReplaceIPTVCatalog(ctx, source, iptv.Playlist{Items: []iptv.Item{
		{ID: "movie", SourceID: "fixture", Kind: iptv.ContentKindMovie, Title: "Filme salvo"},
	}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.SetIPTVItemSaved(ctx, "movie", true); err != nil {
		t.Fatalf("salvar item: %v", err)
	}
	items, err := r.ListIPTVSavedItems(ctx, 10)
	if err != nil || len(items) != 1 || items[0].ID != "movie" {
		t.Fatalf("saved=%+v err=%v", items, err)
	}
	if err := r.SetIPTVItemSaved(ctx, "movie", false); err != nil {
		t.Fatalf("remover item: %v", err)
	}
	items, err = r.ListIPTVSavedItems(ctx, 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("saved após remoção=%+v err=%v", items, err)
	}
	if err := r.SetIPTVItemSaved(ctx, "movie", true); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceIPTVCatalog(ctx, source, iptv.Playlist{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	items, err = r.ListIPTVSavedItems(ctx, 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("stale saved=%+v err=%v", items, err)
	}
}

func TestListIPTVGuideWindow(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	if err := r.ReplaceIPTVCatalog(ctx, iptv.SourceConfig{ID: "fixture", Name: "Fixture", Enabled: true}, iptv.Playlist{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	if err := r.ReplaceIPTVGuide(ctx, "fixture", []iptv.GuideChannel{{ID: "news", Name: "Notícias"}}, []iptv.GuideProgram{{
		ChannelID: "news", Title: "Jornal", Start: base, End: base.Add(time.Hour),
	}}); err != nil {
		t.Fatal(err)
	}
	entries, err := r.ListIPTVGuide(ctx, "fixture", base.Add(-time.Minute), base.Add(2*time.Hour), 10)
	if err != nil || len(entries) != 1 || entries[0].ChannelName != "Notícias" {
		t.Fatalf("entries=%+v err=%v", entries, err)
	}
}

func TestListIPTVGroupsAndPagination(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	source := iptv.SourceConfig{ID: "src1", Name: "Source 1", Enabled: true}

	items := []iptv.Item{
		{ID: "ch1", SourceID: "src1", Kind: iptv.ContentKindTV, Title: "Canal A", Group: "Abertos"},
		{ID: "ch2", SourceID: "src1", Kind: iptv.ContentKindTV, Title: "Canal B", Group: "Esportes"},
		{ID: "ch3", SourceID: "src1", Kind: iptv.ContentKindTV, Title: "Canal C", Group: "Esportes"},
		{ID: "m1", SourceID: "src1", Kind: iptv.ContentKindMovie, Title: "Filme 1", Group: "Ação"},
		{ID: "m2", SourceID: "src1", Kind: iptv.ContentKindMovie, Title: "Filme 2", Group: "Comédia"},
		{ID: "m3", SourceID: "src1", Kind: iptv.ContentKindMovie, Title: "Filme 3", Group: "Ação"},
		{ID: "s1", SourceID: "src1", Kind: iptv.ContentKindSeries, Title: "Anime 1", Group: "Animes"},
	}

	if err := r.ReplaceIPTVCatalog(ctx, source, iptv.Playlist{Items: items}, time.Now()); err != nil {
		t.Fatalf("ReplaceIPTVCatalog: %v", err)
	}

	tvGroups, err := r.ListIPTVGroups(ctx, iptv.ContentKindTV)
	if err != nil {
		t.Fatalf("ListIPTVGroups TV: %v", err)
	}
	if len(tvGroups) != 2 || tvGroups[0] != "Abertos" || tvGroups[1] != "Esportes" {
		t.Fatalf("tvGroups = %+v, want [Abertos Esportes]", tvGroups)
	}

	movieGroups, err := r.ListIPTVGroups(ctx, iptv.ContentKindMovie)
	if err != nil {
		t.Fatalf("ListIPTVGroups Movie: %v", err)
	}
	if len(movieGroups) != 2 || movieGroups[0] != "Ação" || movieGroups[1] != "Comédia" {
		t.Fatalf("movieGroups = %+v, want [Ação Comédia]", movieGroups)
	}

	// Test pagination
	res, err := r.ListIPTVItemsPaginated(ctx, iptv.ItemFilter{
		Kind:     iptv.ContentKindMovie,
		Page:     1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListIPTVItemsPaginated: %v", err)
	}
	if res.TotalCount != 3 || res.TotalPages != 2 || len(res.Items) != 2 {
		t.Fatalf("res page 1 = %+v", res)
	}

	resPage2, err := r.ListIPTVItemsPaginated(ctx, iptv.ItemFilter{
		Kind:     iptv.ContentKindMovie,
		Page:     2,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListIPTVItemsPaginated page 2: %v", err)
	}
	if resPage2.TotalCount != 3 || len(resPage2.Items) != 1 {
		t.Fatalf("res page 2 = %+v", resPage2)
	}

	// Test group filter
	resGroup, err := r.ListIPTVItemsPaginated(ctx, iptv.ItemFilter{
		Kind:     iptv.ContentKindTV,
		Group:    "Esportes",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListIPTVItemsPaginated with group: %v", err)
	}
	if resGroup.TotalCount != 2 || len(resGroup.Items) != 2 {
		t.Fatalf("resGroup = %+v", resGroup)
	}
}
