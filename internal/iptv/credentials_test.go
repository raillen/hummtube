package iptv

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestLoadPlaylistCredentialsValidatesBoundedJSON(t *testing.T) {
	credentials, err := loadPlaylistCredentials(context.Background(), staticSecretStore{
		domain.SecretKey("fixture-ref"): []byte(`{"username":"fixture-user","password":"fixture-pass"}`),
	}, "fixture-ref")
	if err != nil {
		t.Fatalf("loadPlaylistCredentials() error = %v", err)
	}
	if credentials.Username != "fixture-user" || credentials.Password != "fixture-pass" {
		t.Fatalf("credentials = %+v", credentials)
	}
}

func TestLoadPlaylistCredentialsRejectsInvalidPayloadWithoutEchoingSecret(t *testing.T) {
	secret := "fixture-pass"
	for _, payload := range []string{
		`{"username":"fixture-user"}`,
		`{"username":"fixture-user","password":"fixture-pass"} trailing`,
		secret,
	} {
		_, err := loadPlaylistCredentials(context.Background(), staticSecretStore{
			domain.SecretKey("fixture-ref"): []byte(payload),
		}, "fixture-ref")
		if !errors.Is(err, ErrInvalidCredential) || strings.Contains(err.Error(), secret) {
			t.Fatalf("payload inválido: err=%v", err)
		}
	}
}

func TestLoadPlaylistCredentialsRequiresStoreForReference(t *testing.T) {
	_, err := loadPlaylistCredentials(context.Background(), nil, "fixture-ref")
	if !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("erro = %v, want ErrCredentialUnavailable", err)
	}
}

type recordingSecretStore struct {
	values  map[domain.SecretKey][]byte
	deleted []domain.SecretKey
}

func newRecordingSecretStore() *recordingSecretStore {
	return &recordingSecretStore{values: make(map[domain.SecretKey][]byte)}
}

func (store *recordingSecretStore) Get(_ context.Context, key domain.SecretKey) ([]byte, error) {
	value, ok := store.values[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return value, nil
}

func (store *recordingSecretStore) Set(_ context.Context, key domain.SecretKey, value []byte) error {
	store.values[key] = value
	return nil
}

func (store *recordingSecretStore) Delete(_ context.Context, key domain.SecretKey) error {
	delete(store.values, key)
	store.deleted = append(store.deleted, key)
	return nil
}

func TestStorePlaylistCredentialsPersistsReadablePayload(t *testing.T) {
	store := newRecordingSecretStore()
	reference := "iptv/minha-fonte"
	err := StorePlaylistCredentials(context.Background(), store, reference,
		PlaylistCredentials{Username: "fixture-user", Password: "fixture-pass"})
	if err != nil {
		t.Fatalf("StorePlaylistCredentials() error = %v", err)
	}
	stored := store.values[domain.SecretKey(reference)]
	if len(stored) == 0 || strings.Contains(string(stored), "\n") {
		t.Fatalf("payload inesperado: %q", stored)
	}
	loaded, err := loadPlaylistCredentials(context.Background(), store, reference)
	if err != nil || loaded.Username != "fixture-user" {
		t.Fatalf("load após store: %+v err=%v", loaded, err)
	}
}

func TestStorePlaylistCredentialsRejectsIncompleteInput(t *testing.T) {
	store := newRecordingSecretStore()
	cases := []struct {
		name        string
		reference   string
		credentials PlaylistCredentials
	}{
		{"referência vazia", "  ", PlaylistCredentials{Username: "u", Password: "p"}},
		{"usuário vazio", "ref", PlaylistCredentials{Username: " ", Password: "p"}},
		{"senha vazia", "ref", PlaylistCredentials{Username: "u", Password: ""}},
		{"store ausente", "ref", PlaylistCredentials{Username: "u", Password: "p"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var target domain.SecretStore = store
			if testCase.name == "store ausente" {
				target = nil
			}
			err := StorePlaylistCredentials(context.Background(), target,
				testCase.reference, testCase.credentials)
			if err == nil {
				t.Fatalf("esperado erro para %s", testCase.name)
			}
		})
	}
}

func TestDeletePlaylistCredentialsIsIdempotentAndRecordsReference(t *testing.T) {
	store := newRecordingSecretStore()
	if err := DeletePlaylistCredentials(context.Background(), store, "iptv/minha-fonte"); err != nil {
		t.Fatalf("delete sem segredo: %v", err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != domain.SecretKey("iptv/minha-fonte") {
		t.Fatalf("deleted = %v", store.deleted)
	}
	if err := DeletePlaylistCredentials(context.Background(), store, ""); err == nil {
		t.Fatalf("referência vazia deveria falhar")
	}
}

func TestDiagnosePlaylistCredentialsReportsRedactedStatuses(t *testing.T) {
	store := newRecordingSecretStore()
	if status, _ := DiagnosePlaylistCredentials(context.Background(), store, ""); status != CredentialStatusEmpty {
		t.Fatalf("status = %q, want empty", status)
	}
	if status, _ := DiagnosePlaylistCredentials(context.Background(), store, "ausente"); status != CredentialStatusMissing {
		t.Fatalf("status = %q, want missing", status)
	}
	if status, _ := DiagnosePlaylistCredentials(context.Background(), nil, "ref"); status != CredentialStatusMissing {
		t.Fatalf("status = %q, want missing para store nil", status)
	}
	if status, _ := DiagnosePlaylistCredentials(context.Background(), store, "com\nquebra"); status != CredentialStatusInvalid {
		t.Fatalf("status = %q, want invalid", status)
	}
	if err := StorePlaylistCredentials(context.Background(), store, "ok-ref",
		PlaylistCredentials{Username: "u", Password: "p"}); err != nil {
		t.Fatalf("store: %v", err)
	}
	if status, _ := DiagnosePlaylistCredentials(context.Background(), store, "ok-ref"); status != CredentialStatusOK {
		t.Fatalf("status = %q, want ok", status)
	}
}
