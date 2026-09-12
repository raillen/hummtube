package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestMigrateAndFTS5(t *testing.T) {
	db, err := OpenMemory(context.Background())
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	ver, err := MigrationVersion(db)
	if err != nil {
		t.Fatalf("MigrationVersion: %v", err)
	}
	if ver == 0 {
		t.Error("migration version é 0")
	}

	if _, err := db.Exec("INSERT INTO videos(id, title) VALUES ('v1', 'NanoTube teste')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.Exec("INSERT INTO videos_fts(title, description) VALUES ('NanoTube teste', 'fts')"); err != nil {
		t.Fatalf("insert fts: %v", err)
	}

	var title string
	if err := db.QueryRow("SELECT title FROM videos_fts WHERE videos_fts MATCH 'teste'").Scan(&title); err != nil {
		t.Fatalf("fts query: %v", err)
	}
	if title != "NanoTube teste" {
		t.Errorf("got %q", title)
	}

	if got := FTS5Probe(db); got != "ok" {
		t.Errorf("FTS5Probe = %q", got)
	}
}

// latestMigration é a versão que `Migrate` deve alcançar. Fica em um só lugar
// para uma migration nova não quebrar asserções espalhadas.
const latestMigration = 21

func TestMigrationIdempotentUp(t *testing.T) {
	db, err := OpenMemory(context.Background())
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate 1: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate 2 (idempotente): %v", err)
	}
	ver, _ := MigrationVersion(db)
	if ver != latestMigration {
		t.Errorf("version = %d, want %d", ver, latestMigration)
	}
}

func TestMigrationUpgradeFromV1(t *testing.T) {
	db, err := OpenMemory(context.Background())
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrationsFS)
	goose.SetDialect("sqlite3")
	goose.SetLogger(noopLogger{})

	if err := goose.UpTo(db, "migrations", 1); err != nil {
		t.Fatalf("UpTo 1: %v", err)
	}
	if _, err := db.Exec("INSERT INTO videos (id, title) VALUES ('legacy', 'Legado')"); err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	if _, err := db.Exec("INSERT INTO videos_fts (title, description) VALUES ('Legado', 'fts v1')"); err != nil {
		t.Fatalf("insert legacy fts: %v", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("Up: %v", err)
	}
	ver, _ := MigrationVersion(db)
	if ver != latestMigration {
		t.Fatalf("version = %d, want %d", ver, latestMigration)
	}

	var duration int64
	if err := db.QueryRow("SELECT duration FROM videos WHERE id = 'legacy'").Scan(&duration); err != nil {
		t.Fatalf("legacy row: %v", err)
	}
	if duration != 0 {
		t.Errorf("duration = %d, want 0 (default)", duration)
	}

	for _, table := range []string{"channels", "playback_progress", "interest_topics",
		"recommendation_feedback", "queue_items", "settings", "playlists", "playlist_items",
		"iptv_sources", "iptv_items", "iptv_guide_channels", "iptv_guide_programs",
		"iptv_saved_items", "iptv_playback_progress", "profile_settings"} {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Errorf("tabela %s: %v", table, err)
		}
	}
}

func TestMigration15MovesLegacyAccountToDefaultProfileWithoutLoss(t *testing.T) {
	db, err := OpenMemory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	goose.SetBaseFS(migrationsFS)
	goose.SetDialect("sqlite3")
	goose.SetLogger(noopLogger{})
	if err := goose.UpTo(db, "migrations", 14); err != nil {
		t.Fatalf("UpTo 14: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO settings (key, value) VALUES
			('account.email', 'Legacy.User@Example.com'),
			('account.connected_at', '2026-08-13T12:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO profiles (id, name, created_at) VALUES ('existing-profile', 'Existente', '2026-08-12T12:00:00Z');
		INSERT INTO profile_members (profile_id, kind, ref_id) VALUES ('existing-profile', 'playlist', 'playlist-before-migration')`); err != nil {
		t.Fatal(err)
	}

	if err := goose.UpTo(db, "migrations", 15); err != nil {
		t.Fatalf("UpTo 15: %v", err)
	}
	account, ok, err := NewAccountRepository(db, DefaultProfileID).Account(context.Background())
	if err != nil || !ok {
		t.Fatalf("conta migrada: ok=%v err=%v", ok, err)
	}
	if account.Email != "Legacy.User@Example.com" || account.ConnectedAt != "2026-08-13T12:00:00Z" {
		t.Fatalf("conta migrada perdeu dados: %+v", account)
	}
	if account.ProviderSubject != "legacy-email:legacy.user@example.com" {
		t.Fatalf("provider subject legado = %q", account.ProviderSubject)
	}
	activeID, err := NewRepository(db).ActiveProfileID(context.Background())
	if err != nil || activeID != DefaultProfileID {
		t.Fatalf("perfil ativo migrado = %q err=%v", activeID, err)
	}
	var legacyKeys int
	if err := db.QueryRow(`SELECT COUNT(*) FROM settings WHERE key LIKE 'account.%'`).Scan(&legacyKeys); err != nil {
		t.Fatal(err)
	}
	if legacyKeys != 0 {
		t.Fatalf("chaves legadas restantes = %d", legacyKeys)
	}
	var existingProfileKind domain.ProfileKind
	if err := db.QueryRow(`SELECT kind FROM profiles WHERE id = 'existing-profile'`).Scan(&existingProfileKind); err != nil {
		t.Fatalf("perfil existente foi perdido: %v", err)
	}
	if existingProfileKind != domain.ProfileKindPersistent {
		t.Fatalf("kind do perfil existente = %q", existingProfileKind)
	}
	var existingMembers int
	if err := db.QueryRow(`SELECT COUNT(*) FROM profile_members WHERE profile_id = 'existing-profile'`).Scan(&existingMembers); err != nil {
		t.Fatal(err)
	}
	if existingMembers != 1 {
		t.Fatalf("vínculos do perfil existente = %d", existingMembers)
	}
}

func TestKeyringStoreClassifiesUnavailable(t *testing.T) {
	store := NewKeyringSecretStore("nanotube-test")
	key := domain.SecretKey("probe")
	// A roundtrip either works or fails with ErrKeyringUnavailable; never panics.
	_ = store.Set(context.Background(), key, []byte("v"))
	got, err := store.Get(context.Background(), key)
	if err == nil {
		_ = store.Delete(context.Background(), key)
		if string(got) != "v" {
			t.Error("roundtrip divergente")
		}
	}
}

func TestKeyringErrorClassificationFailsClosed(t *testing.T) {
	unavailable := classifyKeyringError("gravar", errors.New(`exec: "dbus-launch": executable file not found`))
	if !errors.Is(unavailable, domain.ErrSecretStoreUnavailable) {
		t.Fatalf("indisponibilidade não classificada: %v", unavailable)
	}
	for _, underlying := range []error{
		errors.New("permission denied"),
		errors.New("I/O integrity failure"),
		errors.New("org.freedesktop.DBus.Error.AccessDenied: access to org.freedesktop.secrets denied"),
	} {
		classified := classifyKeyringError("gravar", underlying)
		if errors.Is(classified, domain.ErrSecretStoreUnavailable) {
			t.Fatalf("erro inesperado virou fallback: %v", underlying)
		}
		if !errors.Is(classified, underlying) {
			t.Fatalf("causa original foi perdida: %v", classified)
		}
	}
}

func TestSQLiteVersion(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	v, err := SQLiteVersion(db)
	if err != nil {
		t.Fatalf("SQLiteVersion: %v", err)
	}
	if v == "" {
		t.Error("sqlite version vazia")
	}
}
