package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
	"golang.org/x/oauth2"
)

type sessionSecretStore struct {
	values   map[domain.SecretKey][]byte
	setError error
	setCalls int
}

func newSessionSecretStore() *sessionSecretStore {
	return &sessionSecretStore{values: make(map[domain.SecretKey][]byte)}
}

func (s *sessionSecretStore) Get(_ context.Context, key domain.SecretKey) ([]byte, error) {
	value, ok := s.values[key]
	if !ok {
		return nil, domain.ErrSecretNotFound
	}
	return append([]byte(nil), value...), nil
}

func (s *sessionSecretStore) Set(_ context.Context, key domain.SecretKey, value []byte) error {
	s.setCalls++
	if s.setError != nil {
		return s.setError
	}
	s.values[key] = append([]byte(nil), value...)
	return nil
}

func (s *sessionSecretStore) Delete(_ context.Context, key domain.SecretKey) error {
	delete(s.values, key)
	return nil
}

func TestSessionRefreshTokensAreIsolatedByProfile(t *testing.T) {
	store := newSessionSecretStore()
	first := NewSessionForProfile(Config{}, store, nil, "profile-a", false)
	second := NewSessionForProfile(Config{}, store, nil, "profile-b", false)

	if persistence, err := first.StoreRefreshToken(context.Background(), &oauth2.Token{RefreshToken: "token-a"}); err != nil || persistence != domain.SessionPersistenceKeyring {
		t.Fatalf("primeira sessão: persistence=%q err=%v", persistence, err)
	}
	if _, err := second.StoreRefreshToken(context.Background(), &oauth2.Token{RefreshToken: "token-b"}); err != nil {
		t.Fatal(err)
	}
	if got := string(store.values[RefreshTokenKey("profile-a")]); got != "token-a" {
		t.Fatalf("token do profile-a = %q", got)
	}
	if got := string(store.values[RefreshTokenKey("profile-b")]); got != "token-b" {
		t.Fatalf("token do profile-b = %q", got)
	}
	if err := first.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.values[RefreshTokenKey("profile-b")]; !ok {
		t.Fatal("logout do profile-a removeu token do profile-b")
	}
}

func TestSessionFallsBackToMemoryWhenKeyringUnavailable(t *testing.T) {
	store := newSessionSecretStore()
	store.setError = domain.ErrSecretStoreUnavailable
	session := NewSessionForProfile(Config{}, store, nil, "profile-a", false)

	persistence, err := session.StoreRefreshToken(context.Background(), &oauth2.Token{RefreshToken: "memory-token"})
	if err != nil {
		t.Fatal(err)
	}
	if persistence != domain.SessionPersistenceMemory || session.Persistence() != domain.SessionPersistenceMemory {
		t.Fatalf("persistência = %q / %q", persistence, session.Persistence())
	}
	if _, err := session.Source(context.Background()); err != nil {
		t.Fatalf("sessão em memória não restaurou token source: %v", err)
	}
}

func TestSessionCachesAndInvalidatesTokenSource(t *testing.T) {
	store := newSessionSecretStore()
	session := NewSessionForProfile(Config{}, store, nil, "profile-a", false)
	if _, err := session.StoreRefreshToken(context.Background(), &oauth2.Token{RefreshToken: "first-token"}); err != nil {
		t.Fatal(err)
	}
	first, err := session.Source(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := session.Source(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("Source recriou o cache OAuth para a mesma sessão")
	}
	if _, err := session.StoreRefreshToken(context.Background(), &oauth2.Token{RefreshToken: "replacement-token"}); err != nil {
		t.Fatal(err)
	}
	replacement, err := session.Source(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if replacement == first {
		t.Fatal("novo login não invalidou o TokenSource antigo")
	}
	if err := session.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Source(context.Background()); err == nil {
		t.Fatal("logout preservou TokenSource utilizável")
	}
}

func TestGuestSessionNeverWritesToKeyring(t *testing.T) {
	store := newSessionSecretStore()
	session := NewSessionForProfile(Config{}, store, nil, "guest-1", true)
	persistence, err := session.StoreRefreshToken(context.Background(), &oauth2.Token{RefreshToken: "guest-token"})
	if err != nil {
		t.Fatal(err)
	}
	if persistence != domain.SessionPersistenceMemory || store.setCalls != 0 {
		t.Fatalf("guest persistence=%q keyring writes=%d", persistence, store.setCalls)
	}
}

func TestMigrateLegacyRefreshTokenMovesAfterSuccessfulWrite(t *testing.T) {
	store := newSessionSecretStore()
	store.values[legacyRefreshTokenKey] = []byte("legacy-token")
	if err := MigrateLegacyRefreshToken(context.Background(), store, "default"); err != nil {
		t.Fatal(err)
	}
	if got := string(store.values[RefreshTokenKey("default")]); got != "legacy-token" {
		t.Fatalf("token migrado = %q", got)
	}
	if _, exists := store.values[legacyRefreshTokenKey]; exists {
		t.Fatal("chave legada permaneceu após migração")
	}

	failing := newSessionSecretStore()
	failing.values[legacyRefreshTokenKey] = []byte("keep-me")
	failing.setError = errors.New("write failed")
	if err := MigrateLegacyRefreshToken(context.Background(), failing, "default"); err == nil {
		t.Fatal("migração com escrita falha deveria retornar erro")
	}
	if _, exists := failing.values[legacyRefreshTokenKey]; !exists {
		t.Fatal("chave legada foi apagada antes da escrita confirmada")
	}

	alreadyMigrated := newSessionSecretStore()
	alreadyMigrated.values[RefreshTokenKey("default")] = []byte("current-token")
	alreadyMigrated.values[legacyRefreshTokenKey] = []byte("stale-token")
	if err := MigrateLegacyRefreshToken(context.Background(), alreadyMigrated, "default"); err != nil {
		t.Fatal(err)
	}
	if _, exists := alreadyMigrated.values[legacyRefreshTokenKey]; exists {
		t.Fatal("chave legada redundante permaneceu depois da migração confirmada")
	}
	if got := string(alreadyMigrated.values[RefreshTokenKey("default")]); got != "current-token" {
		t.Fatalf("token atual foi substituído por legado: %q", got)
	}
}

func TestFetchIdentityRequiresStableProviderSubject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"google-subject-123","email":"person@example.com"}`))
	}))
	defer server.Close()
	identity, err := fetchIdentityAt(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Provider != "google" || identity.Subject != "google-subject-123" || identity.Email != "person@example.com" {
		t.Fatalf("identidade = %+v", identity)
	}

	missingSubject := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"email":"person@example.com"}`))
	}))
	defer missingSubject.Close()
	if _, err := fetchIdentityAt(context.Background(), missingSubject.Client(), missingSubject.URL); err == nil {
		t.Fatal("userinfo sem subject foi aceito")
	}
}
