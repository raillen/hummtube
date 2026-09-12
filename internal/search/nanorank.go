package search

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// RankWithNanoRank reordena apenas a página recebida. Ele preserva um peso
// forte para a relevância do provider e combina sinais locais sem enviar
// inscrições, favoritos, fila ou histórico para a rede.
func RankWithNanoRank(items []domain.Video, options domain.SearchOptions, state PersonalState, now time.Time) {
	if len(items) < 2 || options.Order != domain.SearchOrderRelevance {
		return
	}
	type rankedResult struct {
		video domain.Video
		score float64
		index int
	}
	ranked := make([]rankedResult, 0, len(items))
	for index, video := range items {
		ranked = append(ranked, rankedResult{
			video: video,
			score: nanoRankScore(video, index, options, state, now),
			index: index,
		})
	}
	sort.SliceStable(ranked, func(left, right int) bool {
		if ranked[left].score == ranked[right].score {
			return ranked[left].index < ranked[right].index
		}
		return ranked[left].score > ranked[right].score
	})
	for index := range ranked {
		items[index] = ranked[index].video
	}
}

func nanoRankScore(video domain.Video, providerIndex int, options domain.SearchOptions, state PersonalState, now time.Time) float64 {
	title := strings.ToLower(video.Title)
	channel := strings.ToLower(video.ChannelTitle)
	description := strings.ToLower(video.DescriptionExcerpt)
	query := strings.ToLower(strings.TrimSpace(options.Query))
	exactPhrase := strings.ToLower(strings.Trim(strings.TrimSpace(options.ExactPhrase), `"`))

	// A ordem remota permanece o maior sinal isolado. O denominador suave
	// permite que correspondências claras e preferências locais desempatem.
	score := 4 / (1 + float64(providerIndex)*0.2)
	if query != "" {
		if strings.Contains(title, query) {
			score += 4
		}
		for _, term := range strings.Fields(query + " " + strings.ToLower(options.IncludeTerms)) {
			term = strings.Trim(term, `"`)
			if term == "" {
				continue
			}
			if strings.Contains(title, term) {
				score += 1.5
			}
			if strings.Contains(channel, term) {
				score += 0.75
			}
			if strings.Contains(description, term) {
				score += 0.2
			}
		}
	}
	if exactPhrase != "" && strings.Contains(title, exactPhrase) {
		score += 6
	}
	if state.SubscribedChannels[video.ChannelID] {
		score += 1
	}
	if state.Favorites[video.ID] {
		score += 0.75
	}
	if state.Queued[video.ID] {
		score += 0.25
	}
	if video.ViewCount > 0 {
		score += math.Min(0.8, math.Log10(float64(video.ViewCount)+1)*0.1)
	}
	if !video.PublishedAt.IsZero() && !now.IsZero() {
		days := now.Sub(video.PublishedAt).Hours() / 24
		if days >= 0 && days < 365 {
			score += 0.75 * (1 - days/365)
		}
	}
	return score
}
