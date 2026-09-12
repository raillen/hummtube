package services

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/nanotube/nanotube-web/internal/auth"
	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/storage"
	"golang.org/x/oauth2"
)

type authCommitSecretStore struct {
	mu     sync.Mutex
	values map[domain.SecretKey][]byte
}

func (s *authCommitSecretStore) Get(_ context.Context, key domain.SecretKey) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, exists := s.values[key]
	if !exists {
		return nil, domain.ErrSecretNotFound
	}
	return append([]byte(nil), value...), nil
}

func (s *authCommitSecretStore) Set(_ context.Context, key domain.SecretKey, value []byte) error {
	s.mu.Lock()
	s.values[key] = append([]byte(nil), value...)
	s.mu.Unlock()
	return nil
}

func (s *authCommitSecretStore) Delete(_ context.Context, key domain.SecretKey) error {
	s.mu.Lock()
	delete(s.values, key)
	s.mu.Unlock()
	return nil
}

func newAuthCommitServices(t *testing.T) (*AppServices, *storage.Repository, *authCommitSecretStore) {
	t.Helper()
	db, err := storage.OpenMemory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Migrate(db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	secretStore := &authCommitSecretStore{values: make(map[domain.SecretKey][]byte)}
	repo := storage.NewRepository(db)
	return NewAppServices(repo, secretStore), repo, secretStore
}

func TestFailedReauthenticationPreservesExistingSession(t *testing.T) {
	svc, repo, secrets := newAuthCommitServices(t)
	ctx := context.Background()
	key := auth.RefreshTokenKey(storage.DefaultProfileID)
	secrets.values[key] = []byte("old-refresh")
	accountRepo := storage.NewAccountRepository(repo.DB(), storage.DefaultProfileID)
	if err := accountRepo.SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "old-subject", Email: "old@example.invalid",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}
	svc.identityFetcher = func(context.Context, *http.Client) (auth.ProviderIdentity, error) {
		return auth.ProviderIdentity{}, errors.New("userinfo unavailable")
	}
	profile, err := repo.Profile(ctx, storage.DefaultProfileID)
	if err != nil {
		t.Fatal(err)
	}
	generation := svc.beginLoginAttempt(profile.ID)
	svc.completeLogin(ctx, auth.Config{}, profile, generation, &oauth2.Token{
		AccessToken: "new-access", RefreshToken: "new-refresh",
	})

	if got := string(secrets.values[key]); got != "old-refresh" {
		t.Fatalf("token anterior foi alterado: %q", got)
	}
	account, connected, err := accountRepo.Account(ctx)
	if err != nil || !connected || account.ProviderSubject != "old-subject" {
		t.Fatalf("metadata anterior foi alterado: %+v connected=%v err=%v", account, connected, err)
	}
}

func TestAccountConflictRestoresPreviousRefreshToken(t *testing.T) {
	svc, repo, secrets := newAuthCommitServices(t)
	ctx := context.Background()
	key := auth.RefreshTokenKey(storage.DefaultProfileID)
	secrets.values[key] = []byte("old-refresh")
	defaultAccount := storage.NewAccountRepository(repo.DB(), storage.DefaultProfileID)
	if err := defaultAccount.SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "old-subject", Email: "old@example.invalid",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}
	other, err := repo.CreateProfile(ctx, "Outro")
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.NewAccountRepository(repo.DB(), other.ID).SaveAccount(ctx, domain.AccountInfo{
		Provider: "google", ProviderSubject: "conflicting-subject", Email: "other@example.invalid",
		SessionPersistence: domain.SessionPersistenceKeyring,
	}); err != nil {
		t.Fatal(err)
	}
	svc.identityFetcher = func(context.Context, *http.Client) (auth.ProviderIdentity, error) {
		return auth.ProviderIdentity{Provider: "google", Subject: "conflicting-subject", Email: "new@example.invalid"}, nil
	}
	profile, _ := repo.Profile(ctx, storage.DefaultProfileID)
	generation := svc.beginLoginAttempt(profile.ID)
	svc.completeLogin(ctx, auth.Config{}, profile, generation, &oauth2.Token{
		AccessToken: "new-access", RefreshToken: "new-refresh",
	})

	if got := string(secrets.values[key]); got != "old-refresh" {
		t.Fatalf("rollback do token = %q", got)
	}
	account, _, err := defaultAccount.Account(ctx)
	if err != nil || account.ProviderSubject != "old-subject" {
		t.Fatalf("conta anterior não foi preservada: %+v err=%v", account, err)
	}
	if status := svc.GetLoginStatus(ctx); status.State != "error" {
		t.Fatalf("conflito não foi exposto: %+v", status)
	}
}

func TestStaleLoginCompletionCannotOverwriteNewerAccount(t *testing.T) {
	svc, repo, secrets := newAuthCommitServices(t)
	ctx := context.Background()
	profile, err := repo.Profile(ctx, storage.DefaultProfileID)
	if err != nil {
		t.Fatal(err)
	}
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls int
	var callsMu sync.Mutex
	svc.identityFetcher = func(context.Context, *http.Client) (auth.ProviderIdentity, error) {
		callsMu.Lock()
		calls++
		call := calls
		callsMu.Unlock()
		if call == 1 {
			close(firstEntered)
			<-releaseFirst
			return auth.ProviderIdentity{Provider: "google", Subject: "subject-a", Email: "a@example.invalid"}, nil
		}
		return auth.ProviderIdentity{Provider: "google", Subject: "subject-b", Email: "b@example.invalid"}, nil
	}

	firstGeneration := svc.beginLoginAttempt(profile.ID)
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		svc.completeLogin(ctx, auth.Config{}, profile, firstGeneration, &oauth2.Token{
			AccessToken: "access-a", RefreshToken: "refresh-a",
		})
	}()
	<-firstEntered
	secondGeneration := svc.beginLoginAttempt(profile.ID)
	svc.completeLogin(ctx, auth.Config{}, profile, secondGeneration, &oauth2.Token{
		AccessToken: "access-b", RefreshToken: "refresh-b",
	})
	close(releaseFirst)
	<-firstDone

	if got := string(secrets.values[auth.RefreshTokenKey(profile.ID)]); got != "refresh-b" {
		t.Fatalf("conclusão obsoleta sobrescreveu token: %q", got)
	}
	account, connected, err := storage.NewAccountRepository(repo.DB(), profile.ID).Account(ctx)
	if err != nil || !connected || account.ProviderSubject != "subject-b" {
		t.Fatalf("conclusão obsoleta sobrescreveu conta: %+v connected=%v err=%v", account, connected, err)
	}
}
