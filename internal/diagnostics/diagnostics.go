// Package diagnostics implementa o relatório de diagnóstico do sistema para o NanoTube Web.
package diagnostics

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/app"
	"github.com/nanotube/nanotube-web/internal/playback"
	"github.com/nanotube/nanotube-web/internal/storage"
	"github.com/nanotube/nanotube-web/internal/telemetry"
)

// Version é injetada em tempo de compilação via -ldflags.
var Version = "0.1.0-dev"

// Report armazena informações de diagnóstico sanitizadas.
type Report struct {
	Version          string             `json:"version"`
	Commit           string             `json:"commit,omitempty"`
	GoVersion        string             `json:"go_version"`
	BuildMode        string             `json:"build_mode"`
	Display          string             `json:"display"`
	Session          string             `json:"session"`
	Framework        string             `json:"framework"`
	ResolverStrategy string             `json:"resolver_strategy"`
	YtdlpPath        string             `json:"ytdlp_path"`
	YtdlpVersion     string             `json:"ytdlp_version"`
	YtdlpSource      string             `json:"ytdlp_source"`
	JSRuntime        string             `json:"js_runtime"`
	EJSStatus        string             `json:"ejs_status"`
	POTProvider      string             `json:"pot_provider"`
	POTMode          string             `json:"pot_mode"`
	PlaybackManifest string             `json:"playback_manifest"`
	PlayerClient     string             `json:"player_client"`
	ExtractionConfig string             `json:"extraction_config"`
	ExtractionHint   string             `json:"extraction_hint,omitempty"`
	SQLiteVersion    string             `json:"sqlite_version"`
	FTS5             string             `json:"fts5"`
	MigrationVersion int64              `json:"migration_version"`
	Keyring          string             `json:"keyring"`
	ThumbCacheDir    string             `json:"thumb_cache_dir"`
	RSSBytes         int64              `json:"rss_bytes"`
	Metrics          telemetry.Snapshot `json:"metrics"`
	Errors           []string           `json:"errors,omitempty"`
	Warnings         []string           `json:"warnings,omitempty"`
	LogFile          string             `json:"log_file,omitempty"`
	RecentLogs       []string           `json:"recent_logs,omitempty"`
}

// Collect reúne o relatório de diagnósticos do sistema.
func Collect(ctx context.Context) *Report {
	r := &Report{
		Version:          Version,
		GoVersion:        runtime.Version(),
		Display:          os.Getenv("DISPLAY"),
		Session:          os.Getenv("XDG_SESSION_TYPE"),
		Framework:        "Wails v3 + Svelte 4 + Tailwind CSS",
		BuildMode:        buildMode(),
		ResolverStrategy: "cascading (yt-dlp explícito -> Invidious)",
		PlaybackManifest: "não configurado",
	}
	if r.Display == "" {
		r.Display = "(sem DISPLAY / Wayland nativo)"
	}

	probeExtraction(ctx, r)

	if err := probeStorage(ctx, r); err != nil {
		r.Errors = append(r.Errors, "storage: "+err.Error())
	}

	r.Keyring = storage.KeyringProbe(ctx)

	if dir, err := app.ThumbnailsDir(); err == nil {
		r.ThumbCacheDir = dir
	}

	if rss, err := RSSBytes(); err == nil {
		r.RSSBytes = rss
	} else {
		r.Errors = append(r.Errors, "rss: "+err.Error())
	}
	if logPath, err := app.LogFilePath(); err == nil {
		r.LogFile = logPath
		r.RecentLogs = recentLogLines(logPath, 80)
	}
	r.Metrics = telemetry.Process().Snapshot()

	return r
}

var (
	logURLQueryPattern = regexp.MustCompile(`(?i)(https?://[^?\s]+)\?[^\s]+`)
	logSecretPattern   = regexp.MustCompile(`(?i)(token|password|secret|cookie|authorization|api[_-]?key)=([^\s&]+)`)
)

func recentLogLines(path string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		line := sanitizeLogLine(scanner.Text())
		if line == "" {
			continue
		}
		if len(lines) == limit {
			copy(lines, lines[1:])
			lines[len(lines)-1] = line
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func sanitizeLogLine(line string) string {
	line = sanitize(line, 2000)
	line = logURLQueryPattern.ReplaceAllString(line, "$1?<redacted>")
	return logSecretPattern.ReplaceAllString(line, "$1=<redacted>")
}

func probeExtraction(ctx context.Context, r *Report) {
	cfg := playback.LoadYtdlConfig()
	cfg = loadPersistedExtractionConfig(ctx, cfg)
	r.ExtractionConfig = cfg.Redacted()
	r.PlayerClient = cfg.PlayerClient
	if manifestPath := strings.TrimSpace(os.Getenv("NANOTUBE_PLAYBACK_MANIFEST")); manifestPath != "" {
		if _, err := playback.LoadRuntimeManifest(manifestPath); err != nil {
			r.PlaybackManifest = "inválido"
			r.Warnings = append(r.Warnings, "manifesto de playback rejeitado: "+sanitize(err.Error(), 120))
		} else {
			r.PlaybackManifest = "válido"
		}
	}

	rt := playback.DetectExtractorRuntime(ctx, cfg)

	if rt.YtDlp.Found {
		r.YtdlpPath = rt.YtDlp.Details
		r.YtdlpVersion = rt.YtDlp.Version
		r.YtdlpSource = rt.YtDlp.Source
		if stale, age := staleYtdlp(rt.YtDlp.Version); stale {
			r.Warnings = append(r.Warnings, fmt.Sprintf(
				"yt-dlp %s tem ~%d meses: atualize com `pipx install yt-dlp` ou pelo gerenciador de pacotes",
				rt.YtDlp.Version, int(age.Hours()/24/30)))
		}
	} else {
		r.YtdlpPath = "ausente"
		r.Errors = append(r.Errors, "yt-dlp: "+rt.YtDlp.Details)
	}

	if rt.JSRuntime.Found {
		if rt.JSRuntime.Version != "" {
			r.JSRuntime = fmt.Sprintf("%s %s (%s)", rt.JSRuntime.Name, rt.JSRuntime.Version, rt.JSRuntime.Details)
		} else {
			r.JSRuntime = fmt.Sprintf("%s (%s)", rt.JSRuntime.Name, rt.JSRuntime.Details)
		}
	} else {
		r.JSRuntime = "ausente"
		r.ExtractionHint = "Runtime JS não encontrado. Deno ou Node são recomendados para desafios YouTube."
		r.Warnings = append(r.Warnings, "extração: "+r.ExtractionHint)
	}

	r.EJSStatus = fmt.Sprintf("%s (%s)", rt.EJS.Source, rt.EJS.Details)

	if rt.POTProvider.Found {
		r.POTProvider = fmt.Sprintf("%s (%s)", rt.POTProvider.Name, rt.POTProvider.Source)
		r.POTMode = rt.POTProvider.Mode
	} else {
		r.POTProvider = "ausente (modo automático usa cliente android)"
		r.POTMode = "none"
	}
}

func loadPersistedExtractionConfig(ctx context.Context, cfg playback.YtdlConfig) playback.YtdlConfig {
	dataDir, err := app.DataDir()
	if err != nil {
		return cfg
	}
	dbPath := filepath.Join(dataDir, "nanotube-web.db")
	if _, err := os.Stat(dbPath); err != nil {
		return cfg
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return cfg
	}
	defer db.Close()
	repo := storage.NewRepository(db)
	profileID := storage.DefaultProfileID
	if active, activeErr := repo.ActiveProfileID(ctx); activeErr == nil && active != "" {
		profileID = active
	}

	if os.Getenv(playback.EnvRemoteEJS) == "" {
		if value, ok, err := repo.Setting(ctx, storage.SettingRemoteEJS); err == nil && ok {
			cfg.AllowRemoteComponents = value == "1"
		}
	}
	if os.Getenv(playback.EnvPlayerClient) == "" {
		if value, ok, err := repo.Setting(ctx, storage.SettingPlayerClient); err == nil && ok {
			cfg.PlayerClient = playback.NormalizePlayerClientPreference(value)
		}
	}
	if os.Getenv(playback.EnvCookiesFrom) == "" {
		if profile, profileErr := repo.Profile(ctx, profileID); profileErr == nil && !profile.IsGuest() {
			if value, ok, err := repo.ProfileSetting(ctx, profileID, storage.SettingCookiesFrom); err == nil && ok {
				if normalized, normalizeErr := playback.NormalizeBrowserCookieSource(value); normalizeErr == nil {
					cfg.CookiesFromBrowser = normalized
				}
			}
		}
	}
	return cfg
}

func buildMode() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, s := range info.Settings {
		if s.Key == "-buildmode" {
			return s.Value
		}
	}
	return "default"
}

func probeStorage(ctx context.Context, r *Report) error {
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		return err
	}
	r.MigrationVersion, err = storage.MigrationVersion(db)
	if err != nil {
		return err
	}
	r.SQLiteVersion, err = storage.SQLiteVersion(db)
	if err != nil {
		return err
	}
	r.FTS5 = storage.FTS5Probe(db)
	return nil
}

func (r *Report) Print(w io.Writer) {
	fmt.Fprintf(w, "NanoTube Web diagnostics (v%s)\n", r.Version)
	fmt.Fprintf(w, "  stack:       %s\n", r.Framework)
	fmt.Fprintf(w, "  go:          %s (buildmode %s)\n", r.GoVersion, r.BuildMode)
	fmt.Fprintf(w, "  display:     %s (%s)\n", r.Display, r.Session)
	fmt.Fprintf(w, "  resolver:    %s\n", r.ResolverStrategy)
	fmt.Fprintf(w, "  yt-dlp:      %s %s (origem: %s)\n", r.YtdlpPath, r.YtdlpVersion, r.YtdlpSource)
	fmt.Fprintf(w, "  js runtime:  %s\n", r.JSRuntime)
	fmt.Fprintf(w, "  EJS:         %s\n", r.EJSStatus)
	fmt.Fprintf(w, "  PO Token:    %s (modo: %s)\n", r.POTProvider, r.POTMode)
	fmt.Fprintf(w, "  manifesto:   %s\n", r.PlaybackManifest)
	fmt.Fprintf(w, "  extração:    %s\n", r.ExtractionConfig)
	fmt.Fprintf(w, "  sqlite:      %s (migration %d, FTS5: %s)\n", r.SQLiteVersion, r.MigrationVersion, r.FTS5)
	fmt.Fprintf(w, "  keyring:     %s\n", r.Keyring)
	fmt.Fprintf(w, "  rss:         %s\n", humanBytes(r.RSSBytes))
	fmt.Fprintf(w, "  métricas:    playback=%d (cache=%d, erros=%d, picos=%dµs), busca=%d (erros=%d, picos=%dµs)\n",
		r.Metrics.PlaybackRequests, r.Metrics.PlaybackCacheHits, r.Metrics.PlaybackErrors,
		r.Metrics.PlaybackLatencyMaxUS, r.Metrics.SearchRequests, r.Metrics.SearchErrors,
		r.Metrics.SearchLatencyMaxUS)
	if len(r.Warnings) > 0 {
		fmt.Fprintf(w, "  avisos:\n")
		for _, warn := range r.Warnings {
			fmt.Fprintf(w, "    - %s\n", sanitize(warn, 200))
		}
	}
	if len(r.Errors) > 0 {
		fmt.Fprintf(w, "  erros:\n")
		for _, e := range r.Errors {
			fmt.Fprintf(w, "    - %s\n", sanitize(e, 200))
		}
	}
}

func staleYtdlp(version string) (bool, time.Duration) {
	parts := strings.SplitN(version, ".", 4)
	if len(parts) < 3 {
		return false, 0
	}
	date := parts[0] + "-" + parts[1] + "-" + parts[2]
	release, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false, 0
	}
	age := time.Since(release)
	return age > 182*24*time.Hour, age
}

func humanBytes(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func sanitize(s string, limit int) string {
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

func RSSBytes() (int64, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "VmRSS:" {
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return kb * 1024, nil
		}
	}
	return 0, fmt.Errorf("VmRSS ausente em /proc/self/status")
}
