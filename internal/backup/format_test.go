package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWriteReadRoundTrip(t *testing.T) {
	sections := map[string]json.RawMessage{
		"personal": json.RawMessage(`{"v":1}`),
		"stats":    json.RawMessage(`{"playlists":2}`),
	}
	var buf bytes.Buffer
	if err := Write(&buf, sections); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, manifest, err := Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if manifest.Magic != Magic || manifest.Version != CurrentVersion {
		t.Fatalf("manifest: %+v", manifest)
	}
	if string(got["personal"]) != `{"v":1}` {
		t.Fatalf("personal: %s", got["personal"])
	}
}

func TestWriteRejectsTraversal(t *testing.T) {
	sections := map[string]json.RawMessage{
		"../evil": json.RawMessage(`{}`),
	}
	var buf bytes.Buffer
	if err := Write(&buf, sections); err == nil {
		t.Fatal("deveria rejeitar traversal")
	}
}

func TestReadRejectsFutureVersion(t *testing.T) {
	manifest := Manifest{
		Magic:     Magic,
		Version:   CurrentVersion + 99,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Sections:  []string{"test"},
		Checksums: map[string]string{"test": checksum([]byte(`{}`))},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("test.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	mf, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mf.Write(manifestBytes); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, err = Read(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err == nil {
		t.Fatal("esperava erro ao ler versão futura")
	}
	if !strings.Contains(err.Error(), "versão futura") {
		t.Fatalf("erro inesperado: %v", err)
	}
}
