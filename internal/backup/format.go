// Package backup implements the .ntbackup container (M6 BCK-01/02).
// Threat model: zip-slip/progbox salvo em ~/.commandcode/plans/gain-auth-wizard-subscriptions.md.
package backup

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

const (
	CurrentVersion = 1
	Magic          = "NTBACKUP"
	MaxEntries     = 50000
	MaxJSONBytes   = 32 << 20 // 32 MiB por seção
)

type Manifest struct {
	Magic     string            `json:"magic"`
	Version   int               `json:"version"`
	CreatedAt string            `json:"created_at"`
	Sections  []string          `json:"sections"`
	Checksums map[string]string `json:"checksums"`
}

type Export struct {
	Version  int                        `json:"version"`
	Sections map[string]json.RawMessage `json:"sections"`
}

func NewManifest(sections []string, checksums map[string]string) Manifest {
	return Manifest{
		Magic:     Magic,
		Version:   CurrentVersion,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Sections:  sections,
		Checksums: checksums,
	}
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Write serializa seções em um zip versionado com manifesto e checksums.
func Write(w io.Writer, sections map[string]json.RawMessage) error {
	if len(sections) > MaxEntries {
		return fmt.Errorf("muitas seções: %d", len(sections))
	}
	zw := zip.NewWriter(w)
	defer zw.Close()

	checksums := make(map[string]string, len(sections))
	for name, data := range sections {
		if len(data) > MaxJSONBytes {
			return fmt.Errorf("seção %q excede %d bytes", name, MaxJSONBytes)
		}
		if err := validateEntryName(name); err != nil {
			return err
		}
		checksums[name] = checksum(data)
		f, err := zw.Create(name + ".json")
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
	}
	manifest := NewManifest(keys(sections), checksums)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar manifesto: %w", err)
	}
	f, err := zw.Create("manifest.json")
	if err != nil {
		return err
	}
	if _, err := f.Write(manifestBytes); err != nil {
		return err
	}
	return zw.Close()
}

// Read valida e desserializa um .ntbackup, rejeitando traversal e limites.
func Read(r io.ReaderAt, size int64) (map[string]json.RawMessage, Manifest, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, Manifest{}, fmt.Errorf("zip: %w", err)
	}
	if len(zr.File) > MaxEntries+1 {
		return nil, Manifest{}, fmt.Errorf("muitas entradas no zip")
	}
	raw := make(map[string][]byte, len(zr.File))
	var manifest Manifest
	var manifestFound bool
	for _, f := range zr.File {
		if err := validateEntryName(f.Name); err != nil && f.Name != "manifest.json" {
			return nil, Manifest{}, err
		}
		if f.UncompressedSize64 > MaxJSONBytes+1024 {
			return nil, Manifest{}, fmt.Errorf("entrada %q muito grande", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, Manifest{}, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, MaxJSONBytes+1024))
		rc.Close()
		if err != nil {
			return nil, Manifest{}, err
		}
		if f.Name == "manifest.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, Manifest{}, fmt.Errorf("manifesto inválido: %w", err)
			}
			manifestFound = true
		} else {
			name := strings.TrimSuffix(f.Name, ".json")
			raw[name] = data
		}
	}
	if !manifestFound {
		return nil, Manifest{}, fmt.Errorf("manifesto ausente")
	}
	if manifest.Magic != Magic {
		return nil, Manifest{}, fmt.Errorf("magic inválido: %q", manifest.Magic)
	}
	if manifest.Version > CurrentVersion {
		return nil, Manifest{}, fmt.Errorf("versão futura %d > %d", manifest.Version, CurrentVersion)
	}
	for name, want := range manifest.Checksums {
		got, ok := raw[name]
		if !ok {
			return nil, Manifest{}, fmt.Errorf("seção %q ausente", name)
		}
		if checksum(got) != want {
			return nil, Manifest{}, fmt.Errorf("checksum inválido para %q", name)
		}
	}
	out := make(map[string]json.RawMessage, len(raw))
	for k, v := range raw {
		cp := make([]byte, len(v))
		copy(cp, v)
		out[k] = json.RawMessage(cp)
	}
	return out, manifest, nil
}

func validateEntryName(name string) error {
	if name != filepath.Base(name) {
		return fmt.Errorf("path traversal: %q", name)
	}
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return fmt.Errorf("nome inválido: %q", name)
	}
	return nil
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
