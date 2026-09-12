//go:build native

// Smoke real e opt-in do fluxo OAuth (usa rede + navegador + keyring).
//
//	Run with: NANOTUBE_GOOGLE_CLIENT_FILE=<client_secret>.json \
//	  go test -tags native ./internal/auth/... -run NativeOAuthSmoke -v -count=1
//
// Abre o navegador, aguarda a autorização, salva o refresh token no keyring
// e imprime o e-mail da conta. Não falha se o teste for cancelado pelo
// usuário no navegador.
package auth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/storage"
)

type memAccountStore struct {
	info domain.AccountInfo
	ok   bool
}

func (m *memAccountStore) SaveAccount(_ context.Context, info domain.AccountInfo) error {
	m.info, m.ok = info, true
	return nil
}
func (m *memAccountStore) Account(_ context.Context) (domain.AccountInfo, bool, error) {
	return m.info, m.ok, nil
}
func (m *memAccountStore) ClearAccount(_ context.Context) error {
	m.info, m.ok = domain.AccountInfo{}, false
	return nil
}

func TestNativeOAuthSmoke(t *testing.T) {
	cfg, err := LoadConfig(context.Background())
	if err != nil {
		t.Skipf("sem credenciais: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	flow, err := StartFlow(cfg)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}
	if err := OpenBrowser(ctx, flow.AuthURL()); err != nil {
		t.Fatalf("OpenBrowser: %v", err)
	}
	t.Log("navegador aberto — autorize AGORA; o teste aguarda o loopback…")

	codeAt := make(chan struct{}, 1)
	origCh := flow.codeCh
	flow.codeCh = make(chan string, 1)
	go func() {
		c := <-origCh
		t.Logf("code recebido no loopback em %s", time.Now().Format("15:04:05"))
		close(codeAt)
		flow.codeCh <- c
	}()
	tok, err := flow.AwaitToken(ctx)
	if err != nil {
		select {
		case <-codeAt:
			t.Fatalf("AwaitToken (code chegou, troca falhou): %v", err)
		case <-ctx.Done():
			t.Fatalf("timeout sem autorização no navegador: %v", err)
		default:
			t.Fatalf("AwaitToken: %v", err)
		}
	}
	if tok.RefreshToken == "" {
		t.Fatal("token sem refresh_token")
	}

	session := NewSession(cfg,
		storage.NewKeyringSecretStore("nanotube-smoke"),
		&memAccountStore{})
	if err := session.SaveRefreshToken(ctx, tok); err != nil {
		t.Fatalf("SaveRefreshToken: %v", err)
	}
	src, err := session.Source(ctx)
	if err != nil {
		t.Fatalf("Source: %v", err)
	}
	email, err := FetchEmail(ctx, AuthenticatedClient(ctx, src))
	if err != nil {
		t.Fatalf("FetchEmail: %v", err)
	}
	t.Logf("conectado como %s", email)

	_ = os.Getenv("NANOTUBE_KEEP_SMOKE_TOKEN") // se não setado, limpa
	if os.Getenv("NANOTUBE_KEEP_SMOKE_TOKEN") == "" {
		_ = session.Clear(ctx)
	}
}
