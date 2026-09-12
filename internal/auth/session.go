package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"golang.org/x/oauth2"
)

const (
	legacyRefreshTokenKey = domain.SecretKey("google.refresh_token")
	googleProvider        = "google"
)

const (
	MemorySessionWarning      = "Keyring indisponível: esta sessão ficará apenas na memória e será encerrada ao fechar o aplicativo."
	KeyringUnavailableWarning = "Keyring indisponível: sessões salvas não puderam ser restauradas e novos logins ficarão somente em memória."
	MissingSessionWarning     = "A credencial salva desta conta não foi encontrada. Desconecte e autentique novamente."
)

type ProviderIdentity struct {
	Provider string
	Subject  string
	Email    string
}

// Session isola token e metadados por perfil. O refresh token pode existir em
// memória durante o processo, mas só é persistido pelo SecretStore.
type Session struct {
	cfg       Config
	secrets   domain.SecretStore
	account   domain.AccountStore
	profileID string
	ephemeral bool

	mu          sync.RWMutex
	memoryToken string
	persistence domain.SessionPersistence
	tokenSource oauth2.TokenSource
}

func NewSession(cfg Config, secrets domain.SecretStore, account domain.AccountStore) *Session {
	return NewSessionForProfile(cfg, secrets, account, "default", false)
}

func NewSessionForProfile(
	cfg Config,
	secrets domain.SecretStore,
	account domain.AccountStore,
	profileID string,
	ephemeral bool,
) *Session {
	cfg.Scopes = append([]string(nil), cfg.Scopes...)
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		profileID = "default"
	}
	return &Session{
		cfg: cfg, secrets: secrets, account: account, profileID: profileID, ephemeral: ephemeral,
	}
}

func RefreshTokenKey(profileID string) domain.SecretKey {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		profileID = "default"
	}
	return domain.SecretKey("google.refresh_token/" + profileID)
}

func (s *Session) ProfileID() string {
	if s == nil {
		return ""
	}
	return s.profileID
}

func (s *Session) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.cfg.ClientID,
		ClientSecret: s.cfg.ClientSecret,
		Endpoint:     oauth2.Endpoint{AuthURL: s.cfg.AuthURI, TokenURL: s.cfg.TokenURI},
		Scopes:       s.cfg.Scopes,
	}
}

// StoreRefreshToken salva no keyring quando possível. Guest e indisponibilidade
// do keyring usam memória explicitamente, permitindo que o login prossiga sem
// criar falsa persistência.
func (s *Session) StoreRefreshToken(ctx context.Context, tok *oauth2.Token) (domain.SessionPersistence, error) {
	if s == nil {
		return "", errors.New("sessão OAuth indisponível")
	}
	if ctx == nil {
		return "", errors.New("contexto OAuth nulo")
	}
	if tok == nil || strings.TrimSpace(tok.RefreshToken) == "" {
		return "", errors.New("token sem refresh_token")
	}
	refreshToken := tok.RefreshToken
	if !s.ephemeral && s.secrets != nil {
		err := s.secrets.Set(ctx, RefreshTokenKey(s.profileID), []byte(refreshToken))
		if err == nil {
			s.setMemoryToken(refreshToken, domain.SessionPersistenceKeyring)
			return domain.SessionPersistenceKeyring, nil
		}
		if !errors.Is(err, domain.ErrSecretStoreUnavailable) {
			return "", err
		}
	}
	s.setMemoryToken(refreshToken, domain.SessionPersistenceMemory)
	return domain.SessionPersistenceMemory, nil
}

// SaveRefreshToken preserva o contrato anterior para adaptadores existentes.
func (s *Session) SaveRefreshToken(ctx context.Context, tok *oauth2.Token) error {
	_, err := s.StoreRefreshToken(ctx, tok)
	return err
}

func (s *Session) Persistence() domain.SessionPersistence {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.persistence
}

func (s *Session) Source(ctx context.Context) (oauth2.TokenSource, error) {
	if s == nil {
		return nil, errors.New("sessão OAuth indisponível")
	}
	if ctx == nil {
		return nil, errors.New("contexto OAuth nulo")
	}
	s.mu.RLock()
	if s.tokenSource != nil {
		source := s.tokenSource
		s.mu.RUnlock()
		return source, nil
	}
	refreshToken := s.memoryToken
	s.mu.RUnlock()
	if refreshToken == "" {
		if s.secrets == nil {
			return nil, errors.New("sem refresh token")
		}
		value, err := s.secrets.Get(ctx, RefreshTokenKey(s.profileID))
		if err != nil {
			return nil, err
		}
		refreshToken = string(value)
		if refreshToken == "" {
			return nil, errors.New("sem refresh token")
		}
		s.setMemoryToken(refreshToken, domain.SessionPersistenceKeyring)
	}
	tok := &oauth2.Token{RefreshToken: refreshToken}
	// Token refresh must outlive the individual RPC that first restored the
	// session. HTTP requests still carry their own cancellable contexts.
	source := s.oauthConfig().TokenSource(context.WithoutCancel(ctx), tok)
	s.mu.Lock()
	if s.memoryToken != refreshToken || refreshToken == "" {
		s.mu.Unlock()
		return nil, errors.New("sessão OAuth alterada durante a restauração")
	}
	if s.tokenSource == nil {
		s.tokenSource = source
	}
	source = s.tokenSource
	s.mu.Unlock()
	return source, nil
}

func (s *Session) Clear(ctx context.Context) error {
	if s == nil {
		return errors.New("sessão OAuth indisponível")
	}
	if ctx == nil {
		return errors.New("contexto OAuth nulo")
	}
	s.setMemoryToken("", "")
	if s.ephemeral || s.secrets == nil {
		return nil
	}
	err := s.secrets.Delete(ctx, RefreshTokenKey(s.profileID))
	if errors.Is(err, domain.ErrSecretNotFound) {
		return nil
	}
	return err
}

func (s *Session) SaveAccount(ctx context.Context, info domain.AccountInfo) error {
	if s == nil || s.account == nil {
		return errors.New("account store indisponível")
	}
	if ctx == nil {
		return errors.New("contexto OAuth nulo")
	}
	info.ProfileID = s.profileID
	return s.account.SaveAccount(ctx, info)
}

func (s *Session) Account(ctx context.Context) (domain.AccountInfo, bool, error) {
	if s == nil || s.account == nil {
		return domain.AccountInfo{}, false, errors.New("account store indisponível")
	}
	if ctx == nil {
		return domain.AccountInfo{}, false, errors.New("contexto OAuth nulo")
	}
	return s.account.Account(ctx)
}

func (s *Session) ClearAccount(ctx context.Context) error {
	if s == nil || s.account == nil {
		return errors.New("account store indisponível")
	}
	if ctx == nil {
		return errors.New("contexto OAuth nulo")
	}
	return s.account.ClearAccount(ctx)
}

func (s *Session) setMemoryToken(token string, persistence domain.SessionPersistence) {
	s.mu.Lock()
	s.memoryToken = token
	s.persistence = persistence
	s.tokenSource = nil
	s.mu.Unlock()
}

// MigrateLegacyRefreshToken move a chave global da versão anterior para o
// perfil default. A chave antiga só é apagada depois da gravação confirmada.
func MigrateLegacyRefreshToken(ctx context.Context, secrets domain.SecretStore, profileID string) error {
	if secrets == nil {
		return nil
	}
	current, err := secrets.Get(ctx, RefreshTokenKey(profileID))
	if err == nil && len(current) > 0 {
		deleteErr := secrets.Delete(ctx, legacyRefreshTokenKey)
		if errors.Is(deleteErr, domain.ErrSecretNotFound) {
			return nil
		}
		return deleteErr
	}
	if err != nil && !errors.Is(err, domain.ErrSecretNotFound) {
		return err
	}
	legacy, err := secrets.Get(ctx, legacyRefreshTokenKey)
	if errors.Is(err, domain.ErrSecretNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(legacy) == 0 {
		return nil
	}
	if err := secrets.Set(ctx, RefreshTokenKey(profileID), legacy); err != nil {
		return err
	}
	return secrets.Delete(ctx, legacyRefreshTokenKey)
}

func AuthenticatedClient(ctx context.Context, src oauth2.TokenSource) *http.Client {
	if ctx == nil || src == nil {
		return nil
	}
	return oauth2.NewClient(ctx, src)
}

func FetchIdentity(ctx context.Context, client *http.Client) (ProviderIdentity, error) {
	return fetchIdentityAt(ctx, client, "https://www.googleapis.com/oauth2/v2/userinfo")
}

func fetchIdentityAt(ctx context.Context, client *http.Client, endpoint string) (ProviderIdentity, error) {
	if ctx == nil {
		return ProviderIdentity{}, errors.New("contexto OAuth nulo")
	}
	if client == nil {
		return ProviderIdentity{}, errors.New("cliente HTTP indisponível")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ProviderIdentity{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return ProviderIdentity{}, fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ProviderIdentity{}, fmt.Errorf("userinfo: status %d", resp.StatusCode)
	}
	var payload struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ProviderIdentity{}, fmt.Errorf("userinfo: decode: %w", err)
	}
	if strings.TrimSpace(payload.ID) == "" {
		return ProviderIdentity{}, errors.New("userinfo sem subject")
	}
	if strings.TrimSpace(payload.Email) == "" {
		return ProviderIdentity{}, errors.New("userinfo sem email")
	}
	return ProviderIdentity{Provider: googleProvider, Subject: payload.ID, Email: payload.Email}, nil
}

func FetchEmail(ctx context.Context, client *http.Client) (string, error) {
	identity, err := FetchIdentity(ctx, client)
	return identity.Email, err
}

func NowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
