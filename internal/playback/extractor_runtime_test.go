package playback

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectExtractorRuntime(t *testing.T) {
	cfg := YtdlConfig{
		PlayerClient: "mweb",
	}
	rt := DetectExtractorRuntime(context.Background(), cfg)

	if rt.Config.PlayerClient != "mweb" {
		t.Errorf("expected player client mweb, got %s", rt.Config.PlayerClient)
	}
}

func TestDetectJSRuntimeStatusEmpty(t *testing.T) {
	st := detectJSRuntimeStatus(context.Background(), JSRuntime{})
	if st.Found {
		t.Errorf("expected not found for empty JSRuntime")
	}
}

func TestResolveJSRuntimeRejectsUnavailableExplicitRuntime(t *testing.T) {
	if path, err := exec.LookPath("false"); err == nil {
		status := detectJSRuntimeStatus(context.Background(), JSRuntime{Name: "node", Path: path})
		if status.Found {
			t.Fatalf("runtime que falha foi declarado funcional: %+v", status)
		}
	}
	if path, err := exec.LookPath("true"); err == nil {
		status := detectJSRuntimeStatus(context.Background(), JSRuntime{Name: "node", Path: path})
		if status.Found {
			t.Fatalf("binário sem versão foi declarado runtime Node funcional: %+v", status)
		}
	}
	if runtime := resolveJSRuntime("unsupported"); runtime.Found() {
		t.Fatalf("runtime não suportado foi aceito: %+v", runtime)
	}
}

func TestDetectEJS(t *testing.T) {
	cfgWithout := YtdlConfig{AllowRemoteComponents: false}
	stWithout := detectEJS(cfgWithout)
	if stWithout.Source == "remote" {
		t.Errorf("expected non-remote EJS without flag")
	}

	cfgWith := YtdlConfig{AllowRemoteComponents: true}
	stWith := detectEJS(cfgWith)
	if !stWith.Found || stWith.Source != "remote" {
		t.Errorf("expected remote EJS with flag, got %+v", stWith)
	}
	if !strings.Contains(stWith.Details, "remote-components=ejs:github") {
		t.Errorf("diagnóstico remoto impreciso: %+v", stWith)
	}
}

func TestDetectPOTProviderDoesNotTrustConfiguredName(t *testing.T) {
	originalDirs := potPluginDirs
	potPluginDirs = func() []string { return []string{t.TempDir()} }
	t.Cleanup(func() { potPluginDirs = originalDirs })
	cfg := YtdlConfig{
		POTProvider: "bgutil-custom",
		POTMode:     "script",
	}
	st := detectPOTProvider(cfg)
	if st.Found || st.Name != "bgutil" || st.Mode != "script" || st.Source != "config-unverified" {
		t.Errorf("nome configurado não prova instalação do provider: %+v", st)
	}
	if potProviderCandidateAvailable(cfg) {
		t.Fatal("configuração sem plugin não pode virar candidato")
	}
}

func TestDetectPOTProviderDoesNotTreatEndpointAsVerifiedPlugin(t *testing.T) {
	originalDirs := potPluginDirs
	potPluginDirs = func() []string { return []string{t.TempDir()} }
	t.Cleanup(func() { potPluginDirs = originalDirs })
	st := detectPOTProvider(YtdlConfig{POTProvider: "http://127.0.0.1:4416", POTMode: "http"})
	if st.Found || st.Source != "config-unverified" {
		t.Fatalf("endpoint não comprova que o plugin está carregado: %+v", st)
	}
}

func TestDetectPOTProviderSeparatesFoundFromConfiguredCandidate(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "yt_dlp_plugins", "extractor")
	if err := os.MkdirAll(pluginPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginPath, "getpot_bgutil.py"), []byte("# test"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalDirs := potPluginDirs
	potPluginDirs = func() []string { return []string{root} }
	t.Cleanup(func() { potPluginDirs = originalDirs })

	withoutConfig := detectPOTProvider(YtdlConfig{})
	if !withoutConfig.Found || potProviderCandidateAvailable(YtdlConfig{}) {
		t.Fatalf("plugin encontrado sem configuração não deveria ser candidato: %+v", withoutConfig)
	}
	withConfig := YtdlConfig{POTProvider: "/opt/bgutil/server"}
	withCandidate := detectPOTProvider(withConfig)
	if !withCandidate.Found || !potProviderCandidateAvailable(withConfig) {
		t.Fatalf("plugin encontrado com configuração deveria ser candidato: %+v", withCandidate)
	}
}

func TestManagedPluginDirsIncludeCurrentAndLegacyBgutilLayouts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dirs := strings.Join(managedPluginDirs(), "\n")
	for _, relative := range []string{
		filepath.Join(".local", "share", "nanotube", "runtime", "bgutil-ytdlp-pot-provider", "plugin"),
		filepath.Join(".local", "share", "nanotube", "runtime", "bgutil", "plugin"),
	} {
		if !strings.Contains(dirs, filepath.Join(home, relative)) {
			t.Fatalf("layout gerenciado ausente: %s em %s", relative, dirs)
		}
	}
}

func TestRedactedHidesAllSensitiveData(t *testing.T) {
	cfg := YtdlConfig{
		JSRuntime:             JSRuntime{Name: "deno", Path: "/usr/bin/deno"},
		PlayerClient:          "mweb",
		POTProvider:           "bgutil",
		CookiesFile:           "/home/user/super_secret_cookies.txt",
		CookiesFromBrowser:    "",
		AllowRemoteComponents: true,
		MaxHeight:             1080,
	}

	redacted := cfg.Redacted()

	if strings.Contains(redacted, "super_secret_cookies.txt") {
		t.Fatalf("cookie path leaked in redacted output: %s", redacted)
	}
	if !strings.Contains(redacted, "cookies=arquivo") {
		t.Errorf("expected cookies=arquivo in %s", redacted)
	}
	if !strings.Contains(redacted, "js-runtime=deno") {
		t.Errorf("expected js-runtime=deno in %s", redacted)
	}
	if !strings.Contains(redacted, "player-client=mweb") {
		t.Errorf("expected player-client=mweb in %s", redacted)
	}
	if strings.Contains(redacted, "bgutil") || !strings.Contains(redacted, "pot-provider=configurado") {
		t.Errorf("provider deveria ser indicado sem expor valor/caminho: %s", redacted)
	}
	if !strings.Contains(redacted, "max-height=1080") {
		t.Errorf("expected max-height=1080 in %s", redacted)
	}
}
