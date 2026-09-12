// Notas e marcadores de tempo locais (PLY-02/QOL-01).
// Estado puramente local, nada é sincronizado com o YouTube
// (docs/03-implementation/STORAGE.md).
package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// BookmarkID gera um id curto e estável para um marcador novo.
func BookmarkID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("notas: gerar id: %w", err)
	}
	return "bm_" + hex.EncodeToString(buf), nil
}

// SaveNote substitui a nota do vídeo. Texto vazio apaga a linha (sem nota).
func (r *Repository) SaveNote(ctx context.Context, videoID, text string) error {
	if videoID == "" {
		return errors.New("notas: vídeo vazio")
	}
	text = strings.TrimSpace(text)
	now := fmtTime(time.Now())
	if text == "" {
		if _, err := r.db.ExecContext(ctx,
			`DELETE FROM video_notes WHERE video_id = ?`, videoID); err != nil {
			return fmt.Errorf("notas: apagar: %w", err)
		}
		return nil
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO video_notes (video_id, text, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET
			text = excluded.text,
			updated_at = excluded.updated_at`,
		videoID, text, now); err != nil {
		return fmt.Errorf("notas: salvar: %w", err)
	}
	return nil
}

// Note devolve a nota do vídeo, ou "" quando não existe.
func (r *Repository) Note(ctx context.Context, videoID string) (string, error) {
	var text string
	err := r.db.QueryRowContext(ctx,
		`SELECT text FROM video_notes WHERE video_id = ?`, videoID).Scan(&text)
	if errors.Is(err, context.Canceled) {
		return "", err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("notas: ler: %w", err)
	}
	return text, nil
}

// AddBookmark cria um marcador na posição dada.
func (r *Repository) AddBookmark(ctx context.Context, videoID string, position time.Duration, label string) (domain.VideoBookmark, error) {
	if videoID == "" {
		return domain.VideoBookmark{}, errors.New("notas: vídeo vazio")
	}
	id, err := BookmarkID()
	if err != nil {
		return domain.VideoBookmark{}, err
	}
	now := fmtTime(time.Now())
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO video_bookmarks (id, video_id, position_ms, label, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		id, videoID, position.Milliseconds(), strings.TrimSpace(label), now); err != nil {
		return domain.VideoBookmark{}, fmt.Errorf("notas: marcador: %w", err)
	}
	return domain.VideoBookmark{
		ID: id, VideoID: videoID, Position: position,
		Label: strings.TrimSpace(label), CreatedAt: parseTime(now),
	}, nil
}

// Bookmarks lista os marcadores do vídeo ordenados por posição.
func (r *Repository) Bookmarks(ctx context.Context, videoID string) ([]domain.VideoBookmark, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, video_id, position_ms, label, created_at
		FROM video_bookmarks
		WHERE video_id = ?
		ORDER BY position_ms, created_at`, videoID)
	if err != nil {
		return nil, fmt.Errorf("notas: marcadores: %w", err)
	}
	defer rows.Close()

	var out []domain.VideoBookmark
	for rows.Next() {
		var b domain.VideoBookmark
		var position int64
		var created string
		if err := rows.Scan(&b.ID, &b.VideoID, &position, &b.Label, &created); err != nil {
			return nil, fmt.Errorf("notas: marcadores scan: %w", err)
		}
		b.Position = time.Duration(position) * time.Millisecond
		b.CreatedAt = parseTime(created)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notas: marcadores rows: %w", err)
	}
	return out, nil
}

// DeleteBookmark remove um marcador; remover o que não existe não é erro.
func (r *Repository) DeleteBookmark(ctx context.Context, bookmarkID string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM video_bookmarks WHERE id = ?`, bookmarkID); err != nil {
		return fmt.Errorf("notas: excluir marcador: %w", err)
	}
	return nil
}
