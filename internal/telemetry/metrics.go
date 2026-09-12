// Package telemetry keeps low-cardinality process metrics used by diagnostics.
// It intentionally stores counters and timings only: no URL, profile id,
// video id, query or provider error is retained here.
package telemetry

import (
	"sync/atomic"
	"time"
)

// Snapshot is a point-in-time view of the lightweight runtime counters.
// Durations are expressed in microseconds to keep the diagnostics contract
// independent of Go's time formatting.
type Snapshot struct {
	PlaybackRequests       uint64 `json:"playback_requests"`
	PlaybackCacheHits      uint64 `json:"playback_cache_hits"`
	PlaybackErrors         uint64 `json:"playback_errors"`
	PlaybackLatencyTotalUS uint64 `json:"playback_latency_total_us"`
	PlaybackLatencyMaxUS   uint64 `json:"playback_latency_max_us"`
	SearchRequests         uint64 `json:"search_requests"`
	SearchErrors           uint64 `json:"search_errors"`
	SearchLatencyTotalUS   uint64 `json:"search_latency_total_us"`
	SearchLatencyMaxUS     uint64 `json:"search_latency_max_us"`
}

// Metrics is safe to share between concurrent resolver and search calls.
// The fields are private so new observations cannot accidentally expose a
// high-cardinality label or sensitive request data.
type Metrics struct {
	playbackRequests       atomic.Uint64
	playbackCacheHits      atomic.Uint64
	playbackErrors         atomic.Uint64
	playbackLatencyTotalUS atomic.Uint64
	playbackLatencyMaxUS   atomic.Uint64
	searchRequests         atomic.Uint64
	searchErrors           atomic.Uint64
	searchLatencyTotalUS   atomic.Uint64
	searchLatencyMaxUS     atomic.Uint64
}

var processMetrics Metrics

// Process returns the process-wide recorder used by the application.
func Process() *Metrics { return &processMetrics }

// ObservePlayback records one resolver call and whether it was served by the
// in-memory plan cache. It does not retain request identity or error details.
func (m *Metrics) ObservePlayback(elapsed time.Duration, cacheHit bool, err error) {
	if m == nil {
		return
	}
	m.playbackRequests.Add(1)
	if cacheHit {
		m.playbackCacheHits.Add(1)
	}
	if err != nil {
		m.playbackErrors.Add(1)
	}
	recordDuration(&m.playbackLatencyTotalUS, &m.playbackLatencyMaxUS, elapsed)
}

// ObserveSearch records one remote/local search call without storing the
// query, profile, page token or provider response.
func (m *Metrics) ObserveSearch(elapsed time.Duration, err error) {
	if m == nil {
		return
	}
	m.searchRequests.Add(1)
	if err != nil {
		m.searchErrors.Add(1)
	}
	recordDuration(&m.searchLatencyTotalUS, &m.searchLatencyMaxUS, elapsed)
}

// Snapshot returns counters suitable for diagnostics and support bundles.
func (m *Metrics) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	return Snapshot{
		PlaybackRequests:       m.playbackRequests.Load(),
		PlaybackCacheHits:      m.playbackCacheHits.Load(),
		PlaybackErrors:         m.playbackErrors.Load(),
		PlaybackLatencyTotalUS: m.playbackLatencyTotalUS.Load(),
		PlaybackLatencyMaxUS:   m.playbackLatencyMaxUS.Load(),
		SearchRequests:         m.searchRequests.Load(),
		SearchErrors:           m.searchErrors.Load(),
		SearchLatencyTotalUS:   m.searchLatencyTotalUS.Load(),
		SearchLatencyMaxUS:     m.searchLatencyMaxUS.Load(),
	}
}

func recordDuration(total, maximum *atomic.Uint64, elapsed time.Duration) {
	value := uint64(elapsed / time.Microsecond)
	total.Add(value)
	for current := maximum.Load(); value > current; {
		if maximum.CompareAndSwap(current, value) {
			return
		}
		current = maximum.Load()
	}
}
