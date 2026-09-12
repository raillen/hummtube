package suggestions

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func benchProfile(n int) domain.InterestProfile {
	topics := []domain.TopicScore{
		{Topic: "linux", Score: 1.0},
		{Topic: "golang", Score: 0.8},
		{Topic: "rust", Score: 0.5},
	}
	profile := domain.InterestProfile{
		Topics:             topics,
		Channels:           make(map[string]domain.ChannelAffinity, n),
		WatchState:         make(map[string]domain.ProgressInfo, n),
		ExcludedVideoIDs:   map[string]bool{},
		ExcludedChannelIDs: map[string]bool{},
	}
	for i := 0; i < n; i++ {
		cid := fmt.Sprintf("ch-%d", i)
		profile.Channels[cid] = domain.ChannelAffinity{ChannelID: cid, Score: 0.5 + float64(i%5)/10, Subscribed: i%3 == 0}
	}
	for i := 0; i < n; i++ {
		profile.WatchState[fmt.Sprintf("v-w%d", i)] = domain.ProgressInfo{
			Position: time.Minute, Duration: 10 * time.Minute, UpdatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return profile
}

func benchCandidates(n int) []domain.Video {
	now := time.Now()
	out := make([]domain.Video, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, domain.Video{
			ID:          fmt.Sprintf("v-%d", i),
			ChannelID:   fmt.Sprintf("ch-%d", i%20),
			Title:       fmt.Sprintf("Tutorial de linux e golang número %d", i),
			Category:    "Technology",
			PublishedAt: now.Add(-time.Duration(i) * time.Hour),
		})
	}
	return out
}

func BenchmarkBuildHome(b *testing.B) {
	sizes := []int{50, 500, 2000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("candidates_%d", n), func(b *testing.B) {
			eng := NewEngine()
			profile := benchProfile(100)
			cands := benchCandidates(n)
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := eng.BuildHome(ctx, profile, cands); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
