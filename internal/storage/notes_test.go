package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestNoteLifecycle(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	if note, err := r.Note(ctx, "v1"); err != nil {
		t.Fatalf("Note vazia: %v", err)
	} else if note != "" {
		t.Fatalf("Note = %q, quer vazia", note)
	}

	if err := r.SaveNote(ctx, "v1", "  anotação legal  "); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	note, err := r.Note(ctx, "v1")
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if note != "anotação legal" {
		t.Fatalf("Note = %q, quer anotação legal", note)
	}

	// Texto vazio apaga.
	if err := r.SaveNote(ctx, "v1", "   "); err != nil {
		t.Fatalf("SaveNote vazio: %v", err)
	}
	note, _ = r.Note(ctx, "v1")
	if note != "" {
		t.Fatalf("Note após apagar = %q", note)
	}

	// Vídeos nunca catalogados funcionam (sem FK).
	if err := r.SaveNote(ctx, "v_avulsa", "x"); err != nil {
		t.Fatalf("SaveNote avulsa: %v", err)
	}
}

func TestBookmarkLifecycle(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	b1, err := r.AddBookmark(ctx, "v1", 90*time.Second, "o ponto bom")
	if err != nil {
		t.Fatalf("AddBookmark: %v", err)
	}
	if !strings.HasPrefix(b1.ID, "bm_") {
		t.Fatalf("id = %q, quer prefixo bm_", b1.ID)
	}
	if b1.Position != 90*time.Second {
		t.Fatalf("Position = %v", b1.Position)
	}

	if _, err := r.AddBookmark(ctx, "v1", 5*time.Second, ""); err != nil {
		t.Fatalf("AddBookmark 2: %v", err)
	}

	marks, err := r.Bookmarks(ctx, "v1")
	if err != nil {
		t.Fatalf("Bookmarks: %v", err)
	}
	if len(marks) != 2 {
		t.Fatalf("Bookmarks = %d, quer 2", len(marks))
	}
	// Ordenado por posição, não por criação.
	if marks[0].Position != 5*time.Second || marks[1].Position != 90*time.Second {
		t.Fatalf("ordem = %v, %v", marks[0].Position, marks[1].Position)
	}

	// Outro vídeo não "vaza" marcadores.
	other, err := r.Bookmarks(ctx, "outro")
	if err != nil {
		t.Fatalf("Bookmarks outro: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("Bookmarks outro = %d", len(other))
	}

	if err := r.DeleteBookmark(ctx, b1.ID); err != nil {
		t.Fatalf("DeleteBookmark: %v", err)
	}
	// Excluir o que não existe não é erro.
	if err := r.DeleteBookmark(ctx, "bm_nada"); err != nil {
		t.Fatalf("DeleteBookmark inexistente: %v", err)
	}
	marks, _ = r.Bookmarks(ctx, "v1")
	if len(marks) != 1 {
		t.Fatalf("Bookmarks após delete = %d, quer 1", len(marks))
	}
}

func TestNotesStoreInterface(t *testing.T) {
	// O repositório deve satisfazer o port (contrato de domínio).
	var _ domain.NotesStore = (*Repository)(nil)
}
