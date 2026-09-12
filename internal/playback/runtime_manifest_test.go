package playback

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeManifestValidationAndHash(t *testing.T) {
	file := filepath.Join(t.TempDir(), "yt-dlp")
	content := []byte("runtime")
	if err := os.WriteFile(file, content, 0o700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	manifest := RuntimeManifest{SchemaVersion: "1", Version: "2026.08.31", YtDlp: RuntimeComponent{Version: "2026.08.31", SHA256: hex.EncodeToString(digest[:])}, JSRuntime: RuntimeComponent{Version: "deno-2"}}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := manifest.VerifyComponent("yt-dlp", file); err != nil {
		t.Fatalf("VerifyComponent: %v", err)
	}
	if err := os.WriteFile(file, []byte("tampered"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := manifest.VerifyComponent("yt-dlp", file); err == nil {
		t.Fatal("hash adulterado foi aceito")
	}
}

func TestRuntimeManifestRejectsUnknownFields(t *testing.T) {
	file := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(file, []byte(`{"schema_version":"1","version":"1","yt_dlp":{"version":"1"},"js_runtime":{"version":"1"},"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuntimeManifest(file); err == nil {
		t.Fatal("campo desconhecido aceito")
	}
}
