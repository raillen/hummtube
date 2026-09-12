// Package lastfm implements the opt-in Last.fm API boundary used by music mode.
package lastfm

import (
	"context"
	"crypto/md5" // #nosec G501 -- Last.fm protocol mandates MD5 for api_sig.
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	defaultAPIURL  = "https://ws.audioscrobbler.com/2.0/"
	defaultAuthURL = "https://www.last.fm/api/auth/"
)

// Config is provisioned outside the UI so the shared secret never crosses RPC.
type Config struct {
	APIKey       string
	SharedSecret string
}

// LoadConfig reads operator-provisioned Last.fm application credentials.
func LoadConfig() Config {
	return Config{
		APIKey:       strings.TrimSpace(os.Getenv("NANOTUBE_LASTFM_API_KEY")),
		SharedSecret: strings.TrimSpace(os.Getenv("NANOTUBE_LASTFM_SHARED_SECRET")),
	}
}

func (c Config) Configured() bool {
	return c.APIKey != "" && c.SharedSecret != ""
}

type Client struct {
	Config  Config
	HTTP    *http.Client
	APIURL  string
	AuthURL string
}

type Session struct {
	Name string
	Key  string
}

func NewClient(config Config) *Client {
	return &Client{
		Config:  config,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		APIURL:  defaultAPIURL,
		AuthURL: defaultAuthURL,
	}
}

func (c *Client) StartAuthorization(ctx context.Context) (string, string, error) {
	if err := c.validate(); err != nil {
		return "", "", err
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := c.call(ctx, url.Values{"method": {"auth.getToken"}, "api_key": {c.Config.APIKey}}, false, &response); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(response.Token) == "" {
		return "", "", errors.New("last.fm: resposta sem token de autorização")
	}
	authURL, err := url.Parse(c.AuthURL)
	if err != nil {
		return "", "", fmt.Errorf("last.fm: URL de autorização inválida: %w", err)
	}
	query := authURL.Query()
	query.Set("api_key", c.Config.APIKey)
	query.Set("token", response.Token)
	authURL.RawQuery = query.Encode()
	return response.Token, authURL.String(), nil
}

func (c *Client) ExchangeSession(ctx context.Context, token string) (Session, error) {
	if err := c.validate(); err != nil {
		return Session{}, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, errors.New("last.fm: autorização não iniciada")
	}
	params := url.Values{
		"method":  {"auth.getSession"},
		"api_key": {c.Config.APIKey},
		"token":   {token},
	}
	params.Set("api_sig", c.signature(params))
	var response struct {
		Session struct {
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"session"`
	}
	if err := c.call(ctx, params, true, &response); err != nil {
		return Session{}, err
	}
	if response.Session.Key == "" {
		return Session{}, errors.New("last.fm: resposta sem session key")
	}
	return Session{Name: response.Session.Name, Key: response.Session.Key}, nil
}

func (c *Client) Scrobble(ctx context.Context, sessionKey, artist, track string, startedAt time.Time) error {
	if err := c.validate(); err != nil {
		return err
	}
	sessionKey = strings.TrimSpace(sessionKey)
	artist = cleanMetadata(artist, 255)
	track = cleanMetadata(track, 255)
	if sessionKey == "" || artist == "" || track == "" {
		return errors.New("last.fm: sessão, artista e faixa são obrigatórios")
	}
	if startedAt.IsZero() || startedAt.After(time.Now().Add(time.Minute)) {
		return errors.New("last.fm: início de reprodução inválido")
	}
	params := url.Values{
		"method":    {"track.scrobble"},
		"api_key":   {c.Config.APIKey},
		"sk":        {sessionKey},
		"artist":    {artist},
		"track":     {track},
		"timestamp": {fmt.Sprintf("%d", startedAt.Unix())},
	}
	params.Set("api_sig", c.signature(params))
	return c.call(ctx, params, true, &struct{}{})
}

func EligibleForScrobble(played, duration time.Duration) bool {
	if duration < 30*time.Second || played < 30*time.Second {
		return false
	}
	threshold := duration / 2
	if threshold > 4*time.Minute {
		threshold = 4 * time.Minute
	}
	return played >= threshold
}

func (c *Client) validate() error {
	if c == nil || !c.Config.Configured() {
		return errors.New("last.fm: configure NANOTUBE_LASTFM_API_KEY e NANOTUBE_LASTFM_SHARED_SECRET")
	}
	if c.HTTP == nil {
		return errors.New("last.fm: cliente HTTP ausente")
	}
	return nil
}

func (c *Client) signature(params url.Values) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key != "format" && key != "callback" && key != "api_sig" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var payload strings.Builder
	for _, key := range keys {
		payload.WriteString(key)
		payload.WriteString(params.Get(key))
	}
	payload.WriteString(c.Config.SharedSecret)
	digest := md5.Sum([]byte(payload.String())) // #nosec G401 -- required by Last.fm API signing.
	return hex.EncodeToString(digest[:])
}

func (c *Client) call(ctx context.Context, params url.Values, signed bool, destination any) error {
	params.Set("format", "json")
	method := http.MethodGet
	var body *strings.Reader
	if signed {
		method = http.MethodPost
		body = strings.NewReader(params.Encode())
	} else {
		body = strings.NewReader("")
	}
	endpoint := c.APIURL
	if method == http.MethodGet {
		endpoint += "?" + params.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("last.fm: criar requisição: %w", err)
	}
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return fmt.Errorf("last.fm: comunicação: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("last.fm: HTTP %d", response.StatusCode)
	}
	var envelope json.RawMessage
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("last.fm: resposta inválida: %w", err)
	}
	var apiError struct {
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(envelope, &apiError); err == nil && apiError.Error != 0 {
		return fmt.Errorf("last.fm: %s (código %d)", cleanMetadata(apiError.Message, 200), apiError.Error)
	}
	if err := json.Unmarshal(envelope, destination); err != nil {
		return fmt.Errorf("last.fm: decodificar resposta: %w", err)
	}
	return nil
}

func cleanMetadata(value string, limit int) string {
	value = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' {
			return -1
		}
		return r
	}, value))
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}
