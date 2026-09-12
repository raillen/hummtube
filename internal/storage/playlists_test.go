package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func openTestRepo(t *testing.T) *Repository {
	t.Helper()
	db, err := OpenMemory(context.Background())
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return &Repository{db: db}
}

func seedPlaylistVideos(t *testing.T, r *Repository, ids ...string) {
	t.Helper()
	now := fmtTime(time.Now())
	for _, id := range ids {
		if _, err := r.db.Exec(
			`INSERT INTO videos (id, title, published_at, first_seen_at, last_seen_at)
			 VALUES (?, ?, ?, ?, ?)`, id, "Vídeo "+id, now, now, now); err != nil {
			t.Fatalf("seed video %s: %v", id, err)
		}
	}
}

func TestPlaylistCRUD(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	p, err := r.CreatePlaylist(ctx, "  Favoritos de testes  ", "lista legal", "#e91e63")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if p.Name != "Favoritos de testes" {
		t.Errorf("name = %q", p.Name)
	}
	if !strings.HasPrefix(p.ID, "pl_") {
		t.Errorf("id = %q, quer prefixo pl_", p.ID)
	}

	list, err := r.Playlists(ctx)
	if err != nil {
		t.Fatalf("Playlists: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Favoritos de testes" {
		t.Fatalf("list = %+v", list)
	}

	if err := r.UpdatePlaylist(ctx, p.ID, "Renomeada", "nova desc", "#2196f3"); err != nil {
		t.Fatalf("UpdatePlaylist: %v", err)
	}
	// Campos vazios não apagam os atuais.
	if err := r.UpdatePlaylist(ctx, p.ID, "", "", ""); err != nil {
		t.Fatalf("UpdatePlaylist vazio: %v", err)
	}
	list, _ = r.Playlists(ctx)
	if list[0].Name != "Renomeada" || list[0].Description != "nova desc" || list[0].Color != "#2196f3" {
		t.Fatalf("após update: %+v", list[0])
	}

	if err := r.DeletePlaylist(ctx, p.ID); err != nil {
		t.Fatalf("DeletePlaylist: %v", err)
	}
	list, _ = r.Playlists(ctx)
	if len(list) != 0 {
		t.Fatalf("list após delete = %d", len(list))
	}
}

func TestPlaylistCreateRejectsEmptyName(t *testing.T) {
	r := openTestRepo(t)
	if _, err := r.CreatePlaylist(context.Background(), "   ", "", ""); err == nil {
		t.Error("aceitou nome vazio")
	}
}

func TestPlaylistItemsOrderAndDedup(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1", "v2", "v3")

	p, _ := r.CreatePlaylist(ctx, "Playlist", "", "")
	for _, id := range []string{"v1", "v2", "v3", "v1"} {
		if err := r.AddPlaylistItem(ctx, p.ID, id); err != nil {
			t.Fatalf("AddPlaylistItem %s: %v", id, err)
		}
	}

	videos, err := r.PlaylistVideos(ctx, p.ID)
	if err != nil {
		t.Fatalf("PlaylistVideos: %v", err)
	}
	if len(videos) != 3 {
		t.Fatalf("len = %d, quer 3 (dedup)", len(videos))
	}
	want := []string{"v1", "v2", "v3"}
	for i, v := range videos {
		if v.ID != want[i] {
			t.Errorf("pos %d = %s, quer %s", i, v.ID, want[i])
		}
	}

	n, _ := r.PlaylistItemCount(ctx, p.ID)
	if n != 3 {
		t.Errorf("item count = %d", n)
	}
}

func TestPlaylistMoveAndRemove(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1", "v2", "v3")

	p, _ := r.CreatePlaylist(ctx, "Playlist", "", "")
	for _, id := range []string{"v1", "v2", "v3"} {
		_ = r.AddPlaylistItem(ctx, p.ID, id)
	}

	// Move v3 (pos 2) uma posição para cima → v1, v3, v2
	if err := r.MovePlaylistItem(ctx, p.ID, "v3", -1); err != nil {
		t.Fatalf("MovePlaylistItem: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, p.ID, "v1", "v3", "v2")

	// Move v3 (pos 1) duas para baixo → v1, v2, v3 (clampa no fim)
	if err := r.MovePlaylistItem(ctx, p.ID, "v3", 2); err != nil {
		t.Fatalf("MovePlaylistItem: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, p.ID, "v1", "v2", "v3")

	// Move v1 (pos 0) para baixo → v2, v1, v3
	if err := r.MovePlaylistItem(ctx, p.ID, "v1", 1); err != nil {
		t.Fatalf("MovePlaylistItem: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, p.ID, "v2", "v1", "v3")

	// Remove v1 (pos 1): o buraco é fechado → v2, v3
	if err := r.RemovePlaylistItem(ctx, p.ID, "v1"); err != nil {
		t.Fatalf("RemovePlaylistItem: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, p.ID, "v2", "v3")

	// Remover o que não existe não é erro.
	if err := r.RemovePlaylistItem(ctx, p.ID, "v1"); err != nil {
		t.Fatalf("RemovePlaylistItem repetido: %v", err)
	}
}

func assertPlaylistOrder(t *testing.T, r *Repository, ctx context.Context, id string, want ...string) {
	t.Helper()
	videos, err := r.PlaylistVideos(ctx, id)
	if err != nil {
		t.Fatalf("PlaylistVideos: %v", err)
	}
	if len(videos) != len(want) {
		t.Fatalf("len = %d, quer %v", len(videos), want)
	}
	for i, w := range want {
		if videos[i].ID != w {
			t.Errorf("pos %d = %s, quer %s", i, videos[i].ID, w)
		}
	}
}

func TestPlaylistDuplicate(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1", "v2")

	p, _ := r.CreatePlaylist(ctx, "Original", "desc", "#111111")
	_ = r.AddPlaylistItem(ctx, p.ID, "v1")
	_ = r.AddPlaylistItem(ctx, p.ID, "v2")

	dup, err := r.DuplicatePlaylist(ctx, p.ID, "Cópia")
	if err != nil {
		t.Fatalf("DuplicatePlaylist: %v", err)
	}
	if dup.ID == p.ID {
		t.Error("id duplicado igual ao original")
	}
	assertPlaylistOrder(t, r, ctx, dup.ID, "v1", "v2")
	if dup.Name != "Cópia" || dup.Description != "desc" || dup.Color != "#111111" {
		t.Errorf("dup = %+v", dup)
	}
	if _, err := r.DuplicatePlaylist(ctx, p.ID, "   "); err == nil {
		t.Error("duplicar com nome vazio deveria falhar")
	}
}

func TestPlaylistItemsSurviveCatalogRemoval(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1", "v2")

	p, _ := r.CreatePlaylist(ctx, "Playlist", "", "")
	_ = r.AddPlaylistItem(ctx, p.ID, "v1")
	_ = r.AddPlaylistItem(ctx, p.ID, "v2")

	// Simula o vídeo saindo do catálogo.
	if _, err := r.db.Exec(`DELETE FROM videos WHERE id = 'v1'`); err != nil {
		t.Fatalf("remover vídeo: %v", err)
	}
	videos, _ := r.PlaylistVideos(ctx, p.ID)
	if len(videos) != 1 || videos[0].ID != "v2" {
		t.Fatalf("videos = %+v", videos)
	}
	n, _ := r.PlaylistItemCount(ctx, p.ID)
	if n != 2 {
		t.Errorf("item count = %d, item deve persistir", n)
	}
	// Se o vídeo voltar, reaparece na ordem original.
	seedPlaylistVideos(t, r, "v1")
	videos, _ = r.PlaylistVideos(ctx, p.ID)
	if len(videos) != 2 || videos[0].ID != "v1" {
		t.Fatalf("após re-seed = %+v", videos)
	}
}

func TestPlaylistsContaining(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1")

	a, _ := r.CreatePlaylist(ctx, "A", "", "")
	b, _ := r.CreatePlaylist(ctx, "B", "", "")
	_ = r.AddPlaylistItem(ctx, a.ID, "v1")
	_ = r.AddPlaylistItem(ctx, b.ID, "v1")

	containing, err := r.PlaylistsContaining(ctx, "v1")
	if err != nil {
		t.Fatalf("PlaylistsContaining: %v", err)
	}
	if !containing[a.ID] || !containing[b.ID] {
		t.Errorf("containing = %v", containing)
	}
	other, _ := r.PlaylistsContaining(ctx, "v2")
	if len(other) != 0 {
		t.Errorf("v2 contido em %v", other)
	}

	membership, err := r.PlaylistMembership(ctx)
	if err != nil {
		t.Fatalf("PlaylistMembership: %v", err)
	}
	if !membership["v1"][a.ID] || !membership["v1"][b.ID] {
		t.Errorf("membership = %v", membership)
	}
}

func TestPlaylistDeleteCascades(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1")

	p, _ := r.CreatePlaylist(ctx, "Playlist", "", "")
	_ = r.AddPlaylistItem(ctx, p.ID, "v1")
	if err := r.DeletePlaylist(ctx, p.ID); err != nil {
		t.Fatalf("DeletePlaylist: %v", err)
	}
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM playlist_items WHERE playlist_id = ?`, p.ID).Scan(&n); err != nil {
		t.Fatalf("contar itens órfãos: %v", err)
	}
	if n != 0 {
		t.Errorf("itens órfãos = %d", n)
	}
}

// Garante que *Repository satisfaz a porta PlaylistStore.
var _ domain.PlaylistStore = (*Repository)(nil)

// seedPlaylistRich insere um vídeo com metadata completa para os filtros do
// PLY-04 (duração, canal, categoria, live, published_at). O canal é criado na
// tabela channels quando channelID não é vazio; canais repetidos são mantidos
// (INSERT OR IGNORE).
func seedPlaylistRich(t *testing.T, r *Repository, id, channelID, title, category string, duration time.Duration, published time.Time, liveStatus string) {
	t.Helper()
	if channelID != "" {
		if _, err := r.db.Exec(
			`INSERT OR IGNORE INTO channels (id, title) VALUES (?, ?)`, channelID, "Canal "+channelID); err != nil {
			t.Fatalf("seed channel %s: %v", channelID, err)
		}
	}
	now := fmtTime(time.Now())
	if _, err := r.db.Exec(
		`INSERT INTO videos (id, channel_id, title, category, duration, live_status, published_at, first_seen_at, last_seen_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, channelID, title, category, int64(duration/time.Second), liveStatus, fmtTime(published), now, now); err != nil {
		t.Fatalf("seed video %s: %v", id, err)
	}
}

func TestPlaylistVideosFilteredCombinations(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	// Dois canais, durações e categorias distintas para combinar filtros.
	now := time.Now()
	seedPlaylistRich(t, r, "v1", "c1", "Alpha tutorial", "Educação", 2*time.Minute, now.Add(-2*24*time.Hour), "")
	seedPlaylistRich(t, r, "v2", "c1", "Beta live", "Música", 10*time.Minute, now.Add(-48*24*time.Hour), "live")
	seedPlaylistRich(t, r, "v3", "c2", "Gamma documentário", "Educação", 30*time.Minute, now.Add(-10*24*time.Hour), "")
	// v1 é favorito e assistido (completado).
	if _, err := r.db.Exec(`INSERT INTO favorites (video_id, created_at) VALUES ('v1', ?)`, fmtTime(now)); err != nil {
		t.Fatalf("favoritar v1: %v", err)
	}
	if _, err := r.db.Exec(`INSERT INTO playback_progress (video_id, position_ms, duration_ms, updated_at, completed)
		VALUES ('v1', 120000, 120000, ?, 1)`, fmtTime(now)); err != nil {
		t.Fatalf("progresso v1: %v", err)
	}

	p, _ := r.CreatePlaylist(ctx, "Playlist", "", "")
	for _, id := range []string{"v1", "v2", "v3"} {
		_ = r.AddPlaylistItem(ctx, p.ID, id)
	}

	cases := []struct {
		name   string
		filter domain.PlaylistFilter
		want   []string
	}{
		{"zero", domain.PlaylistFilter{}, []string{"v1", "v2", "v3"}},
		{"canal", domain.PlaylistFilter{Channel: "canal c1"}, []string{"v1", "v2"}},
		{"duracao short", domain.PlaylistFilter{Duration: domain.SearchDurationShort}, []string{"v1"}},
		{"duracao media", domain.PlaylistFilter{Duration: domain.SearchDurationMedium}, []string{"v2"}},
		{"duracao longa", domain.PlaylistFilter{Duration: domain.SearchDurationLong}, []string{"v3"}},
		{"duracao custom", domain.PlaylistFilter{Duration: domain.SearchDurationCustom, MinDuration: 5 * time.Minute, MaxDuration: 20 * time.Minute}, []string{"v2"}},
		{"idade semana", domain.PlaylistFilter{Age: domain.FeedAgeWeek}, []string{"v1"}},
		{"idade mes exclui v2", domain.PlaylistFilter{Age: domain.FeedAgeMonth}, []string{"v1", "v3"}},
		{"categoria", domain.PlaylistFilter{Category: "educação"}, []string{"v1", "v3"}},
		{"favorito", domain.PlaylistFilter{Favorite: true}, []string{"v1"}},
		{"assistido", domain.PlaylistFilter{Watched: domain.SearchWatchWatched}, []string{"v1"}},
		{"nao assistido", domain.PlaylistFilter{Watched: domain.SearchWatchUnwatched}, []string{"v2", "v3"}},
		{"conteudo live", domain.PlaylistFilter{Content: domain.FeedLive}, []string{"v2"}},
		{"conteudo regular", domain.PlaylistFilter{Content: domain.FeedRegular}, []string{"v1", "v3"}},
		{"shorts", domain.PlaylistFilter{ShortsOnly: true}, []string{"v1"}},
		{"combinado canal+duracao", domain.PlaylistFilter{Channel: "canal c1", Duration: domain.SearchDurationShort}, []string{"v1"}},
		{"combinado categoria+favorito", domain.PlaylistFilter{Category: "educação", Favorite: true}, []string{"v1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.PlaylistVideosFiltered(ctx, p.ID, tc.filter, "")
			if err != nil {
				t.Fatalf("PlaylistVideosFiltered: %v", err)
			}
			var ids []string
			for _, v := range got {
				ids = append(ids, v.ID)
			}
			if strings.Join(ids, ",") != strings.Join(tc.want, ",") {
				t.Errorf("ids = %v, quer %v", ids, tc.want)
			}
		})
	}
}

func TestPlaylistVideosFilteredSortOrders(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	now := time.Now()
	seedPlaylistRich(t, r, "v1", "c1", "Charlie", "A", 10*time.Minute, now.Add(-5*24*time.Hour), "")
	seedPlaylistRich(t, r, "v2", "c2", "Alpha", "B", 2*time.Minute, now.Add(-2*24*time.Hour), "")
	seedPlaylistRich(t, r, "v3", "c1", "Bravo", "C", 20*time.Minute, now.Add(-10*24*time.Hour), "")
	p, _ := r.CreatePlaylist(ctx, "Playlist", "", "")
	for _, id := range []string{"v1", "v2", "v3"} {
		_ = r.AddPlaylistItem(ctx, p.ID, id)
	}

	cases := []struct {
		key  domain.PlaylistSortKey
		want []string
	}{
		{"", []string{"v1", "v2", "v3"}},
		{domain.PlaylistSortManual, []string{"v1", "v2", "v3"}},
		{domain.PlaylistSortTitle, []string{"v2", "v3", "v1"}},
		{domain.PlaylistSortChannel, []string{"v1", "v3", "v2"}},   // c1 (2 itens), c2
		{domain.PlaylistSortPublished, []string{"v2", "v1", "v3"}}, // mais recente primeiro
		{domain.PlaylistSortDuration, []string{"v2", "v1", "v3"}},
	}
	for _, tc := range cases {
		got, err := r.PlaylistVideosFiltered(ctx, p.ID, domain.PlaylistFilter{}, tc.key)
		if err != nil {
			t.Fatalf("PlaylistVideosFiltered(%s): %v", tc.key, err)
		}
		var ids []string
		for _, v := range got {
			ids = append(ids, v.ID)
		}
		if strings.Join(ids, ",") != strings.Join(tc.want, ",") {
			t.Errorf("sort %s: ids = %v, quer %v", tc.key, ids, tc.want)
		}
	}
}

func TestPlaylistVideosFilteredIgnoresMissingVideos(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1", "v2")
	p, _ := r.CreatePlaylist(ctx, "Playlist", "", "")
	_ = r.AddPlaylistItem(ctx, p.ID, "v1")
	_ = r.AddPlaylistItem(ctx, p.ID, "v2")
	// v2 some do catálogo; o item persiste, mas a listagem ignora.
	if _, err := r.db.Exec(`DELETE FROM videos WHERE id = 'v2'`); err != nil {
		t.Fatalf("remover v2: %v", err)
	}
	got, err := r.PlaylistVideosFiltered(ctx, p.ID, domain.PlaylistFilter{}, domain.PlaylistSortTitle)
	if err != nil {
		t.Fatalf("PlaylistVideosFiltered: %v", err)
	}
	if len(got) != 1 || got[0].ID != "v1" {
		t.Fatalf("got = %+v, quer só v1", got)
	}
}

func TestPlaylistVideosFilteredRejectsBadSort(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1")
	p, _ := r.CreatePlaylist(ctx, "Playlist", "", "")
	_ = r.AddPlaylistItem(ctx, p.ID, "v1")
	if _, err := r.PlaylistVideosFiltered(ctx, p.ID, domain.PlaylistFilter{}, domain.PlaylistSortKey("nope")); err == nil {
		t.Fatal("chave de ordenação inválida deveria falhar")
	}
}

func TestChannelNames(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistRich(t, r, "v1", "c1", "A", "", 0, time.Time{}, "")
	seedPlaylistRich(t, r, "v2", "c2", "B", "", 0, time.Time{}, "")
	names, err := r.ChannelNames(ctx, []string{"c1", "c2", "missing"})
	if err != nil {
		t.Fatalf("ChannelNames: %v", err)
	}
	if names["c1"] != "Canal c1" || names["c2"] != "Canal c2" {
		t.Errorf("names = %v", names)
	}
	if _, ok := names["missing"]; ok {
		t.Errorf("canal inexistente não deveria estar no mapa")
	}
	empty, _ := r.ChannelNames(ctx, nil)
	if len(empty) != 0 {
		t.Errorf("ids vazios deveriam devolver mapa vazio, veio %v", empty)
	}
}

func TestPlaylistBatchCopyMoveRemove(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1", "v2", "v3")

	a, _ := r.CreatePlaylist(ctx, "A", "", "")
	b, _ := r.CreatePlaylist(ctx, "B", "", "")
	// A: v1, v2, v3. B: v2 (para testar dedup no destino).
	for _, id := range []string{"v1", "v2", "v3"} {
		_ = r.AddPlaylistItem(ctx, a.ID, id)
	}
	_ = r.AddPlaylistItem(ctx, b.ID, "v2")

	// Copiar v1 e v2 para B: v2 não duplica.
	if err := r.CopyPlaylistItems(ctx, a.ID, b.ID, []string{"v1", "v2"}); err != nil {
		t.Fatalf("CopyPlaylistItems: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, b.ID, "v2", "v1")
	// A intacta.
	assertPlaylistOrder(t, r, ctx, a.ID, "v1", "v2", "v3")

	// Mover v1 e v3 para B. v1 já está em B (dedup mantém a posição atual em
	// B); v3 é acrescentado ao fim.
	if err := r.MovePlaylistItems(ctx, a.ID, b.ID, []string{"v1", "v3"}); err != nil {
		t.Fatalf("MovePlaylistItems: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, a.ID, "v2")
	assertPlaylistOrder(t, r, ctx, b.ID, "v2", "v1", "v3")

	// Remover v2 e v3 de B em lote, com buracos fechados.
	if err := r.RemovePlaylistItems(ctx, b.ID, []string{"v2", "v3"}); err != nil {
		t.Fatalf("RemovePlaylistItems: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, b.ID, "v1")
}

func TestPlaylistBatchOpsAreAtomicOnIds(t *testing.T) {
	// ids vazios não podem corromper nada.
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1")
	a, _ := r.CreatePlaylist(ctx, "A", "", "")
	b, _ := r.CreatePlaylist(ctx, "B", "", "")
	_ = r.AddPlaylistItem(ctx, a.ID, "v1")

	if err := r.CopyPlaylistItems(ctx, a.ID, b.ID, nil); err == nil {
		t.Fatal("copiar sem ids deveria falhar")
	}
	if err := r.MovePlaylistItems(ctx, a.ID, b.ID, []string{}); err == nil {
		t.Fatal("mover sem ids deveria falhar")
	}
	if err := r.RemovePlaylistItems(ctx, a.ID, nil); err == nil {
		t.Fatal("remover sem ids deveria falhar")
	}
	assertPlaylistOrder(t, r, ctx, a.ID, "v1")
	assertPlaylistOrder(t, r, ctx, b.ID)
}

func TestPlaylistMoveItemToIndex(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()
	seedPlaylistVideos(t, r, "v1", "v2", "v3")

	p, _ := r.CreatePlaylist(ctx, "Playlist", "", "")
	for _, id := range []string{"v1", "v2", "v3"} {
		_ = r.AddPlaylistItem(ctx, p.ID, id)
	}

	// v1 para o índice 2 → v2, v3, v1
	if err := r.MovePlaylistItemToIndex(ctx, p.ID, "v1", 2); err != nil {
		t.Fatalf("MovePlaylistItemToIndex: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, p.ID, "v2", "v3", "v1")

	// v1 para o índice 0 (voltar ao início) → v1, v2, v3
	if err := r.MovePlaylistItemToIndex(ctx, p.ID, "v1", 0); err != nil {
		t.Fatalf("MovePlaylistItemToIndex: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, p.ID, "v1", "v2", "v3")

	// Índice acima do limite clampa no fim.
	if err := r.MovePlaylistItemToIndex(ctx, p.ID, "v1", 99); err != nil {
		t.Fatalf("MovePlaylistItemToIndex clamp: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, p.ID, "v2", "v3", "v1")

	// Mesmo índice é um no-op.
	if err := r.MovePlaylistItemToIndex(ctx, p.ID, "v2", 0); err != nil {
		t.Fatalf("MovePlaylistItemToIndex no-op: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, p.ID, "v2", "v3", "v1")

	// Item que não está na playlist não quebra nada.
	if err := r.MovePlaylistItemToIndex(ctx, p.ID, "v999", 0); err != nil {
		t.Fatalf("MovePlaylistItemToIndex ausente: %v", err)
	}
	assertPlaylistOrder(t, r, ctx, p.ID, "v2", "v3", "v1")
}
