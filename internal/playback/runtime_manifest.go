package playback

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RuntimeManifest describes the playback components that are allowed to be
// used by a packaged installation. It contains metadata and hashes only; it
// never carries cookies, API keys or arbitrary executable arguments.
type RuntimeManifest struct {
	SchemaVersion string           `json:"schema_version"`
	Version       string           `json:"version"`
	YtDlp         RuntimeComponent `json:"yt_dlp"`
	JSRuntime     RuntimeComponent `json:"js_runtime"`
	EJS           RuntimeComponent `json:"ejs"`
	POTProvider   RuntimeComponent `json:"pot_provider"`
	Licenses      []RuntimeLicense `json:"licenses,omitempty"`
}

type RuntimeComponent struct {
	Version string `json:"version"`
	Path    string `json:"path,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Source  string `json:"source,omitempty"`
}

type RuntimeLicense struct {
	Name    string `json:"name"`
	License string `json:"license"`
}

func LoadRuntimeManifest(path string) (RuntimeManifest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return RuntimeManifest{}, errors.New("manifesto de playback ausente")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return RuntimeManifest{}, fmt.Errorf("ler manifesto de playback: %w", err)
	}
	var manifest RuntimeManifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return RuntimeManifest{}, fmt.Errorf("decodificar manifesto de playback: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return RuntimeManifest{}, err
	}
	return manifest, nil
}

func (m RuntimeManifest) Validate() error {
	if m.SchemaVersion != "1" {
		return fmt.Errorf("manifesto de playback: schema não suportado")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("manifesto de playback: versão ausente")
	}
	for name, component := range map[string]RuntimeComponent{
		"yt-dlp": m.YtDlp, "runtime JS": m.JSRuntime,
	} {
		if strings.TrimSpace(component.Version) == "" {
			return fmt.Errorf("manifesto de playback: versão de %s ausente", name)
		}
		if component.SHA256 != "" {
			if len(component.SHA256) != sha256.Size*2 {
				return fmt.Errorf("manifesto de playback: hash de %s inválido", name)
			}
			if _, err := hex.DecodeString(component.SHA256); err != nil {
				return fmt.Errorf("manifesto de playback: hash de %s inválido", name)
			}
		}
		if component.Path != "" {
			clean := filepath.Clean(component.Path)
			if filepath.IsAbs(component.Path) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("manifesto de playback: caminho de %s inválido", name)
			}
		}
	}
	for name, component := range map[string]RuntimeComponent{
		"EJS": m.EJS, "PO Token Provider": m.POTProvider,
	} {
		if component.Path != "" || component.SHA256 != "" {
			if strings.TrimSpace(component.Version) == "" {
				return fmt.Errorf("manifesto de playback: versão de %s ausente", name)
			}
		}
	}
	return nil
}

func (m RuntimeManifest) VerifyComponent(name, path string) error {
	var component RuntimeComponent
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "yt-dlp", "ytdlp":
		component = m.YtDlp
	case "js", "js_runtime", "runtime js":
		component = m.JSRuntime
	case "ejs":
		component = m.EJS
	case "pot", "pot_provider":
		component = m.POTProvider
	default:
		return fmt.Errorf("manifesto de playback: componente desconhecido")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("verificar %s: %w", name, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("verificar %s: arquivo não é regular", name)
	}
	if component.SHA256 == "" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("verificar %s: %w", name, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("calcular hash de %s: %w", name, err)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), component.SHA256) {
		return fmt.Errorf("hash de %s não corresponde ao manifesto", name)
	}
	return nil
}
