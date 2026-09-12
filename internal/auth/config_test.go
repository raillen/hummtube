package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const sampleClientFile = `{
  "installed": {
    "client_id": "123-abc.apps.googleusercontent.com",
    "client_secret": "segredo-de-teste",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "redirect_uris": ["http://localhost"]
  }
}`

func writeClientFile(t *testing.T, name, content string) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_FILE", "")
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_ID", "")
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_SECRET", "")

	dir := filepath.Join(base, "nanotube-web")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestLoadConfigFindsClientFileInConfigDir(t *testing.T) {
	writeClientFile(t, "client_secret.json", sampleClientFile)

	cfg, err := LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ClientID != "123-abc.apps.googleusercontent.com" {
		t.Errorf("client id: %q", cfg.ClientID)
	}
	if cfg.ClientSecret != "segredo-de-teste" {
		t.Error("client secret não foi lido")
	}
	if len(cfg.Scopes) == 0 {
		t.Error("escopos padrão ausentes")
	}
}

func TestLoadConfigAcceptsGoogleOriginalFileName(t *testing.T) {
	writeClientFile(t, "client_secret_148441613822-abc.apps.googleusercontent.com.json", sampleClientFile)

	cfg, err := LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ClientID == "" {
		t.Error("client id vazio com o nome original do Google")
	}
}

func TestLoadConfigPrefersEnvOverConfigDir(t *testing.T) {
	writeClientFile(t, "client_secret.json", sampleClientFile)
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_ID", "env-id")
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_SECRET", "env-secret")

	cfg, err := LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ClientID != "env-id" {
		t.Errorf("env deveria ter precedência, veio %q", cfg.ClientID)
	}
}

func TestLoadConfigWithoutCredentials(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_FILE", "")
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_ID", "")
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_SECRET", "")

	_, err := LoadConfig(context.Background())
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("esperado ErrNoCredentials, veio %v", err)
	}
}

func TestErrNoCredentialsPointsToLocation(t *testing.T) {
	message := ErrNoCredentials.Error()
	for _, want := range []string{"client_secret.json", ".config/nanotube-web"} {
		if !strings.Contains(message, want) {
			t.Errorf("mensagem não menciona %q: %s", want, message)
		}
	}
}

func TestClientFileLocationIsInConfigDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)

	location := ClientFileLocation()
	if !strings.Contains(location, "nanotube-web") || !strings.Contains(location, "client_secret.json") {
		t.Errorf("caminho inesperado: %s", location)
	}
}

func TestLoadConfigRejectsInvalidClientFile(t *testing.T) {
	writeClientFile(t, "client_secret.json", `{"installed": {}}`)

	if _, err := LoadConfig(context.Background()); err == nil {
		t.Fatal("arquivo sem client_id deveria falhar com erro claro")
	}
}

func TestLoadConfigRejectsInsecurePermissionsAndSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissões POSIX não se aplicam ao Windows")
	}
	path := writeClientFile(t, "client_secret.json", sampleClientFile)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "permissões inseguras") {
		t.Fatalf("arquivo 0644 foi aceito: %v", err)
	}

	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(path), "linked-secret.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateClientFile(link); err == nil || !strings.Contains(err.Error(), "links simbólicos") {
		t.Fatalf("symlink de client secret foi aceito: %v", err)
	}
}
