package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"sync"

	"golang.org/x/oauth2"
)

// Flow is a running PKCE loopback authorization against the OAuth provider.
type Flow struct {
	oauth    *oauth2.Config
	listener net.Listener
	srv      *http.Server
	state    string
	verifier string
	authURL  string
	codeCh   chan string
	errCh    chan error
	done     chan struct{}
	cancel   sync.Once
}

// AuthURL returns the authorization URL to open in a browser.
func (f *Flow) AuthURL() string { return f.authURL }

// StartFlow binds a loopback listener on 127.0.0.1, generates the CSRF
// state and PKCE verifier, and builds the authorization URL.
func StartFlow(cfg Config) (*Flow, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listener loopback: %w", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	redirect := fmt.Sprintf("http://127.0.0.1:%d/", port)

	state, err := randomToken()
	if err != nil {
		_ = l.Close()
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	ocfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     oauth2.Endpoint{AuthURL: cfg.AuthURI, TokenURL: cfg.TokenURI},
		RedirectURL:  redirect,
		Scopes:       cfg.Scopes,
	}

	f := &Flow{
		oauth:    ocfg,
		listener: l,
		state:    state,
		verifier: verifier,
		authURL: ocfg.AuthCodeURL(
			state,
			oauth2.AccessTypeOffline,
			oauth2.S256ChallengeOption(verifier),
			oauth2.SetAuthURLParam("prompt", "consent"),
			oauth2.SetAuthURLParam("include_granted_scopes", "true"),
		),
		codeCh: make(chan string, 1),
		errCh:  make(chan error, 1),
		done:   make(chan struct{}),
	}
	f.srv = &http.Server{Handler: http.HandlerFunc(f.handle)}
	go func() {
		if err := f.srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case f.errCh <- err:
			case <-f.done:
			}
		}
	}()
	return f, nil
}

func (f *Flow) handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if errMsg := q.Get("error"); errMsg != "" {
		f.replyErr(w, "erro de autorização: "+errMsg)
		return
	}
	if got := q.Get("state"); got != f.state {
		f.replyErr(w, "state inválido")
		return
	}
	code := q.Get("code")
	if code == "" {
		f.replyErr(w, "sem code na resposta")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<!doctype html><meta charset=utf-8><h3>NanoTube conectado</h3><p>Você já pode fechar esta aba.</p>")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	select {
	case f.codeCh <- code:
	case <-f.done:
		return
	}
}

func (f *Flow) replyErr(w http.ResponseWriter, msg string) {
	select {
	case f.errCh <- errors.New(msg):
	case <-f.done:
		return
	}
	http.Error(w, msg, http.StatusBadRequest)
}

// AwaitToken waits for the loopback redirect, validates state and exchanges
// the authorization code for a token using the PKCE verifier.
func (f *Flow) AwaitToken(ctx context.Context) (*oauth2.Token, error) {
	if f == nil {
		return nil, errors.New("fluxo OAuth nulo")
	}
	if ctx == nil {
		return nil, errors.New("contexto OAuth nulo")
	}
	defer f.Cancel()
	select {
	case code := <-f.codeCh:
		tok, err := f.oauth.Exchange(ctx, code, oauth2.VerifierOption(f.verifier))
		if err != nil {
			return nil, fmt.Errorf("troca de code: %w", err)
		}
		return tok, nil
	case err := <-f.errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Cancel stops the loopback server and releases the listener.
func (f *Flow) Cancel() {
	if f == nil {
		return
	}
	f.cancel.Do(func() {
		close(f.done)
		if f.srv != nil {
			_ = f.srv.Close()
		}
		if f.listener != nil {
			_ = f.listener.Close()
		}
	})
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// OpenBrowser opens url with the platform default browser. It fails only
// when no browser could be launched.
func OpenBrowser(ctx context.Context, url string) error {
	cmd := exec.CommandContext(ctx, "xdg-open", url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
