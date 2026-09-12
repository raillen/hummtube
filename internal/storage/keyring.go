package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/zalando/go-keyring"
)

// ErrKeyringUnavailable reports that the system keyring could not be used.
var ErrKeyringUnavailable = domain.ErrSecretStoreUnavailable

// KeyringSecretStore implements domain.SecretStore over the system keyring.
type KeyringSecretStore struct {
	Service string
}

// NewKeyringSecretStore builds a store scoped to the given service name.
func NewKeyringSecretStore(service string) *KeyringSecretStore {
	return &KeyringSecretStore{Service: service}
}

// Get reads a secret from the keyring.
func (s *KeyringSecretStore) Get(ctx context.Context, key domain.SecretKey) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("contexto nulo")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	v, err := keyring.Get(s.Service, string(key))
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, domain.ErrSecretNotFound
		}
		return nil, classifyKeyringError("ler", err)
	}
	return []byte(v), nil
}

// Set stores a secret in the keyring.
func (s *KeyringSecretStore) Set(ctx context.Context, key domain.SecretKey, value []byte) error {
	if ctx == nil {
		return errors.New("contexto nulo")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := keyring.Set(s.Service, string(key), string(value)); err != nil {
		return classifyKeyringError("gravar", err)
	}
	return nil
}

// Delete removes a secret from the keyring.
func (s *KeyringSecretStore) Delete(ctx context.Context, key domain.SecretKey) error {
	if ctx == nil {
		return errors.New("contexto nulo")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := keyring.Delete(s.Service, string(key)); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return classifyKeyringError("remover", err)
	}
	return nil
}

func classifyKeyringError(operation string, err error) error {
	if isKeyringUnavailable(err) {
		return fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
	}
	return fmt.Errorf("keyring: %s segredo: %w", operation, err)
}

func isKeyringUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, keyring.ErrUnsupportedPlatform) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, denied := range []string{"accessdenied", "access denied", "permission denied"} {
		if strings.Contains(message, denied) {
			return false
		}
	}
	for _, marker := range []string{
		"cannot autolaunch", "connection refused", "dbus-launch", "failed to connect to socket",
		"name has no owner", "no such file or directory",
		"serviceunknown", "secret service is not available", "unsupported platform",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// KeyringProbe performs a set/get/delete roundtrip and returns a status string.
// Used by diagnostics; never stores a real secret.
func KeyringProbe(ctx context.Context) string {
	store := NewKeyringSecretStore("nanotube-diagnostics")
	key := domain.SecretKey("probe")
	value := []byte("phase0")

	if err := store.Set(ctx, key, value); err != nil {
		return "indisponível (" + shortReason(err) + ")"
	}
	got, err := store.Get(ctx, key)
	if err != nil {
		return "indisponível (" + shortReason(err) + ")"
	}
	_ = store.Delete(ctx, key)
	if string(got) != string(value) {
		return "indisponível (roundtrip divergente)"
	}
	return "disponível"
}

func shortReason(err error) string {
	s := err.Error()
	if len(s) > 80 {
		return s[:80]
	}
	return s
}
