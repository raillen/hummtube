package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/storage"
)

// newSnapshotRepo abre um repo SQLite real (migrações aplicadas) para exercitar
// o contrato completo create → restore.
func newSnapshotRepo(t *testing.T, path string) *storage.Repository {
	t.Helper()
	db, err := storage.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := storage.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return storage.NewRepository(db)
}

func TestSnapshotRoundTripRestoresPersonalAndRules(t *testing.T) {
	dir := t.TempDir()
	src := newSnapshotRepo(t, filepath.Join(dir, "src.db"))
	dstPath := filepath.Join(dir, "dst.db")
	ctx := context.Background()

	// Estado de origem: histórico, favorito, playlist + regra inteligente.
	if err := src.MarkProgress(ctx, "v1", domain.PlaybackProgress{
		VideoID: "v1", Position: 30 * time.Second, Duration: 2 * time.Minute,
		UpdatedAt: time.Now(), Completed: false,
	}); err != nil {
		t.Fatalf("MarkProgress: %v", err)
	}
	pl, err := src.CreatePlaylist(ctx, "Estudo", "", "")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	rule := domain.SmartRule{Version: domain.CurrentSmartRuleVersion,
		Filter: domain.PlaylistFilter{Watched: domain.SearchWatchUnwatched},
		Sort:   domain.PlaylistSortPublished}
	if err := src.SaveSmartRule(ctx, pl.ID, rule); err != nil {
		t.Fatalf("SaveSmartRule: %v", err)
	}

	backupPath := filepath.Join(dir, "backup.ntbackup")
	if err := CreateSnapshot(ctx, src.DB(), src, backupPath, SnapshotOptions{}); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	// Destino vazio (DB inexistente): RestoreSnapshot cria o arquivo,
	// migra e aplica as seções.
	if err := RestoreSnapshot(ctx, dstPath, backupPath, ""); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}
	restored := newSnapshotRepo(t, dstPath)

	data, err := restored.ExportPersonalData(ctx)
	if err != nil {
		t.Fatalf("ExportPersonalData pós-restore: %v", err)
	}
	if len(data.History) != 1 || data.History[0].VideoID != "v1" {
		t.Fatalf("histórico restaurado = %+v", data.History)
	}
	gotRule, err := restored.GetSmartRule(ctx, pl.ID)
	if err != nil {
		t.Fatalf("GetSmartRule pós-restore: %v", err)
	}
	if gotRule.Sort != domain.PlaylistSortPublished ||
		gotRule.Filter.Watched != domain.SearchWatchUnwatched {
		t.Fatalf("regra restaurada = %+v", gotRule)
	}
}

func TestSnapshotEncryptedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := newSnapshotRepo(t, filepath.Join(dir, "src.db"))
	ctx := context.Background()
	if err := src.MarkProgress(ctx, "v9", domain.PlaybackProgress{
		VideoID: "v9", Position: 0, Duration: time.Minute,
		UpdatedAt: time.Now(), Completed: true,
	}); err != nil {
		t.Fatalf("MarkProgress: %v", err)
	}

	backupPath := filepath.Join(dir, "secret.ntbackup")
	if err := CreateSnapshot(ctx, src.DB(), src, backupPath,
		SnapshotOptions{Passphrase: "senha-forte"}); err != nil {
		t.Fatalf("CreateSnapshot criptografado: %v", err)
	}

	// Sem a senha: leitura falha (não é zip).
	dstPath := filepath.Join(dir, "dst.db")
	if err := RestoreSnapshot(ctx, dstPath, backupPath, ""); err == nil {
		t.Fatal("restore sem senha deveria falhar")
	}
	// Com a senha: restaura.
	if err := RestoreSnapshot(ctx, dstPath, backupPath, "senha-forte"); err != nil {
		t.Fatalf("RestoreSnapshot com senha: %v", err)
	}
	restored := newSnapshotRepo(t, dstPath)
	data, err := restored.ExportPersonalData(ctx)
	if err != nil {
		t.Fatalf("export pós-restore: %v", err)
	}
	if len(data.History) != 1 || data.History[0].VideoID != "v9" {
		t.Fatalf("histórico = %+v", data.History)
	}
}

func TestRestoreRejectsTamperedSection(t *testing.T) {
	dir := t.TempDir()
	src := newSnapshotRepo(t, filepath.Join(dir, "src.db"))
	ctx := context.Background()
	backupPath := filepath.Join(dir, "backup.ntbackup")
	if err := CreateSnapshot(ctx, src.DB(), src, backupPath, SnapshotOptions{}); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	// Adultera o arquivo: qualquer byte no meio quebra o checksum/GCM.
	raw, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	raw[len(raw)/2] ^= 0xFF
	tampered := filepath.Join(dir, "tampered.ntbackup")
	if err := os.WriteFile(tampered, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := RestoreSnapshot(ctx, filepath.Join(dir, "dst.db"), tampered, ""); err == nil {
		t.Fatal("backup adulterado deveria ser rejeitado")
	}
}
