package search

import (
	"fmt"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func BenchmarkNormalizeOptions(b *testing.B) {
	input := domain.SearchOptions{Query: "linux desktop", IncludeTerms: "webview player", RegionCode: "br", MaxResults: 50}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NormalizeOptions(input)
	}
}

func BenchmarkNanoRankPage(b *testing.B) {
	videos := make([]domain.Video, 50)
	for i := range videos {
		videos[i] = domain.Video{
			ID: fmt.Sprintf("video-%d", i), Title: "Linux desktop player", ChannelID: fmt.Sprintf("channel-%d", i%5),
			ChannelTitle: "Canal de tecnologia", DescriptionExcerpt: "WebView leve e reprodução local",
			ViewCount: int64(i * 1000), PublishedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	options := NormalizeOptions(domain.SearchOptions{Query: "linux player"})
	state := PersonalState{SubscribedChannels: map[string]bool{"channel-1": true}, Favorites: map[string]bool{"video-2": true}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		page := append([]domain.Video(nil), videos...)
		RankWithNanoRank(page, options, state, time.Now())
	}
}
