package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheDirUsesXDG(t *testing.T) {
	old := os.Getenv("XDG_CACHE_HOME")
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", home)
	t.Setenv("HOME", filepath.Join(home, "home"))
	defer os.Setenv("XDG_CACHE_HOME", old)

	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "nanotube-web")
	if dir != want {
		t.Errorf("CacheDir() = %s, want %s", dir, want)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("CacheDir() não criou o diretório: %v", err)
	}
}

func TestCacheDirFallsBackToHome(t *testing.T) {
	oldXDG := os.Getenv("XDG_CACHE_HOME")
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", home)
	defer os.Setenv("XDG_CACHE_HOME", oldXDG)
	defer os.Setenv("HOME", oldHome)

	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".cache", "nanotube-web")
	if dir != want {
		t.Errorf("CacheDir() = %s, want %s", dir, want)
	}
}

func TestThumbnailsDirNested(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", home)

	dir, err := ThumbnailsDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "thumbnails" {
		t.Errorf("ThumbnailsDir() = %s, want .../thumbnails", dir)
	}
}

func TestDataDirUsesXDG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", home)

	dir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "nanotube-web" {
		t.Errorf("DataDir() = %s, want .../nanotube-web", dir)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("DataDir() não criou o diretório: %v", err)
	}
}

func TestDataDirFallsBackToHome(t *testing.T) {
	old := os.Getenv("XDG_DATA_HOME")
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", home)
	defer os.Setenv("XDG_DATA_HOME", old)

	dir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".local", "share", "nanotube-web")
	if dir != want {
		t.Errorf("DataDir() = %s, want %s", dir, want)
	}
}

func TestStateDirAndLogFilePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", home)

	dir, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "nanotube-web" {
		t.Errorf("StateDir() = %s, want .../nanotube-web", dir)
	}
	logPath, err := LogFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(logPath) != "nanotube-web.log" {
		t.Errorf("LogFilePath() = %s, want .../nanotube-web.log", logPath)
	}
}

func TestProductDirectoriesAreIsolatedAndValidated(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	nanoTubeDir, err := DataDirFor("nanotube-web")
	if err != nil {
		t.Fatal(err)
	}
	nanoIPTVDir, err := DataDirFor("nanoiptv")
	if err != nil {
		t.Fatal(err)
	}
	if nanoTubeDir == nanoIPTVDir {
		t.Fatal("produtos compartilharam o mesmo diretório de dados")
	}
	if _, err := DataDirFor("../escape"); err == nil {
		t.Fatal("namespace inseguro foi aceito")
	}
}

func TestConfiguredNamespaceAppliesToDefaultPathFunctions(t *testing.T) {
	previous := CurrentNamespace()
	t.Cleanup(func() { _ = ConfigureNamespace(previous) })
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := ConfigureNamespace("nanomusic"); err != nil {
		t.Fatal(err)
	}
	directory, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(directory) != "nanomusic" {
		t.Fatalf("ConfigDir() = %s", directory)
	}
}
