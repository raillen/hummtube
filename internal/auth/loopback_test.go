package auth

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

func tokenServer(t *testing.T, body *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("token body: %v", err)
		}
		if body != nil {
			*body = string(b)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rtok","token_type":"Bearer","expires_in":3600}`))
	}))
}

func TestFlowExchangePKCE(t *testing.T) {
	var exchanged string
	ts := tokenServer(t, &exchanged)
	defer ts.Close()

	cfg := Config{
		ClientID:     "cid",
		ClientSecret: "csec",
		AuthURI:      "https://provider.example/auth",
		TokenURI:     ts.URL,
		Scopes:       DefaultScopes,
	}

	f, err := StartFlow(cfg)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	defer f.Cancel()

	authURL, err := url.Parse(f.AuthURL())
	if err != nil {
		t.Fatalf("AuthURL: %v", err)
	}
	q := authURL.Query()
	if q.Get("state") != f.state {
		t.Error("authURL sem state do fluxo")
	}
	if q.Get("code_challenge") == "" {
		t.Error("authURL sem code_challenge (PKCE)")
	}
	if q.Get("redirect_uri") == "" {
		t.Error("authURL sem redirect_uri loopback")
	}
	if q.Get("prompt") != "consent" {
		t.Error("authURL deve solicitar consentimento para obter refresh token em reconexões")
	}

	port := f.listener.Addr().(*net.TCPAddr).Port
	redirect := fmt.Sprintf("http://127.0.0.1:%d/?code=thecode&state=%s", port, f.state)
	done := make(chan error, 1)
	go func() {
		resp, err := http.Get(redirect)
		if err != nil {
			done <- err
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		done <- nil
	}()

	tok, err := f.AwaitToken(context.Background())
	if err != nil {
		t.Fatalf("AwaitToken: %v", err)
	}
	if tok.RefreshToken != "rtok" {
		t.Errorf("RefreshToken = %q, esperado rtok", tok.RefreshToken)
	}
	if err := <-done; err != nil {
		t.Fatalf("redirect GET: %v", err)
	}

	form, err := url.ParseQuery(exchanged)
	if err != nil {
		t.Fatalf("parse token body: %v", err)
	}
	if form.Get("code") != "thecode" {
		t.Errorf("token exchange code = %q", form.Get("code"))
	}
	if form.Get("code_verifier") != f.verifier {
		t.Error("token exchange sem code_verifier (PKCE)")
	}
	if form.Get("grant_type") != "authorization_code" {
		t.Errorf("grant_type = %q", form.Get("grant_type"))
	}
}

func TestFlowRejectsWrongState(t *testing.T) {
	ts := tokenServer(t, nil)
	defer ts.Close()

	cfg := Config{
		ClientID: "cid", ClientSecret: "csec",
		AuthURI: "https://provider.example/auth", TokenURI: ts.URL,
		Scopes: DefaultScopes,
	}
	f, err := StartFlow(cfg)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	defer f.Cancel()

	port := f.listener.Addr().(*net.TCPAddr).Port
	go func() {
		_, _ = http.Get(fmt.Sprintf("http://127.0.0.1:%d/?code=x&state=wrong", port))
	}()
	if _, err := f.AwaitToken(context.Background()); err == nil {
		t.Fatal("AwaitToken com state errado não falhou")
	} else if !strings.Contains(err.Error(), "state inválido") {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func TestFlowCancelIsConcurrentSafe(t *testing.T) {
	f, err := StartFlow(Config{
		ClientID: "cid", ClientSecret: "secret",
		AuthURI: "https://provider.example/auth", TokenURI: "https://provider.example/token",
		Scopes: DefaultScopes,
	})
	if err != nil {
		t.Fatal(err)
	}
	const callers = 32
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			f.Cancel()
		}()
	}
	wg.Wait()
	select {
	case <-f.done:
	default:
		t.Fatal("Cancel não fechou done")
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	path := t.TempDir() + "/client_secret.json"
	content := `{"installed":{"client_id":"cid","project_id":"p","auth_uri":"https://a","token_uri":"https://t","client_secret":"csec","redirect_uris":["http://localhost"]}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_FILE", path)
	cfg, err := LoadConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ClientID != "cid" || cfg.ClientSecret != "csec" {
		t.Errorf("LoadConfig = %+v", cfg)
	}
	if cfg.AuthURI != "https://a" || cfg.TokenURI != "https://t" {
		t.Errorf("URIs = %q %q", cfg.AuthURI, cfg.TokenURI)
	}
	if len(cfg.Scopes) != len(DefaultScopes) {
		t.Errorf("scopes = %v", cfg.Scopes)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	// LoadConfig também procura no diretório de configuração do usuário, então
	// o teste precisa de um HOME isolado. Sem isso ele passa ou falha conforme
	// a máquina tenha ou não credenciais instaladas.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_FILE", "")
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_ID", "")
	t.Setenv("NANOTUBE_GOOGLE_CLIENT_SECRET", "")

	cfg, err := LoadConfig(context.Background())
	if err == nil {
		t.Fatal("LoadConfig sem credenciais não falhou")
	}
	if !strings.Contains(err.Error(), "credenciais") {
		t.Fatalf("erro inesperado: %v", err)
	}
	_ = cfg
}
