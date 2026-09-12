package storage

import (
	"context"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestBuildProfile(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	channels := []domain.Channel{
		{ID: "ch-1", Title: "Canal Linux", Subscribed: true},
		{ID: "ch-2", Title: "Canal Culinária", Subscribed: false},
	}
	for _, c := range channels {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO channels (id, title, subscribed, last_sync_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET title = excluded.title, subscribed = excluded.subscribed`,
			c.ID, c.Title, boolInt(c.Subscribed), fmtTime(c.LastSyncAt)); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.UpsertVideos(ctx, []domain.Video{
		{ID: "v-1", ChannelID: "ch-1", Title: "Tutorial de linux", Category: "Technology", PublishedAt: time.Now()},
		{ID: "v-2", ChannelID: "ch-2", Title: "Receitas rápidas", Category: "Howto", PublishedAt: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkProgress(ctx, "v-1", domain.PlaybackProgress{
		Position: 30 * time.Minute, Duration: 60 * time.Minute, UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	r.db.ExecContext(ctx, `INSERT INTO interest_topics (topic, score, updated_at) VALUES ('linux', 5, ?), ('culinaria', 2, ?)`, fmtTime(time.Now()), fmtTime(time.Now()))
	r.db.ExecContext(ctx, `INSERT INTO recommendation_feedback (video_id, action, created_at) VALUES ('v-2', 'dont_recommend', ?)`, fmtTime(time.Now()))
	r.db.ExecContext(ctx, `INSERT INTO recommendation_feedback (channel_id, action, created_at) VALUES ('ch-2', 'ignore_channel', ?)`, fmtTime(time.Now()))

	profile, err := r.BuildProfile(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(profile.Topics) != 2 {
		t.Fatalf("topics = %d, want 2", len(profile.Topics))
	}
	if profile.Topics[0].Topic != "linux" || profile.Topics[0].Score != 1.0 {
		t.Errorf("topics[0] = %+v, want linux/1.0 (normalizado)", profile.Topics[0])
	}
	if !profile.ExcludedVideoIDs["v-2"] {
		t.Error("v-2 deveria estar excluído")
	}
	if !profile.ExcludedChannelIDs["ch-2"] {
		t.Error("ch-2 deveria estar excluído")
	}
	ch1, ok := profile.Channels["ch-1"]
	if !ok || !ch1.Subscribed {
		t.Errorf("ch-1 deveria estar assinado e presente: %+v", ch1)
	}
	if ch1.Score < 0.5 {
		t.Errorf("ch-1 score = %v, want >= 0.5 (assinado)", ch1.Score)
	}
	pi, ok := profile.WatchState["v-1"]
	if !ok {
		t.Fatal("watch state de v-1 ausente")
	}
	if pi.Position != 30*time.Minute || pi.Duration != 60*time.Minute {
		t.Errorf("progress = %+v", pi)
	}
}
