package playback

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestMediaProxyWrapsPlanAndForwardsRange(t *testing.T) {
	var receivedRange string
	var receivedUserAgent string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRange = r.Header.Get("Range")
		receivedUserAgent = r.Header.Get("User-Agent")
		if receivedRange == "bytes=2-4" {
			w.Header().Set("Content-Range", "bytes 2-4/10")
			w.Header().Set("Content-Length", "3")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, "234")
			return
		}
		_, _ = io.WriteString(w, "0123456789")
	}))
	defer upstream.Close()

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	proxy := NewMediaProxy()
	proxy.client = upstream.Client()
	proxy.allowPrivateNetwork = true
	proxy.clock = func() time.Time { return now }

	plan, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:      domain.PlaybackModeResolvedMedia,
		Primary:   domain.ResolvedStream{URL: upstream.URL + "/video.mp4", Headers: map[string]string{"User-Agent": "NanoTube-Test", "Cookie": "should-not-forward"}},
		ExpiresAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("WrapPlan: %v", err)
	}
	if strings.Contains(plan.Primary.URL, upstream.URL) || !strings.HasPrefix(plan.Primary.URL, mediaProxyPathPrefix) {
		t.Fatalf("URL exposta fora do proxy: %q", plan.Primary.URL)
	}

	request := httptest.NewRequest(http.MethodGet, plan.Primary.URL, nil)
	request.Header.Set("Range", "bytes=2-4")
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusPartialContent || recorder.Body.String() != "234" {
		t.Fatalf("resposta do proxy = %d %q", recorder.Code, recorder.Body.String())
	}
	if receivedRange != "bytes=2-4" {
		t.Fatalf("Range não encaminhado: %q", receivedRange)
	}
	if receivedUserAgent != "NanoTube-Test" {
		t.Fatalf("User-Agent inesperado: %q", receivedUserAgent)
	}
}

func TestMediaProxyCanPublishTokensOnLoopbackHTTP(t *testing.T) {
	proxy := NewMediaProxy()
	if err := proxy.SetLocalBaseURL("http://127.0.0.1:45678"); err != nil {
		t.Fatalf("SetLocalBaseURL: %v", err)
	}
	plan, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:    domain.PlaybackModeResolvedMedia,
		Primary: domain.ResolvedStream{URL: "https://media.example/video.mp4"},
	})
	if err != nil {
		t.Fatalf("WrapPlan: %v", err)
	}
	if !strings.HasPrefix(plan.Primary.URL, "http://127.0.0.1:45678"+mediaProxyPathPrefix) {
		t.Fatalf("URL local inesperada: %q", plan.Primary.URL)
	}
}

func TestMediaProxyRejectsNonLoopbackPublicBaseURL(t *testing.T) {
	proxy := NewMediaProxy()
	for _, baseURL := range []string{
		"https://127.0.0.1:45678",
		"http://example.com:45678",
		"http://127.0.0.1:45678/extra",
		"http://127.0.0.1",
	} {
		if err := proxy.SetLocalBaseURL(baseURL); err == nil {
			t.Fatalf("base pública insegura aceita: %q", baseURL)
		}
	}
}

func TestMediaProxyRejectsUnsafeTarget(t *testing.T) {
	proxy := NewMediaProxy()
	_, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:    domain.PlaybackModeResolvedMedia,
		Primary: domain.ResolvedStream{URL: "http://127.0.0.1:8080/private.mp4"},
	})
	if err == nil || !strings.Contains(err.Error(), "não público") {
		t.Fatalf("destino privado aceito: %v", err)
	}
}

func TestMediaProxyExpiresSessions(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	proxy := NewMediaProxy()
	proxy.allowPrivateNetwork = true
	proxy.clock = func() time.Time { return now }
	plan, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:      domain.PlaybackModeResolvedMedia,
		Primary:   domain.ResolvedStream{URL: "http://127.0.0.1:8080/private.mp4"},
		ExpiresAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("WrapPlan: %v", err)
	}
	now = now.Add(time.Minute + time.Nanosecond)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, plan.Primary.URL, nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("sessão expirada respondeu %d", recorder.Code)
	}
}

func TestMediaProxyRevokeInvalidatesActiveSessions(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock"))
	}))
	defer upstream.Close()

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	proxy := NewMediaProxy()
	proxy.client = upstream.Client()
	proxy.allowPrivateNetwork = true
	proxy.clock = func() time.Time { return now }
	plan, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:      domain.PlaybackModeResolvedMedia,
		Primary:   domain.ResolvedStream{URL: upstream.URL + "/video.mp4"},
		ExpiresAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("WrapPlan: %v", err)
	}
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, plan.Primary.URL, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("sessão antes de revogar respondeu %d", recorder.Code)
	}
	proxy.Revoke()
	recorder = httptest.NewRecorder()
	proxy.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, plan.Primary.URL, nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("sessão revogada respondeu %d", recorder.Code)
	}
}

func TestMediaProxyPreservesHLSMarkerAndDirectPlan(t *testing.T) {
	proxy := NewMediaProxy()
	proxy.allowPrivateNetwork = true
	plan, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:     domain.PlaybackModeResolvedMedia,
		Primary:  domain.ResolvedStream{URL: "https://media.example/live/index.m3u8"},
		Variants: []domain.PlaybackVariant{{ID: "720", Stream: domain.ResolvedStream{URL: "https://media.example/live/720.mp4"}}},
	})
	if err != nil {
		t.Fatalf("WrapPlan: %v", err)
	}
	if !strings.HasSuffix(plan.Primary.URL, ".m3u8") {
		t.Fatalf("proxy perdeu marcador HLS: %q", plan.Primary.URL)
	}
	if !strings.HasPrefix(plan.Variants[0].Stream.URL, mediaProxyPathPrefix) {
		t.Fatalf("variante não protegida: %q", plan.Variants[0].Stream.URL)
	}
	direct := domain.PlaybackPlan{Mode: domain.PlaybackModeDirect, LoadTarget: "/tmp/video.mp4"}
	unchanged, err := proxy.WrapPlan(context.Background(), direct)
	if err != nil || unchanged.LoadTarget != direct.LoadTarget {
		t.Fatalf("plano direto alterado: %+v, err=%v", unchanged, err)
	}
}

func TestMediaProxyRejectsPrivateRedirect(t *testing.T) {
	proxy := NewMediaProxy()
	proxy.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"http://127.0.0.1/private.mp4"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})
	plan, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:    domain.PlaybackModeResolvedMedia,
		Primary: domain.ResolvedStream{URL: "https://media.example/video.mp4"},
	})
	if err != nil {
		t.Fatalf("WrapPlan: %v", err)
	}
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, plan.Primary.URL, nil))
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("redirecionamento privado respondeu %d", recorder.Code)
	}
}

func TestMediaProxyLimitsRedirects(t *testing.T) {
	proxy := NewMediaProxy()
	proxy.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		next := request.URL.Path + "-next"
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{next}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})
	plan, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:    domain.PlaybackModeResolvedMedia,
		Primary: domain.ResolvedStream{URL: "https://media.example/video.mp4"},
	})
	if err != nil {
		t.Fatalf("WrapPlan: %v", err)
	}
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, plan.Primary.URL, nil))
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("loop de redirect respondeu %d", recorder.Code)
	}
}

func TestMediaProxyRewritesHLSReferences(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/live/index.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = io.WriteString(w, "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\n#EXTINF:1,\nsegment.ts\n")
		case "/live/key.bin":
			_, _ = io.WriteString(w, "key")
		case "/live/segment.ts":
			_, _ = io.WriteString(w, "segment")
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	proxy := NewMediaProxy()
	proxy.client = upstream.Client()
	proxy.allowPrivateNetwork = true
	plan, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:    domain.PlaybackModeResolvedMedia,
		Primary: domain.ResolvedStream{URL: upstream.URL + "/live/index.m3u8"},
	})
	if err != nil {
		t.Fatalf("WrapPlan: %v", err)
	}
	now := time.Now().Add(6 * time.Minute)
	proxy.clock = func() time.Time { return now }
	t.Cleanup(proxy.Revoke)
	manifestResponse := httptest.NewRecorder()
	proxy.ServeHTTP(manifestResponse, httptest.NewRequest(http.MethodGet, plan.Primary.URL, nil))
	if manifestResponse.Code != http.StatusOK {
		t.Fatalf("manifesto respondeu %d: %s", manifestResponse.Code, manifestResponse.Body.String())
	}
	manifest := manifestResponse.Body.String()
	if strings.Contains(manifest, upstream.URL) || !strings.Contains(manifest, "URI=\""+mediaProxyPathPrefix) {
		t.Fatalf("manifesto expôs referência externa: %q", manifest)
	}
	var segmentPath string
	for _, line := range strings.Split(manifest, "\n") {
		if strings.HasPrefix(line, mediaProxyPathPrefix) {
			segmentPath = line
			break
		}
	}
	if segmentPath == "" {
		t.Fatalf("segmento não foi proxyado: %q", manifest)
	}
	segmentResponse := httptest.NewRecorder()
	proxy.ServeHTTP(segmentResponse, httptest.NewRequest(http.MethodGet, segmentPath, nil))
	if segmentResponse.Code != http.StatusOK || segmentResponse.Body.String() != "segment" {
		t.Fatalf("segmento proxyado respondeu %d %q", segmentResponse.Code, segmentResponse.Body.String())
	}
	segmentToken, _ := parseMediaToken(segmentPath)
	session, ok := proxy.session(segmentToken)
	if !ok || !session.expiresAt.Equal(plan.ExpiresAt) {
		t.Fatal("HLS child did not inherit plan expiry")
	}
	proxy.Revoke()
	if _, ok := proxy.session(segmentToken); ok {
		t.Fatal("HLS child survived revocation")
	}
}

func TestMediaProxyLateQualitySwitchAndLifetimeBounds(t *testing.T) {
	for _, test := range []struct {
		name    string
		planTTL time.Duration
		urlTTL  time.Duration
		wantTTL time.Duration
	}{
		{name: "upstream", planTTL: time.Hour, wantTTL: time.Hour},
		{name: "unknown", wantTTL: mediaProxyDefaultTTL},
		{name: "hard cap", planTTL: 24 * time.Hour, wantTTL: mediaProxyDefaultTTL},
		{name: "URL expiry", planTTL: time.Hour, urlTTL: 30 * time.Minute, wantTTL: 30 * time.Minute},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Now().Truncate(time.Second)
			start := now
			proxy := NewMediaProxy()
			t.Cleanup(proxy.Revoke)
			proxy.clock = func() time.Time { return now }
			proxy.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Range") != "bytes=2-4" {
					t.Error("Range not forwarded")
				}
				return &http.Response{StatusCode: http.StatusPartialContent, Header: http.Header{"Content-Range": {"bytes 2-4/10"}}, Body: io.NopCloser(strings.NewReader("234")), Request: r}, nil
			})
			streamURL := "https://media.example/video.mp4"
			if test.urlTTL != 0 {
				streamURL += "?expire=" + strconv.FormatInt(now.Add(test.urlTTL).Unix(), 10)
			}
			input := domain.PlaybackPlan{
				Mode:      domain.PlaybackModeResolvedMedia,
				Primary:   domain.ResolvedStream{URL: streamURL},
				Audio:     &domain.ResolvedStream{URL: streamURL},
				AudioOnly: &domain.ResolvedStream{URL: streamURL},
				Variants:  []domain.PlaybackVariant{{ID: "720", Stream: domain.ResolvedStream{URL: streamURL}}},
			}
			if test.planTTL != 0 {
				input.ExpiresAt = now.Add(test.planTTL)
			}
			ctx, cancel := context.WithCancel(context.Background())
			plan, err := proxy.WrapPlan(ctx, input)
			cancel()
			if err != nil {
				t.Fatal(err)
			}
			if input.Variants[0].Stream.URL != streamURL {
				t.Fatal("input plan mutated")
			}
			if !plan.ExpiresAt.Equal(start.Add(test.wantTTL)) {
				t.Fatal("incorrect plan deadline")
			}
			now = start.Add(6 * time.Minute)
			for _, path := range playbackPlanURLs(plan) {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				request.Header.Set("Range", "bytes=2-4")
				response := httptest.NewRecorder()
				proxy.ServeHTTP(response, request)
				if response.Code != http.StatusPartialContent || response.Body.String() != "234" || response.Header().Get("Content-Range") != "bytes 2-4/10" {
					t.Fatalf("late Range failed: %d", response.Code)
				}
			}
			now = start.Add(test.wantTTL)
			for _, path := range playbackPlanURLs(plan) {
				response := httptest.NewRecorder()
				proxy.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				if response.Code != http.StatusNotFound {
					t.Fatal("expired token accepted")
				}
			}
		})
	}
}

func TestMediaProxyReplacementRollbackAndCancellation(t *testing.T) {
	proxy := NewMediaProxy()
	t.Cleanup(proxy.Revoke)
	input := domain.PlaybackPlan{Mode: domain.PlaybackModeResolvedMedia, Primary: domain.ResolvedStream{URL: "https://media.example/video.mp4"}}
	first, err := proxy.WrapPlan(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := parseMediaToken(first.Primary.URL)
	invalid := clonePlaybackPlan(input)
	invalid.Audio = &domain.ResolvedStream{URL: "http://127.0.0.1/private"}
	if _, err := proxy.WrapPlan(context.Background(), invalid); err == nil {
		t.Fatal("invalid plan accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := proxy.WrapPlan(ctx, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, ok := proxy.session(token); !ok {
		t.Fatal("failed replacement revoked active plan")
	}
	started := make(chan struct{})
	done := make(chan struct{})
	proxy.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	go func() {
		defer close(done)
		proxy.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, first.Primary.URL, nil))
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	second, err := proxy.WrapPlan(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("replacement did not cancel upstream")
	}
	if _, ok := proxy.session(token); ok {
		t.Fatal("old token remains valid")
	}
	secondToken, _ := parseMediaToken(second.Primary.URL)
	proxy.Revoke()
	proxy.Revoke()
	if _, ok := proxy.session(secondToken); ok {
		t.Fatal("revoked token remains valid")
	}
	if len(proxy.sessions) != 0 {
		t.Fatal("revoked tokens retained")
	}
}

func TestMediaProxyRejectsOversizedPlanAtomically(t *testing.T) {
	proxy := NewMediaProxy()
	t.Cleanup(proxy.Revoke)
	proxy.maxSessions = 1
	_, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:    domain.PlaybackModeResolvedMedia,
		Primary: domain.ResolvedStream{URL: "https://media.example/video.mp4"},
		Audio:   &domain.ResolvedStream{URL: "https://media.example/audio.mp4"},
	})
	if err == nil || len(proxy.sessions) != 0 {
		t.Fatal("oversized plan partially published")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
