package playback

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeManagerInstallActivateAndRollback(t *testing.T) {
	source := t.TempDir()
	root := filepath.Join(t.TempDir(), "runtime")
	ytdlp := filepath.Join(source, "bin", "yt-dlp")
	js := filepath.Join(source, "bin", "deno")
	if err := os.MkdirAll(filepath.Dir(ytdlp), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ytdlp, []byte("yt-dlp-v1"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(js, []byte("deno-v1"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifestPath := writeRuntimeManifest(t, source, "v1", ytdlp, js)
	manager, err := NewRuntimeManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(manifestPath, source); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := manager.Activate("v1"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	active, err := manager.Active()
	if err != nil || active.Version != "v1" {
		t.Fatalf("Active = %+v, err=%v", active, err)
	}
	componentPath, err := manager.ComponentPath("yt-dlp")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(componentPath) != filepath.Join(root, "v1", "bin") {
		t.Fatalf("componente fora da versão ativa: %s", componentPath)
	}

	if err := os.WriteFile(ytdlp, []byte("yt-dlp-v2"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifestV2 := writeRuntimeManifest(t, source, "v2", ytdlp, js)
	if _, err := manager.Install(manifestV2, source); err != nil {
		t.Fatalf("Install v2: %v", err)
	}
	if err := manager.Activate("v2"); err != nil {
		t.Fatalf("Activate v2: %v", err)
	}
	if err := manager.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	active, err = manager.Active()
	if err != nil || active.Version != "v1" {
		t.Fatalf("rollback ativo = %+v, err=%v", active, err)
	}
	versions, err := manager.InstalledVersions()
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions = %v, err=%v", versions, err)
	}
}

func TestRuntimeManagerRejectsTraversalAndLeavesNoVersion(t *testing.T) {
	source := t.TempDir()
	root := filepath.Join(t.TempDir(), "runtime")
	ytdlp := filepath.Join(source, "yt-dlp")
	if err := os.WriteFile(ytdlp, []byte("runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("runtime"))
	manifestPath := filepath.Join(source, "manifest.json")
	manifest := RuntimeManifest{
		SchemaVersion: "1", Version: "v-traversal",
		YtDlp:     RuntimeComponent{Version: "1", Path: "../yt-dlp", SHA256: hex.EncodeToString(digest[:])},
		JSRuntime: RuntimeComponent{Version: "1"},
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewRuntimeManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(manifestPath, source); err == nil {
		t.Fatal("path traversal aceito")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("staging residual após falha: %v", entries)
	}
}

func TestRuntimeManagerRequiresHashForDeclaredComponent(t *testing.T) {
	source := t.TempDir()
	ytdlp := filepath.Join(source, "yt-dlp")
	if err := os.WriteFile(ytdlp, []byte("runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(source, "manifest.json")
	manifest := RuntimeManifest{
		SchemaVersion: "1", Version: "v-no-hash",
		YtDlp:     RuntimeComponent{Version: "1", Path: "yt-dlp"},
		JSRuntime: RuntimeComponent{Version: "1"},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewRuntimeManager(filepath.Join(t.TempDir(), "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(manifestPath, source); err == nil || !strings.Contains(err.Error(), "hash SHA-256") {
		t.Fatalf("manifesto sem hash foi aceito: %v", err)
	}
}

func TestRuntimeManagerRejectsSymlinkEscapeFromSource(t *testing.T) {
	source := t.TempDir()
	outside := t.TempDir()
	external := filepath.Join(outside, "yt-dlp")
	if err := os.WriteFile(external, []byte("outside"), 0o700); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(source, "yt-dlp")
	if err := os.Symlink(external, linked); err != nil {
		t.Skipf("symlink indisponível neste ambiente: %v", err)
	}
	digest := sha256.Sum256([]byte("outside"))
	manifest := RuntimeManifest{
		SchemaVersion: "1", Version: "v-symlink",
		YtDlp:     RuntimeComponent{Version: "1", Path: "yt-dlp", SHA256: hex.EncodeToString(digest[:])},
		JSRuntime: RuntimeComponent{Version: "1"},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(source, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewRuntimeManager(filepath.Join(t.TempDir(), "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(manifestPath, source); err == nil {
		t.Fatal("symlink para fora da origem foi aceito")
	}
}

func TestRuntimeManagerRejectsTamperedActiveComponent(t *testing.T) {
	source := t.TempDir()
	ytdlp := filepath.Join(source, "bin", "yt-dlp")
	js := filepath.Join(source, "bin", "deno")
	if err := os.MkdirAll(filepath.Dir(ytdlp), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ytdlp, []byte("yt-dlp"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(js, []byte("deno 2.0.0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifestPath := writeRuntimeManifest(t, source, "tampered", ytdlp, js)
	manager, err := NewRuntimeManager(filepath.Join(t.TempDir(), "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(manifestPath, source); err != nil {
		t.Fatal(err)
	}
	if err := manager.Activate("tampered"); err != nil {
		t.Fatal(err)
	}
	componentPath, err := manager.ComponentPath("yt-dlp")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(componentPath, []byte("altered"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ComponentPath("yt-dlp"); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("componente alterado foi aceito: %v", err)
	}
}

func TestManagedActiveRuntimeIsPreferredByResolvers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	ytdlp := filepath.Join(source, "bin", "yt-dlp")
	js := filepath.Join(source, "bin", "deno")
	if err := os.WriteFile(ytdlp, []byte("#!/bin/sh\nprintf 'runtime'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(js, []byte("#!/bin/sh\nprintf 'deno 2.0.0\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifestPath := writeRuntimeManifest(t, source, "active", ytdlp, js)
	manager, err := NewRuntimeManager(filepath.Join(home, ".local", "share", "nanotube", "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(manifestPath, source); err != nil {
		t.Fatal(err)
	}
	if err := manager.Activate("active"); err != nil {
		t.Fatal(err)
	}
	resolved, sourceName, err := resolveYtDlpBinary("")
	if err != nil || sourceName != "managed-active" || !strings.HasSuffix(resolved, filepath.Join("active", "bin", "yt-dlp")) {
		t.Fatalf("yt-dlp ativo = %s (%s), err=%v", resolved, sourceName, err)
	}
	jsRuntime := resolveJSRuntime("")
	if !jsRuntime.Found() || jsRuntime.Name != "deno" || !strings.HasSuffix(jsRuntime.Path, filepath.Join("active", "bin", "deno")) {
		t.Fatalf("runtime JS ativo = %+v", jsRuntime)
	}
}

func writeRuntimeManifest(t *testing.T, source, version, ytdlp, js string) string {
	t.Helper()
	ytdlpData, err := os.ReadFile(ytdlp)
	if err != nil {
		t.Fatal(err)
	}
	jsData, err := os.ReadFile(js)
	if err != nil {
		t.Fatal(err)
	}
	ytdlpHash := sha256.Sum256(ytdlpData)
	jsHash := sha256.Sum256(jsData)
	manifest := RuntimeManifest{
		SchemaVersion: "1", Version: version,
		YtDlp:     RuntimeComponent{Version: version, Path: filepath.ToSlash(filepath.Join("bin", "yt-dlp")), SHA256: hex.EncodeToString(ytdlpHash[:])},
		JSRuntime: RuntimeComponent{Version: version, Path: filepath.ToSlash(filepath.Join("bin", "deno")), SHA256: hex.EncodeToString(jsHash[:])},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(source, "manifest-"+version+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
