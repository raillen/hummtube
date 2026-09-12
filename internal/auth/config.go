// Package auth implements the desktop OAuth2/PKCE loopback flow against
// Google (docs/01-architecture/adrs/ADR-004-oauth.md,
// docs/03-implementation/YOUTUBE_AND_AUTH.md). It never logs tokens.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/nanotube/nanotube-web/internal/app"
)

// Escopos OAuth mínimos (somente leitura; ADR-004 pede o menor escopo útil).
const (
	ScopeYouTubeReadOnly = "https://www.googleapis.com/auth/youtube.readonly"
	ScopeUserInfoEmail   = "https://www.googleapis.com/auth/userinfo.email"
)

// DefaultScopes is the read-only scope set used on first login.
var DefaultScopes = []string{ScopeYouTubeReadOnly, ScopeUserInfoEmail}

// ErrNoCredentials reports missing Google OAuth client credentials.
var ErrNoCredentials = errors.New(
	"credenciais Google ausentes: coloque o client_secret JSON em " +
		"~/.config/nanotube-web/client_secret.json (ou aponte NANOTUBE_GOOGLE_CLIENT_FILE)")

const (
	defaultAuthURI  = "https://accounts.google.com/o/oauth2/auth"
	defaultTokenURI = "https://oauth2.googleapis.com/token"
)

// Config holds the OAuth2 client settings.
type Config struct {
	ClientID     string
	ClientSecret string
	AuthURI      string
	TokenURI     string
	Scopes       []string
}

type clientSecretFile struct {
	Installed *clientInfo `json:"installed"`
	Web       *clientInfo `json:"web"`
}

type clientInfo struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	AuthURI      string   `json:"auth_uri"`
	TokenURI     string   `json:"token_uri"`
	RedirectURIs []string `json:"redirect_uris"`
}

// LoadConfig procura as credenciais OAuth.
func LoadConfig(ctx context.Context) (Config, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return Config{}, err
		}
	}
	if file := os.Getenv("NANOTUBE_GOOGLE_CLIENT_FILE"); file != "" {
		return configFromFile(file)
	}
	id, secret := os.Getenv("NANOTUBE_GOOGLE_CLIENT_ID"), os.Getenv("NANOTUBE_GOOGLE_CLIENT_SECRET")
	if id == "" || secret == "" {
		if file, ok := discoverClientFile(); ok {
			return configFromFile(file)
		}
		return Config{}, ErrNoCredentials
	}
	return Config{
		ClientID:     id,
		ClientSecret: secret,
		AuthURI:      defaultAuthURI,
		TokenURI:     defaultTokenURI,
		Scopes:       DefaultScopes,
	}, nil
}

// ClientFileNames são os nomes aceitos no diretório de configuração.
var ClientFileNames = []string{"client_secret.json", "credentials.json"}

func discoverClientFile() (string, bool) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, ".config")
		}
	}

	dirs := make([]string, 0, 2)
	if base != "" {
		dirs = append(dirs, filepath.Join(base, "nanotube-web"), filepath.Join(base, "nanotube"))
	}

	for _, dir := range dirs {
		for _, name := range ClientFileNames {
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, true
			}
		}
		matches, err := filepath.Glob(filepath.Join(dir, "client_secret*.json"))
		if err == nil && len(matches) > 0 {
			sort.Strings(matches)
			return matches[0], true
		}
	}
	return "", false
}

func ClientFileLocation() string {
	dir, err := app.ConfigDir()
	if err != nil {
		return "~/.config/nanotube-web"
	}
	return filepath.Join(dir, "client_secret.json")
}

func configFromFile(path string) (Config, error) {
	if err := validateClientFileSecurity(path); err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("client file: %w", err)
	}
	var f clientSecretFile
	if err := json.Unmarshal(data, &f); err != nil {
		return Config{}, fmt.Errorf("client file inválido: %w", err)
	}
	info := f.Installed
	if info == nil {
		info = f.Web
	}
	if info == nil || info.ClientID == "" {
		return Config{}, fmt.Errorf("client file sem client_id: %s", path)
	}
	return Config{
		ClientID:     info.ClientID,
		ClientSecret: info.ClientSecret,
		AuthURI:      firstNonEmpty(info.AuthURI, defaultAuthURI),
		TokenURI:     firstNonEmpty(info.TokenURI, defaultTokenURI),
		Scopes:       DefaultScopes,
	}, nil
}

func validateClientFileSecurity(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("client file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("client file: links simbólicos não são aceitos")
	}
	if !info.Mode().IsRegular() {
		return errors.New("client file: caminho não é um arquivo regular")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("client file: permissões inseguras %04o; use 0600", info.Mode().Perm())
	}
	return nil
}

func ValidateClientFile(path string) (Config, error) {
	return configFromFile(path)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
