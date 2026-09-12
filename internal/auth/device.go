package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// DeviceCodeResponse contém as informações para o fluxo Device Authorization (TV).
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

const defaultDeviceCodeURI = "https://oauth2.googleapis.com/device/code"

// RequestDeviceCode inicia a solicitação do código de dispositivo para login em TV/outros aparelhos.
func RequestDeviceCode(ctx context.Context, cfg Config) (*DeviceCodeResponse, error) {
	if cfg.ClientID == "" {
		return nil, ErrNoCredentials
	}

	data := url.Values{}
	data.Set("client_id", cfg.ClientID)
	data.Set("scope", strings.Join(cfg.Scopes, " "))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, defaultDeviceCodeURI, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errObj map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errObj)
		return nil, fmt.Errorf("google device auth error (status %d): %v", resp.StatusCode, errObj)
	}

	var dcr DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcr); err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}

	if dcr.Interval <= 0 {
		dcr.Interval = 5
	}
	if dcr.VerificationURL == "" {
		dcr.VerificationURL = "https://www.google.com/device"
	}

	return &dcr, nil
}

// PollDeviceToken fica em polling aguardando o usuário autorizar o código em google.com/device.
func PollDeviceToken(ctx context.Context, cfg Config, deviceCode string, intervalSeconds int) (*oauth2.Token, error) {
	if intervalSeconds <= 0 {
		intervalSeconds = 5
	}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	tokenURL := cfg.TokenURI
	if tokenURL == "" {
		tokenURL = defaultTokenURI
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			data := url.Values{}
			data.Set("client_id", cfg.ClientID)
			if cfg.ClientSecret != "" {
				data.Set("client_secret", cfg.ClientSecret)
			}
			data.Set("device_code", deviceCode)
			data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				continue
			}

			var body map[string]interface{}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				tok := &oauth2.Token{}
				if at, ok := body["access_token"].(string); ok {
					tok.AccessToken = at
				}
				if rt, ok := body["refresh_token"].(string); ok {
					tok.RefreshToken = rt
				}
				if exp, ok := body["expires_in"].(float64); ok {
					tok.Expiry = time.Now().Add(time.Duration(exp) * time.Second)
				}
				if tok.RefreshToken == "" && tok.AccessToken == "" {
					return nil, errors.New("resposta sem tokens válidos")
				}
				return tok, nil
			}

			errCode, _ := body["error"].(string)
			switch errCode {
			case "authorization_pending":
				// Aguarda próxima iteração
				continue
			case "slow_down":
				ticker.Reset(time.Duration(intervalSeconds+5) * time.Second)
				continue
			case "expired_token":
				return nil, errors.New("o código de verificação expirou")
			case "access_denied":
				return nil, errors.New("autorização negada pelo usuário")
			default:
				if errCode != "" {
					return nil, fmt.Errorf("erro oauth: %s", errCode)
				}
			}
		}
	}
}
