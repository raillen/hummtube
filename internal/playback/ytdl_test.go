package playback

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRawOptionsEmptyByDefault(t *testing.T) {
	raw, err := YtdlConfig{}.RawOptions()
	if err != nil {
		t.Fatalf("RawOptions: %v", err)
	}
	if raw != "" {
		t.Fatalf("config vazia deveria não passar nada ao yt-dlp, veio %q", raw)
	}
}

func TestRawOptionsBuildsFlags(t *testing.T) {
	cfg := YtdlConfig{
		JSRuntime:             JSRuntime{Name: "node", Path: "/usr/bin/node"},
		PlayerClient:          "tv",
		AllowRemoteComponents: true,
		CookiesFromBrowser:    "firefox",
		MaxHeight:             720,
	}
	raw, err := cfg.RawOptions()
	if err != nil {
		t.Fatalf("RawOptions: %v", err)
	}
	for _, want := range []string{
		"js-runtimes=node:/usr/bin/node",
		"extractor-args=youtube:player_client=tv",
		"remote-components=ejs:github",
		"cookies-from-browser=firefox",
		"format=bestvideo[height<=?720]+bestaudio/best[height<=?720]/best",
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("faltou %q em %q", want, raw)
		}
	}
}

// O mpv separa os pares por vírgula e não tem escape confiável: melhor recusar
// do que montar um comando silenciosamente errado.
func TestRawOptionsRejectsCommaInValue(t *testing.T) {
	cfg := YtdlConfig{PlayerClient: "tv,web_safari"}
	if _, err := cfg.RawOptions(); err == nil {
		t.Fatal("valor com vírgula deveria ser recusado")
	}
}

func TestMpvOptionsAlwaysCarriesKey(t *testing.T) {
	options, err := YtdlConfig{}.MpvOptions()
	if err != nil {
		t.Fatalf("MpvOptions: %v", err)
	}
	value, ok := options["ytdl-raw-options"]
	if !ok {
		t.Fatal("a chave precisa existir mesmo vazia, para limpar o vídeo anterior")
	}
	if value != "" {
		t.Fatalf("esperado valor vazio, veio %q", value)
	}
}

func TestFormatSelector(t *testing.T) {
	if got := (YtdlConfig{}).FormatSelector(); got != "" {
		t.Errorf("sem teto de altura não deveria haver seletor, veio %q", got)
	}
	got := YtdlConfig{MaxHeight: 480}.FormatSelector()
	if !strings.Contains(got, "height<=?480") {
		t.Errorf("seletor não respeitou a altura: %q", got)
	}
}

func TestResolveJSRuntime(t *testing.T) {
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		pinned   string
		wantName string
		wantPath string
	}{
		{"caminho explícito", "node:" + truePath, "node", truePath},
		{"caminho ausente", "node:/opt/node", "", ""},
		{"desligado", "off", "", ""},
		{"desligado em maiúscula", "NONE", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := resolveJSRuntime(c.pinned)
			if got.Name != c.wantName || got.Path != c.wantPath {
				t.Errorf("resolveJSRuntime(%q) = %+v, esperado %s/%s", c.pinned, got, c.wantName, c.wantPath)
			}
		})
	}
}

func TestJSRuntimeString(t *testing.T) {
	if got := (JSRuntime{}).String(); got != "" {
		t.Errorf("runtime ausente deveria render vazio, veio %q", got)
	}
	if got := (JSRuntime{Name: "deno", Path: "/usr/bin/deno"}).String(); got != "deno:/usr/bin/deno" {
		t.Errorf("formato inesperado: %q", got)
	}
}

func TestLoadYtdlConfigFromEnv(t *testing.T) {
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvJSRuntime, "node:"+truePath)
	t.Setenv(EnvPlayerClient, "tv")
	t.Setenv(EnvCookiesFrom, "firefox")
	t.Setenv(EnvRemoteEJS, "1")
	t.Setenv(EnvMaxHeight, "480")

	cfg := LoadYtdlConfig()
	if cfg.JSRuntime.Path != truePath {
		t.Errorf("runtime: %+v", cfg.JSRuntime)
	}
	if cfg.PlayerClient != "tv" {
		t.Errorf("player client: %q", cfg.PlayerClient)
	}
	if !cfg.AllowRemoteComponents {
		t.Error("remote components deveria estar habilitado")
	}
	if cfg.MaxHeight != 480 {
		t.Errorf("max height: %d", cfg.MaxHeight)
	}
}

func TestLoadYtdlConfigIgnoresInvalidHeight(t *testing.T) {
	t.Setenv(EnvMaxHeight, "não é número")
	if got := LoadYtdlConfig().MaxHeight; got != 0 {
		t.Errorf("altura inválida deveria ser ignorada, veio %d", got)
	}
}

func TestLoadYtdlConfigDefaultsToAutomaticClientAndOptInFeatures(t *testing.T) {
	t.Setenv(EnvPlayerClient, "")
	t.Setenv(EnvRemoteEJS, "")
	t.Setenv(EnvCookiesFile, "")
	t.Setenv(EnvCookiesFrom, "")
	cfg := LoadYtdlConfig()
	if cfg.PlayerClient != AutoPlayerClient {
		t.Fatalf("cliente padrão = %q, esperado %q", cfg.PlayerClient, AutoPlayerClient)
	}
	if cfg.AllowRemoteComponents {
		t.Fatal("EJS remoto deve ser opt-in")
	}
	if cfg.CookiesFile != "" || cfg.CookiesFromBrowser != "" {
		t.Fatalf("cookies devem ser opt-in: %+v", cfg)
	}
	if args := strings.Join(cfg.CommandArgs(), " "); strings.Contains(args, "player_client") {
		t.Fatalf("auto não é cliente do yt-dlp e não pode vazar para argv: %q", args)
	}
}

func TestLoadYtdlConfigDetectsManagedPOTServer(t *testing.T) {
	serverHome := filepath.Join(t.TempDir(), "server")
	scriptPath := filepath.Join(serverHome, "src", "generate_once.ts")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, []byte("// fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	previousCandidates := potServerHomeCandidates
	potServerHomeCandidates = func() []string { return []string{serverHome} }
	t.Cleanup(func() { potServerHomeCandidates = previousCandidates })
	t.Setenv(EnvPOTProvider, "")
	t.Setenv(EnvPOTMode, "")

	cfg := LoadYtdlConfig()
	if cfg.POTProvider != serverHome || cfg.POTMode != "script" {
		t.Fatalf("provider gerenciado não foi detectado: %+v", cfg)
	}
}

func TestDefaultPOTServerCandidatesIncludeCurrentAndLegacyManagedLayouts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	candidates := strings.Join(defaultPOTServerHomeCandidates(), "\n")
	for _, relative := range []string{
		filepath.Join(".local", "share", "nanotube", "runtime", "bgutil-ytdlp-pot-provider", "server"),
		filepath.Join(".local", "share", "nanotube", "runtime", "bgutil", "server"),
	} {
		if !strings.Contains(candidates, filepath.Join(home, relative)) {
			t.Fatalf("layout de server ausente: %s em %s", relative, candidates)
		}
	}
}

func TestExplicitPOTProviderTakesPrecedenceOverDetection(t *testing.T) {
	previousCandidates := potServerHomeCandidates
	potServerHomeCandidates = func() []string { return []string{t.TempDir()} }
	t.Cleanup(func() { potServerHomeCandidates = previousCandidates })
	t.Setenv(EnvPOTProvider, "http://127.0.0.1:4416")
	t.Setenv(EnvPOTMode, "http")

	cfg := LoadYtdlConfig()
	if cfg.POTProvider != "http://127.0.0.1:4416" || cfg.POTMode != "http" {
		t.Fatalf("configuração explícita foi alterada: %+v", cfg)
	}
}

func TestNormalizeBrowserCookieSource(t *testing.T) {
	for _, source := range []string{"firefox", "brave+gnomekeyring", "chromium+kwallet6", "chrome+basictext"} {
		got, err := NormalizeBrowserCookieSource(source)
		if err != nil || got != source {
			t.Errorf("NormalizeBrowserCookieSource(%q) = %q, %v", source, got, err)
		}
	}
	for _, source := range []string{"safari", "brave:/tmp/profile", "brave+secret", "brave+gnomekeyring+extra"} {
		if _, err := NormalizeBrowserCookieSource(source); err == nil {
			t.Errorf("seletor não permitido foi aceito: %q", source)
		}
	}
}

func TestNormalizePlayerClientPreferenceMigratesBrokenTVClients(t *testing.T) {
	for _, legacy := range []string{"", "tv", "tv_simply"} {
		if got := NormalizePlayerClientPreference(legacy); got != AutoPlayerClient {
			t.Errorf("NormalizePlayerClientPreference(%q) = %q", legacy, got)
		}
	}
	if got := NormalizePlayerClientPreference("android"); got != "android" {
		t.Fatalf("escolha manual válida foi alterada: %q", got)
	}
}

func TestCommandArgsConfiguresPOTProviderModes(t *testing.T) {
	tests := []struct {
		name     string
		cfg      YtdlConfig
		expected string
	}{
		{
			name:     "script",
			cfg:      YtdlConfig{POTProvider: "/opt/bgutil/server", POTMode: "script"},
			expected: "youtubepot-bgutilscript:server_home=/opt/bgutil/server",
		},
		{
			name:     "http",
			cfg:      YtdlConfig{POTProvider: "http://127.0.0.1:4416", POTMode: "http"},
			expected: "youtubepot-bgutilhttp:base_url=http://127.0.0.1:4416",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := strings.Join(test.cfg.CommandArgs(), " ")
			if !strings.Contains(args, test.expected) {
				t.Fatalf("provider não chegou ao yt-dlp: %q", args)
			}
		})
	}
}

func TestCommandArgsIgnoreExternalConfigAndPlugins(t *testing.T) {
	args := strings.Join(YtdlConfig{}.CommandArgs(), " ")
	if !strings.Contains(args, "--ignore-config") || !strings.Contains(args, "--no-plugin-dirs") {
		t.Fatalf("boundary hermético ausente em %q", args)
	}
}

func TestCommandArgsAllowsOnlyDetectedManagedPluginDirectory(t *testing.T) {
	managedRoot := t.TempDir()
	pluginPayload := filepath.Join(managedRoot, "provider", "plugin")
	pluginFile := filepath.Join(pluginPayload, "yt_dlp_plugins", "extractor", "getpot_bgutil.py")
	if err := os.MkdirAll(filepath.Dir(pluginFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginFile, []byte("# fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalDirs := potPluginDirs
	potPluginDirs = func() []string { return []string{pluginPayload} }
	t.Cleanup(func() { potPluginDirs = originalDirs })

	args := strings.Join(YtdlConfig{POTProvider: "/runtime/provider"}.CommandArgs(), " ")
	if !strings.Contains(args, "--plugin-dirs "+filepath.Dir(pluginPayload)) {
		t.Fatalf("diretório gerenciado não foi liberado: %q", args)
	}
}

func TestCommandArgsRejectsRemoteHTTPPOTProvider(t *testing.T) {
	cfg := YtdlConfig{POTProvider: "https://tokens.example.invalid", POTMode: "http"}
	if args := strings.Join(cfg.CommandArgs(), " "); strings.Contains(args, "tokens.example.invalid") {
		t.Fatalf("provider POT HTTP remoto não pode receber dados de attestation: %q", args)
	}
}

// NFR-SEC-001: nada sensível em log/diagnóstico.
func TestRedactedHidesCookieLocation(t *testing.T) {
	cfg := YtdlConfig{CookiesFile: "/home/raillen/segredo/cookies.txt"}
	got := cfg.Redacted()
	if strings.Contains(got, "segredo") || strings.Contains(got, "cookies.txt") {
		t.Fatalf("caminho de cookies vazou no diagnóstico: %q", got)
	}
	if !strings.Contains(got, "cookies=arquivo") {
		t.Errorf("deveria indicar que há cookies de arquivo: %q", got)
	}
}
