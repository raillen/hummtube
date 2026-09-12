// Classificação de falhas de reprodução remota.
// Documento canônico: docs/03-implementation/YOUTUBE_PLAYBACK_MODERNIZATION.md (Seção 26).
package playback

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// FailureKind classifica por que a extração ou reprodução de um vídeo falhou.
type FailureKind int

const (
	FailureUnknown FailureKind = iota
	// FailureBotCheck é a verificação anti-bot do YouTube.
	FailureBotCheck
	// FailureRateLimited é HTTP 429 vindo do YouTube.
	FailureRateLimited
	// FailureNoFormats é extração bem-sucedida sem faixas reproduzíveis.
	FailureNoFormats
	// FailureMediaBlocked é HTTP 403 no download da mídia (CDN GVS).
	FailureMediaBlocked
	// FailureYtdlpMissing é ausência do executável yt-dlp.
	FailureYtdlpMissing
	// FailureUnavailable é vídeo privado, removido ou restrito.
	FailureUnavailable
	// FailureNetwork é falha de rede/DNS/timeout.
	FailureNetwork
	// FailureExtractorMissing é falha quando nenhum extractor compatível existe.
	FailureExtractorMissing
	// FailureExtractorOutdated indica versão desatualizada do yt-dlp.
	FailureExtractorOutdated
	// FailureJSRuntimeMissing indica ausência de Deno/Node/QuickJS.
	FailureJSRuntimeMissing
	// FailureEJSMissing indica ausência de script EJS para desafios JS.
	FailureEJSMissing
	// FailurePOTProviderMissing indica falta do plugin de PO Token Provider.
	FailurePOTProviderMissing
	// FailurePOTGeneration indica falha interna na geração do PO Token.
	FailurePOTGeneration
	// FailureVisitorDataMissing indica que o cliente não recebeu Visitor Data obrigatório.
	FailureVisitorDataMissing
	// FailureExtractorTimeout indica que uma tentativa excedeu o limite de tempo.
	FailureExtractorTimeout
	// FailureExtractorOutputLimit indica saída excessiva do subprocesso.
	FailureExtractorOutputLimit
	// FailureGVSForbidden é 403 direto de URL do Google Video.
	FailureGVSForbidden
	// FailureDRM indica conteúdo protegido por DRM.
	FailureDRM
	// FailureRestricted indica restrição por idade ou membros.
	FailureRestricted
	// FailureCookieDecryption indica que o navegador foi encontrado, mas o
	// keyring selecionado não conseguiu descriptografar a sessão.
	FailureCookieDecryption
	// FailureExpiredMediaURL indica URL temporária expirada.
	FailureExpiredMediaURL
	// FailurePlayer é erro interno do player libmpv.
	FailurePlayer
)

// Failure é um erro de reprodução já traduzido para o usuário.
type Failure struct {
	Kind    FailureKind `json:"kind"`
	Summary string      `json:"summary"`
	Hint    string      `json:"hint,omitempty"`
	Detail  string      `json:"detail,omitempty"`
}

// Error implements the error interface for Failure.
func (f Failure) Error() string {
	if f.Hint != "" {
		return fmt.Sprintf("%s (%s)", f.Summary, f.Hint)
	}
	return f.Summary
}

// needle é um par (marcador, classificação) avaliado em ordem.
var failureNeedles = []struct {
	match string
	kind  FailureKind
}{
	{"could not be decrypted", FailureCookieDecryption},
	{"cannot decrypt", FailureCookieDecryption},
	{"no key found", FailureCookieDecryption},
	{"secretstorage not available", FailureCookieDecryption},
	{"sign in to confirm your age", FailureRestricted},
	{"confirm your age", FailureRestricted},
	{"age-restricted", FailureRestricted},
	{"members-only", FailureRestricted},
	{"sign in to confirm", FailureBotCheck},
	{"not a bot", FailureBotCheck},
	{"confirm you", FailureBotCheck},
	{"botguard", FailurePOTGeneration},
	{"missing required visitor data", FailureVisitorDataMissing},
	{"po token which was not provided", FailurePOTProviderMissing},
	{"no po token providers registered", FailurePOTProviderMissing},
	{"pot provider", FailurePOTProviderMissing},
	{"po token", FailurePOTGeneration},
	{"429", FailureRateLimited},
	{"too many requests", FailureRateLimited},
	{"403 forbidden", FailureMediaBlocked},
	{"403: forbidden", FailureMediaBlocked},
	{"requested format is not available", FailureNoFormats},
	{"only images are available", FailureNoFormats},
	{"missing a url", FailureNoFormats},
	{"sabr", FailureNoFormats},
	{"n challenge", FailureNoFormats},
	{"no such file or directory: 'yt-dlp'", FailureYtdlpMissing},
	{"youtube-dl not found", FailureYtdlpMissing},
	{"ytdl_hook", FailureYtdlpMissing},
	{"drm", FailureDRM},
	{"protected content", FailureDRM},
	{"private video", FailureUnavailable},
	{"video unavailable", FailureUnavailable},
	{"live stream recording is not available", FailureUnavailable},
	{"temporary failure in name resolution", FailureNetwork},
	{"connection refused", FailureNetwork},
	{"timed out", FailureNetwork},
	{"extractor excedeu o tempo limite", FailureExtractorTimeout},
	{"extractor exceeded timeout", FailureExtractorTimeout},
	{"saída do extractor excedeu o limite seguro", FailureExtractorOutputLimit},
	{"extractor output exceeded safe limit", FailureExtractorOutputLimit},
	{"url has expired", FailureExpiredMediaURL},
	{"url expired", FailureExpiredMediaURL},
	{"signature has expired", FailureExpiredMediaURL},
	{"js-runtime", FailureJSRuntimeMissing},
	{"ejs", FailureEJSMissing},
}

// ClassifyFailure traduz a saída do yt-dlp/mpv em uma falha acionável.
func ClassifyFailure(message string, cfg YtdlConfig) Failure {
	lowered := strings.ToLower(message)
	if strings.Contains(lowered, "url has expired") ||
		strings.Contains(lowered, "signature has expired") ||
		strings.Contains(lowered, "requested media url has expired") {
		failure := describe(FailureExpiredMediaURL, cfg)
		failure.Detail = sanitize(message, 300)
		return failure
	}
	kind := FailureUnknown
	for _, needle := range failureNeedles {
		if strings.Contains(lowered, needle.match) {
			kind = needle.kind
			break
		}
	}
	f := describe(kind, cfg)
	f.Detail = sanitize(message, 300)
	return f
}

func classifyExtractorError(err error, message string, cfg YtdlConfig) Failure {
	var failure Failure
	switch {
	case errors.Is(err, errExtractorTimeout):
		failure = describe(FailureExtractorTimeout, cfg)
	case errors.Is(err, errExtractorOutputLimit):
		failure = describe(FailureExtractorOutputLimit, cfg)
	default:
		failure = ClassifyFailure(message, cfg)
	}
	failure.Detail = sanitize(message, 300)
	return failure
}

func describe(kind FailureKind, cfg YtdlConfig) Failure {
	switch kind {
	case FailureBotCheck, FailureNoFormats:
		return Failure{
			Kind:    kind,
			Summary: botCheckSummary(kind),
			Hint:    extractionHint(cfg),
		}
	case FailurePOTProviderMissing, FailurePOTGeneration:
		return Failure{
			Kind:    kind,
			Summary: "não foi possível obter a autorização de playback (PO Token)",
			Hint:    "Verifique o PO Token Provider em Diagnóstico (`nanotube --diagnostics`).",
		}
	case FailureVisitorDataMissing:
		return Failure{
			Kind:    kind,
			Summary: "o YouTube não forneceu o Visitor Data obrigatório para este cliente",
			Hint:    "Use a política Automática; se persistir, verifique EJS e PO Token Provider em Diagnóstico.",
		}
	case FailureExtractorTimeout:
		return Failure{
			Kind:    kind,
			Summary: "o yt-dlp excedeu o tempo limite de preparação",
			Hint:    "Verifique a rede e tente novamente; a tentativa foi encerrada para não deixar subprocessos presos.",
		}
	case FailureExtractorOutputLimit:
		return Failure{
			Kind:    kind,
			Summary: "o yt-dlp produziu uma saída maior que o limite seguro",
			Hint:    "Atualize o yt-dlp e verifique o diagnóstico; a saída foi descartada sem ser exibida integralmente.",
		}
	case FailureJSRuntimeMissing:
		return Failure{
			Kind:    kind,
			Summary: "runtime JavaScript não encontrado",
			Hint:    "Instale o Deno (recomendado) ou Node para o yt-dlp resolver os desafios do YouTube.",
		}
	case FailureEJSMissing:
		return Failure{
			Kind:    kind,
			Summary: "script solucionador de desafios (EJS) ausente",
			Hint:    "Defina NANOTUBE_YTDL_REMOTE_EJS=1 ou instale pacote EJS compatível.",
		}
	case FailureRateLimited:
		return Failure{
			Kind:    kind,
			Summary: "o YouTube limitou as requisições deste IP (HTTP 429)",
			Hint:    "Espere alguns minutos antes de tentar de novo. Repetir agora só prolonga o bloqueio.",
		}
	case FailureMediaBlocked, FailureGVSForbidden:
		summary := "o YouTube bloqueou o download da mídia deste vídeo (HTTP 403)"
		hint := "Pode ser bloqueio temporário de IP ou token expirado. Tente novamente em instantes."
		if cfg.PlayerClient != "" && cfg.PlayerClient != AutoPlayerClient {
			hint = "O cliente configurado (" + cfg.PlayerClient + ") devolveu formatos bloqueados no download. " +
				"Troque para Automático em Configurações."
		}
		return Failure{Kind: kind, Summary: summary, Hint: hint}
	case FailureYtdlpMissing, FailureExtractorMissing:
		return Failure{
			Kind:    kind,
			Summary: "yt-dlp não encontrado",
			Hint:    "Instale o yt-dlp e confirme com `nanotube --diagnostics`.",
		}
	case FailureDRM:
		return Failure{
			Kind:    kind,
			Summary: "vídeo protegido por DRM não suportado",
			Hint:    "O NanoTube não reproduz conteúdo com criptografia DRM proprietária.",
		}
	case FailureUnavailable:
		return Failure{
			Kind:    kind,
			Summary: "conteúdo indisponível (privado, removido ou live encerrada)",
			Hint:    "Este vídeo não está mais disponível publicamente no YouTube.",
		}
	case FailureRestricted:
		return Failure{
			Kind:    kind,
			Summary: "vídeo restrito (membros ou confirmação de idade)",
			Hint:    "Abra Entrar → Cookies, selecione o navegador e o cofre que contém uma sessão do YouTube com acesso.",
		}
	case FailureCookieDecryption:
		return Failure{
			Kind:    kind,
			Summary: "não foi possível ler os cookies do navegador",
			Hint:    "Abra Entrar → Cookies e selecione o cofre correto. No GNOME/Secret Service, use GNOME Keyring; no KDE, use KWallet.",
		}
	case FailureNetwork:
		return Failure{
			Kind:    kind,
			Summary: "falha de rede ao preparar o vídeo",
			Hint:    "Verifique a conexão com a internet e tente de novo.",
		}
	case FailureExpiredMediaURL:
		return Failure{
			Kind:    kind,
			Summary: "endereço temporário de vídeo expirado",
			Hint:    "Solicitando nova resolução do stream...",
		}
	default:
		return Failure{
			Kind:    FailureUnknown,
			Summary: "não consegui preparar este vídeo",
			Hint:    "Rode `nanotube --diagnostics` para inspecionar o estado do runtime.",
		}
	}
}

func botCheckSummary(kind FailureKind) string {
	if kind == FailureNoFormats {
		return "a extração não devolveu nenhuma faixa reproduzível"
	}
	return "o YouTube pediu verificação anti-bot para extrair este vídeo"
}

// extractionHint orienta a escada de compatibilidade moderna:
// 1. JS Runtime (Deno preferencial)
// 2. EJS
// 3. Cliente mweb + PO Token Provider
// 4. Cookies (apenas se conteúdo for restrito)
func extractionHint(cfg YtdlConfig) string {
	switch {
	case !cfg.JSRuntime.Found():
		return "Falta um runtime JavaScript para o yt-dlp resolver os desafios do YouTube. " +
			"Instale o Deno (recomendado) ou Node. Isto não tem relação com OAuth (Data API)."
	case !cfg.AllowRemoteComponents:
		return "O runtime JS está presente, mas falta o script de compatibilidade (EJS). " +
			"Ative Ajustes → Sistema → Reprodução e qualidade → Permitir baixar o solver EJS " +
			"(ou defina NANOTUBE_YTDL_REMOTE_EJS=1)."
	case cfg.PlayerClient != "" && cfg.PlayerClient != AutoPlayerClient:
		return "Troque o cliente de extração para Automático em Configurações."
	case cfg.POTProvider == "":
		return "O modo automático tentou clientes compatíveis, mas nenhum abriu este vídeo. " +
			"Instale um PO Token Provider para habilitar o caminho mweb attestado."
	case cfg.CookiesFromBrowser == "" && cfg.CookiesFile == "":
		return "O YouTube bloqueou este IP com verificação anti-bot. " +
			"Selecione seu navegador ou importe um arquivo cookies.txt em Configurações → Extração (ou defina NANOTUBE_YTDL_COOKIES_FROM_BROWSER)."
	default:
		return "Extração configurada com runtime JS e cookies; pode ser bloqueio temporário do IP. Tente mais tarde."
	}
}

// Diagnose roda o yt-dlp em modo simulação para descobrir o motivo de falhas em hook mode.
func Diagnose(ctx context.Context, cfg YtdlConfig, sourceURL string) Failure {
	binary, _, err := resolveYtDlpBinary(cfg.YtDlpPath)
	if err != nil {
		return describe(FailureYtdlpMissing, cfg)
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	args := append(cfg.CommandArgs(), "--simulate", "--", sourceURL)
	stdout, stderr, err := runExtractorCommand(ctx, exec.CommandContext, binary, args...)
	if err == nil {
		return Failure{
			Kind:    FailureUnknown,
			Summary: "o yt-dlp extrai este vídeo, mas o mpv não conseguiu abri-lo",
			Hint:    "Pode ser codec ou saída de vídeo. Veja `nanotube --diagnostics`.",
		}
	}
	output := stderr
	if len(output) == 0 {
		output = stdout
	}
	if len(output) == 0 {
		output = []byte(err.Error())
	}
	return classifyExtractorError(err, string(output), cfg)
}

var (
	sensitiveHeaderPattern = regexp.MustCompile(`(?i)\b(authorization|proxy-authorization|cookie|set-cookie)\s*[:=]\s*[^\r\n]+`)
	bearerPattern          = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`)
	sensitiveValuePattern  = regexp.MustCompile(`(?i)\b(sig|signature|lsig|token|po_token|pot)\s*[=:]\s*[^\s&,;]+`)
	urlPattern             = regexp.MustCompile(`https?://[^\s]+`)
)

func sanitize(s string, limit int) string {
	s = sensitiveHeaderPattern.ReplaceAllString(s, "$1=<redigido>")
	s = bearerPattern.ReplaceAllString(s, "Bearer <redigido>")
	s = sensitiveValuePattern.ReplaceAllString(s, "$1=<redigido>")
	s = urlPattern.ReplaceAllStringFunc(s, func(rawURL string) string {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "<url-redigida>"
		}
		return parsed.Scheme + "://" + parsed.Host + "/<redigido>"
	})
	s = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
	if len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}
