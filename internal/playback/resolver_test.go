package playback

import (
	"context"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestResolve(t *testing.T) {
	r := MpvYtdlHookResolver{}

	cases := []struct {
		name   string
		source string
		mode   domain.PlaybackMode
	}{
		{"youtube url", "https://www.youtube.com/watch?v=aqz-KE-bpKQ", domain.PlaybackModeMpvYtdlHook},
		{"http url", "http://example.com/v.mp4", domain.PlaybackModeMpvYtdlHook},
		{"https url", "https://example.com/v.mp4", domain.PlaybackModeMpvYtdlHook},
		{"local file", "/home/user/video.mp4", domain.PlaybackModeDirect},
		{"relative file", "video.mp4", domain.PlaybackModeDirect},
		{"empty", "", domain.PlaybackModeDirect},
		{"unsupported scheme", "ftp://example.com/x", domain.PlaybackModeDirect},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := r.Resolve(context.Background(), domain.PlaybackRequest{SourceURL: tc.source})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if plan.Mode != tc.mode {
				t.Errorf("mode = %q, want %q", plan.Mode, tc.mode)
			}
			if plan.LoadTarget != tc.source {
				t.Errorf("loadTarget = %q, want %q", plan.LoadTarget, tc.source)
			}
		})
	}
}

func TestResolveCancelledContext(t *testing.T) {
	r := MpvYtdlHookResolver{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Resolve(ctx, domain.PlaybackRequest{SourceURL: "https://example.com/v.mp4"}); err == nil {
		t.Fatal("esperava erro de contexto cancelado")
	}
}

// O plano precisa carregar a política de extração: sem isso o adapter do mpv
// não tem como configurar o yt-dlp (era o contrato morto de PlaybackPlan.Options).
func TestResolveRemoteCarriesYtdlOptions(t *testing.T) {
	resolver := MpvYtdlHookResolver{Ytdl: YtdlConfig{
		JSRuntime: JSRuntime{Name: "node", Path: "/usr/bin/node"},
	}}
	plan, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if plan.Mode != domain.PlaybackModeMpvYtdlHook {
		t.Fatalf("modo inesperado: %v", plan.Mode)
	}
	if got := plan.Options["ytdl-raw-options"]; got != "js-runtimes=node:/usr/bin/node" {
		t.Fatalf("opções do plano: %q", got)
	}
}

// Arquivo local não usa yt-dlp, mas precisa limpar a política do vídeo remoto
// anterior — senão ela vaza para a próxima reprodução.
func TestResolveLocalClearsYtdlOptions(t *testing.T) {
	resolver := MpvYtdlHookResolver{Ytdl: YtdlConfig{
		JSRuntime: JSRuntime{Name: "node", Path: "/usr/bin/node"},
	}}
	plan, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "/home/raillen/video.mp4",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if plan.Mode != domain.PlaybackModeDirect {
		t.Fatalf("modo inesperado: %v", plan.Mode)
	}
	if got, ok := plan.Options["ytdl-raw-options"]; !ok || got != "" {
		t.Fatalf("plano local deveria limpar as opções, veio %q (presente=%v)", got, ok)
	}
}

func TestResolveRejectsUnusableConfig(t *testing.T) {
	resolver := MpvYtdlHookResolver{Ytdl: YtdlConfig{PlayerClient: "tv,web"}}
	if _, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	}); err == nil {
		t.Fatal("config impossível de serializar deveria falhar no resolver, não no mpv")
	}
}
