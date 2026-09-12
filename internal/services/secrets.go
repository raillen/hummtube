package services

import (
	"context"
	"errors"
	"time"

	"github.com/nanotube/nanotube-web/internal/auth"
	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/storage"
)

type SecretMetadata struct {
	ID        string                 `json:"id"`
	Provider  string                 `json:"provider"`
	Label     string                 `json:"label"`
	ProfileID string                 `json:"profile_id"`
	State     domain.CredentialState `json:"state"`
	Storage   string                 `json:"storage"`
	RotatedAt time.Time              `json:"rotated_at,omitempty"`
	ExpiresAt time.Time              `json:"expires_at,omitempty"`
	CanRotate bool                   `json:"can_rotate"`
	CanRevoke bool                   `json:"can_revoke"`
	Warning   string                 `json:"warning,omitempty"`
}

type SecretInventory struct {
	Secrets []SecretMetadata               `json:"secrets"`
	Audit   []storage.CredentialAuditEvent `json:"audit"`
}

// GetSecretInventory expõe apenas metadados. IDs internos do Keyring e valores
// secretos nunca atravessam o bridge RPC.
func (s *AppServices) GetSecretInventory(ctx context.Context) (SecretInventory, error) {
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return SecretInventory{}, err
	}
	audit, err := s.Repo.CredentialAudit(ctx, profile.ID, 30)
	if err != nil {
		return SecretInventory{}, err
	}
	inventory := SecretInventory{Secrets: []SecretMetadata{}, Audit: audit}
	configState := domain.CredentialStateAvailable
	configWarning := ""
	if _, configErr := auth.LoadConfig(ctx); configErr != nil {
		configState = domain.CredentialStateMissing
		configWarning = "Credenciais OAuth do aplicativo ausentes ou inválidas. Configure-as fora da interface com permissões restritas."
	}
	inventory.Secrets = append(inventory.Secrets, SecretMetadata{
		ID: "google-oauth-client", Provider: "google-config", Label: "Configuração OAuth do aplicativo", ProfileID: profile.ID,
		State: configState, Storage: "configuração externa", Warning: configWarning,
	})
	account, connected, err := storage.NewAccountRepository(s.Repo.DB(), profile.ID).Account(ctx)
	if err != nil {
		return inventory, err
	}
	if connected {
		s.assessAccountCredential(ctx, &account)
		inventory.Secrets = append(inventory.Secrets, SecretMetadata{
			ID: "google-oauth", Provider: "google", Label: account.Email, ProfileID: profile.ID,
			State: account.CredentialState, Storage: string(account.SessionPersistence), RotatedAt: parseRFC3339(account.ConnectedAt),
			CanRotate: !profile.IsGuest(), CanRevoke: true, Warning: account.Warning,
		})
	}
	if s.LastFM != nil && s.LastFM.Config.Configured() && !profile.IsGuest() {
		credential, credentialErr := s.loadLastFMCredential(ctx, profile.ID)
		state := domain.CredentialStateAvailable
		warning := ""
		if errors.Is(credentialErr, domain.ErrSecretNotFound) {
			state = domain.CredentialStateMissing
		} else if credentialErr != nil {
			state = domain.CredentialStateUnavailable
			warning = "Não foi possível verificar a sessão Last.fm no Keyring."
		}
		inventory.Secrets = append(inventory.Secrets, SecretMetadata{
			ID: "lastfm-session", Provider: "lastfm", Label: credential.Username, ProfileID: profile.ID,
			State: state, Storage: "keyring", RotatedAt: latestCredentialChange(audit, "lastfm"),
			CanRotate: true, CanRevoke: state == domain.CredentialStateAvailable,
			Warning: warning,
		})
	}
	return inventory, nil
}

func latestCredentialChange(events []storage.CredentialAuditEvent, provider string) time.Time {
	for _, event := range events {
		if event.Provider == provider && (event.Action == "connected" || event.Action == "rotated") {
			return event.OccurredAt
		}
	}
	return time.Time{}
}

func parseRFC3339(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed
}

func (s *AppServices) auditCredential(ctx context.Context, profileID, provider, action, detail string) {
	_ = s.Repo.RecordCredentialAudit(context.WithoutCancel(ctx), profileID, provider, action, detail)
}
