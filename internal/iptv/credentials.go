package iptv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const maxPlaylistCredentialBytes = 8 << 10

var (
	ErrCredentialUnavailable = errors.New("credencial IPTV indisponível")
	ErrInvalidCredential     = errors.New("credencial IPTV inválida")
)

// PlaylistCredentials is the minimal provider credential payload understood
// by the M3U adapter. The payload belongs in SecretStore, never in SQLite or
// SourceConfig.
type PlaylistCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func loadPlaylistCredentials(ctx context.Context, store domain.SecretStore, reference string) (*PlaylistCredentials, error) {
	if ctx == nil {
		return nil, fmt.Errorf("credencial IPTV: contexto nil")
	}
	reference, err := normalizeCredentialReference(reference)
	if err != nil {
		return nil, err
	}
	if reference == "" {
		return nil, nil
	}
	if store == nil {
		return nil, fmt.Errorf("credencial IPTV: %w", ErrCredentialUnavailable)
	}

	value, err := store.Get(ctx, domain.SecretKey(reference))
	if err != nil {
		return nil, fmt.Errorf("credencial IPTV: %w", ErrCredentialUnavailable)
	}
	if len(value) == 0 || len(value) > maxPlaylistCredentialBytes {
		return nil, fmt.Errorf("credencial IPTV: %w", ErrInvalidCredential)
	}

	var credentials PlaylistCredentials
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(value), maxPlaylistCredentialBytes))
	if err := decoder.Decode(&credentials); err != nil {
		return nil, fmt.Errorf("credencial IPTV: %w", ErrInvalidCredential)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("credencial IPTV: %w", ErrInvalidCredential)
	}
	if strings.TrimSpace(credentials.Username) == "" || strings.TrimSpace(credentials.Password) == "" {
		return nil, fmt.Errorf("credencial IPTV: %w", ErrInvalidCredential)
	}
	return &credentials, nil
}

func normalizeCredentialReference(reference string) (string, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", nil
	}
	if len(reference) > 128 {
		return "", fmt.Errorf("credencial IPTV: %w", ErrInvalidCredential)
	}
	for _, character := range reference {
		if unicode.IsControl(character) {
			return "", fmt.Errorf("credencial IPTV: %w", ErrInvalidCredential)
		}
	}
	return reference, nil
}

// StorePlaylistCredentials writes the minimal payload to SecretStore under the
// given reference. The payload never reaches SQLite, logs or errors.
func StorePlaylistCredentials(ctx context.Context, store domain.SecretStore, reference string, credentials PlaylistCredentials) error {
	if ctx == nil {
		return fmt.Errorf("armazenar credencial IPTV: contexto nil")
	}
	reference, err := normalizeCredentialReference(reference)
	if err != nil || reference == "" {
		return fmt.Errorf("armazenar credencial IPTV: %w", ErrInvalidCredential)
	}
	if store == nil {
		return fmt.Errorf("armazenar credencial IPTV: %w", ErrCredentialUnavailable)
	}
	if strings.TrimSpace(credentials.Username) == "" || strings.TrimSpace(credentials.Password) == "" {
		return fmt.Errorf("armazenar credencial IPTV: %w", ErrInvalidCredential)
	}
	payload, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("armazenar credencial IPTV: %w", ErrInvalidCredential)
	}
	if err := store.Set(ctx, domain.SecretKey(reference), payload); err != nil {
		return fmt.Errorf("armazenar credencial IPTV: %w", ErrCredentialUnavailable)
	}
	return nil
}

// DeletePlaylistCredentials removes the secret under the reference. A missing
// secret is not an error: removal is idempotent.
func DeletePlaylistCredentials(ctx context.Context, store domain.SecretStore, reference string) error {
	if ctx == nil {
		return fmt.Errorf("remover credencial IPTV: contexto nil")
	}
	reference, err := normalizeCredentialReference(reference)
	if err != nil || reference == "" {
		return fmt.Errorf("remover credencial IPTV: %w", ErrInvalidCredential)
	}
	if store == nil {
		return fmt.Errorf("remover credencial IPTV: %w", ErrCredentialUnavailable)
	}
	if err := store.Delete(ctx, domain.SecretKey(reference)); err != nil {
		return fmt.Errorf("remover credencial IPTV: %w", ErrCredentialUnavailable)
	}
	return nil
}

// CredentialStatus is a redacted diagnostic outcome. It never carries the
// username or password itself.
type CredentialStatus string

const (
	CredentialStatusOK      CredentialStatus = "ok"
	CredentialStatusEmpty   CredentialStatus = "sem-referencia"
	CredentialStatusMissing CredentialStatus = "segredo-ausente-ou-indisponivel"
	CredentialStatusInvalid CredentialStatus = "invalido"
)

// DiagnosePlaylistCredentials validates that the reference resolves to a well
// formed payload in SecretStore. The result is safe to display. A failed read
// reports Missing because the SecretStore port cannot separate an absent
// secret from an unreachable keyring.
func DiagnosePlaylistCredentials(ctx context.Context, store domain.SecretStore, reference string) (CredentialStatus, error) {
	normalized, err := normalizeCredentialReference(reference)
	if err != nil {
		return CredentialStatusInvalid, nil
	}
	if normalized == "" {
		return CredentialStatusEmpty, nil
	}
	if _, err := loadPlaylistCredentials(ctx, store, normalized); err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredential):
			return CredentialStatusInvalid, nil
		default:
			return CredentialStatusMissing, nil
		}
	}
	return CredentialStatusOK, nil
}
