package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/lastfm"
)

type LastFMStatus struct {
	Configured bool   `json:"configured"`
	Connected  bool   `json:"connected"`
	Username   string `json:"username,omitempty"`
	Warning    string `json:"warning,omitempty"`
}

type lastFMCredential struct {
	Username   string `json:"username"`
	SessionKey string `json:"session_key"`
}

func lastFMSecretKey(profileID string) domain.SecretKey {
	return domain.SecretKey("lastfm.session/" + profileID)
}

func (s *AppServices) GetLastFMStatus(ctx context.Context) (LastFMStatus, error) {
	status := LastFMStatus{Configured: s.LastFM != nil && s.LastFM.Config.Configured()}
	if !status.Configured {
		status.Warning = "Configure as credenciais do aplicativo Last.fm fora da interface."
		return status, nil
	}
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return status, err
	}
	if profile.IsGuest() {
		status.Warning = "Last.fm não é persistido em perfis convidados."
		return status, nil
	}
	credential, err := s.loadLastFMCredential(ctx, profile.ID)
	if errors.Is(err, domain.ErrSecretNotFound) {
		return status, nil
	}
	if errors.Is(err, domain.ErrSecretStoreUnavailable) {
		status.Warning = "O Keyring está indisponível; a conexão Last.fm não pode ser restaurada."
		return status, nil
	}
	if err != nil {
		return status, err
	}
	status.Connected = true
	status.Username = credential.Username
	return status, nil
}

func (s *AppServices) StartLastFMAuthorization(ctx context.Context) (string, error) {
	if s.LastFM == nil || !s.LastFM.Config.Configured() {
		return "", errors.New("last.fm não configurado: use NANOTUBE_LASTFM_API_KEY e NANOTUBE_LASTFM_SHARED_SECRET")
	}
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return "", err
	}
	if profile.IsGuest() {
		return "", errors.New("last.fm requer um perfil persistente")
	}
	token, authorizationURL, err := s.LastFM.StartAuthorization(ctx)
	if err != nil {
		return "", err
	}
	s.lastFMMu.Lock()
	s.lastFMPending[profile.ID] = token
	s.lastFMMu.Unlock()
	return authorizationURL, nil
}

func (s *AppServices) CompleteLastFMAuthorization(ctx context.Context) (LastFMStatus, error) {
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return LastFMStatus{}, err
	}
	s.lastFMMu.Lock()
	token := s.lastFMPending[profile.ID]
	s.lastFMMu.Unlock()
	if token == "" {
		s.auditCredential(ctx, profile.ID, "lastfm", "verification_failed", "Autorização não iniciada")
		return LastFMStatus{}, errors.New("last.fm: inicie a autorização antes de concluir")
	}
	session, err := s.LastFM.ExchangeSession(ctx, token)
	if err != nil {
		s.auditCredential(ctx, profile.ID, "lastfm", "verification_failed", "Sessão não confirmada pelo provedor")
		return LastFMStatus{}, err
	}
	credential := lastFMCredential{Username: session.Name, SessionKey: session.Key}
	payload, err := json.Marshal(credential)
	if err != nil {
		return LastFMStatus{}, fmt.Errorf("last.fm: preparar credencial: %w", err)
	}
	if s.SecretStore == nil {
		s.auditCredential(ctx, profile.ID, "lastfm", "verification_failed", "Keyring indisponível")
		return LastFMStatus{}, errors.New("last.fm: Keyring obrigatório e indisponível")
	}
	_, previousCredentialErr := s.loadLastFMCredential(ctx, profile.ID)
	if err := s.SecretStore.Set(ctx, lastFMSecretKey(profile.ID), payload); err != nil {
		s.auditCredential(ctx, profile.ID, "lastfm", "verification_failed", "Persistência segura da sessão falhou")
		return LastFMStatus{}, fmt.Errorf("last.fm: persistir sessão no Keyring: %w", err)
	}
	s.lastFMMu.Lock()
	delete(s.lastFMPending, profile.ID)
	s.lastFMMu.Unlock()
	action := "connected"
	if previousCredentialErr == nil {
		action = "rotated"
	}
	s.auditCredential(ctx, profile.ID, "lastfm", action, "Sessão Last.fm autorizada")
	return LastFMStatus{Configured: true, Connected: true, Username: session.Name}, nil
}

func (s *AppServices) DisconnectLastFM(ctx context.Context) error {
	profileID, err := s.Repo.ActiveProfileID(ctx)
	if err != nil {
		return err
	}
	if s.SecretStore == nil {
		return errors.New("last.fm: Keyring indisponível")
	}
	if err := s.SecretStore.Delete(ctx, lastFMSecretKey(profileID)); err != nil {
		return fmt.Errorf("last.fm: remover sessão: %w", err)
	}
	s.lastFMMu.Lock()
	delete(s.lastFMPending, profileID)
	s.lastFMMu.Unlock()
	s.auditCredential(ctx, profileID, "lastfm", "revoked", "Sessão Last.fm desconectada")
	return nil
}

func (s *AppServices) ScrobbleLastFM(ctx context.Context, artist, track string, playedMs, durationMs int64) error {
	if s.LastFM == nil || !s.LastFM.Config.Configured() {
		return nil
	}
	if playedMs < 0 || durationMs < 0 || playedMs > durationMs+5000 {
		return errors.New("last.fm: duração de reprodução inválida")
	}
	played := time.Duration(playedMs) * time.Millisecond
	duration := time.Duration(durationMs) * time.Millisecond
	if !lastfm.EligibleForScrobble(played, duration) {
		return nil
	}
	profileID, err := s.Repo.ActiveProfileID(ctx)
	if err != nil {
		return err
	}
	credential, err := s.loadLastFMCredential(ctx, profileID)
	if errors.Is(err, domain.ErrSecretNotFound) || errors.Is(err, domain.ErrSecretStoreUnavailable) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.LastFM.Scrobble(ctx, credential.SessionKey, strings.TrimSpace(artist), strings.TrimSpace(track), time.Now().Add(-played))
}

func (s *AppServices) loadLastFMCredential(ctx context.Context, profileID string) (lastFMCredential, error) {
	if s.SecretStore == nil {
		return lastFMCredential{}, domain.ErrSecretStoreUnavailable
	}
	payload, err := s.SecretStore.Get(ctx, lastFMSecretKey(profileID))
	if err != nil {
		return lastFMCredential{}, err
	}
	if len(payload) == 0 {
		return lastFMCredential{}, domain.ErrSecretNotFound
	}
	var credential lastFMCredential
	if err := json.Unmarshal(payload, &credential); err != nil {
		return lastFMCredential{}, errors.New("last.fm: credencial do Keyring inválida")
	}
	if credential.SessionKey == "" {
		return lastFMCredential{}, errors.New("last.fm: session key vazia")
	}
	return credential, nil
}
