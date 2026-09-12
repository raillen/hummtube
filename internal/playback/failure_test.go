package playback

import (
	"strings"
	"testing"
)

// As mensagens abaixo foram capturadas do yt-dlp nesta máquina.
func TestClassifyFailureRealMessages(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    FailureKind
	}{
		{
			"anti-bot",
			"ERROR: [youtube] aqz-KE-bpKQ: Sign in to confirm you’re not a bot. Use --cookies-from-browser",
			FailureBotCheck,
		},
		{
			"cookies criptografados com cofre incorreto",
			"WARNING: cannot decrypt v11 cookies: no key found; 988 could not be decrypted",
			FailureCookieDecryption,
		},
		{
			"rate limit",
			"WARNING: [youtube] Unable to download webpage: HTTP Error 429: Too Many Requests",
			FailureRateLimited,
		},
		{
			"mídia bloqueada 403",
			"ERROR: unable to download video data: HTTP Error 403: Forbidden",
			FailureMediaBlocked,
		},
		{
			"mídia bloqueada 403 via mpv",
			"ffmpeg: https: HTTP error 403 Forbidden",
			FailureMediaBlocked,
		},
		{
			"sem formatos",
			"ERROR: [youtube] aqz-KE-bpKQ: Requested format is not available. Use --list-formats",
			FailureNoFormats,
		},
		{
			"só storyboards",
			"WARNING: Only images are available for download. use --list-formats to see them",
			FailureNoFormats,
		},
		{
			"desafio n falhou",
			"WARNING: [youtube] aqz-KE-bpKQ: n challenge solving failed: Some formats may be missing.",
			FailureNoFormats,
		},
		{
			"sabr",
			"Some tv client https formats have been skipped as they are missing a URL. YouTube may have enabled the SABR-only streaming experiment",
			FailureNoFormats,
		},
		{
			"pot generation",
			"ERROR: [youtube] botguard execution failed",
			FailurePOTGeneration,
		},
		{
			"pot provider missing",
			"WARNING: pot provider not found",
			FailurePOTProviderMissing,
		},
		{
			"visitor data missing",
			"ERROR: [youtube] Missing required Visitor Data",
			FailureVisitorDataMissing,
		},
		{
			"drm",
			"ERROR: [youtube] drm protected stream",
			FailureDRM,
		},
		// "Sign in if you've been granted access" não é a verificação anti-bot:
		// é falta de permissão, e a dica precisa ser outra.
		{"privado", "ERROR: [youtube] xyz: Private video. Sign in if you've been granted access", FailureUnavailable},
		{"indisponível", "ERROR: [youtube] xyz: Video unavailable", FailureUnavailable},
		{"live encerrada sem gravação", "ERROR: [youtube] xyz: This live stream recording is not available.", FailureUnavailable},
		{"rede", "getaddrinfo failed: Temporary failure in name resolution", FailureNetwork},
		{"url expirada", "ERROR: requested media URL has expired", FailureExpiredMediaURL},
		{"url expirada com 403", "HTTP 403: requested media URL has expired", FailureExpiredMediaURL},
		{"desconhecido", "algo completamente diferente", FailureUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyFailure(c.message, YtdlConfig{})
			if got.Kind != c.want {
				t.Errorf("ClassifyFailure(%q).Kind = %d, esperado %d", c.name, got.Kind, c.want)
			}
			if got.Summary == "" {
				t.Error("toda falha precisa de um resumo legível")
			}
		})
	}
}

func TestClassifyExtractorBoundaries(t *testing.T) {
	for _, test := range []struct {
		message string
		kind    FailureKind
	}{
		{"extractor excedeu o tempo limite", FailureExtractorTimeout},
		{"saída do extractor excedeu o limite seguro", FailureExtractorOutputLimit},
	} {
		failure := ClassifyFailure(test.message, YtdlConfig{})
		if failure.Kind != test.kind {
			t.Errorf("%q = %v, esperado %v", test.message, failure.Kind, test.kind)
		}
	}
}

// A dica escala: só pede o degrau seguinte quando o anterior já está resolvido.
func TestExtractionHintEscalates(t *testing.T) {
	node := JSRuntime{Name: "node", Path: "/usr/bin/node"}

	semRuntime := extractionHint(YtdlConfig{})
	if !strings.Contains(semRuntime, "runtime JavaScript") {
		t.Errorf("sem runtime a dica deveria pedir um: %q", semRuntime)
	}
	if !strings.Contains(semRuntime, "OAuth") {
		t.Error("a dica precisa desfazer a confusão com o login, que é a pergunta natural do usuário")
	}

	comRuntime := extractionHint(YtdlConfig{JSRuntime: node})
	if !strings.Contains(comRuntime, "NANOTUBE_YTDL_REMOTE_EJS") {
		t.Errorf("com runtime a dica deveria pedir o solucionador: %q", comRuntime)
	}
	if !strings.Contains(comRuntime, "Ajustes") {
		t.Errorf("a dica do EJS precisa apontar o controle visual: %q", comRuntime)
	}

	// Um cliente fixado inadequado deve sugerir a política automática.
	comClienteRuim := extractionHint(YtdlConfig{
		JSRuntime:             node,
		AllowRemoteComponents: true,
		PlayerClient:          "tv",
	})
	if !strings.Contains(comClienteRuim, "Automático") {
		t.Errorf("cliente fixado que falhou deveria sugerir o modo automático: %q", comClienteRuim)
	}

	comCliente := extractionHint(YtdlConfig{
		JSRuntime:             node,
		AllowRemoteComponents: true,
		PlayerClient:          AutoPlayerClient,
		POTProvider:           "bgutil",
	})
	if !strings.Contains(comCliente, "COOKIES_FROM_BROWSER") {
		t.Errorf("esgotadas as opções seguras, a dica deveria chegar aos cookies: %q", comCliente)
	}

	completo := extractionHint(YtdlConfig{
		JSRuntime:             node,
		AllowRemoteComponents: true,
		PlayerClient:          AutoPlayerClient,
		POTProvider:           "bgutil",
		CookiesFromBrowser:    "firefox",
	})
	if strings.Contains(completo, "Instale") {
		t.Errorf("com tudo configurado a dica não deveria mandar instalar nada: %q", completo)
	}
}

func TestClassifyFailureCarriesHintForExtraction(t *testing.T) {
	failure := ClassifyFailure("Sign in to confirm you're not a bot", YtdlConfig{})
	if failure.Hint == "" {
		t.Fatal("falha de extração precisa dizer o próximo passo")
	}
}

func TestClassifyMissingGVSPOTAsProviderMissing(t *testing.T) {
	failure := ClassifyFailure("mweb formats require a GVS PO Token which was not provided", YtdlConfig{})
	if failure.Kind != FailurePOTProviderMissing {
		t.Fatalf("kind = %v, esperado FailurePOTProviderMissing", failure.Kind)
	}
}

// 403 com cliente fixado deve apontar o cliente como culpado.
func TestClassifyFailureMediaBlockedHintsAgainstFixedClient(t *testing.T) {
	failure := ClassifyFailure("HTTP Error 403: Forbidden", YtdlConfig{PlayerClient: "tv"})
	if !strings.Contains(failure.Hint, "tv") || !strings.Contains(failure.Hint, "Automático") {
		t.Errorf("a dica do 403 com cliente fixado deveria sugerir trocar de cliente: %q", failure.Hint)
	}

	failure = ClassifyFailure("HTTP Error 403: Forbidden", YtdlConfig{PlayerClient: AutoPlayerClient})
	if strings.Contains(failure.Hint, "O cliente configurado") {
		t.Errorf("403 no modo automático não deveria reclamar de cliente fixo: %q", failure.Hint)
	}
}

func TestCommandArgsMirrorsRawOptions(t *testing.T) {
	cfg := YtdlConfig{
		JSRuntime:          JSRuntime{Name: "node", Path: "/usr/bin/node"},
		PlayerClient:       "mweb",
		CookiesFromBrowser: "firefox",
		MaxHeight:          720,
	}
	args := strings.Join(cfg.CommandArgs(), " ")
	for _, want := range []string{
		"--js-runtimes node:/usr/bin/node",
		"--extractor-args youtube:player_client=mweb",
		"--cookies-from-browser firefox",
		"--format bestvideo[height<=?720]",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("faltou %q em %q", want, args)
		}
	}
}

func TestClassifyAgeConfirmationBeforeGenericSignIn(t *testing.T) {
	failure := ClassifyFailure("ERROR: Sign in to confirm your age. This video may be inappropriate", YtdlConfig{})
	if failure.Kind != FailureRestricted {
		t.Fatalf("kind = %v, esperado FailureRestricted", failure.Kind)
	}
	if canTryNextClient(failure) {
		t.Fatal("restrição de idade não pode provocar outra tentativa")
	}
}

func TestCookieDecryptionTakesPrecedenceOverGenericSignIn(t *testing.T) {
	failure := ClassifyFailure(
		"WARNING: cannot decrypt v11 cookies: no key found\nERROR: Sign in to confirm you’re not a bot",
		YtdlConfig{},
	)
	if failure.Kind != FailureCookieDecryption || !strings.Contains(failure.Hint, "cofre") {
		t.Fatalf("falha de cookies não foi tornada acionável: %+v", failure)
	}
}

func TestFailureDetailRedactsSecrets(t *testing.T) {
	failure := ClassifyFailure(
		"Authorization: Bearer abc.def\nCookie: session=secret\nhttps://user:password@googlevideo.example/secret-token?sig=secret&token=hidden",
		YtdlConfig{},
	)
	for _, secret := range []string{"abc.def", "session=secret", "sig=secret", "token=hidden", "user:password", "secret-token"} {
		if strings.Contains(failure.Detail, secret) {
			t.Fatalf("segredo %q vazou em Detail: %q", secret, failure.Detail)
		}
	}
}

func BenchmarkClassifyFailure(b *testing.B) {
	msg := "ERROR: [youtube] aqz-KE-bpKQ: Sign in to confirm you’re not a bot. Use --cookies-from-browser"
	cfg := YtdlConfig{
		JSRuntime:    JSRuntime{Name: "deno", Path: "/usr/bin/deno"},
		PlayerClient: "mweb",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ClassifyFailure(msg, cfg)
	}
}
