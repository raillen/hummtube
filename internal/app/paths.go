// Package app wires infrastructure paths and lifecycle for the NanoTube
// desktop application (XDG dirs, cache locations).
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

const defaultNamespace = "nanotube-web"

var namespacePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

var namespaceState = struct {
	sync.RWMutex
	value string
}{value: defaultNamespace}

// ConfigureNamespace define o namespace XDG usado pelo processo atual. Deve
// ser chamado pelo launcher antes de inicializar serviços concorrentes.
func ConfigureNamespace(namespace string) error {
	if !namespacePattern.MatchString(namespace) {
		return fmt.Errorf("namespace de produto inválido: %q", namespace)
	}
	namespaceState.Lock()
	namespaceState.value = namespace
	namespaceState.Unlock()
	return nil
}

// CurrentNamespace retorna o namespace XDG ativo do processo.
func CurrentNamespace() string {
	namespaceState.RLock()
	defer namespaceState.RUnlock()
	return namespaceState.value
}

// CacheDir returns the base cache directory for NanoTube following the
// XDG Base Directory specification, creating it if needed.
func CacheDir() (string, error) {
	return CacheDirFor(CurrentNamespace())
}

// CacheDirFor retorna um diretório de cache isolado para um produto da suíte.
func CacheDirFor(namespace string) (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".cache")
	}
	dir, err := scopedDir(base, namespace)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ThumbnailsDir returns the directory holding compressed thumbnail files.
func ThumbnailsDir() (string, error) {
	base, err := CacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "thumbnails")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ConfigDir returns the base configuration directory following the XDG Base
// Directory specification, creating it if needed. É onde o app procura as
// credenciais OAuth quando nenhuma variável de ambiente aponta para elas.
func ConfigDir() (string, error) {
	return ConfigDirFor(CurrentNamespace())
}

// ConfigDirFor retorna um diretório de configuração isolado para um produto.
func ConfigDirFor(namespace string) (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	dir, err := scopedDir(base, namespace)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// DataDir returns the base data directory for NanoTube following the XDG
// Base Directory specification, creating it if needed. It holds the local
// SQLite database (metadata/state; never secrets).
func DataDir() (string, error) {
	return DataDirFor(CurrentNamespace())
}

// DataDirFor retorna um diretório de dados isolado para um produto.
func DataDirFor(namespace string) (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	dir, err := scopedDir(base, namespace)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// StateDir returns the base state directory for NanoTube following the XDG
// Base Directory specification (~/.local/state/nanotube-web), creating it if needed.
func StateDir() (string, error) {
	return StateDirFor(CurrentNamespace())
}

// StateDirFor retorna um diretório de estado isolado para um produto.
func StateDirFor(namespace string) (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir, err := scopedDir(base, namespace)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// LogFilePath returns the path to the main application log file.
func LogFilePath() (string, error) {
	return LogFilePathFor(CurrentNamespace())
}

// LogFilePathFor retorna o arquivo de log isolado para um produto.
func LogFilePathFor(namespace string) (string, error) {
	dir, err := StateDirFor(namespace)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, namespace+".log"), nil
}

func scopedDir(base, namespace string) (string, error) {
	if !namespacePattern.MatchString(namespace) {
		return "", fmt.Errorf("namespace de produto inválido: %q", namespace)
	}
	return filepath.Join(base, namespace), nil
}
