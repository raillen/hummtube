package launcher

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/nanotube/nanotube-web/internal/product"
	"github.com/nanotube/nanotube-web/internal/storage"
)

func TestCloneSQLiteDatabaseCreatesIndependentSnapshot(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source.db")
	targetPath := filepath.Join(directory, "target.db")
	source, err := storage.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec("CREATE TABLE sample (value TEXT); INSERT INTO sample VALUES ('legado')"); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	if err := cloneSQLiteDatabase(sourcePath, targetPath); err != nil {
		t.Fatal(err)
	}
	target, err := sql.Open("sqlite", targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	var value string
	if err := target.QueryRow("SELECT value FROM sample").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "legado" {
		t.Fatalf("snapshot = %q", value)
	}
	if _, err := target.Exec("UPDATE sample SET value = 'novo'"); err != nil {
		t.Fatal(err)
	}
	source, err = storage.Open(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := source.QueryRow("SELECT value FROM sample").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "legado" {
		t.Fatalf("snapshot alterou origem: %q", value)
	}
}

func TestPruneLegacyDataKeepsOnlyProductRows(t *testing.T) {
	ctx := context.Background()
	for _, testCase := range []struct {
		name                string
		spec                product.Spec
		wantVideos          int
		wantIPTVSources     int
		wantProfileAccounts int
	}{
		{name: "nanoiptv", spec: product.NanoIPTV, wantVideos: 0, wantIPTVSources: 1, wantProfileAccounts: 0},
		{name: "nanomusic", spec: product.NanoMusic, wantVideos: 1, wantIPTVSources: 0, wantProfileAccounts: 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db, err := storage.Open(filepath.Join(t.TempDir(), testCase.name+".db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := storage.Migrate(db); err != nil {
				t.Fatal(err)
			}
			for _, statement := range []string{
				"INSERT INTO videos (id, title) VALUES ('video-1', 'Vídeo')",
				"INSERT INTO iptv_sources (id, name) VALUES ('source-1', 'Lista')",
				"INSERT INTO profile_accounts (profile_id, provider_subject, email) VALUES ('default', 'subject-1', 'user@example.test')",
			} {
				if _, err := db.ExecContext(ctx, statement); err != nil {
					t.Fatal(err)
				}
			}
			if err := pruneLegacyData(db, testCase.spec); err != nil {
				t.Fatal(err)
			}
			for table, expected := range map[string]int{
				"videos": testCase.wantVideos, "iptv_sources": testCase.wantIPTVSources,
				"profile_accounts": testCase.wantProfileAccounts,
			} {
				var count int
				if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != expected {
					t.Fatalf("%s: %s = %d, esperado %d", testCase.name, table, count, expected)
				}
			}
		})
	}
}
