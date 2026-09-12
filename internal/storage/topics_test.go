package storage

import (
	"context"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func seedWatchedVideo(t *testing.T, r *Repository, id, title, category string) {
	t.Helper()
	ctx := context.Background()
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO videos (id, title, category, published_at, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, title, category, fmtTime(time.Now()), fmtTime(time.Now()), fmtTime(time.Now())); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO playback_progress (video_id, position_ms, duration_ms, updated_at, completed)
		VALUES (?, 1000, 2000, ?, 0)`,
		id, fmtTime(time.Now())); err != nil {
		t.Fatalf("seed progress: %v", err)
	}
}

func topicScore(t *testing.T, r *Repository, topic string) (float64, bool) {
	t.Helper()
	var score float64
	err := r.db.QueryRow(`SELECT score FROM interest_topics WHERE topic = ?`, topic).
		Scan(&score)
	if err != nil {
		return 0, false
	}
	return score, true
}

func TestRefreshInterestTopicsSeedsFromHistory(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	seedWatchedVideo(t, r, "v1", "Tutorial Linux para iniciantes", "Tecnologia")
	seedWatchedVideo(t, r, "v2", "Linux avançado no terminal", "")
	seedWatchedVideo(t, r, "v3", "Receita de bolo simples", "Culinária")
	// Token único (count 1) não vira tema.
	seedWatchedVideo(t, r, "v4", "Astronomia", "")

	if err := r.RefreshInterestTopics(ctx); err != nil {
		t.Fatalf("RefreshInterestTopics: %v", err)
	}
	if s, ok := topicScore(t, r, "linux"); !ok || s < 2 {
		t.Fatalf("tema linux = %v, %v", s, ok)
	}
	if _, ok := topicScore(t, r, "astronomia"); ok {
		t.Fatal("token com count 1 não deveria virar tema")
	}

	// Idempotente: rodar de novo não duplica nem sobrescreve.
	if err := r.RefreshInterestTopics(ctx); err != nil {
		t.Fatalf("RefreshInterestTopics 2: %v", err)
	}
	if s, _ := topicScore(t, r, "linux"); s != 2 {
		t.Fatalf("linux após re-seed = %v", s)
	}
}

func TestRefreshInterestTopicsPreservesManualAdjustments(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	seedWatchedVideo(t, r, "v1", "Tutorial Linux parte 1", "")
	seedWatchedVideo(t, r, "v2", "Tutorial Linux parte 2", "")

	// Redução em tema inexistente é no-op, não erro.
	if err := r.AdjustTopicScore(ctx, "inexistente", -1); err != nil {
		t.Fatalf("redução em tema inexistente: %v", err)
	}
	if err := r.RecordFeedback(ctx, domain.RecommendationFeedback{
		VideoID: "v1", Topic: "tutorial", Action: domain.FeedbackMoreTopic,
	}); err != nil {
		t.Fatalf("RecordFeedback more_topic: %v", err)
	}
	if err := r.RefreshInterestTopics(ctx); err != nil {
		t.Fatalf("RefreshInterestTopics: %v", err)
	}
	// O ajuste manual (score 1) deve sobreviver ao re-seed.
	if s, ok := topicScore(t, r, "tutorial"); !ok || s != 1 {
		t.Fatalf("ajuste manual perdido: tutorial = %v, %v", s, ok)
	}
}

func TestTopicFeedbackAdjustsScores(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	fb := func(action domain.FeedbackAction) error {
		return r.RecordFeedback(ctx, domain.RecommendationFeedback{
			VideoID: "v1", Topic: "culinaria", Action: action,
		})
	}

	if err := fb(domain.FeedbackMoreTopic); err != nil {
		t.Fatalf("more: %v", err)
	}
	if err := fb(domain.FeedbackMoreTopic); err != nil {
		t.Fatalf("more 2: %v", err)
	}
	if s, ok := topicScore(t, r, "culinaria"); !ok || s != 2 {
		t.Fatalf("culinaria após 2 reforços = %v, %v", s, ok)
	}

	if err := fb(domain.FeedbackLessTopic); err != nil {
		t.Fatalf("less: %v", err)
	}
	if s, _ := topicScore(t, r, "culinaria"); s != 1 {
		t.Fatalf("culinaria após redução = %v", s)
	}

	// Em 0 o tema sai do perfil.
	if err := fb(domain.FeedbackLessTopic); err != nil {
		t.Fatalf("less 2: %v", err)
	}
	if _, ok := topicScore(t, r, "culinaria"); ok {
		t.Fatal("tema em score 0 deveria ser removido")
	}
	// A linha de feedback permanece como registro/auditoria.
	var n int
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM recommendation_feedback WHERE topic = 'culinaria'`).Scan(&n); err != nil {
		t.Fatalf("count feedback: %v", err)
	}
	if n != 4 {
		t.Fatalf("linhas de feedback = %d", n)
	}
}

func TestTopicFeedbackRequiresTopic(t *testing.T) {
	r := newTestRepo(t)
	err := r.RecordFeedback(context.Background(), domain.RecommendationFeedback{
		VideoID: "v1", Action: domain.FeedbackMoreTopic,
	})
	if err == nil {
		t.Fatal("more_topic sem tema deveria falhar")
	}
}

func TestBuildProfileConsumesTopicScores(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	seedWatchedVideo(t, r, "v1", "Primeiro vídeo de linux", "")
	seedWatchedVideo(t, r, "v2", "Segundo vídeo de linux", "")
	if err := r.RefreshInterestTopics(ctx); err != nil {
		t.Fatalf("RefreshInterestTopics: %v", err)
	}
	profile, err := r.BuildProfile(ctx)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if len(profile.Topics) == 0 {
		t.Fatal("perfil sem temas após seed")
	}
	if profile.Topics[0].Topic != "linux" || profile.Topics[0].Score != 1 {
		t.Fatalf("topico principal = %+v", profile.Topics[0])
	}
}
