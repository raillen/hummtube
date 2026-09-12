// Configuração do yt-dlp e do extractor runtime.
// Documento canônico: docs/03-implementation/YOUTUBE_PLAYBACK_MODERNIZATION.md.
//
// Importante: nada aqui tem relação com o OAuth do NanoTube. O token OAuth
// autoriza a YouTube Data API (catálogo); a extração de mídia acontece por
// outro caminho, que não aceita esse token
// (docs/03-implementation/YOUTUBE_AND_AUTH.md).
package playback

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Variáveis de ambiente que ajustam a extração sem recompilar.
const (
	EnvYtDlpPath    = "NANOTUBE_YTDLP_PATH"
	EnvJSRuntime    = "NANOTUBE_YTDL_JS_RUNTIME"
	EnvPlayerClient = "NANOTUBE_YTDL_PLAYER_CLIENT"
	EnvPOTProvider  = "NANOTUBE_YTDL_POT_PROVIDER"
	EnvPOTMode      = "NANOTUBE_YTDL_POT_MODE"
	EnvCookiesFrom  = "NANOTUBE_YTDL_COOKIES_FROM_BROWSER"
	EnvCookiesFile  = "NANOTUBE_YTDL_COOKIES_FILE"
	EnvRemoteEJS    = "NANOTUBE_YTDL_REMOTE_EJS"
	EnvMaxHeight    = "NANOTUBE_MAX_HEIGHT"
)

const (
	// AutoPlayerClient deixa o resolver escolher uma política compatível com o
	// runtime disponível. Não é um nome enviado ao yt-dlp.
	AutoPlayerClient = "auto"
	// DefaultPlayerClient evita fixar um fingerprint volátil no fluxo comum.
	DefaultPlayerClient = AutoPlayerClient
)

// JSRuntime é o runtime JavaScript que o yt-dlp usa para resolver os desafios
// do YouTube (o "n challenge" e scripts EJS).
type JSRuntime struct {
	// Name é o nome que o yt-dlp reconhece em --js-runtimes.
	Name string
	// Path é o executável encontrado no PATH.
	Path string
}

// Found reports whether a runtime was detected.
func (r JSRuntime) Found() bool { return r.Name != "" && r.Path != "" }

// String renders the value expected by --js-runtimes (RUNTIME[:PATH]).
func (r JSRuntime) String() string {
	if !r.Found() {
		return ""
	}
	return r.Name + ":" + r.Path
}

// jsRuntimeBinaries mapeia o nome usado pelo yt-dlp ao executável procurado.
// A ordem é a política recomendada: Deno preferencial, Node e QuickJS como
// fallbacks suportados. Bun foi removido da lista recomendada (seção 12.1).
var jsRuntimeBinaries = []JSRuntime{
	{Name: "deno", Path: "deno"},
	{Name: "node", Path: "node"},
	{Name: "quickjs", Path: "qjs"},
}

// DetectJSRuntime returns the first usable JavaScript runtime on PATH.
func DetectJSRuntime() JSRuntime {
	for _, candidate := range jsRuntimeBinaries {
		path, err := exec.LookPath(candidate.Path)
		if err != nil {
			continue
		}
		return JSRuntime{Name: candidate.Name, Path: path}
	}
	return JSRuntime{}
}

// YtdlConfig descreve como o yt-dlp deve ser invocado. O zero value já é
// utilizável: significa "usar a política automática do NanoTube".
type YtdlConfig struct {
	// YtDlpPath é o caminho explícito do executável yt-dlp (opcional).
	YtDlpPath string
	// JSRuntime resolve os desafios JS.
	JSRuntime JSRuntime
	// PlayerClient força um cliente InnerTube (ex.: "mweb", "android", "tv").
	PlayerClient string
	// POTProvider configura o endpoint HTTP ou o diretório server_home do
	// bgutil. O plugin continua sendo descoberto e carregado pelo próprio yt-dlp.
	POTProvider string
	// POTMode seleciona o modo do provider ("script" sob demanda ou "http" local).
	POTMode string
	// CookiesFromBrowser lê cookies de um navegador instalado (ex.: "firefox").
	CookiesFromBrowser string
	// CookiesFile é um arquivo de cookies no formato Netscape.
	CookiesFile string
	// AllowRemoteComponents autoriza o yt-dlp a baixar o script solucionador de
	// desafios (EJS) do GitHub. Opt-in: é execução de código remoto.
	AllowRemoteComponents bool
	// MaxHeight limita a resolução escolhida (FR-016). Zero não limita.
	MaxHeight int
}

func (c YtdlConfig) withoutCookies() YtdlConfig {
	c.CookiesFromBrowser = ""
	c.CookiesFile = ""
	return c
}

func (c YtdlConfig) hasCookies() bool {
	return strings.TrimSpace(c.CookiesFromBrowser) != "" || strings.TrimSpace(c.CookiesFile) != ""
}

// LoadYtdlConfig builds the configuration from the environment, detecting a
// JavaScript runtime when the user did not pin one, and defaulting to the
// compatibility policy owned by ExplicitYtDlpResolver.
func LoadYtdlConfig() YtdlConfig {
	client := strings.TrimSpace(os.Getenv(EnvPlayerClient))
	if client == "" {
		client = DefaultPlayerClient
	}

	allowRemote := false
	if val := os.Getenv(EnvRemoteEJS); val != "" {
		allowRemote = envBool(val)
	}

	cookiesFile := strings.TrimSpace(os.Getenv(EnvCookiesFile))

	cfg := YtdlConfig{
		YtDlpPath:             strings.TrimSpace(os.Getenv(EnvYtDlpPath)),
		PlayerClient:          client,
		POTProvider:           strings.TrimSpace(os.Getenv(EnvPOTProvider)),
		POTMode:               strings.TrimSpace(os.Getenv(EnvPOTMode)),
		CookiesFromBrowser:    strings.TrimSpace(os.Getenv(EnvCookiesFrom)),
		CookiesFile:           cookiesFile,
		AllowRemoteComponents: allowRemote,
	}
	configureDetectedPOTProvider(&cfg)
	if height, err := strconv.Atoi(strings.TrimSpace(os.Getenv(EnvMaxHeight))); err == nil && height > 0 {
		cfg.MaxHeight = height
	}
	cfg.JSRuntime = resolveJSRuntime(os.Getenv(EnvJSRuntime))
	return cfg
}

var potServerHomeCandidates = defaultPOTServerHomeCandidates

func defaultPOTServerHomeCandidates() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	runtimeRoot := filepath.Join(home, ".local", "share", "nanotube", "runtime")
	return []string{
		filepath.Join(runtimeRoot, "bgutil-ytdlp-pot-provider", "server"),
		filepath.Join(runtimeRoot, "bgutil", "server"),
		filepath.Join(home, "bgutil-ytdlp-pot-provider", "server"),
	}
}

// configureDetectedPOTProvider habilita apenas instalações locais completas do
// provider em modo script. A variável de ambiente explícita continua tendo
// precedência e endpoints HTTP nunca são descobertos automaticamente.
func configureDetectedPOTProvider(cfg *YtdlConfig) {
	if cfg == nil || strings.TrimSpace(cfg.POTProvider) != "" {
		return
	}
	for _, serverHome := range potServerHomeCandidates() {
		if !validPOTServerHome(serverHome) {
			continue
		}
		cfg.POTProvider = serverHome
		cfg.POTMode = "script"
		return
	}
}

func validPOTServerHome(serverHome string) bool {
	if strings.TrimSpace(serverHome) == "" {
		return false
	}
	for _, script := range []string{
		filepath.Join(serverHome, "src", "generate_once.ts"),
		filepath.Join(serverHome, "build", "generate_once.js"),
	} {
		if info, err := os.Stat(script); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

// NormalizeBrowserCookieSource accepts only yt-dlp browser/keyring selectors
// exposed by the NanoTube UI. This prevents the local RPC boundary from being
// used to request arbitrary browser profiles or filesystem locations.
func NormalizeBrowserCookieSource(source string) (string, error) {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		return "", nil
	}
	browser, keyring, hasKeyring := strings.Cut(source, "+")
	allowedBrowsers := map[string]bool{
		"brave": true, "chrome": true, "chromium": true, "edge": true,
		"firefox": true, "opera": true, "vivaldi": true,
	}
	if !allowedBrowsers[browser] {
		return "", fmt.Errorf("navegador não suportado para cookies: %q", browser)
	}
	if !hasKeyring {
		return browser, nil
	}
	allowedKeyrings := map[string]bool{
		"basictext": true, "gnomekeyring": true, "kwallet": true,
		"kwallet5": true, "kwallet6": true,
	}
	if keyring == "" || strings.Contains(keyring, "+") || !allowedKeyrings[keyring] {
		return "", fmt.Errorf("cofre de cookies não suportado: %q", keyring)
	}
	return browser + "+" + keyring, nil
}

// resolveJSRuntime honra a escolha explícita do usuário e cai na detecção
// automática quando ela está ausente. "off" desliga a detecção.
func resolveJSRuntime(pinned string) JSRuntime {
	pinned = strings.TrimSpace(pinned)
	switch {
	case pinned == "":
		for _, candidate := range jsRuntimeBinaries {
			if managedPath, err := managedRuntimeComponentPath(candidate.Name); err == nil {
				if filepath.Base(managedPath) != candidate.Path {
					continue
				}
				return JSRuntime{Name: candidate.Name, Path: managedPath}
			}
		}
		return DetectJSRuntime()
	case strings.EqualFold(pinned, "off"), strings.EqualFold(pinned, "none"):
		return JSRuntime{}
	}
	name, path, hasPath := strings.Cut(pinned, ":")
	name = strings.ToLower(strings.TrimSpace(name))
	if !supportedJSRuntime(name) {
		return JSRuntime{}
	}
	if hasPath {
		path = strings.TrimSpace(path)
		if path == "" {
			return JSRuntime{}
		}
		resolved, err := exec.LookPath(path)
		if err != nil {
			return JSRuntime{}
		}
		return JSRuntime{Name: name, Path: resolved}
	}
	resolved, err := exec.LookPath(binaryFor(name))
	if err != nil {
		return JSRuntime{}
	}
	return JSRuntime{Name: name, Path: resolved}
}

func supportedJSRuntime(name string) bool {
	for _, candidate := range jsRuntimeBinaries {
		if candidate.Name == name {
			return true
		}
	}
	return false
}

func binaryFor(name string) string {
	for _, candidate := range jsRuntimeBinaries {
		if candidate.Name == name {
			return candidate.Path
		}
	}
	if name == "bun" {
		return "bun"
	}
	return name
}

func envBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// FormatSelector renders the yt-dlp format expression for the height cap.
func (c YtdlConfig) FormatSelector() string {
	if c.MaxHeight <= 0 {
		return ""
	}
	height := strconv.Itoa(c.MaxHeight)
	return "bestvideo[height<=?" + height + "]+bestaudio/best[height<=?" + height + "]/best"
}

// RawOptions renders the value of mpv's `ytdl-raw-options`, which the ytdl_hook
// forwards to yt-dlp as command-line flags (kept for fallback compatibility).
func (c YtdlConfig) RawOptions() (string, error) {
	pairs := make([]string, 0, 6)
	add := func(key, value string) error {
		if value == "" {
			return nil
		}
		if strings.ContainsAny(value, ",") {
			return fmt.Errorf("ytdl-raw-options: valor de %q contém vírgula, que o mpv não sabe separar: %q", key, value)
		}
		pairs = append(pairs, key+"="+value)
		return nil
	}

	if err := add("js-runtimes", c.JSRuntime.String()); err != nil {
		return "", err
	}
	if c.PlayerClient != "" && c.PlayerClient != AutoPlayerClient {
		if err := add("extractor-args", "youtube:player_client="+c.PlayerClient); err != nil {
			return "", err
		}
	}
	for _, providerArg := range c.potExtractorArgs() {
		if err := add("extractor-args", providerArg); err != nil {
			return "", err
		}
	}
	if c.AllowRemoteComponents {
		if err := add("remote-components", "ejs:github"); err != nil {
			return "", err
		}
	}
	if c.CookiesFromBrowser != "" {
		if err := add("cookies-from-browser", c.CookiesFromBrowser); err != nil {
			return "", err
		}
	}
	if c.CookiesFile != "" {
		if err := add("cookies", c.CookiesFile); err != nil {
			return "", err
		}
	}
	if err := add("format", c.FormatSelector()); err != nil {
		return "", err
	}
	return strings.Join(pairs, ","), nil
}

// MpvOptions renders the mpv options a PlaybackPlan carries for remote sources in hook mode.
func (c YtdlConfig) MpvOptions() (map[string]string, error) {
	raw, err := c.RawOptions()
	if err != nil {
		return nil, err
	}
	return map[string]string{"ytdl-raw-options": raw}, nil
}

// CommandArgs renders configuration as discrete flags without shell escaping,
// suitable for exec.CommandContext.
func (c YtdlConfig) CommandArgs() []string {
	// A extração nunca herda arquivos de configuração ou plugins globais do
	// usuário. Esses pontos de extensão executam código e são especialmente
	// perigosos quando a tentativa autenticada lê cookies do navegador.
	args := []string{"--ignore-config", "--no-plugin-dirs"}
	if pluginDir := configuredPOTPluginDir(c); pluginDir != "" {
		args = append(args, "--plugin-dirs", pluginDir)
	}
	if c.JSRuntime.Found() {
		args = append(args, "--js-runtimes", c.JSRuntime.String())
	}
	if c.PlayerClient != "" && c.PlayerClient != AutoPlayerClient {
		args = append(args, "--extractor-args", "youtube:player_client="+c.PlayerClient)
	}
	for _, providerArg := range c.potExtractorArgs() {
		args = append(args, "--extractor-args", providerArg)
	}
	if c.AllowRemoteComponents {
		args = append(args, "--remote-components", "ejs:github")
	}
	if c.CookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", c.CookiesFromBrowser)
	}
	if c.CookiesFile != "" {
		args = append(args, "--cookies", c.CookiesFile)
	}
	if selector := c.FormatSelector(); selector != "" {
		args = append(args, "--format", selector)
	}
	return args
}

func (c YtdlConfig) potExtractorArgs() []string {
	provider := strings.TrimSpace(c.POTProvider)
	if provider == "" {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(c.POTMode), "http") {
		if !loopbackProviderURL(provider) {
			return nil
		}
		return []string{"youtubepot-bgutilhttp:base_url=" + provider}
	}
	return []string{"youtubepot-bgutilscript:server_home=" + provider}
}

func loopbackProviderURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User != nil || !httpURLScheme(parsed.Scheme) || parsed.Hostname() == "" {
		return false
	}
	if strings.EqualFold(parsed.Hostname(), "localhost") {
		return true
	}
	ip := net.ParseIP(parsed.Hostname())
	return ip != nil && ip.IsLoopback()
}

func httpURLScheme(scheme string) bool {
	return strings.EqualFold(scheme, "http") || strings.EqualFold(scheme, "https")
}

// NormalizePlayerClientPreference migrates client fingerprints that are no
// longer safe defaults. Environment overrides remain explicit and are not
// normalized by the caller.
func NormalizePlayerClientPreference(client string) string {
	switch strings.TrimSpace(client) {
	case "", "tv", "tv_simply":
		return AutoPlayerClient
	default:
		return strings.TrimSpace(client)
	}
}

// Redacted renders the configuration for diagnostics and logs. Caminhos de
// cookies, segredos e tokens nunca aparecem por extenso (NFR-SEC-001).
func (c YtdlConfig) Redacted() string {
	parts := make([]string, 0, 6)
	if c.JSRuntime.Found() {
		parts = append(parts, "js-runtime="+c.JSRuntime.Name)
	} else {
		parts = append(parts, "js-runtime=ausente")
	}
	if c.PlayerClient != "" {
		parts = append(parts, "player-client="+c.PlayerClient)
	}
	if c.POTProvider != "" {
		parts = append(parts, "pot-provider=configurado")
	}
	if c.AllowRemoteComponents {
		parts = append(parts, "remote-components=ejs:github")
	}
	if c.CookiesFromBrowser != "" {
		parts = append(parts, "cookies=navegador")
	} else if c.CookiesFile != "" {
		parts = append(parts, "cookies=arquivo")
	} else {
		parts = append(parts, "cookies=não")
	}
	if c.MaxHeight > 0 {
		parts = append(parts, "max-height="+strconv.Itoa(c.MaxHeight))
	}
	return strings.Join(parts, " ")
}
