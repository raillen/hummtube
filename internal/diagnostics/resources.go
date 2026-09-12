package diagnostics

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ResourceUsage struct {
	RSSBytes               *int64   `json:"rss_bytes"`
	CPUPercent             *float64 `json:"cpu_percent"`
	ReceiveBytesPerSecond  *float64 `json:"receive_bytes_per_second"`
	TransmitBytesPerSecond *float64 `json:"transmit_bytes_per_second"`
}

type cpuCounters struct{ total, idle uint64 }
type networkCounters struct{ received, sent uint64 }

type ResourceSampler struct {
	mu      sync.Mutex
	at      time.Time
	cpu     *cpuCounters
	network map[string]networkCounters
	cached  ResourceUsage
}

func (s *ResourceSampler) Sample(ctx context.Context) ResourceUsage {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil || runtime.GOOS != "linux" {
		return ResourceUsage{}
	}
	now := time.Now()
	if !s.at.IsZero() && now.Sub(s.at) < 3*time.Second {
		return s.cached
	}
	result := ResourceUsage{}
	if rss, err := RSSBytes(); err == nil && rss >= 0 {
		result.RSSBytes = &rss
	}
	var cpu *cpuCounters
	if content, err := os.ReadFile("/proc/stat"); err == nil {
		cpu = parseCPU(string(content))
	}
	var network map[string]networkCounters
	if content, err := os.ReadFile("/proc/net/dev"); err == nil {
		network = parseNetwork(string(content))
	}
	if s.cpu != nil && cpu != nil && cpu.total > s.cpu.total && cpu.idle >= s.cpu.idle {
		total, idle := cpu.total-s.cpu.total, cpu.idle-s.cpu.idle
		if idle <= total {
			percent := 100 * float64(total-idle) / float64(total)
			result.CPUPercent = &percent
		}
	}
	if !s.at.IsZero() {
		result.ReceiveBytesPerSecond, result.TransmitBytesPerSecond = networkRates(s.network, network, now.Sub(s.at).Seconds())
	}
	s.at, s.cpu, s.network, s.cached = now, cpu, network, result
	return result
}

func parseCPU(content string) *cpuCounters {
	fields := strings.Fields(strings.SplitN(content, "\n", 2)[0])
	if len(fields) < 9 || fields[0] != "cpu" {
		return nil
	}
	result := &cpuCounters{}
	for index, field := range fields[1:9] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil
		}
		result.total += value
		if index == 3 || index == 4 {
			result.idle += value
		}
	}
	return result
}

func parseNetwork(content string) map[string]networkCounters {
	result := make(map[string]networkCounters)
	for _, line := range strings.Split(content, "\n") {
		name, counters, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "lo" {
			continue
		}
		fields := strings.Fields(counters)
		if name == "" || len(fields) != 16 {
			return nil
		}
		received, receiveErr := strconv.ParseUint(fields[0], 10, 64)
		sent, sendErr := strconv.ParseUint(fields[8], 10, 64)
		if receiveErr != nil || sendErr != nil {
			return nil
		}
		result[name] = networkCounters{received, sent}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func networkRates(previous, current map[string]networkCounters, seconds float64) (*float64, *float64) {
	if len(previous) == 0 || len(previous) != len(current) || seconds <= 0 {
		return nil, nil
	}
	var received, sent float64
	for name, next := range current {
		last, ok := previous[name]
		if !ok || next.received < last.received || next.sent < last.sent {
			return nil, nil
		}
		received += float64(next.received-last.received) / seconds
		sent += float64(next.sent-last.sent) / seconds
	}
	return &received, &sent
}
