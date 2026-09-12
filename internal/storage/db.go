// Package storage implements SQLite/FTS5/Goose persistence and keyring
// adapters (docs/03-implementation/STORAGE.md).
package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens a SQLite database file using the modernc driver.
//
// `foreign_keys=1` é obrigatório: sem o pragma, SQLite ignora as FKs por
// padrão e a cascata de playlists (ON DELETE CASCADE) não aconteceria.
func Open(path string) (*sql.DB, error) {
	dsn := path
	if !strings.Contains(dsn, "_pragma=") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite em modo arquivo opera de forma segura com 1 writer por vez
	if !strings.Contains(path, "mode=memory") {
		db.SetMaxOpenConns(1)
	}
	return db, nil
}

// OpenMemory opens a fresh shared in-memory SQLite database.
//
// `cache=shared` é obrigatório: com ":memory:" puro, cada conexão do pool
// ganharia um banco próprio, e leituras em paralelo com uma escrita não
// veriam os dados (a UI roda storage em goroutine e lê na main thread).
func OpenMemory(ctx context.Context) (*sql.DB, error) {
	return Open("file:nanotube_test?mode=memory&cache=shared")
}

// Migrate applies embedded goose migrations.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetDialect("sqlite3")
	goose.SetLogger(noopLogger{})
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

type noopLogger struct{}

func (noopLogger) Fatalf(format string, v ...interface{}) {}
func (noopLogger) Printf(format string, v ...interface{}) {}

// MigrationVersion returns the current goose migration version.
func MigrationVersion(db *sql.DB) (int64, error) {
	v, err := goose.GetDBVersion(db)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// SQLiteVersion returns the driver runtime version string.
func SQLiteVersion(db *sql.DB) (string, error) {
	var v string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&v); err != nil {
		return "", err
	}
	return v, nil
}

// FTS5Probe creates a temporary FTS5 virtual table, inserts, queries and drops it.
// It returns "ok" or the error string.
func FTS5Probe(db *sql.DB) string {
	if _, err := db.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS temp.fts_probe USING fts5(text)"); err != nil {
		return "erro: " + err.Error()
	}
	defer func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS temp.fts_probe")
	}()
	if _, err := db.Exec("INSERT INTO temp.fts_probe(text) VALUES ('nanotube spike')"); err != nil {
		return "erro: " + err.Error()
	}
	var got string
	if err := db.QueryRow("SELECT text FROM temp.fts_probe WHERE fts_probe MATCH 'spike'").Scan(&got); err != nil {
		return "erro: " + err.Error()
	}
	if got != "nanotube spike" {
		return "erro: resultado inesperado"
	}
	return "ok"
}

// escapeLikePattern sanitizes LIKE metacharacters and surrounds the string with '%'.
func escapeLikePattern(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return "%" + s + "%"
}
