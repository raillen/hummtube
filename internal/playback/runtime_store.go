package playback

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const activeRuntimeFile = "active.json"

// RuntimeManager instala componentes de playback previamente verificados em
// diretórios versionados e alterna a versão ativa com uma gravação atômica.
// Ele não baixa código nem aceita argumentos de execução.
type RuntimeManager struct {
	root string
}

type RuntimeInstallResult struct {
	Version string `json:"version"`
	Path    string `json:"path"`
}

type RuntimeStatus struct {
	ActiveVersion     string   `json:"active_version,omitempty"`
	PreviousVersion   string   `json:"previous_version,omitempty"`
	InstalledVersions []string `json:"installed_versions"`
}

type runtimeActivation struct {
	Version  string `json:"version"`
	Previous string `json:"previous,omitempty"`
}

func NewRuntimeManager(root string) (*RuntimeManager, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("diretório do runtime ausente")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolver diretório do runtime: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return nil, fmt.Errorf("criar diretório do runtime: %w", err)
	}
	return &RuntimeManager{root: filepath.Clean(abs)}, nil
}

// Install valida o manifesto e copia apenas os componentes declarados a
// partir de sourceDir. A instalação é publicada por rename de diretório,
// portanto uma falha não deixa uma versão parcialmente visível.
func (m *RuntimeManager) Install(manifestPath, sourceDir string) (RuntimeInstallResult, error) {
	if m == nil || m.root == "" {
		return RuntimeInstallResult{}, errors.New("gerenciador de runtime ausente")
	}
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		return RuntimeInstallResult{}, errors.New("origem do runtime ausente")
	}
	manifest, err := LoadRuntimeManifest(manifestPath)
	if err != nil {
		return RuntimeInstallResult{}, err
	}
	if err := validateRuntimeVersion(manifest.Version); err != nil {
		return RuntimeInstallResult{}, err
	}
	sourceRoot, err := filepath.Abs(sourceDir)
	if err != nil {
		return RuntimeInstallResult{}, fmt.Errorf("resolver origem do runtime: %w", err)
	}
	if info, err := os.Lstat(sourceRoot); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		if err == nil {
			err = errors.New("origem não é um diretório real")
		}
		return RuntimeInstallResult{}, fmt.Errorf("origem do runtime inválida: %w", err)
	}
	resolvedSourceRoot, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return RuntimeInstallResult{}, fmt.Errorf("resolver origem do runtime: %w", err)
	}

	finalPath := filepath.Join(m.root, manifest.Version)
	if _, err := os.Stat(finalPath); err == nil {
		return RuntimeInstallResult{}, fmt.Errorf("runtime %q já está instalado", manifest.Version)
	} else if !errors.Is(err, os.ErrNotExist) {
		return RuntimeInstallResult{}, fmt.Errorf("consultar runtime existente: %w", err)
	}

	token, err := randomRuntimeToken()
	if err != nil {
		return RuntimeInstallResult{}, err
	}
	stagePath := filepath.Join(m.root, ".staging-"+token)
	if err := os.Mkdir(stagePath, 0o700); err != nil {
		return RuntimeInstallResult{}, fmt.Errorf("criar staging do runtime: %w", err)
	}
	defer os.RemoveAll(stagePath)

	for _, component := range runtimeComponents(manifest) {
		if strings.TrimSpace(component.value.Path) == "" {
			continue
		}
		if strings.TrimSpace(component.value.SHA256) == "" {
			return RuntimeInstallResult{}, fmt.Errorf("%s precisa de hash SHA-256", component.name)
		}
		sourcePath, err := safeRuntimeJoin(sourceRoot, component.value.Path)
		if err != nil {
			return RuntimeInstallResult{}, fmt.Errorf("%s: %w", component.name, err)
		}
		resolvedSourcePath, err := filepath.EvalSymlinks(sourcePath)
		if err != nil {
			return RuntimeInstallResult{}, fmt.Errorf("resolver origem de %s: %w", component.name, err)
		}
		relativeResolved, err := filepath.Rel(resolvedSourceRoot, resolvedSourcePath)
		if err != nil || relativeResolved == ".." || strings.HasPrefix(relativeResolved, ".."+string(filepath.Separator)) {
			return RuntimeInstallResult{}, fmt.Errorf("%s aponta para fora da origem", component.name)
		}
		st, err := os.Lstat(sourcePath)
		if err != nil {
			return RuntimeInstallResult{}, fmt.Errorf("ler %s: %w", component.name, err)
		}
		if !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 {
			return RuntimeInstallResult{}, fmt.Errorf("%s não é um arquivo regular", component.name)
		}
		destination, err := safeRuntimeJoin(stagePath, component.value.Path)
		if err != nil {
			return RuntimeInstallResult{}, fmt.Errorf("destino de %s: %w", component.name, err)
		}
		if err := copyRuntimeFile(sourcePath, destination, st.Mode().Perm()); err != nil {
			return RuntimeInstallResult{}, fmt.Errorf("copiar %s: %w", component.name, err)
		}
		if err := manifest.VerifyComponent(component.name, destination); err != nil {
			return RuntimeInstallResult{}, err
		}
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return RuntimeInstallResult{}, fmt.Errorf("serializar manifesto: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stagePath, "manifest.json"), append(manifestData, '\n'), 0o600); err != nil {
		return RuntimeInstallResult{}, fmt.Errorf("gravar manifesto instalado: %w", err)
	}
	if err := os.Rename(stagePath, finalPath); err != nil {
		return RuntimeInstallResult{}, fmt.Errorf("publicar runtime: %w", err)
	}
	return RuntimeInstallResult{Version: manifest.Version, Path: finalPath}, nil
}

func (m *RuntimeManager) Activate(version string) error {
	version = strings.TrimSpace(version)
	if err := validateRuntimeVersion(version); err != nil {
		return err
	}
	manifest, err := m.loadVersion(version)
	if err != nil {
		return err
	}
	current, _ := m.readActivation()
	previous := current.Version
	if previous == version {
		previous = current.Previous
	}
	return m.writeActivation(runtimeActivation{Version: manifest.Version, Previous: previous})
}

func (m *RuntimeManager) Rollback() error {
	current, err := m.readActivation()
	if err != nil {
		return err
	}
	if current.Previous == "" {
		return errors.New("não há versão anterior do runtime")
	}
	if _, err := m.loadVersion(current.Previous); err != nil {
		return fmt.Errorf("validar runtime anterior: %w", err)
	}
	return m.writeActivation(runtimeActivation{Version: current.Previous, Previous: current.Version})
}

func (m *RuntimeManager) Active() (RuntimeManifest, error) {
	activation, err := m.readActivation()
	if err != nil {
		return RuntimeManifest{}, err
	}
	if activation.Version == "" {
		return RuntimeManifest{}, errors.New("nenhum runtime ativo")
	}
	return m.loadVersion(activation.Version)
}

func (m *RuntimeManager) Status() (RuntimeStatus, error) {
	if m == nil || m.root == "" {
		return RuntimeStatus{}, errors.New("gerenciador de runtime ausente")
	}
	activation, err := m.readActivation()
	if err != nil {
		return RuntimeStatus{}, err
	}
	versions, err := m.InstalledVersions()
	if err != nil {
		return RuntimeStatus{}, err
	}
	return RuntimeStatus{
		ActiveVersion:     activation.Version,
		PreviousVersion:   activation.Previous,
		InstalledVersions: versions,
	}, nil
}

func (m *RuntimeManager) InstalledVersions() ([]string, error) {
	entries, err := os.ReadDir(m.root)
	if err != nil {
		return nil, fmt.Errorf("listar runtimes: %w", err)
	}
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if _, err := m.loadVersion(entry.Name()); err == nil {
			versions = append(versions, entry.Name())
		}
	}
	sort.Strings(versions)
	return versions, nil
}

func (m *RuntimeManager) ComponentPath(name string) (string, error) {
	activation, err := m.readActivation()
	if err != nil {
		return "", err
	}
	manifest, err := m.loadVersion(activation.Version)
	if err != nil {
		return "", err
	}
	var component RuntimeComponent
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "yt-dlp", "ytdlp":
		component = manifest.YtDlp
	case "js", "js_runtime", "runtime js", "deno", "node", "qjs", "quickjs":
		component = manifest.JSRuntime
	case "ejs":
		component = manifest.EJS
	case "pot", "pot_provider":
		component = manifest.POTProvider
	default:
		return "", errors.New("componente de runtime desconhecido")
	}
	if component.Path == "" {
		return "", errors.New("componente não possui caminho instalado")
	}
	path, err := safeRuntimeJoin(filepath.Join(m.root, activation.Version), component.Path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		if err == nil {
			err = errors.New("arquivo não é regular")
		}
		return "", fmt.Errorf("componente indisponível: %w", err)
	}
	return path, nil
}

func (m *RuntimeManager) loadVersion(version string) (RuntimeManifest, error) {
	if err := validateRuntimeVersion(version); err != nil {
		return RuntimeManifest{}, err
	}
	versionRoot := filepath.Join(m.root, version)
	versionInfo, err := os.Lstat(versionRoot)
	if err != nil {
		return RuntimeManifest{}, fmt.Errorf("runtime %q indisponível: %w", version, err)
	}
	if !versionInfo.IsDir() || versionInfo.Mode()&os.ModeSymlink != 0 {
		return RuntimeManifest{}, fmt.Errorf("runtime %q não é um diretório real", version)
	}
	manifestPath := filepath.Join(versionRoot, "manifest.json")
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil {
		return RuntimeManifest{}, fmt.Errorf("manifesto do runtime %q indisponível: %w", version, err)
	}
	if !manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0 {
		return RuntimeManifest{}, fmt.Errorf("manifesto do runtime %q não é um arquivo regular", version)
	}
	manifest, err := LoadRuntimeManifest(manifestPath)
	if err != nil {
		return RuntimeManifest{}, err
	}
	for _, component := range runtimeComponents(manifest) {
		if strings.TrimSpace(component.value.Path) == "" {
			continue
		}
		path, err := safeRuntimeJoin(versionRoot, component.value.Path)
		if err != nil {
			return RuntimeManifest{}, fmt.Errorf("%s: %w", component.name, err)
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			if err == nil {
				err = errors.New("arquivo não é regular")
			}
			return RuntimeManifest{}, fmt.Errorf("componente %s indisponível: %w", component.name, err)
		}
		if err := manifest.VerifyComponent(component.name, path); err != nil {
			return RuntimeManifest{}, err
		}
	}
	return manifest, nil
}

func (m *RuntimeManager) readActivation() (runtimeActivation, error) {
	data, err := os.ReadFile(filepath.Join(m.root, activeRuntimeFile))
	if errors.Is(err, os.ErrNotExist) {
		return runtimeActivation{}, nil
	}
	if err != nil {
		return runtimeActivation{}, fmt.Errorf("ler runtime ativo: %w", err)
	}
	var activation runtimeActivation
	if err := json.Unmarshal(data, &activation); err != nil {
		return runtimeActivation{}, fmt.Errorf("decodificar runtime ativo: %w", err)
	}
	if activation.Version != "" {
		if err := validateRuntimeVersion(activation.Version); err != nil {
			return runtimeActivation{}, err
		}
	}
	if activation.Previous != "" {
		if err := validateRuntimeVersion(activation.Previous); err != nil {
			return runtimeActivation{}, err
		}
	}
	return activation, nil
}

func (m *RuntimeManager) writeActivation(activation runtimeActivation) error {
	data, err := json.Marshal(activation)
	if err != nil {
		return fmt.Errorf("serializar runtime ativo: %w", err)
	}
	token, err := randomRuntimeToken()
	if err != nil {
		return err
	}
	temporary := filepath.Join(m.root, ".active-"+token)
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("gravar runtime ativo: %w", err)
	}
	if err := os.Rename(temporary, filepath.Join(m.root, activeRuntimeFile)); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publicar runtime ativo: %w", err)
	}
	return nil
}

type runtimeComponentRef struct {
	name  string
	value RuntimeComponent
}

func runtimeComponents(manifest RuntimeManifest) []runtimeComponentRef {
	return []runtimeComponentRef{
		{name: "yt-dlp", value: manifest.YtDlp},
		{name: "js", value: manifest.JSRuntime},
		{name: "ejs", value: manifest.EJS},
		{name: "pot", value: manifest.POTProvider},
	}
}

func validateRuntimeVersion(version string) error {
	version = strings.TrimSpace(version)
	if version == "" || len(version) > 96 || version == "." || version == ".." || strings.ContainsAny(version, `/\\`) {
		return errors.New("versão do runtime inválida")
	}
	for _, r := range version {
		if r < 0x20 || r == 0x7f {
			return errors.New("versão do runtime contém caractere inválido")
		}
	}
	return nil
}

func safeRuntimeJoin(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", errors.New("caminho absoluto não permitido")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("caminho fora do diretório permitido")
	}
	return filepath.Join(root, clean), nil
}

func copyRuntimeFile(source, destination string, permissions os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	mode := permissions & 0o700
	if mode&0o111 == 0 {
		mode = 0o600
	} else {
		mode = 0o700
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func randomRuntimeToken() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("gerar identificador temporário: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func defaultManagedRuntimeManager() (*RuntimeManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return NewRuntimeManager(filepath.Join(home, ".local", "share", "nanotube", "runtime"))
}

// NewManagedRuntimeManager returns the per-user manager used by NanoTube.
// Creating the directory is intentional: it reserves a private 0700 root for
// future, explicitly authorized runtime installations.
func NewManagedRuntimeManager() (*RuntimeManager, error) {
	return defaultManagedRuntimeManager()
}

func managedRuntimeComponentPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	for _, appName := range []string{"hummtube", "nanotube"} {
		root := filepath.Join(home, ".local", "share", appName, "runtime")
		if _, err := os.Stat(root); err == nil {
			manager := &RuntimeManager{root: filepath.Clean(root)}
			if path, err := manager.ComponentPath(name); err == nil && path != "" {
				return path, nil
			}
		}
	}
	return "", exec.ErrNotFound
}
