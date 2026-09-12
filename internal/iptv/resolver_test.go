package iptv

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestM3UStreamResolverRefetchesCurrentStream(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		_, _ = fmt.Fprintf(writer, "#EXTM3U\n#EXTINF:-1 tvg-id=\"fixture.channel\" group-title=\"TV\",Canal fixture\nhttps://stream.invalid/live/%d.m3u8\n", requestCount)
	}))
	defer server.Close()

	source := SourceConfig{ID: "fixture", Name: "Fixture", PlaylistURL: server.URL}
	httpSource := HTTPM3USource{Client: server.Client()}
	initial, err := httpSource.Fetch(context.Background(), source)
	if err != nil {
		t.Fatalf("fetch inicial: %v", err)
	}
	resolved, err := (M3UStreamResolver{Source: httpSource}).Resolve(context.Background(), source, initial.Items[0])
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved != "https://stream.invalid/live/2.m3u8" || requestCount != 2 {
		t.Fatalf("resolved=%q requests=%d", resolved, requestCount)
	}
}

func TestM3UStreamResolverUsesUniqueMetadataFallback(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		_, _ = fmt.Fprintf(writer, "#EXTM3U\n#EXTINF:-1 group-title=\"Filmes\",Filme fixture\nhttps://stream.invalid/movie/%d.mp4\n", requestCount)
	}))
	defer server.Close()

	source := SourceConfig{ID: "fixture", Name: "Fixture", PlaylistURL: server.URL}
	httpSource := HTTPM3USource{Client: server.Client()}
	initial, err := httpSource.Fetch(context.Background(), source)
	if err != nil {
		t.Fatalf("fetch inicial: %v", err)
	}
	resolved, err := (M3UStreamResolver{Source: httpSource}).Resolve(context.Background(), source, initial.Items[0])
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved != "https://stream.invalid/movie/2.mp4" {
		t.Fatalf("resolved=%q, want rotating fallback URL", resolved)
	}
}

func TestM3UStreamResolverRejectsUnsafeStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("#EXTM3U\n#EXTINF:-1 group-title=\"TV\",Canal fixture\nfile:///tmp/secret.m3u8\n"))
	}))
	defer server.Close()

	source := SourceConfig{ID: "fixture", PlaylistURL: server.URL}
	httpSource := HTTPM3USource{Client: server.Client()}
	playlist, err := httpSource.Fetch(context.Background(), source)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	_, err = (M3UStreamResolver{Source: httpSource}).Resolve(context.Background(), source, playlist.Items[0])
	if !errors.Is(err, ErrInvalidStream) {
		t.Fatalf("erro = %v, want ErrInvalidStream", err)
	}
}

func TestResolveFirstStreamsAndEarlyExitsOnExactIDMatch(t *testing.T) {
	const playlistContent = `#EXTM3U
#EXTINF:-1 group-title="TV",Canal A
https://a.example.invalid/live.m3u8
#EXTINF:-1 group-title="TV",Canal B
https://b.example.invalid/live.m3u8
#EXTINF:-1 group-title="TV",Canal C
https://c.example.invalid/live.m3u8
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(playlistContent))
	}))
	defer server.Close()

	source := SourceConfig{ID: "fixture", Name: "Fixture", PlaylistURL: server.URL}
	resolver := M3UStreamResolver{Source: HTTPM3USource{Client: server.Client(), MaxBodyBytes: 1 << 20}}

	calls := 0
	url, err := resolver.ResolveFirst(context.Background(), source, ParseOptions{}, func(item Item) bool {
		calls++
		return item.Title == "Canal B"
	})
	if err != nil {
		t.Fatalf("ResolveFirst() error = %v", err)
	}
	if url != "https://b.example.invalid/live.m3u8" {
		t.Fatalf("url = %q", url)
	}
}

func TestResolveFirstReturnsNotFoundWhenNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:-1 group-title=\"TV\",A\nhttps://a.invalid/live.m3u8\n"))
	}))
	defer server.Close()

	source := SourceConfig{ID: "f", Name: "F", PlaylistURL: server.URL}
	resolver := M3UStreamResolver{Source: HTTPM3USource{Client: server.Client(), MaxBodyBytes: 1 << 20}}

	_, err := resolver.ResolveFirst(context.Background(), source, ParseOptions{}, func(Item) bool {
		return false
	})
	if !errors.Is(err, ErrStreamNotFound) {
		t.Fatalf("err = %v, want ErrStreamNotFound", err)
	}
}

func TestResolveFirstRequiresNilContext(t *testing.T) {
	resolver := M3UStreamResolver{Source: HTTPM3USource{}}
	_, err := resolver.ResolveFirst(nil, SourceConfig{}, ParseOptions{}, func(Item) bool { return true }) //lint:ignore SA1012 intentional nil-context defense test
	if err == nil {
		t.Fatalf("nil context should fail")
	}
}

func TestResolveFirstRejectsNilPredicate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\n"))
	}))
	defer server.Close()
	resolver := M3UStreamResolver{Source: HTTPM3USource{Client: server.Client(), MaxBodyBytes: 1 << 20}}
	_, err := resolver.ResolveFirst(context.Background(), SourceConfig{PlaylistURL: server.URL}, ParseOptions{}, nil)
	if err == nil {
		t.Fatalf("nil predicate should fail")
	}
}
