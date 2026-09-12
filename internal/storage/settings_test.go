package storage

import (
	"context"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/pressly/goose/v3"
)

func TestSettingRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)

	if _, ok, err := r.Setting(ctx, SettingMaxHeight); err != nil || ok {
		t.Fatalf("chave inexistente deveria devolver ok=false: ok=%v err=%v", ok, err)
	}
	if err := r.SetSetting(ctx, SettingMaxHeight, "720"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	value, ok, err := r.Setting(ctx, SettingMaxHeight)
	if err != nil || !ok || value != "720" {
		t.Fatalf("leitura: value=%q ok=%v err=%v", value, ok, err)
	}
	// Sobrescrever não duplica linha nem falha por conflito.
	if err := r.SetSetting(ctx, SettingMaxHeight, "480"); err != nil {
		t.Fatalf("SetSetting sobrescrita: %v", err)
	}
	if value, _, _ := r.Setting(ctx, SettingMaxHeight); value != "480" {
		t.Fatalf("sobrescrita não aplicada: %q", value)
	}
}

func TestProfileSettingIsolationAndForget(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	second, err := r.CreateProfile(ctx, "Segundo")
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	if err := r.SetProfileSetting(ctx, DefaultProfileID, SettingCookiesFrom, "firefox+gnomekeyring"); err != nil {
		t.Fatalf("SetProfileSetting default: %v", err)
	}
	if err := r.SetProfileSetting(ctx, second.ID, SettingCookiesFrom, "brave+kwallet"); err != nil {
		t.Fatalf("SetProfileSetting second: %v", err)
	}

	defaultValue, ok, err := r.ProfileSetting(ctx, DefaultProfileID, SettingCookiesFrom)
	if err != nil || !ok || defaultValue != "firefox+gnomekeyring" {
		t.Fatalf("setting default = %q/%v, err=%v", defaultValue, ok, err)
	}
	secondValue, ok, err := r.ProfileSetting(ctx, second.ID, SettingCookiesFrom)
	if err != nil || !ok || secondValue != "brave+kwallet" {
		t.Fatalf("setting second = %q/%v, err=%v", secondValue, ok, err)
	}

	if err := r.DeleteProfileSetting(ctx, second.ID, SettingCookiesFrom); err != nil {
		t.Fatalf("DeleteProfileSetting: %v", err)
	}
	if _, ok, err := r.ProfileSetting(ctx, second.ID, SettingCookiesFrom); err != nil || ok {
		t.Fatalf("setting removido = ok=%v err=%v", ok, err)
	}
}

func TestProfileSettingMigratesLegacyDefaultCookieSource(t *testing.T) {
	ctx := context.Background()
	db, err := OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	goose.SetBaseFS(migrationsFS)
	goose.SetDialect("sqlite3")
	goose.SetLogger(noopLogger{})
	if err := goose.UpTo(db, "migrations", 20); err != nil {
		t.Fatalf("migrate to v20: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?)`, SettingCookiesFrom, "firefox+gnomekeyring"); err != nil {
		t.Fatalf("insert legacy setting: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate v21: %v", err)
	}
	r := NewRepository(db)
	value, ok, err := r.ProfileSetting(ctx, DefaultProfileID, SettingCookiesFrom)
	if err != nil || !ok || value != "firefox+gnomekeyring" {
		t.Fatalf("setting migrado = %q/%v, err=%v", value, ok, err)
	}
	if _, ok, err := r.Setting(ctx, SettingCookiesFrom); err != nil || ok {
		t.Fatalf("setting global legado permaneceu = ok=%v err=%v", ok, err)
	}
}

func TestQueueOrderAndDedup(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	for _, id := range []string{"v3", "v1", "v2"} {
		if err := r.Enqueue(ctx, id); err != nil {
			t.Fatalf("Enqueue %s: %v", id, err)
		}
	}
	// Reenfileirar não duplica nem reordena.
	if err := r.Enqueue(ctx, "v3"); err != nil {
		t.Fatalf("Enqueue repetido: %v", err)
	}

	ids, err := r.QueuedIDs(ctx)
	if err != nil {
		t.Fatalf("QueuedIDs: %v", err)
	}
	want := []string{"v3", "v1", "v2"}
	if len(ids) != len(want) {
		t.Fatalf("fila com %d itens, esperado %d: %v", len(ids), len(want), ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ordem da fila: %v, esperado %v", ids, want)
		}
	}

	videos, err := r.QueuedVideos(ctx)
	if err != nil {
		t.Fatalf("QueuedVideos: %v", err)
	}
	if len(videos) != 3 || videos[0].ID != "v3" || videos[0].Title == "" {
		t.Fatalf("fila com metadata inesperada: %+v", videos)
	}
}

func TestDequeue(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	for _, id := range []string{"v1", "v2"} {
		if err := r.Enqueue(ctx, id); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}
	if err := r.Dequeue(ctx, "v1"); err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	ids, _ := r.QueuedIDs(ctx)
	if len(ids) != 1 || ids[0] != "v2" {
		t.Fatalf("após remover v1 a fila deveria ser [v2], veio %v", ids)
	}
	// Remover algo que não está na fila não é erro.
	if err := r.Dequeue(ctx, "inexistente"); err != nil {
		t.Fatalf("Dequeue de ausente deveria ser no-op: %v", err)
	}
}

func TestEnqueueRejectsEmptyID(t *testing.T) {
	if err := newTestRepo(t).Enqueue(context.Background(), ""); err == nil {
		t.Fatal("id vazio deveria ser recusado")
	}
}

func TestEnqueuePlaylistAppendsInOrder(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	p, err := r.CreatePlaylist(ctx, "P", "", "")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	// Ordem da playlist intencionalmente diferente da de criação.
	for _, id := range []string{"v3", "v1", "v2"} {
		if err := r.AddPlaylistItem(ctx, p.ID, id); err != nil {
			t.Fatalf("AddPlaylistItem: %v", err)
		}
	}
	// v1 já está na fila antes: não pode duplicar.
	if err := r.Enqueue(ctx, "v1"); err != nil {
		t.Fatalf("Enqueue v1: %v", err)
	}

	if err := r.EnqueuePlaylist(ctx, p.ID); err != nil {
		t.Fatalf("EnqueuePlaylist: %v", err)
	}
	ids, err := r.QueuedIDs(ctx)
	if err != nil {
		t.Fatalf("QueuedIDs: %v", err)
	}
	want := []string{"v1", "v3", "v2"}
	if len(ids) != len(want) {
		t.Fatalf("fila = %v, esperado %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("fila = %v, esperado %v", ids, want)
		}
	}

	// Reenfileirar a mesma playlist não duplica nada.
	if err := r.EnqueuePlaylist(ctx, p.ID); err != nil {
		t.Fatalf("EnqueuePlaylist repetida: %v", err)
	}
	if n := len(mustQueueIDs(t, ctx, r)); n != 3 {
		t.Fatalf("fila com %d itens após repetir, esperado 3", n)
	}
	// Playlist vazia é um no-op.
	empty, _ := r.CreatePlaylist(ctx, "Vazia", "", "")
	if err := r.EnqueuePlaylist(ctx, empty.ID); err != nil {
		t.Fatalf("EnqueuePlaylist vazia: %v", err)
	}
	if err := r.EnqueuePlaylist(ctx, ""); err == nil {
		t.Fatal("id vazio deveria ser recusado")
	}
}

func mustQueueIDs(t *testing.T, ctx context.Context, r *Repository) []string {
	t.Helper()
	ids, err := r.QueuedIDs(ctx)
	if err != nil {
		t.Fatalf("QueuedIDs: %v", err)
	}
	return ids
}

// A fila mostra só o que ainda existe no catálogo; item órfão não quebra a tela.
func TestQueuedVideosSkipsUnknownVideos(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	if err := r.Enqueue(ctx, "v1"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO queue_items (video_id, order_index, opened_at) VALUES ('fantasma', 99, '')`); err != nil {
		t.Fatalf("insert órfão: %v", err)
	}

	videos, err := r.QueuedVideos(ctx)
	if err != nil {
		t.Fatalf("QueuedVideos: %v", err)
	}
	if len(videos) != 1 || videos[0].ID != "v1" {
		t.Fatalf("item órfão deveria ser ignorado, veio %+v", videos)
	}
}

// O feedback precisa chegar ao perfil: é o que faz o botão da UI ter efeito.
func TestRecordFeedbackExcludesFromProfile(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)

	if err := r.RecordFeedback(ctx, domain.RecommendationFeedback{
		VideoID: "v1", Action: domain.FeedbackDontRecommend,
	}); err != nil {
		t.Fatalf("RecordFeedback vídeo: %v", err)
	}
	if err := r.RecordFeedback(ctx, domain.RecommendationFeedback{
		ChannelID: "UC2", Action: domain.FeedbackIgnoreChannel,
	}); err != nil {
		t.Fatalf("RecordFeedback canal: %v", err)
	}

	profile, err := r.BuildProfile(ctx)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if !profile.ExcludedVideoIDs["v1"] {
		t.Error("vídeo rejeitado não entrou nas exclusões do perfil")
	}
	if !profile.ExcludedChannelIDs["UC2"] {
		t.Error("canal rejeitado não entrou nas exclusões do perfil")
	}
}

func TestRecordFeedbackRejectsEmptyAction(t *testing.T) {
	err := newTestRepo(t).RecordFeedback(context.Background(), domain.RecommendationFeedback{VideoID: "v1"})
	if err == nil {
		t.Fatal("ação vazia deveria ser recusada")
	}
}
