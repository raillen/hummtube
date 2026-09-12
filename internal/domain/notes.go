// Notas e marcadores de tempo locais por vídeo (PLY-02/QOL-01). Estado
// puramente local: nada é sincronizado com o YouTube.
// Documentos canônicos: docs/03-implementation/STORAGE.md,
// docs/00-product/SCOPE_AND_ROADMAP.md (M7, M9).
package domain

import (
	"context"
	"time"
)

// VideoNote é a nota de texto livre anexada a um vídeo (uma por vídeo).
type VideoNote struct {
	VideoID string
	Text    string
	// UpdatedAt é o momento do último salvamento.
	UpdatedAt time.Time
}

// VideoBookmark é um marcador de tempo em um vídeo: uma posição que vale a
// pena revisitar, com um rótulo opcional.
type VideoBookmark struct {
	ID        string        `json:"id"`
	VideoID   string        `json:"video_id"`
	Position  time.Duration `json:"position"`
	Label     string        `json:"label"`
	CreatedAt time.Time     `json:"created_at"`
}

// NotesStore persiste notas e marcadores locais por vídeo
// (docs/03-implementation/STORAGE.md).
type NotesStore interface {
	// SaveNote substitui a nota do vídeo pelo texto dado. Texto vazio apaga a
	// nota (é o "sem nota" explícito).
	SaveNote(ctx context.Context, videoID, text string) error
	// Note devolve a nota do vídeo, ou "" quando não existe.
	Note(ctx context.Context, videoID string) (string, error)

	// AddBookmark cria um marcador na posição dada e devolve o marcador
	// persistido. Rótulo vazio é permitido (a UI mostra só o tempo).
	AddBookmark(ctx context.Context, videoID string, position time.Duration, label string) (VideoBookmark, error)
	// Bookmarks lista os marcadores do vídeo ordenados por posição.
	Bookmarks(ctx context.Context, videoID string) ([]VideoBookmark, error)
	// DeleteBookmark remove um marcador; remover o que não existe não é erro.
	DeleteBookmark(ctx context.Context, bookmarkID string) error
}
