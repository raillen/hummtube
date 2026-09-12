package iptv

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestHTTPM3USourceFetchesPlaylist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("método = %s, want GET", request.Method)
		}
		writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = writer.Write([]byte("#EXTM3U\n#EXTINF:-1 group-title=\"Filmes\",Filme fixture\nhttps://example.invalid/movie.mp4\n"))
	}))
	defer server.Close()

	source := HTTPM3USource{Client: server.Client()}
	playlist, err := source.Fetch(context.Background(), SourceConfig{
		ID:          "fixture",
		Name:        "Fixture",
		Format:      SourceFormatM3U,
		PlaylistURL: server.URL + "/playlist.m3u",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(playlist.Items) != 1 || playlist.Items[0].Kind != ContentKindMovie {
		t.Fatalf("playlist = %+v, want one movie", playlist)
	}
}

func TestHTTPM3USourceUsesSecretStoreWithoutMutatingSource(t *testing.T) {
	const username = "fixture-user"
	const password = "fixture-pass"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("username") != username || request.URL.Query().Get("password") != password {
			t.Fatalf("credenciais ausentes ou incorretas: query=%v", request.URL.Query())
		}
		_, _ = writer.Write([]byte("#EXTM3U\n#EXTINF:-1 group-title=\"TV\",Canal autenticado\nhttps://stream.invalid/live.m3u8\n"))
	}))
	defer server.Close()

	config := SourceConfig{
		ID: "fixture", Name: "Fixture", Format: SourceFormatM3U,
		PlaylistURL: server.URL + "/playlist.m3u?output=hls", CredentialRef: "fixture-ref",
	}
	playlist, err := (HTTPM3USource{
		Client:      server.Client(),
		Credentials: staticSecretStore{domain.SecretKey("fixture-ref"): []byte(`{"username":"fixture-user","password":"fixture-pass"}`)},
	}).Fetch(context.Background(), config)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(playlist.Items) != 1 {
		t.Fatalf("itens = %d, want 1", len(playlist.Items))
	}
	if strings.Contains(config.PlaylistURL, username) || strings.Contains(config.PlaylistURL, password) {
		t.Fatalf("configuração foi contaminada por credenciais: %q", config.PlaylistURL)
	}
}

func TestHTTPM3USourceRejectsUnsupportedEndpoint(t *testing.T) {
	_, err := (HTTPM3USource{}).Fetch(context.Background(), SourceConfig{
		ID:          "fixture",
		PlaylistURL: "file:///tmp/playlist.m3u",
	})
	if !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("erro = %v, want ErrInvalidSource", err)
	}
}

func TestHTTPM3USourceProductionClientRejectsLoopback(t *testing.T) {
	_, err := (HTTPM3USource{}).Fetch(context.Background(), SourceConfig{
		ID:          "fixture",
		PlaylistURL: "http://127.0.0.1/private/list.m3u",
	})
	if !errors.Is(err, ErrUnsafeEndpoint) {
		t.Fatalf("erro = %v, want ErrUnsafeEndpoint", err)
	}
}

func TestHTTPM3USourceRejectsInlineCredentials(t *testing.T) {
	for _, endpoint := range []string{
		"https://user:secret@example.invalid/playlist.m3u",
		"https://example.invalid/playlist.m3u?username=user&password=secret",
	} {
		_, err := (HTTPM3USource{}).Fetch(context.Background(), SourceConfig{
			ID: "fixture", PlaylistURL: endpoint,
		})
		if !errors.Is(err, ErrInvalidSource) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("endpoint sensível %q: erro=%v", endpoint, err)
		}
	}
}

func TestHTTPM3USourceBoundsResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", 128)))
	}))
	defer server.Close()

	_, err := (HTTPM3USource{Client: server.Client(), MaxBodyBytes: 32}).Fetch(context.Background(), SourceConfig{
		ID:          "fixture",
		PlaylistURL: server.URL,
	})
	if !errors.Is(err, ErrPlaylistTooLarge) {
		t.Fatalf("erro = %v, want ErrPlaylistTooLarge", err)
	}
}

func TestHTTPM3USourceHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (HTTPM3USource{Client: server.Client()}).Fetch(ctx, SourceConfig{
		ID:          "fixture",
		PlaylistURL: server.URL,
	})
	if err == nil {
		t.Fatal("Fetch() retornou nil após cancelamento")
	}
}

func TestHTTPM3USourceRejectsCrossOriginRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("#EXTM3U\n"))
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer redirect.Close()

	_, err := (HTTPM3USource{Client: redirect.Client()}).Fetch(context.Background(), SourceConfig{
		ID:          "fixture",
		PlaylistURL: redirect.URL,
	})
	if !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("erro = %v, want cross-origin ErrInvalidSource", err)
	}
}

func TestHTTPM3USourceDoesNotExposeURLOnTransportError(t *testing.T) {
	client := &http.Client{Transport: roundTripError{}}
	_, err := (HTTPM3USource{Client: client}).Fetch(context.Background(), SourceConfig{
		ID:          "fixture",
		PlaylistURL: "https://user:secret@example.invalid/playlist.m3u",
	})
	if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "example.invalid") {
		t.Fatalf("erro expôs detalhes da URL: %v", err)
	}
}

type roundTripError struct{}

func (roundTripError) RoundTrip(request *http.Request) (*http.Response, error) {
	return nil, &url.Error{Op: http.MethodGet, URL: request.URL.String(), Err: errors.New("transport failed")}
}

type staticSecretStore map[domain.SecretKey][]byte

func (store staticSecretStore) Get(_ context.Context, key domain.SecretKey) ([]byte, error) {
	value, ok := store[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), value...), nil
}

func (staticSecretStore) Set(context.Context, domain.SecretKey, []byte) error {
	return nil
}

func (staticSecretStore) Delete(context.Context, domain.SecretKey) error {
	return nil
}

func TestFetchStreamFailsWhenBodyExceedsLimit(t *testing.T) {
	var big strings.Builder
	big.WriteString("#EXTM3U\n")
	for i := 0; i < 500; i++ {
		big.WriteString("#EXTINF:-1 group-title=\"TV\",Canal\nhttps://example.invalid/x.m3u8\n")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(big.String()))
	}))
	defer server.Close()

	source := HTTPM3USource{Client: server.Client(), MaxBodyBytes: 1024}
	var got int
	_, err := source.FetchStream(context.Background(),
		SourceConfig{ID: "s", PlaylistURL: server.URL},
		ParseOptions{SourceID: "s"}, func(batch []Item) error {
			got += len(batch)
			return nil
		})
	if !errors.Is(err, ErrPlaylistTooLarge) {
		t.Fatalf("err = %v, want ErrPlaylistTooLarge", err)
	}
}
