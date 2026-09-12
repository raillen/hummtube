package playback

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestInvidiousResolverBuildsCombinedPlan(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/videos/test-id") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"title": "Vídeo Teste Invidious",
			"videoId": "test-id",
			"lengthSeconds": 300,
			"formatStreams": [
				{
					"url": "https://stream.example.com/720p.mp4",
					"resolution": "1280x720",
					"qualityLabel": "720p"
				},
				{
					"url": "https://stream.example.com/360p.mp4",
					"resolution": "640x360",
					"qualityLabel": "360p"
				}
			],
			"adaptiveFormats": [
				{"url": "https://stream.example.com/audio.m4a", "type": "audio/mp4; codecs=\"mp4a.40.2\""}
			]
		}`))
	}))
	defer mockServer.Close()

	resolver := NewInvidiousResolver(mockServer.URL, 720)
	resolver.AllowLoopback = true
	// Substitui o cliente padrão pelo cliente do httptest para permitir URLs locais de teste
	resolver.Client = mockServer.Client()

	plan, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{
		VideoID:   "test-id",
		SourceURL: "https://www.youtube.com/watch?v=test-id",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if plan.Mode != domain.PlaybackModeResolvedMedia {
		t.Errorf("Mode = %v, want %v", plan.Mode, domain.PlaybackModeResolvedMedia)
	}
	if plan.Primary.URL != "https://stream.example.com/720p.mp4" {
		t.Errorf("Primary URL = %q, want 720p", plan.Primary.URL)
	}
	if plan.Metadata.Title != "Vídeo Teste Invidious" {
		t.Errorf("Title = %q", plan.Metadata.Title)
	}
	if plan.Metadata.Duration != 300*time.Second {
		t.Errorf("Duration = %v", plan.Metadata.Duration)
	}
	if len(plan.Variants) != 2 || plan.Variants[0].Label != "720p" || plan.Variants[1].Label != "360p" {
		t.Errorf("Variants = %+v", plan.Variants)
	}
	if plan.AudioOnly == nil || plan.AudioOnly.URL != "https://stream.example.com/audio.m4a" {
		t.Errorf("AudioOnly = %+v", plan.AudioOnly)
	}
}

func TestInvidiousResolverAdaptiveFormatsFallback(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"title": "Vídeo Adaptativo",
			"videoId": "adapt-id",
			"lengthSeconds": 120,
			"adaptiveFormats": [
				{
					"url": "https://stream.example.com/video-1080.webm",
					"type": "video/webm; codecs=\"vp9\"",
					"resolution": "1920x1080"
				},
				{
					"url": "https://stream.example.com/audio-opus.webm",
					"type": "audio/webm; codecs=\"opus\""
				}
			]
		}`))
	}))
	defer mockServer.Close()

	resolver := NewInvidiousResolver(mockServer.URL, 0)
	resolver.AllowLoopback = true
	resolver.Client = mockServer.Client()

	plan, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "https://youtu.be/adapt-id",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if plan.Primary.URL != "https://stream.example.com/video-1080.webm" {
		t.Errorf("Primary.URL = %q", plan.Primary.URL)
	}
	if plan.Audio == nil || plan.Audio.URL != "https://stream.example.com/audio-opus.webm" {
		t.Errorf("Audio = %+v", plan.Audio)
	}
	if len(plan.Variants) != 1 || plan.Variants[0].Height != 1080 || plan.Variants[0].HasAudio {
		t.Errorf("adaptive variants = %+v", plan.Variants)
	}
}

func TestInvidiousResolverPrefersHigherAdaptiveResolution(t *testing.T) {
	resolver := NewInvidiousResolver("https://example.invalid", 0)
	plan, err := resolver.buildPlan(InvidiousVideoJSON{
		VideoID: "mixed", Title: "Mixed",
		FormatStreams: []InvidiousFormat{
			{URL: "https://stream.invalid/360.mp4", ITag: "18", Resolution: "640x360", QualityLabel: "360p"},
		},
		AdaptiveFormats: []InvidiousFormat{
			{URL: "https://stream.invalid/1080.webm", ITag: "248", Type: "video/webm", Resolution: "1920x1080", QualityLabel: "1080p"},
			{URL: "https://stream.invalid/audio.webm", ITag: "251", Type: "audio/webm"},
		},
	})
	if err != nil {
		t.Fatalf("buildPlan: %v", err)
	}
	if plan.Primary.URL != "https://stream.invalid/1080.webm" || plan.Audio == nil {
		t.Fatalf("plano adaptativo = %+v", plan)
	}
	if len(plan.Variants) != 2 || plan.Variants[0].Height != 1080 || plan.Variants[0].HasAudio || !plan.Variants[1].HasAudio {
		t.Fatalf("variantes mistas = %+v", plan.Variants)
	}
}

func TestExtractVideoID(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/shorts/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
	}
	for _, c := range cases {
		got := extractVideoID(c.url)
		if got != c.want {
			t.Errorf("extractVideoID(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}
