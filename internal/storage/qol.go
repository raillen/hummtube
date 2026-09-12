// Operações de estado e histórico (M9/QOL-01): marcar assistido, resetar
// progresso e limpar histórico. Tudo é estado local, nada é enviado ao YouTube.
package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// SetWatched marca o vídeo como assistido: mantém a linha de progresso com
// completed=1 (o vídeo continua no histórico, sem barra de retomada).
func (r *Repository) SetWatched(ctx context.Context, videoID string, duration time.Duration) error {
	if videoID == "" {
		return errors.New("progresso: video id vazio")
	}
	if duration <= 0 {
		return errors.New("progresso: duração inválida")
	}
	return r.MarkProgress(ctx, videoID, domain.PlaybackProgress{
		VideoID:   videoID,
		Duration:  duration,
		UpdatedAt: time.Now(),
		Completed: true,
	})
}

// MarkUnwatched apaga a linha de progresso do vídeo: ele sai do histórico e
// do "continuar assistindo" (equivale a nunca ter sido reproduzido).
func (r *Repository) MarkUnwatched(ctx context.Context, videoID string) error {
	if videoID == "" {
		return errors.New("progresso: video id vazio")
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM playback_progress WHERE video_id = ?`, videoID)
	if err != nil {
		return fmt.Errorf("marcar não assistido %s: %w", videoID, err)
	}
	return nil
}

// ResetProgress zera posição e completed do vídeo, mantendo a duração: ele
// volta para o início e continua no histórico.
func (r *Repository) ResetProgress(ctx context.Context, videoID string) error {
	if videoID == "" {
		return errors.New("progresso: video id vazio")
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE playback_progress
		SET position_ms = 0, updated_at = ?, completed = 0
		WHERE video_id = ?`, fmtTime(time.Now()), videoID); err != nil {
		return fmt.Errorf("resetar progresso %s: %w", videoID, err)
	}
	return nil
}

// ClearHistory apaga todo o histórico de reprodução (linhas de progresso).
func (r *Repository) ClearHistory(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM playback_progress`); err != nil {
		return fmt.Errorf("limpar histórico: %w", err)
	}
	return nil
}
