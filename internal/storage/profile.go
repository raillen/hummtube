// Profile derivation for the local recommendation engine
// (docs/03-implementation/RECOMMENDATIONS.md).
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// BuildProfile derives the local InterestProfile from SQLite state:
// interest topics, channel affinity, watch history, playback progress and
// explicit feedback exclusions. It never leaves the machine.
func (r *Repository) BuildProfile(ctx context.Context) (domain.InterestProfile, error) {
	profile := domain.InterestProfile{
		Channels:           make(map[string]domain.ChannelAffinity),
		WatchState:         make(map[string]domain.ProgressInfo),
		ExcludedVideoIDs:   make(map[string]bool),
		ExcludedChannelIDs: make(map[string]bool),
	}

	topics, err := r.interestTopics(ctx)
	if err != nil {
		return profile, err
	}
	profile.Topics = topics

	channels, err := r.channelAffinities(ctx)
	if err != nil {
		return profile, err
	}
	for _, c := range channels {
		profile.Channels[c.ChannelID] = c
	}

	watch, err := r.watchState(ctx)
	if err != nil {
		return profile, err
	}
	profile.WatchState = watch

	exVideos, exChannels, err := r.feedbackExclusions(ctx)
	if err != nil {
		return profile, err
	}
	for _, v := range exVideos {
		profile.ExcludedVideoIDs[v] = true
	}
	for _, c := range exChannels {
		profile.ExcludedChannelIDs[c] = true
	}
	return profile, nil
}

func (r *Repository) interestTopics(ctx context.Context) ([]domain.TopicScore, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT topic, score FROM interest_topics WHERE score > 0 ORDER BY score DESC`)
	if err != nil {
		return nil, fmt.Errorf("interest topics: %w", err)
	}
	defer rows.Close()

	var raw []domain.TopicScore
	max := 0
	for rows.Next() {
		var t domain.TopicScore
		if err := rows.Scan(&t.Topic, &t.Score); err != nil {
			return nil, fmt.Errorf("interest topics scan: %w", err)
		}
		raw = append(raw, t)
		if int(t.Score) > max {
			max = int(t.Score)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("interest topics rows: %w", err)
	}
	if max <= 0 {
		return nil, nil
	}
	out := make([]domain.TopicScore, 0, len(raw))
	for _, t := range raw {
		out = append(out, domain.TopicScore{Topic: t.Topic, Score: t.Score / float64(max)})
	}
	return out, nil
}

func (r *Repository) channelAffinities(ctx context.Context) ([]domain.ChannelAffinity, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.title, c.subscribed,
		       EXISTS(SELECT 1 FROM channel_favorites cf WHERE cf.channel_id = c.id) AS is_fav,
		       COALESCE(SUM(
		           CASE
		               WHEN pp.duration_ms > 0 THEN MIN(pp.position_ms * 1.0 / pp.duration_ms, 1.0)
		               ELSE 0
		           END
		       ), 0) AS watched_vol,
		       COALESCE(COUNT(CASE WHEN pp.completed = 1 OR (pp.duration_ms > 0 AND pp.position_ms >= pp.duration_ms * 0.85) THEN 1 END), 0) AS completed_cnt,
		       COALESCE(MAX(pp.updated_at), '') AS last_watched
		FROM channels c
		LEFT JOIN videos v ON v.channel_id = c.id
		LEFT JOIN playback_progress pp ON pp.video_id = v.id
		GROUP BY c.id, c.title, c.subscribed
		ORDER BY is_fav DESC, c.subscribed DESC, c.title ASC`)
	if err != nil {
		return nil, fmt.Errorf("channel affinities: %w", err)
	}
	defer rows.Close()

	var out []domain.ChannelAffinity
	now := time.Now()
	for rows.Next() {
		var c domain.ChannelAffinity
		var subscribed, isFav int
		var watchedVol float64
		var completedCnt int
		var lastWatchedStr string
		if err := rows.Scan(&c.ChannelID, &c.ChannelName, &subscribed, &isFav, &watchedVol, &completedCnt, &lastWatchedStr); err != nil {
			return nil, fmt.Errorf("channel affinities scan: %w", err)
		}
		c.Subscribed = subscribed == 1
		c.Favorite = isFav == 1
		c.WatchVolume = watchedVol
		c.CompletedCount = completedCnt
		if lastWatchedStr != "" {
			c.LastWatched = parseTime(lastWatchedStr)
		}

		// Scoring v2 refinado e determinístico:
		// - Inscrito: +0.50 (garante afinidade imediata)
		// - Favorito (estrela): +0.30 (destaque máximo)
		// - Volume assistido: +0.20 * min(watched / 5.0, 1.0)
		// - Conclusões consistentes: +0.10 * min(completed / 3.0, 1.0)
		baseScore := 0.0
		if c.Subscribed {
			baseScore += 0.50
		}
		if c.Favorite {
			baseScore += 0.30
		}
		baseScore += 0.20 * minFloat(watchedVol/5.0, 1.0)
		baseScore += 0.10 * minFloat(float64(completedCnt)/3.0, 1.0)

		// Decaimento suave de recência para o volume ativo assistido
		if !c.LastWatched.IsZero() {
			days := now.Sub(c.LastWatched).Hours() / 24.0
			if days > 14.0 {
				// Decaimento progressivo preservando o piso de inscrições/favoritos
				floor := 0.0
				if c.Subscribed {
					floor += 0.50
				}
				if c.Favorite {
					floor += 0.30
				}
				decay := maxFloat(0.0, 1.0-(days-14.0)/60.0)
				baseScore = floor + (baseScore-floor)*decay
			}
		}

		c.Score = minFloat(baseScore, 1.0)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("channel affinities rows: %w", err)
	}
	return out, nil
}

func (r *Repository) watchState(ctx context.Context) (map[string]domain.ProgressInfo, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT video_id, position_ms, duration_ms, updated_at, completed
		FROM playback_progress`)
	if err != nil {
		return nil, fmt.Errorf("watch state: %w", err)
	}
	defer rows.Close()

	out := make(map[string]domain.ProgressInfo)
	for rows.Next() {
		var videoID string
		var position, duration int64
		var updated string
		var completed int
		if err := rows.Scan(&videoID, &position, &duration, &updated, &completed); err != nil {
			return nil, fmt.Errorf("watch state scan: %w", err)
		}
		out[videoID] = domain.ProgressInfo{
			Position:  time.Duration(position) * time.Millisecond,
			Duration:  time.Duration(duration) * time.Millisecond,
			UpdatedAt: parseTime(updated),
			Completed: completed == 1,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("watch state rows: %w", err)
	}
	return out, nil
}

func (r *Repository) feedbackExclusions(ctx context.Context) ([]string, []string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(video_id, ''), COALESCE(channel_id, ''), COALESCE(action, '')
		FROM recommendation_feedback`)
	if err != nil {
		return nil, nil, fmt.Errorf("feedback: %w", err)
	}
	defer rows.Close()

	var videos, channels []string
	for rows.Next() {
		var videoID, channelID, action string
		if err := rows.Scan(&videoID, &channelID, &action); err != nil {
			return nil, nil, fmt.Errorf("feedback scan: %w", err)
		}
		switch action {
		case "dont_recommend", "hide_video":
			if videoID != "" {
				videos = append(videos, videoID)
			}
		case "ignore_channel":
			if channelID != "" {
				channels = append(channels, channelID)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("feedback rows: %w", err)
	}
	return videos, channels, nil
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
