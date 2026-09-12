// Package suggestions implements the deterministic, local recommendation
// engine v1 (docs/03-implementation/RECOMMENDATIONS.md).
//
// Scoring is a weighted sum of explainable signals over local data only:
// topic affinity, channel affinity, freshness, watch affinity, novelty and
// diversity. No LLM, no embeddings, no network.
package suggestions

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// Ranking weights (docs/03-implementation/RECOMMENDATIONS.md baseline).
const (
	WTopic     = 0.30
	WChannel   = 0.30
	WFresh     = 0.15
	WWatch     = 0.10
	WNovelty   = 0.10
	WDiversity = 0.05
)

// Section sizes (v1 proposal, subject to benchmark).
const (
	ContinueWatchingLimit = 10
	// A Home carrega uma janela local maior para que o botão "Carregar mais"
	// revele novos itens sem repetir uma chamada cara ao banco a cada clique.
	ForYouLimit       = 100
	RecentSubsLimit   = 10
	TopicSectionsMax  = 3
	TopicSectionLimit = 6
	RediscoveryLimit  = 8
)

// Reason kinds for Explain().
const (
	ReasonTopic      = "topic"
	ReasonChannel    = "channel"
	ReasonFresh      = "fresh"
	ReasonWatch      = "watch"
	ReasonSubscribed = "subscribed"
)

// Engine implements domain.RecommendationEngine over a candidate set.
type Engine struct {
	mu      sync.Mutex
	profile domain.InterestProfile
	last    map[string]domain.HomeVideo
}

// NewEngine creates an empty engine. Use SetProfile before BuildHome or pass
// the profile in; BuildHome keeps the last profile/scored set for Explain.
func NewEngine() *Engine {
	return &Engine{last: make(map[string]domain.HomeVideo)}
}

// SetProfile stores the profile used by the next BuildHome/Explain.
func (e *Engine) SetProfile(p domain.InterestProfile) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.profile = p
}

// BuildHome assembles the local Home from candidates, filtering feedback
// exclusions and ranking each section deterministically.
func (e *Engine) BuildHome(ctx context.Context, profile domain.InterestProfile, candidates []domain.Video) (domain.HomeModel, error) {
	e.SetProfile(profile)

	cands := make([]domain.Video, 0, len(candidates))
	for _, v := range candidates {
		if profile.ExcludedVideoIDs[v.ID] || profile.ExcludedChannelIDs[v.ChannelID] {
			continue
		}
		// A Home não mostra Shorts: a duração é a única heurística disponível
		// no catálogo (v0.6.1; docs/02-ui-ux/UI_UX.md "Home").
		if v.IsShort() {
			continue
		}
		cands = append(cands, v)
	}

	scored := make(map[string]domain.HomeVideo, len(cands))
	tokens := make(map[string]map[string]bool, len(cands))
	topicTokens := topicTokenSets(profile.Topics)
	for _, v := range cands {
		vt := tokenSet(v.Title + " " + v.Category)
		s := e.score(v, profile, vt, topicTokens)
		scored[v.ID] = domain.HomeVideo{Video: v, Score: s.Score, Reasons: s.Reasons}
		tokens[v.ID] = vt
	}

	home := domain.HomeModel{
		ContinueWatching:    e.buildContinueWatching(cands, profile, scored),
		ForYou:              e.buildForYou(cands, profile, scored),
		RecentSubscriptions: e.buildRecentSubscriptions(cands, profile, scored),
		Rediscovery:         e.buildRediscovery(cands, profile, scored),
		TopicSections:       e.buildTopicSections(cands, profile, scored, tokens, topicTokens),
	}

	e.mu.Lock()
	e.last = scored
	e.mu.Unlock()
	return home, nil
}

// Explain returns the last deterministic explanation for a video.
func (e *Engine) Explain(videoID string) domain.RecommendationExplanation {
	e.mu.Lock()
	defer e.mu.Unlock()
	hv, ok := e.last[videoID]
	if !ok {
		return domain.RecommendationExplanation{VideoID: videoID}
	}
	reasons := make([]domain.RecommendationReason, 0, len(hv.Reasons))
	for _, r := range hv.Reasons {
		reasons = append(reasons, domain.RecommendationReason{Label: r, Kind: reasonKind(r)})
	}
	return domain.RecommendationExplanation{VideoID: videoID, Score: hv.Score, Reasons: reasons}
}

type scoredVideo struct {
	home  domain.HomeVideo
	score float64
}

func (e *Engine) buildContinueWatching(cands []domain.Video, p domain.InterestProfile, scored map[string]domain.HomeVideo) []domain.HomeVideo {
	var inProgress []scoredVideo
	for _, v := range cands {
		pi, ok := p.WatchState[v.ID]
		if !ok {
			continue
		}
		f := fraction(pi)
		if f > 0 && f <= 0.95 && !pi.Completed {
			inProgress = append(inProgress, scoredVideo{home: scored[v.ID], score: recencyScore(pi.UpdatedAt)})
		}
	}
	sort.SliceStable(inProgress, func(i, j int) bool {
		return inProgress[i].score > inProgress[j].score
	})
	return topN(inProgress, ContinueWatchingLimit)
}

func (e *Engine) buildForYou(cands []domain.Video, p domain.InterestProfile, scored map[string]domain.HomeVideo) []domain.HomeVideo {
	skip := inContinueWatching(cands, p)
	ranked := make([]scoredVideo, 0, len(cands))
	for _, v := range cands {
		if skip[v.ID] {
			continue
		}
		ranked = append(ranked, scoredVideo{home: scored[v.ID], score: scored[v.ID].Score})
	}
	return rankWithDiversity(ranked, ForYouLimit)
}

func (e *Engine) buildRecentSubscriptions(cands []domain.Video, p domain.InterestProfile, scored map[string]domain.HomeVideo) []domain.HomeVideo {
	var out []scoredVideo
	for _, v := range cands {
		ch, ok := p.Channels[v.ChannelID]
		if !ok || !ch.Subscribed {
			continue
		}
		hv := scored[v.ID]
		out = append(out, scoredVideo{home: hv, score: hv.Score})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return topN(out, RecentSubsLimit)
}

func (e *Engine) buildRediscovery(cands []domain.Video, p domain.InterestProfile, scored map[string]domain.HomeVideo) []domain.HomeVideo {
	var out []scoredVideo
	for _, v := range cands {
		pi, ok := p.WatchState[v.ID]
		if !ok {
			continue
		}
		f := fraction(pi)
		finished := pi.Completed || f > 0.95
		old := pi.UpdatedAt.IsZero() || time.Since(pi.UpdatedAt) >= 7*24*time.Hour
		if !finished || !old {
			continue
		}
		hv := scored[v.ID]
		out = append(out, scoredVideo{home: hv, score: hv.Score})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return topN(out, RediscoveryLimit)
}

func (e *Engine) buildTopicSections(cands []domain.Video, p domain.InterestProfile, scored map[string]domain.HomeVideo, tokens map[string]map[string]bool, topicTokens map[string]map[string]bool) []domain.HomeSection {
	top := topTopics(p.Topics, TopicSectionsMax)
	if len(top) == 0 {
		return nil
	}
	skip := inContinueWatching(cands, p)
	sections := make([]domain.HomeSection, 0, len(top))
	for _, topic := range top {
		var ranked []scoredVideo
		for _, v := range cands {
			if skip[v.ID] {
				continue
			}
			overlap := overlap(topicTokens[topic.Topic], tokens[v.ID])
			if overlap <= 0 {
				continue
			}
			hv := scored[v.ID]
			ranked = append(ranked, scoredVideo{home: hv, score: hv.Score})
		}
		videos := rankWithDiversity(ranked, TopicSectionLimit)
		if len(videos) == 0 {
			continue
		}
		sections = append(sections, domain.HomeSection{
			ID:     "topic_" + topic.Topic,
			Title:  strings.ToUpper(topic.Topic),
			Videos: videos,
		})
	}
	return sections
}

// score computes the baseline weighted signals plus a reason list.
func (e *Engine) score(v domain.Video, p domain.InterestProfile, vt map[string]bool, topicTokens map[string]map[string]bool) domain.HomeVideo {
	hv := domain.HomeVideo{Video: v}

	topic := topicAffinity(p, vt, topicTokens)
	channel := channelAffinity(v, p)
	fresh := freshness(v.PublishedAt)
	pi, watched := p.WatchState[v.ID]
	watch := 0.0
	if watched {
		watch = fraction(pi)
	}
	nov := novelty(v, p)

	hv.Score = WTopic*topic + WChannel*channel + WFresh*fresh + WWatch*watch + WNovelty*nov
	hv.Reasons = reasons(v, p, topic, channel, fresh, watch, vt, topicTokens)
	return hv
}

func topicAffinity(p domain.InterestProfile, vt map[string]bool, topicTokens map[string]map[string]bool) float64 {
	if len(p.Topics) == 0 {
		return 0
	}
	best := 0.0
	for _, t := range p.Topics {
		overlap := overlap(topicTokens[t.Topic], vt)
		if overlap > best {
			best = overlap
		}
	}
	return best
}

// overlap is the fraction of topic tokens found in the video token set.
func overlap(topicTokens, vt map[string]bool) float64 {
	if len(topicTokens) == 0 {
		return 0
	}
	if len(vt) == 0 {
		return 0
	}
	matched := 0
	for tok := range topicTokens {
		if vt[tok] {
			matched++
		}
	}
	return float64(matched) / float64(len(topicTokens))
}

// topicTokenSets precomputes the token set of each profile topic so
// per-candidate scoring never re-tokenizes the topic list.
func topicTokenSets(topics []domain.TopicScore) map[string]map[string]bool {
	out := make(map[string]map[string]bool, len(topics))
	for _, t := range topics {
		out[t.Topic] = tokenSet(t.Topic)
	}
	return out
}

func channelAffinity(v domain.Video, p domain.InterestProfile) float64 {
	if ch, ok := p.Channels[v.ChannelID]; ok {
		return ch.Score
	}
	return 0
}

func freshness(published time.Time) float64 {
	if published.IsZero() {
		return 0
	}
	days := time.Since(published).Hours() / 24
	if days < 0 {
		days = 0
	}
	if days > 90 {
		return 0
	}
	return 1 - days/90
}

func novelty(v domain.Video, p domain.InterestProfile) float64 {
	pi, ok := p.WatchState[v.ID]
	if !ok {
		return 1.0
	}
	if pi.UpdatedAt.IsZero() {
		return 0.5
	}
	days := time.Since(pi.UpdatedAt).Hours() / 24
	switch {
	case days >= 30:
		return 0.9
	case days >= 7:
		return 0.5
	default:
		return 0.2
	}
}

func reasons(v domain.Video, p domain.InterestProfile, topic, channel, fresh, watch float64, vt map[string]bool, topicTokens map[string]map[string]bool) []string {
	var out []string
	if topic >= 0.5 {
		t := topTopicFor(p.Topics, vt, topicTokens)
		out = append(out, "tema "+t+" com alta afinidade")
	}
	if ch, ok := p.Channels[v.ChannelID]; ok && channel >= 0.3 {
		if ch.Favorite {
			out = append(out, "canal favorito")
		} else if ch.CompletedCount >= 2 {
			out = append(out, "canal frequentemente assistido")
		} else {
			label := "canal com afinidade"
			if ch.ChannelName != "" {
				label = "canal " + ch.ChannelName
			}
			out = append(out, label)
		}
		if ch.Subscribed && !ch.Favorite {
			out = append(out, "canal assinado")
		}
	}
	if fresh >= 0.7 {
		out = append(out, "upload recente")
	}
	if watch > 0 {
		out = append(out, "você já assistiu este vídeo")
	}
	return out
}

func topTopicFor(topics []domain.TopicScore, vt map[string]bool, topicTokens map[string]map[string]bool) string {
	best, bestTopic := 0.0, ""
	for _, t := range topics {
		if o := overlap(topicTokens[t.Topic], vt); o > best {
			best, bestTopic = o, t.Topic
		}
	}
	return bestTopic
}

func rankWithDiversity(items []scoredVideo, n int) []domain.HomeVideo {
	if len(items) == 0 || n <= 0 {
		return nil
	}
	remaining := make([]scoredVideo, len(items))
	copy(remaining, items)

	picks := make(map[string]int)
	out := make([]domain.HomeVideo, 0, min(n, len(items)))

	for len(out) < n && len(remaining) > 0 {
		bestIdx := -1
		bestScore := -1e9

		for i, it := range remaining {
			effectiveScore := it.score + WDiversity*(1.0/float64(1+picks[it.home.Video.ChannelID]))
			if effectiveScore > bestScore {
				bestScore = effectiveScore
				bestIdx = i
			}
		}

		if bestIdx == -1 {
			break
		}

		selected := remaining[bestIdx]
		picks[selected.home.Video.ChannelID]++
		out = append(out, selected.home)

		remaining[bestIdx] = remaining[len(remaining)-1]
		remaining = remaining[:len(remaining)-1]
	}
	return out
}

func inContinueWatching(cands []domain.Video, p domain.InterestProfile) map[string]bool {
	skip := make(map[string]bool)
	for _, v := range cands {
		pi, ok := p.WatchState[v.ID]
		if !ok {
			continue
		}
		f := fraction(pi)
		if f > 0 && f <= 0.95 && !pi.Completed {
			skip[v.ID] = true
		}
	}
	return skip
}

func fraction(pi domain.ProgressInfo) float64 {
	if pi.Duration <= 0 {
		return 0
	}
	f := float64(pi.Position) / float64(pi.Duration)
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func recencyScore(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	return float64(t.Unix())
}

func topTopics(topics []domain.TopicScore, n int) []domain.TopicScore {
	sorted := make([]domain.TopicScore, len(topics))
	copy(sorted, topics)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

func topN(in []scoredVideo, n int) []domain.HomeVideo {
	out := make([]domain.HomeVideo, 0, min(n, len(in)))
	for _, s := range in {
		if len(out) >= n {
			break
		}
		out = append(out, s.home)
	}
	return out
}

func tokenSet(s string) map[string]bool {
	out := make(map[string]bool)
	for _, tok := range domain.TopicTokens(s) {
		out[tok] = true
	}
	return out
}

func reasonKind(label string) string {
	switch {
	case strings.HasPrefix(label, "tema "):
		return ReasonTopic
	case strings.HasPrefix(label, "canal assinado"):
		return ReasonSubscribed
	case strings.HasPrefix(label, "canal "):
		return ReasonChannel
	case label == "upload recente":
		return ReasonFresh
	case strings.Contains(label, "assistiu"):
		return ReasonWatch
	}
	return ReasonTopic
}
