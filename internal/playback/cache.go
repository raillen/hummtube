package playback

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const (
	defaultPlaybackCacheTTL     = 2 * time.Minute
	maximumPlaybackCacheTTL     = 10 * time.Minute
	mediaExpirySafetyMargin     = 90 * time.Second
	maximumPlaybackCacheEntries = 64
)

type playbackCacheEntry struct {
	plan       domain.PlaybackPlan
	validUntil time.Time
}

func playbackRequestKey(req domain.PlaybackRequest) string {
	return strings.TrimSpace(req.VideoID) + "\x00" + strings.TrimSpace(req.SourceURL)
}

func (c *CascadingResolver) cachedPlan(key string) (domain.PlaybackPlan, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[key]
	if !ok || !c.clockNow().Before(entry.validUntil) {
		delete(c.cache, key)
		return domain.PlaybackPlan{}, false
	}
	return clonePlaybackPlan(entry.plan), true
}

func (c *CascadingResolver) storePlan(key string, plan domain.PlaybackPlan) {
	if plan.Mode != domain.PlaybackModeResolvedMedia || strings.TrimSpace(plan.Primary.URL) == "" {
		return
	}
	now := c.clockNow()
	validUntil := playbackPlanCacheDeadline(plan, now)
	if !now.Before(validUntil) {
		return
	}
	c.mu.Lock()
	c.pruneCacheLocked(now)
	c.cache[key] = playbackCacheEntry{plan: clonePlaybackPlan(plan), validUntil: validUntil}
	c.mu.Unlock()
}

func (c *CascadingResolver) pruneCacheLocked(now time.Time) {
	for key, entry := range c.cache {
		if !now.Before(entry.validUntil) {
			delete(c.cache, key)
		}
	}
	if len(c.cache) < maximumPlaybackCacheEntries {
		return
	}
	var oldestKey string
	var oldestDeadline time.Time
	for key, entry := range c.cache {
		if oldestKey == "" || entry.validUntil.Before(oldestDeadline) {
			oldestKey = key
			oldestDeadline = entry.validUntil
		}
	}
	delete(c.cache, oldestKey)
}

func playbackPlanCacheDeadline(plan domain.PlaybackPlan, now time.Time) time.Time {
	expiresAt := plan.ExpiresAt
	for _, streamURL := range playbackPlanURLs(plan) {
		candidate := mediaURLExpiry(streamURL)
		if candidate.IsZero() || (!expiresAt.IsZero() && !candidate.Before(expiresAt)) {
			continue
		}
		expiresAt = candidate
	}
	if expiresAt.IsZero() {
		expiresAt = now.Add(defaultPlaybackCacheTTL)
	}
	deadline := expiresAt.Add(-mediaExpirySafetyMargin)
	maximum := now.Add(maximumPlaybackCacheTTL)
	if maximum.Before(deadline) {
		deadline = maximum
	}
	return deadline
}

func playbackPlanURLs(plan domain.PlaybackPlan) []string {
	urls := []string{plan.Primary.URL}
	if plan.Audio != nil {
		urls = append(urls, plan.Audio.URL)
	}
	if plan.AudioOnly != nil {
		urls = append(urls, plan.AudioOnly.URL)
	}
	for _, variant := range plan.Variants {
		urls = append(urls, variant.Stream.URL)
	}
	return urls
}

func mediaURLExpiry(rawURL string) time.Time {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return time.Time{}
	}
	rawExpiry := parsed.Query().Get("expire")
	if rawExpiry == "" {
		rawExpiry = parsed.Query().Get("expires")
	}
	seconds, err := strconv.ParseInt(rawExpiry, 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}

func clonePlaybackPlan(plan domain.PlaybackPlan) domain.PlaybackPlan {
	clone := plan
	clone.Options = cloneStringMap(plan.Options)
	clone.Primary.Headers = cloneStringMap(plan.Primary.Headers)
	if plan.Audio != nil {
		audio := *plan.Audio
		audio.Headers = cloneStringMap(plan.Audio.Headers)
		clone.Audio = &audio
	}
	if plan.AudioOnly != nil {
		audioOnly := *plan.AudioOnly
		audioOnly.Headers = cloneStringMap(plan.AudioOnly.Headers)
		clone.AudioOnly = &audioOnly
	}
	clone.Variants = append([]domain.PlaybackVariant(nil), plan.Variants...)
	for index := range clone.Variants {
		clone.Variants[index].Stream.Headers = cloneStringMap(plan.Variants[index].Stream.Headers)
	}
	clone.Subtitles = append([]domain.SubtitleTrack(nil), plan.Subtitles...)
	return clone
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
