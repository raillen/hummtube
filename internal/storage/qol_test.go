package storage

import (
	"context"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func seedProgress(t *testing.T, r *Repository) {
	t.Helper()
	ctx := context.Background()
	for id, dur := range map[string]time.Duration{"v1": 10 * time.Minute, "v2": 5 * time.Minute, "v3": 2 * time.Minute} {
		if err := r.MarkProgress(ctx, id, domain.PlaybackProgress{
			VideoID:   id,
			Position:  dur / 2,
			Duration:  dur,
			UpdatedAt: time.Now(),
			Completed: false,
		}); err != nil {
			t.Fatalf("seed progress %s: %v", id, err)
		}
	}
}

func countProgress(t *testing.T, r *Repository) int {
	t.Helper()
	var n int
	if err := r.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM playback_progress`).Scan(&n); err != nil {
		t.Fatalf("count progress: %v", err)
	}
	return n
}

func TestMarkUnwatchedRemovesFromHistory(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)
	seedProgress(t, r)

	hist, err := r.History(ctx, 50)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 3 {
		t.Fatalf("histórico inicial = %d, esperado 3", len(hist))
	}

	if err := r.MarkUnwatched(ctx, "v1"); err != nil {
		t.Fatalf("MarkUnwatched: %v", err)
	}
	hist, _ = r.History(ctx, 50)
	if len(hist) != 2 {
		t.Fatalf("histórico após marcar não assistido = %d, esperado 2", len(hist))
	}
	for _, v := range hist {
		if v.ID == "v1" {
			t.Fatal("v1 deveria sair do histórico")
		}
	}
}

func TestSetWatchedMarksCompleted(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)
	seedProgress(t, r)

	if err := r.SetWatched(ctx, "v1", 10*time.Minute); err != nil {
		t.Fatalf("SetWatched: %v", err)
	}
	var completed int
	if err := r.db.QueryRowContext(ctx,
		`SELECT completed FROM playback_progress WHERE video_id = 'v1'`).Scan(&completed); err != nil {
		t.Fatalf("leitura: %v", err)
	}
	if completed != 1 {
		t.Fatalf("completed = %d, esperado 1", completed)
	}
	// Continua no histórico, mas com barra cheia (ProgressInfo.Completed).
	info, ok := progressOf(t, r, "v1")
	if !ok || !info.Completed {
		t.Fatalf("ProgressInfo deveria refletir completed: %+v ok=%v", info, ok)
	}
}

func TestResetProgressClearsPosition(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)
	seedProgress(t, r)

	if err := r.ResetProgress(ctx, "v1"); err != nil {
		t.Fatalf("ResetProgress: %v", err)
	}
	info, ok := progressOf(t, r, "v1")
	if !ok {
		t.Fatal("resetar progresso deveria manter a linha")
	}
	if info.Position != 0 || info.Completed {
		t.Fatalf("progresso após reset = %+v", info)
	}
}

func TestClearHistoryWipesProgress(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	seedLibrary(t, r)
	seedProgress(t, r)

	if err := r.ClearHistory(ctx); err != nil {
		t.Fatalf("ClearHistory: %v", err)
	}
	if n := countProgress(t, r); n != 0 {
		t.Fatalf("histórico não zerou: %d linhas", n)
	}
}

func TestQOLEdgeCases(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	if err := r.SetWatched(ctx, "", time.Minute); err == nil {
		t.Error("id vazio deveria ser recusado no SetWatched")
	}
	if err := r.SetWatched(ctx, "v1", 0); err == nil {
		t.Error("duração inválida deveria ser recusada no SetWatched")
	}
	if err := r.MarkUnwatched(ctx, "inexistente"); err != nil {
		t.Errorf("marcar não assistido ausente deveria ser no-op: %v", err)
	}
	if err := r.ResetProgress(ctx, "inexistente"); err != nil {
		t.Errorf("resetar progresso ausente deveria ser no-op: %v", err)
	}
	if err := r.ClearHistory(ctx); err != nil {
		t.Errorf("limpar histórico vazio deveria ser no-op: %v", err)
	}
}

// progressOf lê a linha de progresso de um vídeo.
func progressOf(t *testing.T, r *Repository, videoID string) (domain.ProgressInfo, bool) {
	t.Helper()
	rows, err := r.db.QueryContext(context.Background(), `
		SELECT position_ms, duration_ms, updated_at, completed
		FROM playback_progress WHERE video_id = ?`, videoID)
	if err != nil {
		t.Fatalf("query progresso: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return domain.ProgressInfo{}, false
	}
	var posMS, durMS int64
	var updated string
	var completed int
	if err := rows.Scan(&posMS, &durMS, &updated, &completed); err != nil {
		t.Fatalf("scan progresso: %v", err)
	}
	return domain.ProgressInfo{
		Position:  time.Duration(posMS) * time.Millisecond,
		Duration:  time.Duration(durMS) * time.Millisecond,
		UpdatedAt: parseTime(updated),
		Completed: completed != 0,
	}, true
}
