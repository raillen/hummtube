package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/auth"
	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/iptv"
	"github.com/nanotube/nanotube-web/internal/search"
	"github.com/nanotube/nanotube-web/internal/services"
	"github.com/nanotube/nanotube-web/internal/storage"
)

type mockSecretStore struct {
	data        map[domain.SecretKey][]byte
	getError    error
	setError    error
	deleteError error
}

func (m *mockSecretStore) Get(ctx context.Context, key domain.SecretKey) ([]byte, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	return m.data[key], nil
}

func (m *mockSecretStore) Set(ctx context.Context, key domain.SecretKey, value []byte) error {
	if m.setError != nil {
		return m.setError
	}
	if m.data == nil {
		m.data = make(map[domain.SecretKey][]byte)
	}
	m.data[key] = value
	return nil
}

func (m *mockSecretStore) Delete(ctx context.Context, key domain.SecretKey) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	delete(m.data, key)
	return nil
}

func setupTestServices(t *testing.T) (*services.AppServices, *storage.Repository) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	if err := storage.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo := storage.NewRepository(db)
	secretStore := &mockSecretStore{data: make(map[domain.SecretKey][]byte)}
	appServices := services.NewAppServices(repo, secretStore)
	return appServices, repo
}

func TestSettingListenersReceiveValidatedChanges(t *testing.T) {
	svc, _ := setupTestServices(t)
	ctx := context.Background()
	received := make(chan string, 1)
	unsubscribe := svc.SubscribeSettingChanges(func(key, value string) { received <- key + "=" + value })
	defer unsubscribe()

	if err := svc.SaveSetting(ctx, storage.SettingTrayEnabled, "0"); err != nil {
		t.Fatal(err)
	}
	if got := <-received; got != storage.SettingTrayEnabled+"=0" {
		t.Fatalf("mudança recebida = %q", got)
	}
	if err := svc.SaveSetting(ctx, storage.SettingTrayEnabled, "talvez"); err == nil {
		t.Fatal("valor inválido da bandeja foi persistido")
	}
}

func TestPlaybackRuntimeStatusDoesNotExposePaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc, _ := setupTestServices(t)
	status, err := svc.GetPlaybackRuntime(context.Background())
	if err != nil {
		t.Fatalf("GetPlaybackRuntime: %v", err)
	}
	if status.ActiveVersion != "" || status.PreviousVersion != "" || len(status.InstalledVersions) != 0 {
		t.Fatalf("runtime inesperado em instalação vazia: %+v", status)
	}
	payload, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), string(filepath.Separator)+"tmp") {
		t.Fatalf("status expôs caminho local: %s", payload)
	}
}

func TestSecretInventoryExposesMetadataWithoutSecretValues(t *testing.T) {
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_FILE", "")
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_ID", "client-id-test")
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_SECRET", "never-expose-this-secret")
	svc, repo := setupTestServices(t)
	ctx := context.Background()
	if err := storage.NewAccountRepository(repo.DB()).SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "subject", Email: "user@example.test",
		ConnectedAt: time.Now().UTC().Format(time.RFC3339), SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SecretStore.Set(ctx, auth.RefreshTokenKey(storage.DefaultProfileID), []byte("refresh-token-never-expose")); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordCredentialAudit(ctx, storage.DefaultProfileID, "google", "connected", "Sessão OAuth autorizada"); err != nil {
		t.Fatal(err)
	}

	inventory, err := svc.GetSecretInventory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal(err)
	}
	payload := string(encoded)
	if strings.Contains(payload, "never-expose-this-secret") || strings.Contains(payload, "refresh-token-never-expose") || strings.Contains(payload, "client-id-test") {
		t.Fatalf("inventário expôs material secreto: %s", payload)
	}
	if !strings.Contains(payload, "user@example.test") || len(inventory.Audit) != 1 {
		t.Fatalf("inventário sem metadados esperados: %+v", inventory)
	}
}

func fakeYtDlpBinary(t *testing.T, payload string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "fake-yt-dlp")
	script := "#!/bin/sh\nprintf '%s' '" + payload + "'\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return binary
}

func TestAppServices_CatalogAndHome(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()
	if err := repo.UpsertChannels(ctx, []domain.Channel{{ID: "ch1", Title: "Test Channel"}}); err != nil {
		t.Fatalf("UpsertChannels: %v", err)
	}

	// Seed some videos
	videos := []domain.Video{
		{
			ID:           "vid1",
			Title:        "Test Video One",
			ChannelID:    "ch1",
			ChannelTitle: "Test Channel",
			PublishedAt:  time.Now().Add(-1 * time.Hour),
			Duration:     300 * time.Second,
		},
		{
			ID:           "vid2",
			Title:        "Test Video Two (Long)",
			ChannelID:    "ch1",
			ChannelTitle: "Test Channel",
			PublishedAt:  time.Now().Add(-2 * time.Hour),
			Duration:     1800 * time.Second,
		},
	}
	if err := repo.UpsertVideos(ctx, videos); err != nil {
		t.Fatalf("UpsertVideos: %v", err)
	}

	home, err := svc.GetHome(ctx, "")
	if err != nil {
		t.Fatalf("GetHome: %v", err)
	}
	if len(home.ForYou) == 0 {
		t.Errorf("expected ForYou videos in Home, got 0")
	}
	if home.ForYou[0].ChannelTitle != "Test Channel" {
		t.Errorf("Home channel title = %q", home.ForYou[0].ChannelTitle)
	}
}

func TestAppServices_SubscriptionVideosComeFromPersistedCatalog(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()

	if err := repo.UpsertChannels(ctx, []domain.Channel{
		{ID: "subscribed-channel", Title: "Canal Inscrito", Subscribed: true},
		{ID: "other-channel", Title: "Canal Não Inscrito", Subscribed: false},
	}); err != nil {
		t.Fatalf("UpsertChannels: %v", err)
	}
	if err := repo.UpsertVideos(ctx, []domain.Video{
		{
			ID: "subscribed-video", ChannelID: "subscribed-channel", Title: "Vídeo da inscrição",
			PublishedAt: time.Now().Add(-time.Hour), Duration: 10 * time.Minute,
		},
		{
			ID: "other-video", ChannelID: "other-channel", Title: "Vídeo de fora",
			PublishedAt: time.Now(), Duration: 10 * time.Minute,
		},
	}); err != nil {
		t.Fatalf("UpsertVideos: %v", err)
	}

	assertSubscriptionFeed := func(t *testing.T, appServices *services.AppServices) {
		t.Helper()
		home, err := appServices.GetHome(ctx, "")
		if err != nil {
			t.Fatalf("GetHome: %v", err)
		}
		if len(home.RecentSubscriptions) != 1 || home.RecentSubscriptions[0].ID != "subscribed-video" {
			t.Fatalf("recent_subscriptions = %+v", home.RecentSubscriptions)
		}
	}

	assertSubscriptionFeed(t, svc)
	// Um novo container simula a reabertura do aplicativo: o feed deve nascer
	// novamente do mesmo SQLite, sem depender de estado em memória da tela.
	restartedServices := services.NewAppServices(repo, svc.SecretStore)
	assertSubscriptionFeed(t, restartedServices)
}

func TestAppServices_ChannelsAndFolders(t *testing.T) {
	svc, _ := setupTestServices(t)
	ctx := context.Background()

	// Subscribe
	if err := svc.SubscribeChannel(ctx, "ch_dev", "Developer Channel"); err != nil {
		t.Fatalf("SubscribeChannel: %v", err)
	}

	channels, err := svc.GetChannels(ctx)
	if err != nil {
		t.Fatalf("GetChannels: %v", err)
	}
	if len(channels) != 1 || channels[0].ID != "ch_dev" {
		t.Errorf("unexpected channels: %+v", channels)
	}

	// Favorite channel
	if err := svc.AddChannelFavorite(ctx, "ch_dev"); err != nil {
		t.Fatalf("AddChannelFavorite: %v", err)
	}
	favs, err := svc.GetFavoriteChannelIDs(ctx)
	if err != nil || !favs["ch_dev"] {
		t.Errorf("channel favorite not found: %v", err)
	}

	// Folders
	folder, err := svc.CreateChannelFolder(ctx, "Tech")
	if err != nil {
		t.Fatalf("CreateChannelFolder: %v", err)
	}
	if err := svc.AddChannelToFolder(ctx, folder.ID, "ch_dev"); err != nil {
		t.Fatalf("AddChannelToFolder: %v", err)
	}

	memberships, err := svc.GetFolderMembership(ctx)
	if err != nil || len(memberships[folder.ID]) != 1 {
		t.Errorf("unexpected folder membership: %+v", memberships)
	}
}

func TestAppServices_Playlists(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()

	// Seed video
	_ = repo.UpsertVideos(ctx, []domain.Video{
		{ID: "v_pl_1", Title: "Playlist Video", Duration: 100 * time.Second},
	})

	pl, err := svc.CreatePlaylist(ctx, "My Favorites", "Description", "#ff0000")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	if err := svc.AddVideoToPlaylist(ctx, pl.ID, "v_pl_1"); err != nil {
		t.Fatalf("AddVideoToPlaylist: %v", err)
	}

	detail, err := svc.GetPlaylist(ctx, pl.ID)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if len(detail.Videos) != 1 || detail.Videos[0].ID != "v_pl_1" {
		t.Errorf("unexpected playlist videos: %+v", detail.Videos)
	}

	if err := svc.RemoveVideoFromPlaylist(ctx, pl.ID, "v_pl_1"); err != nil {
		t.Fatalf("RemoveVideoFromPlaylist: %v", err)
	}
	if err := svc.DeletePlaylist(ctx, pl.ID); err != nil {
		t.Fatalf("DeletePlaylist: %v", err)
	}
}

func TestAppServices_LibraryAndBookmarks(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()

	_ = repo.UpsertVideos(ctx, []domain.Video{
		{ID: "v_lib", Title: "Library Test", Duration: 120 * time.Second},
	})

	// Favorite
	if err := svc.AddFavorite(ctx, "v_lib"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}
	favs, err := svc.GetFavorites(ctx)
	if err != nil || len(favs) != 1 {
		t.Fatalf("unexpected favorites: %+v", favs)
	}

	// Notes
	if err := svc.SaveNote(ctx, "v_lib", "Important timestamp at 01:20"); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	note, err := svc.GetNote(ctx, "v_lib")
	if err != nil || note != "Important timestamp at 01:20" {
		t.Errorf("unexpected note: %q", note)
	}

	// Bookmarks
	bm, err := svc.AddBookmark(ctx, "v_lib", 80000, "Key Moment")
	if err != nil {
		t.Fatalf("AddBookmark: %v", err)
	}
	bms, err := svc.GetBookmarks(ctx, "v_lib")
	if err != nil || len(bms) != 1 || bms[0].ID != bm.ID {
		t.Errorf("unexpected bookmarks: %+v", bms)
	}

	// Progress / History
	if err := svc.SaveProgress(ctx, "v_lib", 40000, 120000, false); err != nil {
		t.Fatalf("SaveProgress: %v", err)
	}
	stats, err := svc.GetLocalStats(ctx)
	if err != nil {
		t.Fatalf("GetLocalStats: %v", err)
	}
	if stats.FavoritesCount != 1 {
		t.Errorf("expected 1 favorite count, got %d", stats.FavoritesCount)
	}
}

func TestRememberVideoKeepsRemoteHistoryVisible(t *testing.T) {
	svc, _ := setupTestServices(t)
	ctx := context.Background()
	video := domain.Video{
		ID: "remote-video", Title: "Resultado remoto", ChannelID: "remote-channel",
		PublishedAt: time.Now(), Duration: 2 * time.Minute,
	}
	if err := svc.RememberVideo(ctx, video); err != nil {
		t.Fatalf("RememberVideo: %v", err)
	}
	if err := svc.SaveProgress(ctx, video.ID, 30_000, 120_000, false); err != nil {
		t.Fatalf("SaveProgress: %v", err)
	}
	history, err := svc.GetHistory(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].ID != video.ID {
		t.Fatalf("histórico remoto = %+v", history)
	}
}

func TestDisconnectAccountClearsPersistedSessionAfterRestart(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()
	secretStore := svc.SecretStore.(*mockSecretStore)
	secretStore.data[auth.RefreshTokenKey(storage.DefaultProfileID)] = []byte("refresh-token")
	accountRepo := storage.NewAccountRepository(repo.DB())
	if err := accountRepo.SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "subject-1", Email: "user@example.invalid",
		ConnectedAt: time.Now().UTC().Format(time.RFC3339), SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}

	// OAuthSession intentionally remains nil, which is the state after a new
	// process restores the account metadata but has not used the token yet.
	if err := svc.DisconnectAccount(ctx); err != nil {
		t.Fatalf("DisconnectAccount: %v", err)
	}
	if _, exists := secretStore.data[auth.RefreshTokenKey(storage.DefaultProfileID)]; exists {
		t.Fatal("refresh token permaneceu no SecretStore")
	}
	if _, exists, err := accountRepo.Account(ctx); err != nil || exists {
		t.Fatalf("conta persistida após desconexão: exists=%v err=%v", exists, err)
	}
}

func TestDeleteDefaultProfileDoesNotTouchAccountOrToken(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()
	secretStore := svc.SecretStore.(*mockSecretStore)
	key := auth.RefreshTokenKey(storage.DefaultProfileID)
	secretStore.data[key] = []byte("keep-token")
	accountRepo := storage.NewAccountRepository(repo.DB(), storage.DefaultProfileID)
	if err := accountRepo.SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "keep-subject", Email: "keep@example.invalid",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteProfile(ctx, storage.DefaultProfileID); err == nil {
		t.Fatal("exclusão do perfil default deveria falhar")
	}
	if got := string(secretStore.data[key]); got != "keep-token" {
		t.Fatalf("token do default foi alterado: %q", got)
	}
	if _, connected, err := accountRepo.Account(ctx); err != nil || !connected {
		t.Fatalf("metadata do default foi removido: connected=%v err=%v", connected, err)
	}
	if _, err := repo.Profile(ctx, storage.DefaultProfileID); err != nil {
		t.Fatalf("perfil default foi removido: %v", err)
	}
}

func TestDisconnectKeepsMetadataWhenSecretDeletionFails(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()
	secretStore := svc.SecretStore.(*mockSecretStore)
	key := auth.RefreshTokenKey(storage.DefaultProfileID)
	secretStore.data[key] = []byte("retry-token")
	secretStore.deleteError = errors.New("keyring permission denied")
	accountRepo := storage.NewAccountRepository(repo.DB(), storage.DefaultProfileID)
	if err := accountRepo.SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "retry-subject", Email: "retry@example.invalid",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActiveProfile(ctx, storage.DefaultProfileID); err != nil {
		t.Fatal(err)
	}

	if err := svc.DisconnectAccount(ctx); err == nil {
		t.Fatal("logout deveria informar falha do SecretStore")
	}
	if got := string(secretStore.data[key]); got != "retry-token" {
		t.Fatalf("token mudou apesar da falha: %q", got)
	}
	if _, connected, err := accountRepo.Account(ctx); err != nil || !connected {
		t.Fatalf("metadata foi removido apesar da falha: connected=%v err=%v", connected, err)
	}
	if status := svc.GetLoginStatus(ctx); status.State != "connected" {
		t.Fatalf("UI recebeu logout falso: %+v", status)
	}
}

func TestAppServicesAccountMetadataFollowsActiveProfile(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()

	work, err := svc.CreateProfile(ctx, "Trabalho")
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.NewAccountRepository(repo.DB(), work.ID).SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "google-work", Email: "work@example.invalid",
		ConnectedAt: time.Now().UTC().Format(time.RFC3339), SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}
	personal, err := svc.CreateProfile(ctx, "Pessoal")
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.NewAccountRepository(repo.DB(), personal.ID).SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "google-personal", Email: "personal@example.invalid",
		ConnectedAt: time.Now().UTC().Format(time.RFC3339), SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}

	account, connected, err := svc.GetAccount(ctx)
	if err != nil || !connected || account.ProfileID != personal.ID || account.Email != "personal@example.invalid" {
		t.Fatalf("conta do perfil pessoal = %+v connected=%v err=%v", account, connected, err)
	}
	if err := svc.SetActiveProfile(ctx, work.ID); err != nil {
		t.Fatal(err)
	}
	account, connected, err = svc.GetAccount(ctx)
	if err != nil || !connected || account.ProfileID != work.ID || account.Email != "work@example.invalid" {
		t.Fatalf("conta do perfil trabalho = %+v connected=%v err=%v", account, connected, err)
	}

	if err := svc.DisconnectAccount(ctx); err != nil {
		t.Fatal(err)
	}
	if _, connected, err := svc.GetAccount(ctx); err != nil || connected {
		t.Fatalf("perfil trabalho continuou conectado: connected=%v err=%v", connected, err)
	}
	if err := svc.SetActiveProfile(ctx, personal.ID); err != nil {
		t.Fatal(err)
	}
	account, connected, err = svc.GetAccount(ctx)
	if err != nil || !connected || account.Email != "personal@example.invalid" {
		t.Fatalf("logout vazou para perfil pessoal: %+v connected=%v err=%v", account, connected, err)
	}
}

func TestAppServicesReportsUnavailableKeyringAtBootstrap(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	svc := services.NewAppServices(storage.NewRepository(db), &mockSecretStore{getError: domain.ErrSecretStoreUnavailable})
	if warning := svc.GetLoginStatus(ctx).Warning; !strings.Contains(warning, "Keyring indisponível") {
		t.Fatalf("aviso do bootstrap = %q", warning)
	}
}

func TestPersistedAccountReportsUnavailableCredential(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()
	secretStore := svc.SecretStore.(*mockSecretStore)
	if err := storage.NewAccountRepository(repo.DB(), storage.DefaultProfileID).SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "saved-subject", Email: "saved@example.invalid",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}
	secretStore.getError = domain.ErrSecretStoreUnavailable
	account, connected, err := svc.GetAccount(ctx)
	if err != nil || !connected {
		t.Fatalf("metadata da conta: connected=%v err=%v", connected, err)
	}
	if account.CredentialState != domain.CredentialStateUnavailable || !strings.Contains(account.Warning, "Keyring indisponível") {
		t.Fatalf("estado da credencial = %+v", account)
	}
	if err := svc.SetActiveProfile(ctx, storage.DefaultProfileID); err != nil {
		t.Fatal(err)
	}
	if status := svc.GetLoginStatus(ctx); status.State != "error" || status.Warning == "" {
		t.Fatalf("status afirmou conexão restaurável: %+v", status)
	}
}

func TestGuestCleanupFailureQuarantinesProfileAPIs(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	repo := storage.NewRepository(db)
	if _, err := repo.CreateGuestProfile(ctx, "Abandonado"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER block_guest_cleanup BEFORE DELETE ON profiles
		WHEN OLD.kind = 'guest'
		BEGIN SELECT RAISE(ABORT, 'cleanup blocked'); END`); err != nil {
		t.Fatal(err)
	}
	svc := services.NewAppServices(repo, &mockSecretStore{})
	if _, err := svc.ListProfiles(ctx); err == nil || !strings.Contains(err.Error(), "quarentena") {
		t.Fatalf("profiles não foram colocados em quarentena: %v", err)
	}
	if status := svc.GetLoginStatus(ctx); status.State != "error" || status.Warning == "" {
		t.Fatalf("status de quarentena = %+v", status)
	}
}

func TestLoginStatusIsScopedToActiveProfile(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()
	temporary, err := svc.CreateProfile(ctx, "Sessão temporária")
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.NewAccountRepository(repo.DB(), temporary.ID).SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "temporary-subject", Email: "temporary@example.invalid",
		SessionPersistence: domain.SessionPersistenceMemory,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActiveProfile(ctx, temporary.ID); err != nil {
		t.Fatal(err)
	}
	temporaryStatus := svc.GetLoginStatus(ctx)
	if temporaryStatus.State != "connected" || temporaryStatus.ProfileID != temporary.ID || temporaryStatus.Warning == "" {
		t.Fatalf("status temporário = %+v", temporaryStatus)
	}
	if err := svc.SetActiveProfile(ctx, storage.DefaultProfileID); err != nil {
		t.Fatal(err)
	}
	defaultStatus := svc.GetLoginStatus(ctx)
	if defaultStatus.State != "idle" || defaultStatus.ProfileID != storage.DefaultProfileID || defaultStatus.Warning != "" {
		t.Fatalf("status default contaminado = %+v", defaultStatus)
	}
	if err := svc.SetActiveProfile(ctx, temporary.ID); err != nil {
		t.Fatal(err)
	}
	if status := svc.GetLoginStatus(ctx); status.ProfileID != temporary.ID || status.State != "connected" {
		t.Fatalf("status temporário não foi restaurado: %+v", status)
	}
}

func TestAppServices_IPTV(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()

	// Save source
	err := svc.SaveIPTVSource(ctx, "src_test", "Test IPTV", "https://example.com/playlist.m3u", "https://example.com/epg.xml", true)
	if err != nil {
		t.Fatalf("SaveIPTVSource: %v", err)
	}

	sources, err := svc.ListIPTVSources(ctx)
	if err != nil {
		t.Fatalf("ListIPTVSources: %v", err)
	}
	if len(sources) != 1 || sources[0].Config.ID != "src_test" {
		t.Errorf("unexpected sources: %+v", sources)
	}

	// Insert catalog item
	err = repo.ReplaceIPTVCatalog(ctx, sources[0].Config, iptv.Playlist{
		Items: []iptv.Item{
			{
				ID:        "vod_item_1",
				SourceID:  "src_test",
				Kind:      iptv.ContentKindMovie,
				Title:     "Sample Movie",
				StreamURL: "https://example.com/movie.mp4",
			},
		},
	}, time.Now())
	if err != nil {
		t.Fatalf("ReplaceIPTVCatalog: %v", err)
	}

	// Saved items
	if err := svc.SetIPTVItemSaved(ctx, "vod_item_1", true); err != nil {
		t.Fatalf("SetIPTVItemSaved: %v", err)
	}

	// VOD progress
	if err := svc.SaveIPTVPlaybackProgress(ctx, "vod_item_1", 60000, 3600000, false); err != nil {
		t.Fatalf("SaveIPTVPlaybackProgress: %v", err)
	}

	// Resume list
	resume, err := svc.ListIPTVResume(ctx, 10)
	if err != nil {
		t.Fatalf("ListIPTVResume: %v", err)
	}
	if len(resume) != 1 || resume[0].Item.ID != "vod_item_1" {
		t.Errorf("unexpected resume: %+v", resume)
	}

	if err := svc.DeleteIPTVSource(ctx, "src_test"); err != nil {
		t.Fatalf("DeleteIPTVSource: %v", err)
	}
}

func TestSearchAdvancedAndLegacyPaginationUseTypedPage(t *testing.T) {
	svc, _ := setupTestServices(t)
	svc.SearchService.Public = search.YtDlp{Binary: fakeYtDlpBinary(t,
		`{"entries":[{"id":"one","title":"Linux One"},{"id":"two","title":"Linux Two"}]}`)}
	ctx := context.Background()

	page, err := svc.SearchAdvanced(ctx, domain.SearchOptions{Query: "linux", MaxResults: 1})
	if err != nil {
		t.Fatalf("SearchAdvanced: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "one" || page.NextPageToken != "1" || page.Source != domain.SearchSourcePublic {
		t.Fatalf("página tipada = %+v", page)
	}

	legacyPage, err := svc.Search(ctx, "linux", 1, 1)
	if err != nil {
		t.Fatalf("Search legado: %v", err)
	}
	if len(legacyPage.Items) != 1 || legacyPage.Items[0].ID != "two" {
		t.Fatalf("paginação legada = %+v", legacyPage)
	}
}

func TestSearchAllResourcesFallsBackToPublicVideosForGuest(t *testing.T) {
	svc, _ := setupTestServices(t)
	svc.SearchService.Public = search.YtDlp{Binary: fakeYtDlpBinary(t,
		`{"entries":[{"id":"public-video","title":"Vídeo público"}]}`)}
	ctx := context.Background()
	if _, err := svc.CreateGuestProfile(ctx, "Visitante"); err != nil {
		t.Fatal(err)
	}
	page, err := svc.SearchAdvanced(ctx, domain.SearchOptions{
		Query: "linux", ResourceTypes: []domain.SearchResourceType{
			domain.SearchResourceVideo, domain.SearchResourceChannel, domain.SearchResourcePlaylist,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "public-video" || len(page.Notices) == 0 {
		t.Fatalf("fallback visitante = %+v", page)
	}
}

func TestRemotePlaylistCanBeReadAndImportedLocally(t *testing.T) {
	svc, repo := setupTestServices(t)
	svc.SearchService.Public = search.YtDlp{Binary: fakeYtDlpBinary(t,
		`{"entries":[{"id":"remote-one","title":"Remote One","channel_id":"channel"},{"id":"remote-two","title":"Remote Two","channel_id":"channel"}]}`)}
	ctx := context.Background()

	remote, err := svc.GetRemotePlaylistPage(ctx, "PL_fixture", "", 2)
	if err != nil {
		t.Fatalf("GetRemotePlaylistPage: %v", err)
	}
	if remote.PlaylistID != "PL_fixture" || len(remote.Videos) != 2 || remote.NextPageToken != "2" {
		t.Fatalf("playlist remota = %+v", remote)
	}
	local, err := svc.ImportRemotePlaylist(ctx, "PL_fixture", "Minha remota", "Importada sob demanda", 2)
	if err != nil {
		t.Fatalf("ImportRemotePlaylist: %v", err)
	}
	if local.Playlist.Name != "Minha remota" || local.Playlist.ItemCount != 2 || len(local.Videos) != 2 {
		t.Fatalf("playlist local = %+v", local)
	}
	persisted, err := repo.PlaylistVideos(ctx, local.Playlist.ID)
	if err != nil || len(persisted) != 2 {
		t.Fatalf("itens persistidos = %+v err=%v", persisted, err)
	}
	target, err := svc.CreatePlaylist(ctx, "Destino", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddVideoToPlaylist(ctx, target.ID, "remote-one"); err != nil {
		t.Fatal(err)
	}
	added, err := svc.AddRemotePlaylistToPlaylist(ctx, "PL_fixture", target.ID, 2)
	if err != nil || added != 1 {
		t.Fatalf("combinar playlist: added=%d err=%v", added, err)
	}
	targetVideos, err := repo.PlaylistVideos(ctx, target.ID)
	if err != nil || len(targetVideos) != 2 {
		t.Fatalf("destino deduplicado = %+v err=%v", targetVideos, err)
	}
}

func TestRemoteChannelVideosCanBePagedWithoutPersistence(t *testing.T) {
	svc, repo := setupTestServices(t)
	svc.SearchService.Public = search.YtDlp{Binary: fakeYtDlpBinary(t,
		`{"entries":[{"id":"channel-one","title":"Channel One","channel_id":"UC_fixture"},{"id":"channel-two","title":"Channel Two","channel_id":"UC_fixture"}]}`)}
	ctx := context.Background()

	page, err := svc.GetRemoteChannelVideosPage(ctx, "UC_fixture", "", 2)
	if err != nil {
		t.Fatalf("GetRemoteChannelVideosPage: %v", err)
	}
	if page.ChannelID != "UC_fixture" || len(page.Videos) != 2 || page.NextPageToken != "2" {
		t.Fatalf("página do canal = %+v", page)
	}
	persisted, err := repo.Recent(ctx, domain.VideoFilter{ChannelID: "UC_fixture", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 0 {
		t.Fatalf("listagem remota persistiu vídeos sem ação do usuário: %+v", persisted)
	}
	if _, err := svc.GetRemoteChannelVideosPage(ctx, "UC_fixture", "inválido", 24); err == nil {
		t.Fatal("token de canal inválido foi aceito")
	}
}

func TestSaveIPTVSourceWithCredentialsSanitizesAndPersistsEditableConfiguration(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()
	const sourceID = "editable-source"

	err := svc.SaveIPTVSourceWithCredentials(
		ctx,
		sourceID,
		"Lista original",
		"https://provider.example/get.php?username=alice&password=super-secret&output=mpegts&type=m3u_plus",
		"https://provider.example/epg.xml",
		true,
		"",
		"",
		false,
		"hls",
	)
	if err != nil {
		t.Fatalf("SaveIPTVSourceWithCredentials: %v", err)
	}

	source, exists, err := repo.GetIPTVSource(ctx, sourceID)
	if err != nil || !exists {
		t.Fatalf("GetIPTVSource: exists=%v err=%v", exists, err)
	}
	playlistURL, err := url.Parse(source.PlaylistURL)
	if err != nil {
		t.Fatalf("URL persistida inválida: %v", err)
	}
	if playlistURL.User != nil || playlistURL.Query().Get("username") != "" || playlistURL.Query().Get("password") != "" {
		t.Fatalf("URL persistida contém credenciais: %q", source.PlaylistURL)
	}
	if playlistURL.Query().Get("output") != "m3u8" || playlistURL.Query().Get("type") != "m3u_plus" {
		t.Fatalf("query persistida inesperada: %v", playlistURL.Query())
	}
	if source.CredentialRef != "iptv_cred_"+sourceID {
		t.Fatalf("credential_ref = %q", source.CredentialRef)
	}

	secretStore := svc.SecretStore.(*mockSecretStore)
	var credentials iptv.PlaylistCredentials
	if err := json.Unmarshal(secretStore.data[domain.SecretKey(source.CredentialRef)], &credentials); err != nil {
		t.Fatalf("credencial no keyring mock: %v", err)
	}
	if credentials.Username != "alice" || credentials.Password != "super-secret" {
		t.Fatalf("credencial inesperada: username=%q password_present=%v", credentials.Username, credentials.Password != "")
	}

	if err := svc.SaveIPTVSourceWithCredentials(
		ctx, sourceID, "Lista editada", source.PlaylistURL, "", false,
		"", "", false, "preserve",
	); err != nil {
		t.Fatalf("editar preservando credenciais: %v", err)
	}
	updated, exists, err := repo.GetIPTVSource(ctx, sourceID)
	if err != nil || !exists {
		t.Fatalf("fonte editada: exists=%v err=%v", exists, err)
	}
	if updated.Name != "Lista editada" || updated.Enabled || updated.CredentialRef != source.CredentialRef {
		t.Fatalf("edição não persistiu/preservou: %+v", updated)
	}
	if _, ok := secretStore.data[domain.SecretKey(source.CredentialRef)]; !ok {
		t.Fatal("credencial existente foi removida por campos vazios")
	}

	if err := svc.SaveIPTVSourceWithCredentials(
		ctx, sourceID, updated.Name, updated.PlaylistURL, updated.GuideURL, updated.Enabled,
		"", "", true, "preserve",
	); err != nil {
		t.Fatalf("limpar credenciais: %v", err)
	}
	cleared, _, err := repo.GetIPTVSource(ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.CredentialRef != "" {
		t.Fatalf("credential_ref não foi limpo: %q", cleared.CredentialRef)
	}
	if _, ok := secretStore.data[domain.SecretKey(source.CredentialRef)]; ok {
		t.Fatal("credencial permaneceu no keyring mock")
	}
}

func TestSaveIPTVSourceWithCredentialsRejectsIncompleteOrTokenCredentials(t *testing.T) {
	svc, _ := setupTestServices(t)
	ctx := context.Background()
	testCases := []struct {
		name, endpoint, username, password string
	}{
		{name: "separate incomplete", endpoint: "https://provider.example/list.m3u", username: "alice"},
		{name: "inline incomplete", endpoint: "https://provider.example/list.m3u?username=alice"},
		{name: "token unsupported", endpoint: "https://provider.example/list.m3u?token=secret-token"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := svc.SaveIPTVSourceWithCredentials(
				ctx, testCase.name, "Lista", testCase.endpoint, "", true,
				testCase.username, testCase.password, false, "preserve",
			)
			if err == nil {
				t.Fatal("configuração sensível inválida foi aceita")
			}
		})
	}
}

func TestSyncIPTVSourceDeduplicatesAndReturnsPersistedCount(t *testing.T) {
	playlistServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\n" +
			"#EXTINF:-1 tvg-id=\"duplicate\" group-title=\"Filmes\",Primeiro\nhttps://media.example.invalid/one.mp4\n" +
			"#EXTINF:-1 tvg-id=\"duplicate\" group-title=\"Filmes\",Último\nhttps://media.example.invalid/two.mp4\n"))
	}))
	defer playlistServer.Close()

	svc, _ := setupTestServices(t)
	svc.IPTVM3USource = iptv.HTTPM3USource{Client: playlistServer.Client()}
	ctx := context.Background()
	if err := svc.SaveIPTVSource(ctx, "duplicate-source", "Duplicados", playlistServer.URL, "", true); err != nil {
		t.Fatal(err)
	}
	count, err := svc.SyncIPTVSource(ctx, "duplicate-source")
	if err != nil {
		t.Fatalf("SyncIPTVSource: %v", err)
	}
	if count != 1 {
		t.Fatalf("total persistido = %d, want 1", count)
	}
}

func TestAppServices_SettingsAndDiagnostics(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()

	diag := svc.GetDiagnostics(ctx)
	if diag == nil || diag.Version == "" {
		t.Errorf("invalid diagnostics report")
	}

	if err := svc.SaveSetting(ctx, "theme", "tokyonight"); err != nil {
		t.Fatalf("SaveSetting: %v", err)
	}
	if err := repo.SetSetting(ctx, storage.SettingCookiesFile, "/tmp/private-cookies.txt"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetBrowserCookies(ctx, "brave+gnomekeyring"); err != nil {
		t.Fatalf("SetBrowserCookies: %v", err)
	}
	settings, err := svc.GetSettings(ctx)
	if err != nil || settings["theme"] != "tokyonight" {
		t.Errorf("unexpected settings: %+v", settings)
	}
	if settings[storage.SettingCookiesFrom] != "brave+gnomekeyring" {
		t.Fatalf("seletor validado não foi persistido: %+v", settings)
	}
	if _, exposed := settings[storage.SettingCookiesFile]; exposed {
		t.Fatal("GetSettings expôs o caminho do arquivo de cookies")
	}
	if err := svc.SetBrowserCookies(ctx, "brave:/tmp/profile"); err == nil {
		t.Fatal("SetBrowserCookies aceitou perfil/caminho arbitrário")
	}
	if err := svc.SaveSetting(ctx, storage.SettingCookiesFrom, "brave+basictext"); err == nil {
		t.Fatal("SaveSetting contornou o contrato dedicado de cookies")
	}
	if err := svc.SaveSetting(ctx, storage.SettingActiveProfile, "attacker-controlled"); err == nil {
		t.Fatal("SaveSetting permitiu burlar SetActiveProfile")
	}
	activeID, err := svc.GetActiveProfile(ctx)
	if err != nil || activeID.ID != storage.DefaultProfileID {
		t.Fatalf("perfil ativo foi alterado por SaveSetting: %+v err=%v", activeID, err)
	}

	data, err := svc.ExportPersonalData(ctx)
	if err != nil || data == "" {
		t.Fatalf("ExportPersonalData failed: %v", err)
	}
}

func TestAppServices_BrowserCookiesAreIsolatedByProfile(t *testing.T) {
	svc, repo := setupTestServices(t)
	ctx := context.Background()

	if err := svc.SetBrowserCookies(ctx, "firefox+gnomekeyring"); err != nil {
		t.Fatalf("SetBrowserCookies default: %v", err)
	}
	second, err := svc.CreateProfile(ctx, "Trabalho")
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if _, ok, err := repo.ProfileSetting(ctx, second.ID, storage.SettingCookiesFrom); err != nil || ok {
		t.Fatalf("novo perfil herdou cookies: ok=%v err=%v", ok, err)
	}
	if err := svc.SetBrowserCookies(ctx, "brave+kwallet"); err != nil {
		t.Fatalf("SetBrowserCookies second: %v", err)
	}

	settings, err := svc.GetSettings(ctx)
	if err != nil || settings[storage.SettingCookiesFrom] != "brave+kwallet" {
		t.Fatalf("cookies do perfil second = %q err=%v", settings[storage.SettingCookiesFrom], err)
	}
	if err := svc.SetActiveProfile(ctx, storage.DefaultProfileID); err != nil {
		t.Fatalf("SetActiveProfile default: %v", err)
	}
	settings, err = svc.GetSettings(ctx)
	if err != nil || settings[storage.SettingCookiesFrom] != "firefox+gnomekeyring" {
		t.Fatalf("cookies do perfil default = %q err=%v", settings[storage.SettingCookiesFrom], err)
	}

	guest, err := svc.CreateGuestProfile(ctx, "Convidado")
	if err != nil {
		t.Fatalf("CreateGuestProfile: %v", err)
	}
	if err := svc.SetBrowserCookies(ctx, "firefox+gnomekeyring"); err == nil {
		t.Fatal("perfil guest permitiu persistir cookies do navegador")
	}
	if _, ok, err := repo.ProfileSetting(ctx, guest.ID, storage.SettingCookiesFrom); err != nil || ok {
		t.Fatalf("guest possui configuração de cookies: ok=%v err=%v", ok, err)
	}
}
