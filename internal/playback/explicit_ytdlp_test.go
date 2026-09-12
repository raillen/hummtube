package playback

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestExplicitYtDlpResolverLocalFile(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{})
	plan, err := r.Resolve(context.Background(), domain.PlaybackRequest{SourceURL: "/home/user/video.mp4"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if plan.Mode != domain.PlaybackModeDirect {
		t.Errorf("mode = %q, want direct", plan.Mode)
	}
	if plan.LoadTarget != "/home/user/video.mp4" {
		t.Errorf("loadTarget = %q", plan.LoadTarget)
	}
}

func TestExplicitYtDlpResolverCancelledContext(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Resolve(ctx, domain.PlaybackRequest{SourceURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ"})
	if err == nil {
		t.Fatal("esperava erro de contexto cancelado")
	}
}

func TestBuildPlanFromInfoSingleStream(t *testing.T) {
	info := YtDlpVideoJSON{
		ID:          "xyz123",
		Title:       "Test Video",
		Duration:    120.5,
		URL:         "https://googlevideo.com/videoplayback?id=xyz123",
		HTTPHeaders: map[string]string{"User-Agent": "TestUA"},
	}

	plan, err := buildPlanFromInfo(info)
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if plan.Mode != domain.PlaybackModeResolvedMedia {
		t.Errorf("mode = %v, want PlaybackModeResolvedMedia", plan.Mode)
	}
	if plan.Primary.URL != info.URL {
		t.Errorf("primary URL = %q, want %q", plan.Primary.URL, info.URL)
	}
	if plan.Primary.Headers["User-Agent"] != "TestUA" {
		t.Errorf("headers = %v", plan.Primary.Headers)
	}
	if plan.Metadata.Title != "Test Video" {
		t.Errorf("title = %q", plan.Metadata.Title)
	}
	if plan.Metadata.Duration != 120500*time.Millisecond {
		t.Errorf("duration = %v", plan.Metadata.Duration)
	}
	if plan.Audio != nil {
		t.Errorf("audio não deveria estar presente em single stream")
	}
}

func TestBuildPlanSanitizesHeadersBeforeRPC(t *testing.T) {
	plan, err := buildPlanFromInfo(YtDlpVideoJSON{
		URL: "https://googlevideo.invalid/media",
		HTTPHeaders: map[string]string{
			"User-Agent":          "NanoTube",
			"Referer":             "https://www.youtube.com/",
			"Cookie":              "SID=secret",
			"Authorization":       "Bearer secret",
			"Proxy-Authorization": "Basic secret",
			"Set-Cookie":          "SID=secret",
			"X-Injected":          "value\r\nCookie: stolen",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Primary.Headers["User-Agent"] != "NanoTube" || plan.Primary.Headers["Referer"] == "" {
		t.Fatalf("headers necessários foram removidos: %+v", plan.Primary.Headers)
	}
	for _, forbidden := range []string{"Cookie", "Authorization", "Proxy-Authorization", "Set-Cookie", "X-Injected"} {
		if _, ok := plan.Primary.Headers[forbidden]; ok {
			t.Fatalf("header sensível %q atravessou o plano", forbidden)
		}
	}
}

func TestBuildPlanFromInfoCarriesExpiration(t *testing.T) {
	expires := time.Now().Add(10 * time.Minute).Unix()
	plan, err := buildPlanFromInfo(YtDlpVideoJSON{
		Expires: expires,
		URL:     "https://googlevideo.com/videoplayback?id=expiring",
	})
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if !plan.ExpiresAt.Equal(time.Unix(expires, 0)) {
		t.Fatalf("expiresAt = %v, esperado %v", plan.ExpiresAt, time.Unix(expires, 0))
	}
}

func TestBuildPlanDerivesEarliestExpirationFromMediaURLs(t *testing.T) {
	expires := time.Now().Add(20 * time.Minute).Unix()
	plan, err := buildPlanFromInfo(YtDlpVideoJSON{
		Expires: expires + 600,
		URL:     "https://googlevideo.invalid/media?expire=" + fmt.Sprint(expires),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.ExpiresAt.Equal(time.Unix(expires, 0)) {
		t.Fatalf("expiração = %v, esperada %v", plan.ExpiresAt, time.Unix(expires, 0))
	}
}

func TestBuildPlanFromInfoAdaptiveStreamsWithSharedHeaders(t *testing.T) {
	sharedHeaders := map[string]string{"User-Agent": "NanoTube"}
	info := YtDlpVideoJSON{
		ID:       "adaptive1",
		Title:    "Adaptive Video",
		Duration: 300,
		RequestedFormats: []YtDlpFormatJSON{
			{
				FormatID:    "398",
				URL:         "https://googlevideo.com/video_720p",
				VCodec:      "av01.0.05M.08",
				ACodec:      "none",
				Height:      720,
				Width:       1280,
				HTTPHeaders: sharedHeaders,
			},
			{
				FormatID:    "251",
				URL:         "https://googlevideo.com/audio_opus",
				VCodec:      "none",
				ACodec:      "opus",
				HTTPHeaders: sharedHeaders,
			},
		},
	}

	plan, err := buildPlanFromInfo(info)
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if plan.Mode != domain.PlaybackModeResolvedMedia {
		t.Errorf("mode = %v", plan.Mode)
	}
	if plan.Primary.URL != "https://googlevideo.com/video_720p" {
		t.Errorf("primary video URL = %q", plan.Primary.URL)
	}
	if plan.Audio == nil || plan.Audio.URL != "https://googlevideo.com/audio_opus" {
		t.Errorf("audio stream = %+v", plan.Audio)
	}
	if plan.Primary.Headers["User-Agent"] != "NanoTube" {
		t.Errorf("video headers = %v", plan.Primary.Headers)
	}
	if plan.Audio.Headers["User-Agent"] != "NanoTube" {
		t.Errorf("audio headers = %v", plan.Audio.Headers)
	}
}

func TestBuildPlanFromInfoAdaptiveStreamsFallsBackToMuxedWhenHeadersDiffer(t *testing.T) {
	info := YtDlpVideoJSON{
		RequestedFormats: []YtDlpFormatJSON{
			{URL: "https://googlevideo.com/video_720p", VCodec: "h264", ACodec: "none", Height: 720, HTTPHeaders: map[string]string{"User-Agent": "VideoUA"}},
			{URL: "https://googlevideo.com/audio_opus", VCodec: "none", ACodec: "opus", HTTPHeaders: map[string]string{"User-Agent": "AudioUA"}},
		},
		Formats: []YtDlpFormatJSON{
			{URL: "https://googlevideo.com/muxed_360p", VCodec: "h264", ACodec: "aac", Height: 360, HTTPHeaders: map[string]string{"User-Agent": "MuxedUA"}},
		},
	}
	plan, err := buildPlanFromInfo(info)
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if plan.Primary.URL != "https://googlevideo.com/muxed_360p" || plan.Audio != nil {
		t.Fatalf("plano adaptativo inseguro não foi substituído: %+v", plan)
	}
	if plan.Primary.Headers["User-Agent"] != "MuxedUA" {
		t.Fatalf("headers do formato muxado = %v", plan.Primary.Headers)
	}
}

func TestBuildPlanFromInfoAdaptiveStreamsRejectsDifferentHeadersWithoutMuxedFormat(t *testing.T) {
	info := YtDlpVideoJSON{
		RequestedFormats: []YtDlpFormatJSON{
			{URL: "https://googlevideo.com/video_720p", VCodec: "h264", ACodec: "none", Height: 720, HTTPHeaders: map[string]string{"User-Agent": "VideoUA"}},
			{URL: "https://googlevideo.com/audio_opus", VCodec: "none", ACodec: "opus", HTTPHeaders: map[string]string{"User-Agent": "AudioUA"}},
		},
	}
	if _, err := buildPlanFromInfo(info); err == nil || !strings.Contains(err.Error(), "cabeçalhos HTTP incompatíveis") {
		t.Fatalf("esperava rejeição de faixas incompatíveis, erro = %v", err)
	}
}

func TestBuildPlanFromInfoAdaptiveStreamsFallsBackForCrossOriginHeader(t *testing.T) {
	info := YtDlpVideoJSON{
		RequestedFormats: []YtDlpFormatJSON{
			{URL: "https://video.example.invalid/video", VCodec: "h264", ACodec: "none", Height: 720, HTTPHeaders: map[string]string{"User-Agent": "NanoTube", "Referer": "https://youtube.example.invalid/"}},
			{URL: "https://audio.example.invalid/audio", VCodec: "none", ACodec: "opus", HTTPHeaders: map[string]string{"User-Agent": "NanoTube", "Referer": "https://youtube.example.invalid/"}},
		},
		Formats: []YtDlpFormatJSON{
			{URL: "https://video.example.invalid/muxed", VCodec: "h264", ACodec: "aac", Height: 360, HTTPHeaders: map[string]string{"User-Agent": "NanoTube"}},
		},
	}
	plan, err := buildPlanFromInfo(info)
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if plan.Primary.URL != "https://video.example.invalid/muxed" || plan.Audio != nil {
		t.Fatalf("plano cross-origin inseguro não foi substituído: %+v", plan)
	}
}

func TestBuildPlanFromInfoNoPlayableURL(t *testing.T) {
	info := YtDlpVideoJSON{
		ID:    "empty",
		Title: "Empty Video",
	}
	_, err := buildPlanFromInfo(info)
	if err == nil {
		t.Fatal("esperava erro quando nenhuma URL de mídia é encontrada")
	}
	if !strings.Contains(err.Error(), "nenhuma url") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}
}

func TestExplicitResolverAutomaticClientFallback(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{
		YtDlpPath:    testExecutable(t),
		PlayerClient: AutoPlayerClient,
		POTProvider:  "bgutil-test",
	})
	r.potProviderAvailable = func(YtdlConfig) bool { return true }
	var clients []string
	r.commandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		client := extractorClientArg(args)
		clients = append(clients, client)
		if client == "android" {
			return exec.CommandContext(ctx, "printf", "%s", `{"id":"no-formats"}`)
		}
		payload := `{"id":"fallback","title":"Fallback","url":"https://media.invalid/video.mp4","vcodec":"h264","acodec":"aac"}`
		return exec.CommandContext(ctx, "printf", "%s", payload)
	}

	plan, err := r.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "https://www.youtube.com/watch?v=fallback",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if plan.Primary.URL != "https://media.invalid/video.mp4" {
		t.Fatalf("fallback não produziu plano: %+v", plan)
	}
	want := []string{"android", "mweb"}
	if len(clients) != len(want) || clients[0] != want[0] || clients[1] != want[1] {
		t.Fatalf("ordem de clientes = %v, esperado %v", clients, want)
	}
}

func TestAutomaticResolverDoesNotAttemptMwebWithoutFunctionalPOT(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{
		YtDlpPath:    testExecutable(t),
		PlayerClient: AutoPlayerClient,
	})
	r.potProviderAvailable = func(YtdlConfig) bool { return false }
	var clients []string
	r.commandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		clients = append(clients, extractorClientArg(args))
		return exec.CommandContext(ctx, "printf", "%s", `{"id":"no-formats"}`)
	}

	_, err := r.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "https://www.youtube.com/watch?v=no-mweb",
	})
	if err == nil {
		t.Fatal("esperava falha")
	}
	if strings.Join(clients, ",") != "android" {
		t.Fatalf("clientes tentados = %v, mweb não deveria ser tentado sem POT", clients)
	}
}

func TestAutomaticResolverReturnsLastClassifiedFailure(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{
		YtDlpPath:    testExecutable(t),
		PlayerClient: AutoPlayerClient,
		POTProvider:  "bgutil-test",
	})
	r.potProviderAvailable = func(YtdlConfig) bool { return true }
	r.commandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		if extractorClientArg(args) == "android" {
			return exec.CommandContext(ctx, "sh", "-c", "printf '%s' 'Requested format is not available' >&2; exit 1")
		}
		return exec.CommandContext(ctx, "sh", "-c", "printf '%s' 'mweb formats require a GVS PO Token which was not provided' >&2; exit 1")
	}

	_, err := r.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "https://www.youtube.com/watch?v=terminal-failure",
	})
	var failure Failure
	if !errors.As(err, &failure) {
		t.Fatalf("erro não estruturado: %v", err)
	}
	if failure.Kind != FailurePOTProviderMissing {
		t.Fatalf("falha final = %v, esperado POT ausente", failure.Kind)
	}
}

func TestExplicitResolverClassifiesExtractorBoundaries(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		originalTimeout := extractorAttemptTimeout
		extractorAttemptTimeout = 20 * time.Millisecond
		t.Cleanup(func() { extractorAttemptTimeout = originalTimeout })

		r := NewExplicitYtDlpResolver(YtdlConfig{
			YtDlpPath:    testExecutable(t),
			PlayerClient: "android",
		})
		r.commandContext = helperCommand("hang")
		_, err := r.Resolve(context.Background(), domain.PlaybackRequest{SourceURL: "https://www.youtube.com/watch?v=timeout"})
		var failure Failure
		if !errors.As(err, &failure) || failure.Kind != FailureExtractorTimeout {
			t.Fatalf("falha = %v, esperado timeout estruturado", err)
		}
	})

	t.Run("overflow", func(t *testing.T) {
		originalLimit := extractorStdoutLimit
		extractorStdoutLimit = 1024
		t.Cleanup(func() { extractorStdoutLimit = originalLimit })

		r := NewExplicitYtDlpResolver(YtdlConfig{
			YtDlpPath:    testExecutable(t),
			PlayerClient: "android",
		})
		r.commandContext = helperCommand("overflow")
		_, err := r.Resolve(context.Background(), domain.PlaybackRequest{SourceURL: "https://www.youtube.com/watch?v=overflow"})
		var failure Failure
		if !errors.As(err, &failure) || failure.Kind != FailureExtractorOutputLimit {
			t.Fatalf("falha = %v, esperado overflow estruturado", err)
		}
	})
}

func TestAutomaticResolverDoesNotRetryRateLimit(t *testing.T) {
	if canTryNextClient(ClassifyFailure("HTTP Error 429: Too Many Requests", YtdlConfig{})) {
		t.Fatal("429 não pode disparar outra chamada ao YouTube")
	}
	if !canTryNextClient(ClassifyFailure("Requested format is not available", YtdlConfig{})) {
		t.Fatal("ausência de formatos deveria permitir um cliente compatível")
	}
}

func TestAutomaticResolverTerminalHintDoesNotSuggestAutomaticAgain(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{YtDlpPath: testExecutable(t), PlayerClient: AutoPlayerClient})
	r.commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "printf", "%s", `{"id":"no-formats"}`)
	}
	_, err := r.Resolve(context.Background(), domain.PlaybackRequest{SourceURL: "https://www.youtube.com/watch?v=none"})
	if err == nil {
		t.Fatal("esperava falha após os clientes automáticos")
	}
	if strings.Contains(err.Error(), "Troque o cliente de extração para Automático") {
		t.Fatalf("dica terminal incorreta: %v", err)
	}
}

func TestExplicitResolverWrapsInvalidJSONAsStructuredFailure(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{YtDlpPath: testExecutable(t), PlayerClient: "android"})
	r.commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "printf", "%s", `{invalid`)
	}
	_, err := r.Resolve(context.Background(), domain.PlaybackRequest{SourceURL: "https://www.youtube.com/watch?v=invalid"})
	var failure Failure
	if !errors.As(err, &failure) {
		t.Fatalf("erro não estruturado: %v", err)
	}
}

func TestExplicitResolverRejectsManualMwebWithoutFunctionalPOT(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{
		YtDlpPath:    testExecutable(t),
		PlayerClient: "mweb",
	})
	r.potProviderAvailable = func(YtdlConfig) bool { return false }
	var calls int
	r.commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		calls++
		return exec.CommandContext(ctx, "true")
	}

	_, err := r.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "https://www.youtube.com/watch?v=manual-mweb",
	})
	var failure Failure
	if !errors.As(err, &failure) || failure.Kind != FailurePOTProviderMissing {
		t.Fatalf("falha = %v, esperada ausência de provider POT", err)
	}
	if calls != 0 {
		t.Fatalf("yt-dlp foi executado %d vezes sem provider funcional", calls)
	}
}

func TestExplicitResolverAllowsManualMwebWithFunctionalPOT(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{
		YtDlpPath:    testExecutable(t),
		PlayerClient: "mweb",
		POTProvider:  "bgutil-test",
	})
	r.potProviderAvailable = func(YtdlConfig) bool { return true }
	r.commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "printf", "%s", `{"id":"manual-mweb","url":"https://media.invalid/video.mp4"}`)
	}

	plan, err := r.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "https://www.youtube.com/watch?v=manual-mweb",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if plan.Primary.URL != "https://media.invalid/video.mp4" {
		t.Fatalf("plano = %+v", plan)
	}
}

func TestExplicitResolverManualClientDoesNotRotate(t *testing.T) {
	r := NewExplicitYtDlpResolver(YtdlConfig{
		YtDlpPath:    testExecutable(t),
		PlayerClient: "android",
	})
	var calls int
	r.commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		calls++
		return exec.CommandContext(ctx, "false")
	}

	_, err := r.Resolve(context.Background(), domain.PlaybackRequest{
		SourceURL: "https://www.youtube.com/watch?v=manual",
	})
	if err == nil {
		t.Fatal("falha do cliente manual deveria ser devolvida")
	}
	if calls != 1 {
		t.Fatalf("cliente manual executou %d tentativas, esperado 1", calls)
	}
}

func TestBuildPlanFallsBackToFormatsList(t *testing.T) {
	info := YtDlpVideoJSON{Formats: []YtDlpFormatJSON{
		{URL: "https://media.invalid/muxed-360.mp4", VCodec: "h264", ACodec: "aac", Height: 360},
		{URL: "https://media.invalid/video-720.mp4", VCodec: "h264", ACodec: "none", Height: 720},
		{URL: "https://media.invalid/audio.m4a", VCodec: "none", ACodec: "aac"},
	}}
	plan, err := buildPlanFromInfo(info)
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if plan.Primary.URL != "https://media.invalid/video-720.mp4" {
		t.Fatalf("vídeo selecionado = %q", plan.Primary.URL)
	}
	if plan.Audio == nil || plan.Audio.URL != "https://media.invalid/audio.m4a" {
		t.Fatalf("áudio selecionado = %+v", plan.Audio)
	}
}

func TestBuildPlanFallsBackWhenRequestedFormatsAreNotPlayable(t *testing.T) {
	info := YtDlpVideoJSON{
		RequestedFormats: []YtDlpFormatJSON{{FormatID: "storyboard", VCodec: "none", ACodec: "none"}},
		Formats:          []YtDlpFormatJSON{{URL: "https://media.invalid/video.mp4", VCodec: "h264", ACodec: "aac", Height: 360}},
	}
	plan, err := buildPlanFromInfo(info)
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if plan.Primary.URL != "https://media.invalid/video.mp4" {
		t.Fatalf("fallback selecionou %q", plan.Primary.URL)
	}
}

func TestSelectPlayableFormatsHonorsMaxHeight(t *testing.T) {
	info := YtDlpVideoJSON{Formats: []YtDlpFormatJSON{
		{URL: "https://media.invalid/1080.mp4", VCodec: "h264", ACodec: "aac", Height: 1080},
		{URL: "https://media.invalid/360.mp4", VCodec: "h264", ACodec: "aac", Height: 360},
	}}
	plan, err := buildPlanFromInfoWithMaxHeight(info, 480)
	if err != nil {
		t.Fatalf("buildPlanFromInfoWithMaxHeight: %v", err)
	}
	if plan.Primary.URL != "https://media.invalid/360.mp4" {
		t.Fatalf("teto ignorado, selecionado %q", plan.Primary.URL)
	}
}

func TestBuildPlanExposesSortedWebQualityVariants(t *testing.T) {
	info := YtDlpVideoJSON{Formats: []YtDlpFormatJSON{
		{FormatID: "18", URL: "https://media.invalid/360.mp4", VCodec: "h264", ACodec: "aac", Height: 360},
		{FormatID: "22", URL: "https://media.invalid/720.mp4", VCodec: "h264", ACodec: "aac", Height: 720},
		{FormatID: "video-only", URL: "https://media.invalid/1080.mp4", VCodec: "h264", ACodec: "none", Height: 1080},
	}}

	plan, err := buildPlanFromInfo(info)
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if len(plan.Variants) != 3 {
		t.Fatalf("variantes = %+v", plan.Variants)
	}
	if plan.Variants[0].Label != "1080p" || plan.Variants[1].Label != "720p" || plan.Variants[2].Label != "360p" {
		t.Fatalf("ordem das variantes = %+v", plan.Variants)
	}
	if plan.Variants[0].HasAudio || !plan.Variants[1].HasAudio {
		t.Fatalf("classificação de áudio das variantes = %+v", plan.Variants)
	}
}

func TestBuildPlanIncludesRequestedFormatWhenFormatsListIsSparse(t *testing.T) {
	plan, err := buildPlanFromInfo(YtDlpVideoJSON{
		RequestedFormats: []YtDlpFormatJSON{
			{FormatID: "137", URL: "https://media.invalid/1080.mp4", Ext: "mp4", Protocol: "https", VCodec: "avc1", ACodec: "none", Height: 1080},
			{FormatID: "140", URL: "https://media.invalid/audio.m4a", Ext: "m4a", Protocol: "https", VCodec: "none", ACodec: "mp4a.40.2"},
		},
		Formats: []YtDlpFormatJSON{
			{FormatID: "18", URL: "https://media.invalid/360.mp4", Ext: "mp4", Protocol: "https", VCodec: "avc1", ACodec: "mp4a.40.2", Height: 360},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Variants) != 2 || plan.Variants[0].Height != 1080 || plan.Variants[1].Height != 360 {
		t.Fatalf("variantes incompletas: %+v", plan.Variants)
	}
}

func TestBuildWebVariantsPrefersMuxedStreamAtSameHeight(t *testing.T) {
	variants := buildWebVariants([]YtDlpFormatJSON{
		{FormatID: "video-only", URL: "https://media.invalid/video-720.mp4", Ext: "mp4", Protocol: "https", VCodec: "h264", ACodec: "none", Height: 720},
		{FormatID: "muxed", URL: "https://media.invalid/muxed-720.mp4", Ext: "mp4", Protocol: "https", VCodec: "h264", ACodec: "aac", Height: 720},
	}, 0)
	if len(variants) != 1 || variants[0].ID != "muxed" || !variants[0].HasAudio {
		t.Fatalf("variante preferida = %+v", variants)
	}
}

func TestBuildPlanExposesBestCompatibleAudioOnlyStream(t *testing.T) {
	info := YtDlpVideoJSON{Formats: []YtDlpFormatJSON{
		{URL: "https://media.invalid/video.mp4", Ext: "mp4", Protocol: "https", VCodec: "h264", ACodec: "aac", Height: 360},
		{URL: "https://media.invalid/audio-low.m4a", Ext: "m4a", Protocol: "https", VCodec: "none", ACodec: "mp4a.40.2", ABR: 64},
		{URL: "https://media.invalid/audio-high.webm", Ext: "webm", Protocol: "https", VCodec: "none", ACodec: "opus", ABR: 128},
		{URL: "https://media.invalid/audio-dash.m4a", Ext: "m4a", Protocol: "http_dash_segments", VCodec: "none", ACodec: "mp4a.40.2", ABR: 256},
		{URL: "https://media.invalid/audio-unsupported.flac", Ext: "flac", Protocol: "https", VCodec: "none", ACodec: "flac", ABR: 320},
	}}

	plan, err := buildPlanFromInfo(info)
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if plan.AudioOnly == nil || plan.AudioOnly.URL != "https://media.invalid/audio-low.m4a" {
		t.Fatalf("áudio web selecionado = %+v", plan.AudioOnly)
	}
}

func TestBuildPlanPrefersH264AACOverYtDlpDefaultAV1Opus(t *testing.T) {
	plan, err := buildPlanFromInfo(YtDlpVideoJSON{
		RequestedFormats: []YtDlpFormatJSON{
			{FormatID: "401", URL: "https://media.invalid/default-av1.webm", Ext: "webm", Protocol: "https", VCodec: "av01.0.08M.08", ACodec: "none", Height: 2160},
			{FormatID: "251", URL: "https://media.invalid/default-opus.webm", Ext: "webm", Protocol: "https", VCodec: "none", ACodec: "opus", ABR: 160},
		},
		Formats: []YtDlpFormatJSON{
			{FormatID: "399", URL: "https://media.invalid/av1-1080.webm", Ext: "webm", Protocol: "https", VCodec: "av01.0.08M.08", ACodec: "none", Height: 1080},
			{FormatID: "137", URL: "https://media.invalid/h264-1080.mp4", Ext: "mp4", Protocol: "https", VCodec: "avc1.640028", ACodec: "none", Height: 1080},
			{FormatID: "140", URL: "https://media.invalid/aac.m4a", Ext: "m4a", Protocol: "https", VCodec: "none", ACodec: "mp4a.40.2", ABR: 129},
			{FormatID: "18", URL: "https://media.invalid/h264-aac-360.mp4", Ext: "mp4", Protocol: "https", VCodec: "avc1.42001E", ACodec: "mp4a.40.2", Height: 360},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Primary.URL != "https://media.invalid/h264-1080.mp4" || plan.Audio == nil || plan.Audio.URL != "https://media.invalid/aac.m4a" {
		t.Fatalf("plano automático incompatível: primary=%+v audio=%+v", plan.Primary, plan.Audio)
	}
	for _, variant := range plan.Variants {
		if strings.Contains(variant.Stream.URL, "av1") {
			t.Fatalf("variante AV1 exposta apesar de H.264 disponível: %+v", variant)
		}
	}
}

func TestSelectMuxedFormatPrefersCompatibleCodecBeforeResolution(t *testing.T) {
	selected := selectMuxedFormat([]YtDlpFormatJSON{
		{URL: "https://media.invalid/av1-2160.webm", Ext: "webm", Protocol: "https", VCodec: "av01", ACodec: "opus", Height: 2160},
		{URL: "https://media.invalid/h264-720.mp4", Ext: "mp4", Protocol: "https", VCodec: "avc1.4d401f", ACodec: "mp4a.40.2", Height: 720},
	}, 0)
	if selected == nil || selected.URL != "https://media.invalid/h264-720.mp4" {
		t.Fatalf("fallback muxado = %+v", selected)
	}
}

func TestRequestedStoryboardAndAudioFallBackToPlayableFormats(t *testing.T) {
	info := YtDlpVideoJSON{
		RequestedFormats: []YtDlpFormatJSON{
			{URL: "https://media.invalid/storyboard.jpg", VCodec: "none", ACodec: "none"},
			{URL: "https://media.invalid/audio.m4a", VCodec: "none", ACodec: "aac"},
		},
		Formats: []YtDlpFormatJSON{{URL: "https://media.invalid/muxed.mp4", VCodec: "h264", ACodec: "aac", Height: 360}},
	}
	plan, err := buildPlanFromInfo(info)
	if err != nil {
		t.Fatalf("buildPlanFromInfo: %v", err)
	}
	if plan.Primary.URL != "https://media.invalid/muxed.mp4" {
		t.Fatalf("storyboard foi aceito como vídeo: %+v", plan.Primary)
	}
}

func TestRequestedFormatsHonorMaxHeight(t *testing.T) {
	info := YtDlpVideoJSON{
		RequestedFormats: []YtDlpFormatJSON{
			{URL: "https://media.invalid/video-1080.mp4", VCodec: "h264", ACodec: "none", Height: 1080},
			{URL: "https://media.invalid/audio.m4a", VCodec: "none", ACodec: "aac"},
		},
		Formats: []YtDlpFormatJSON{{URL: "https://media.invalid/muxed-360.mp4", VCodec: "h264", ACodec: "aac", Height: 360}},
	}
	plan, err := buildPlanFromInfoWithMaxHeight(info, 480)
	if err != nil {
		t.Fatalf("buildPlanFromInfoWithMaxHeight: %v", err)
	}
	if plan.Primary.URL != "https://media.invalid/muxed-360.mp4" {
		t.Fatalf("requested acima do teto foi aceito: %q", plan.Primary.URL)
	}
}

func extractorClientArg(args []string) string {
	for i, arg := range args {
		if arg == "--extractor-args" && i+1 < len(args) {
			return strings.TrimPrefix(args[i+1], "youtube:player_client=")
		}
	}
	return ""
}

func testExecutable(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func BenchmarkBuildPlanFromInfo(b *testing.B) {
	info := YtDlpVideoJSON{
		ID:       "adaptive1",
		Title:    "Adaptive Video Title",
		Duration: 300,
		RequestedFormats: []YtDlpFormatJSON{
			{
				FormatID:    "398",
				URL:         "https://googlevideo.com/video_720p",
				VCodec:      "av01.0.05M.08",
				ACodec:      "none",
				Height:      720,
				Width:       1280,
				HTTPHeaders: map[string]string{"User-Agent": "VideoUA"},
			},
			{
				FormatID:    "251",
				URL:         "https://googlevideo.com/audio_opus",
				VCodec:      "none",
				ACodec:      "opus",
				HTTPHeaders: map[string]string{"User-Agent": "AudioUA"},
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = buildPlanFromInfo(info)
	}
}
