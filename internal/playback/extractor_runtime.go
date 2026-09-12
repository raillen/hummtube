// Package playback implements extractor runtime management, playback resolvers
// and failure classification (docs/03-implementation/YOUTUBE_PLAYBACK_MODERNIZATION.md).
package playback

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ComponentStatus describes the detection state of a playback runtime dependency.
type ComponentStatus struct {
	Found   bool   `json:"found"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Source  string `json:"source,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Details string `json:"details,omitempty"`
}

// ExtractorRuntime gathers the concrete binaries and plugins used for playback extraction.
type ExtractorRuntime struct {
	YtDlp       ComponentStatus `json:"ytdlp"`
	JSRuntime   ComponentStatus `json:"js_runtime"`
	EJS         ComponentStatus `json:"ejs"`
	POTProvider ComponentStatus `json:"pot_provider"`

	Config YtdlConfig `json:"config"`
}

// DetectExtractorRuntime inspects the environment and file system to assemble
// the active extractor runtime.
func DetectExtractorRuntime(ctx context.Context, cfg YtdlConfig) ExtractorRuntime {
	rt := ExtractorRuntime{
		Config: cfg,
	}

	// 1. Locate yt-dlp binary
	rt.YtDlp = detectYtDlp(ctx, cfg.YtDlpPath)

	// 2. Locate JS runtime
	rt.JSRuntime = detectJSRuntimeStatus(ctx, cfg.JSRuntime)

	// 3. Locate EJS capability
	rt.EJS = detectEJS(cfg)

	// 4. Locate PO Token Provider
	rt.POTProvider = detectPOTProvider(cfg)

	return rt
}

func detectYtDlp(ctx context.Context, explicitPath string) ComponentStatus {
	st := ComponentStatus{Name: "yt-dlp"}
	resolvedPath, source, err := resolveYtDlpBinary(explicitPath)
	if err != nil {
		st.Found = false
		st.Details = "yt-dlp não encontrado no PATH ou locais gerenciados"
		return st
	}

	st.Found = true
	st.Source = source
	st.Details = resolvedPath

	// Probe version with short timeout
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(probeCtx, resolvedPath, "--version").Output()
	if err == nil {
		st.Version = strings.TrimSpace(string(out))
	}

	return st
}

func resolveYtDlpBinary(explicitPath string) (string, string, error) {
	type candidate struct {
		path   string
		source string
	}
	candidates := make([]candidate, 0, 4)
	if explicitPath != "" {
		candidates = append(candidates, candidate{path: explicitPath, source: "custom"})
	} else {
		if managedPath, err := managedRuntimeComponentPath("yt-dlp"); err == nil {
			candidates = append(candidates, candidate{path: managedPath, source: "managed-active"})
		}
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates,
				candidate{
					path:   filepath.Join(home, ".local", "share", "hummtube", "runtime", "yt-dlp"),
					source: "managed",
				},
				candidate{
					path:   filepath.Join(home, ".local", "share", "nanotube", "runtime", "yt-dlp"),
					source: "managed",
				},
			)
		}
		candidates = append(candidates,
			candidate{path: "/usr/lib/hummtube/yt-dlp", source: "system-package"},
			candidate{path: "/usr/lib/nanotube/yt-dlp", source: "system-package"},
		)
		if path, err := findExecutable("yt-dlp"); err == nil {
			candidates = append(candidates, candidate{path: path, source: "system"})
		}
	}

	for _, candidate := range candidates {
		resolved, err := filepath.EvalSymlinks(candidate.path)
		if err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return resolved, candidate.source, nil
		}
	}
	return "", "", exec.ErrNotFound
}

func detectJSRuntimeStatus(ctx context.Context, js JSRuntime) ComponentStatus {
	if !js.Found() {
		return ComponentStatus{
			Found:   false,
			Details: "nenhum runtime JS (deno/node/quickjs/bun) detectado",
		}
	}

	st := ComponentStatus{
		Found:   true,
		Name:    js.Name,
		Details: js.Path,
	}

	// Probe version
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch js.Name {
	case "deno":
		cmd = exec.CommandContext(probeCtx, js.Path, "--version")
	case "node":
		cmd = exec.CommandContext(probeCtx, js.Path, "--version")
	case "bun":
		cmd = exec.CommandContext(probeCtx, js.Path, "--version")
	case "quickjs":
		cmd = exec.CommandContext(probeCtx, js.Path, "-h")
	default:
		cmd = exec.CommandContext(probeCtx, js.Path, "--version")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		st.Found = false
		st.Details = "runtime JS configurado, mas não executou corretamente"
		return st
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || !validJSRuntimeVersion(js.Name, lines[0]) {
		st.Found = false
		st.Details = "runtime JS configurado, mas não devolveu uma versão reconhecível"
		return st
	}
	st.Version = strings.TrimSpace(lines[0])

	return st
}

func validJSRuntimeVersion(name, output string) bool {
	output = strings.ToLower(strings.TrimSpace(output))
	switch name {
	case "deno":
		return strings.HasPrefix(output, "deno ")
	case "node":
		return strings.HasPrefix(output, "v")
	case "bun":
		return output != ""
	case "quickjs":
		return output != ""
	default:
		return false
	}
}

func potProviderCandidateAvailable(cfg YtdlConfig) bool {
	return detectPOTProvider(cfg).Found && strings.TrimSpace(cfg.POTProvider) != ""
}

func detectEJS(cfg YtdlConfig) ComponentStatus {
	st := ComponentStatus{
		Name: "ejs",
	}

	if cfg.AllowRemoteComponents {
		st.Found = true
		st.Source = "remote"
		st.Details = "remote-components=ejs:github habilitado"
		return st
	}

	// Check if bundled in plugin / system dirs
	candidates := standardPluginDirs()
	for _, dir := range candidates {
		ejsPath := filepath.Join(dir, "ejs")
		if _, err := os.Stat(ejsPath); err == nil {
			st.Found = true
			st.Source = "bundled"
			st.Details = ejsPath
			return st
		}
	}

	st.Found = false
	st.Source = "none"
	st.Details = "desabilitado (ative em Ajustes → Sistema → Reprodução e qualidade ou instale pacote EJS compatível)"
	return st
}

func detectPOTProvider(cfg YtdlConfig) ComponentStatus {
	st := ComponentStatus{
		Name: "bgutil-pot",
		Mode: "script",
	}

	if cfg.POTMode != "" {
		st.Mode = cfg.POTMode
	}

	if cfg.POTProvider != "" {
		st.Name = "bgutil"
		st.Source = "config-unverified"
		st.Details = "endpoint/diretório configurado; plugin ainda precisa ser descoberto pelo yt-dlp"
	}

	// Look for provider plugin in plugin directories
	for _, dir := range potPluginDirs() {
		for _, entry := range potPluginEntries(dir) {
			if _, err := os.Stat(entry); err == nil {
				st.Found = true
				st.Name = "bgutil-ytdlp-pot-provider"
				st.Source = "plugin"
				st.Details = entry
				return st
			}
		}
	}

	st.Found = false
	if cfg.POTProvider == "" {
		st.Name = "none"
		st.Source = "none"
		st.Details = "provider não detectado; o modo automático prioriza cliente móvel"
	} else {
		st.Details = "provider configurado, mas o plugin do yt-dlp não foi detectado"
	}
	return st
}

var potPluginDirs = managedPluginDirs

func managedPluginDirs() []string {
	dirs := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		runtimeRoot := filepath.Join(home, ".local", "share", "nanotube", "runtime")
		dirs = append(dirs,
			filepath.Join(runtimeRoot, "bgutil-ytdlp-pot-provider", "plugin"),
			filepath.Join(runtimeRoot, "bgutil", "plugin"),
			filepath.Join(runtimeRoot, "plugins"),
		)
	}
	return append(dirs, "/usr/lib/nanotube/plugins")
}

func configuredPOTPluginDir(cfg YtdlConfig) string {
	if strings.TrimSpace(cfg.POTProvider) == "" {
		return ""
	}
	for _, dir := range potPluginDirs() {
		if hasPOTPlugin(dir) {
			// yt-dlp recebe a pasta que contém o diretório do plugin. O layout
			// gerenciado é `<provider>/plugin/yt_dlp_plugins`; portanto passar
			// `<provider>/plugin` faria o loader procurar um nível abaixo e o
			// provider apareceria como indisponível.
			return filepath.Dir(dir)
		}
	}
	return ""
}

func hasPOTPlugin(dir string) bool {
	for _, entry := range potPluginEntries(dir) {
		info, err := os.Stat(entry)
		if err == nil && (info.Mode().IsRegular() || info.IsDir()) {
			return true
		}
	}
	return false
}

func potPluginEntries(dir string) []string {
	return []string{
		filepath.Join(dir, "yt-dlp-plugins", "extractor", "pot.py"),
		filepath.Join(dir, "yt-dlp-plugins", "yt_dlp_plugins", "extractor", "pot.py"),
		filepath.Join(dir, "yt_dlp_plugins", "extractor", "getpot_bgutil.py"),
		filepath.Join(dir, "yt_dlp_plugins", "extractor", "getpot_bgutil_http.py"),
		filepath.Join(dir, "yt_dlp_plugins", "extractor", "getpot_bgutil_script.py"),
		filepath.Join(dir, "bgutil_ytdlp_pot_provider"),
	}
}

func standardPluginDirs() []string {
	dirs := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			filepath.Join(home, ".config", "yt-dlp", "plugins"),
			filepath.Join(home, ".local", "share", "nanotube", "runtime", "plugins"),
			filepath.Join(home, ".local", "share", "yt-dlp", "plugins"),
		)
	}
	dirs = append(dirs,
		"/usr/lib/nanotube/plugins",
		"/etc/yt-dlp/plugins",
	)
	return dirs
}
