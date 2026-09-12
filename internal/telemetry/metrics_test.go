package telemetry

import (
	"errors"
	"testing"
	"time"
)

func TestMetricsRecordCountersAndDurations(t *testing.T) {
	var metrics Metrics
	metrics.ObservePlayback(12*time.Millisecond, true, nil)
	metrics.ObservePlayback(4*time.Millisecond, false, errors.New("provider falhou"))
	metrics.ObserveSearch(8*time.Millisecond, nil)
	metrics.ObserveSearch(2*time.Millisecond, errors.New("busca falhou"))

	snapshot := metrics.Snapshot()
	if snapshot.PlaybackRequests != 2 || snapshot.PlaybackCacheHits != 1 || snapshot.PlaybackErrors != 1 {
		t.Fatalf("playback counters = %+v", snapshot)
	}
	if snapshot.SearchRequests != 2 || snapshot.SearchErrors != 1 {
		t.Fatalf("search counters = %+v", snapshot)
	}
	if snapshot.PlaybackLatencyTotalUS != 16_000 || snapshot.PlaybackLatencyMaxUS != 12_000 {
		t.Fatalf("playback timings = %+v", snapshot)
	}
	if snapshot.SearchLatencyTotalUS != 10_000 || snapshot.SearchLatencyMaxUS != 8_000 {
		t.Fatalf("search timings = %+v", snapshot)
	}
}

func TestMetricsNilReceiverIsSafe(t *testing.T) {
	var metrics *Metrics
	metrics.ObservePlayback(time.Second, false, errors.New("ignored"))
	metrics.ObserveSearch(time.Second, errors.New("ignored"))
	if snapshot := metrics.Snapshot(); snapshot != (Snapshot{}) {
		t.Fatalf("nil snapshot = %+v", snapshot)
	}
}
