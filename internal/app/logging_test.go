package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLogFileCreatesPrivateFile(t *testing.T) {
	previous := CurrentNamespace()
	t.Cleanup(func() { _ = ConfigureNamespace(previous) })
	if err := ConfigureNamespace("nanotube-log-test"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	file, err := OpenLogFile()
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if _, err := file.WriteString("evento\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		t.Fatalf("permissões do log = %o, esperado 600", permissions)
	}
	if filepath.Base(path) != "nanotube-log-test.log" {
		t.Fatalf("arquivo de log inesperado: %s", path)
	}
}
