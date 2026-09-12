// Snapshot: criação e restauração do .ntbackup (M6 BCK-01/02/03, PLY-06).
// O restore é atômico: valida o backup, copia o DB atual como recuperação,
// aplica as seções em um DB de staging e só então substitui o original.
package backup

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/storage"
)

// SnapshotReader lê as seções suportadas do repo para um backup.
type SnapshotReader interface {
	LocalStats(ctx context.Context) (domain.LocalStats, error)
	ExportPersonalData(ctx context.Context) (domain.PersonalData, error)
	ExportSmartRules(ctx context.Context) (domain.SmartRulesBackup, error)
}

// SnapshotWriter aplica seções de um backup em um repositório.
type SnapshotWriter interface {
	ImportPersonalData(ctx context.Context, data domain.PersonalData) error
	ImportSmartRules(ctx context.Context, rules domain.SmartRulesBackup) error
}

// SnapshotOptions controla o que entra no arquivo e a criptografia.
// Passphrase vazio grava zip puro; com passphrase, AES-256-GCM + PBKDF2
// (crypto.go), sem dependência nova — é a mesma stdlib/x/crypto do go.mod.
type SnapshotOptions struct {
	IncludeCatalog bool
	Sections       []string // vazio = todas as aplicáveis
	Passphrase     string
}

// CreateSnapshot cria um .ntbackup atômico em path (tmp + rename).
func CreateSnapshot(ctx context.Context, db *sql.DB, reader SnapshotReader, path string, opts SnapshotOptions) error {
	if err := checkIntegrity(ctx, db); err != nil {
		return err
	}
	want := map[string]bool{}
	for _, s := range opts.Sections {
		want[s] = true
	}
	include := func(name string) bool {
		if len(opts.Sections) == 0 {
			return true
		}
		return want[name]
	}

	sections := make(map[string]json.RawMessage)
	if include("personal") {
		if data, err := exportPersonalDataRaw(ctx, reader); err == nil {
			sections["personal"] = data
		}
	}
	if include("stats") {
		if stats, err := exportStatsRaw(ctx, reader); err == nil {
			sections["stats"] = stats
		}
	}
	if include("rules") {
		if data, err := exportRulesRaw(ctx, reader); err == nil && len(data) > 2 {
			sections["rules"] = data
		}
	}
	if opts.IncludeCatalog && include("catalog") {
		if data, err := exportCatalogRaw(ctx, db); err == nil && len(data) > 2 {
			sections["catalog"] = data
		}
	}

	var buf bytes.Buffer
	if err := Write(&buf, sections); err != nil {
		return err
	}
	payload := buf.Bytes()
	if opts.Passphrase != "" {
		encrypted, err := Encrypt([]byte(opts.Passphrase), payload)
		if err != nil {
			return fmt.Errorf("criptografar backup: %w", err)
		}
		payload = encrypted
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ntbackup-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// RestoreSnapshot valida o .ntbackup, preserva o DB atual como cópia de
// recuperação e restaura personal+rules em staging antes de substituir.
// Backup criptografado exige a mesma passphrase da gravação.
func RestoreSnapshot(ctx context.Context, dbPath, backupPath, passphrase string) error {
	payload, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	if passphrase != "" {
		plain, err := Decrypt([]byte(passphrase), payload)
		if err != nil {
			return err
		}
		payload = plain
	}
	sections, _, err := Read(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return err
	}

	// Cópia de recuperação do estado atual (best effort: não bloqueia o
	// restore; o staging já protege o original contra falha parcial).
	if _, err := os.Stat(dbPath); err == nil {
		recovery := dbPath + ".pre-restore-" + time.Now().UTC().Format("20060102-150405")
		_ = copyFile(dbPath, recovery)
	}

	// Staging: copia o DB e migra para o schema atual antes de aplicar.
	staging := dbPath + ".staging"
	_ = os.Remove(staging)
	defer os.Remove(staging)
	if _, err := os.Stat(dbPath); err == nil {
		if err := copyFile(dbPath, staging); err != nil {
			return fmt.Errorf("preparar staging: %w", err)
		}
	}
	sdb, err := storage.Open(staging)
	if err != nil {
		return fmt.Errorf("abrir staging: %w", err)
	}
	if err := storage.Migrate(sdb); err != nil {
		sdb.Close()
		return fmt.Errorf("migrar staging: %w", err)
	}
	writer := storage.NewRepository(sdb)

	if data, ok := sections["personal"]; ok {
		var personal domain.PersonalData
		if err := json.Unmarshal(data, &personal); err != nil {
			sdb.Close()
			return fmt.Errorf("seção personal inválida: %w", err)
		}
		if err := writer.ImportPersonalData(ctx, personal); err != nil {
			sdb.Close()
			return err
		}
	}
	if data, ok := sections["rules"]; ok {
		var rules domain.SmartRulesBackup
		if err := json.Unmarshal(data, &rules); err != nil {
			sdb.Close()
			return fmt.Errorf("seção rules inválida: %w", err)
		}
		if err := writer.ImportSmartRules(ctx, rules); err != nil {
			sdb.Close()
			return err
		}
	}
	if err := sdb.Close(); err != nil {
		return err
	}
	return os.Rename(staging, dbPath)
}

func checkIntegrity(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity_check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity_check falhou: %s", result)
	}
	return nil
}

func exportPersonalDataRaw(ctx context.Context, r SnapshotReader) (json.RawMessage, error) {
	data, err := r.ExportPersonalData(ctx)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(data)
	return json.RawMessage(b), err
}
func exportStatsRaw(ctx context.Context, r SnapshotReader) (json.RawMessage, error) {
	s, err := r.LocalStats(ctx)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(s)
	return json.RawMessage(b), err
}
func exportRulesRaw(ctx context.Context, r SnapshotReader) (json.RawMessage, error) {
	rules, err := r.ExportSmartRules(ctx)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(rules)
	return json.RawMessage(b), err
}
func exportCatalogRaw(ctx context.Context, db *sql.DB) (json.RawMessage, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, title, channel_id FROM videos ORDER BY published_at DESC, id LIMIT 5000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		ChannelID string `json:"channel_id"`
	}
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.Title, &r.ChannelID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("serializar catalogo: %w", err)
	}
	return json.RawMessage(b), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		_ = os.Remove(dst)
		return cpErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	return nil
}
