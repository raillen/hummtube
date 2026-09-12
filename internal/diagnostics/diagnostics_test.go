package diagnostics

import (
	"context"
	"runtime"
	"testing"
)

func TestCollectRuns(t *testing.T) {
	ctx := context.Background()
	rep := Collect(ctx)
	if rep == nil {
		t.Fatal("Collect retornou nil")
	}
	if rep.Framework == "" {
		t.Errorf("Framework vazio no relatório")
	}
}

func TestParseCPUSample(t *testing.T) {
	got := parseCPU("cpu  2255 34 2290 22625563 6290 127 456 0 0 0\ncpu0 1132 17 1441 11311771\n")
	if got == nil || got.total != 22637015 || got.idle != 22631853 {
		t.Fatalf("parseCPU inesperado: %+v", got)
	}
	if parseCPU("bogus") != nil {
		t.Fatal("parseCPU deveria rejeitar entrada inválida")
	}
}

func TestParseNetworkSkipsLoopback(t *testing.T) {
	sample := "Inter-|   Receive                                                |  Transmit\n" +
		" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n" +
		"    lo: 100 0 0 0 0 0 0 0 200 0 0 0 0 0 0 0\n" +
		"  eth0: 1000 0 0 0 0 0 0 0 3000 0 0 0 0 0 0 0\n"
	got := parseNetwork(sample)
	if len(got) != 1 || got["eth0"].received != 1000 || got["eth0"].sent != 3000 {
		t.Fatalf("parseNetwork inesperado: %+v", got)
	}
	if parseNetwork("malformed") != nil {
		t.Fatal("parseNetwork deveria rejeitar entrada inválida")
	}
}

func TestNetworkRatesRejectsResets(t *testing.T) {
	prev := map[string]networkCounters{"eth0": {100, 200}}
	if rx, tx := networkRates(prev, map[string]networkCounters{"eth0": {50, 200}}, 5); rx != nil || tx != nil {
		t.Fatal("contador reiniciado deveria retornar nil")
	}
	rx, tx := networkRates(prev, map[string]networkCounters{"eth0": {200, 400}}, 5)
	if rx == nil || tx == nil || *rx != 20 || *tx != 40 {
		t.Fatalf("taxas inesperadas: %v %v", rx, tx)
	}
}

func TestResourceSamplerThrottle(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	var s ResourceSampler
	ctx := context.Background()
	first := s.Sample(ctx)
	if first.RSSBytes == nil {
		t.Fatal("expected RSS on Linux")
	}
	second := s.Sample(ctx)
	if second.RSSBytes == nil || *second.RSSBytes != *first.RSSBytes {
		t.Fatal("throttle should return cached value within 3s")
	}
}

func TestResourceSamplerCancelledContext(t *testing.T) {
	var s ResourceSampler
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	usage := s.Sample(ctx)
	if usage.RSSBytes != nil || usage.CPUPercent != nil {
		t.Fatal("cancelled context should return empty usage")
	}
}

func TestStaleYtdlp(t *testing.T) {
	cases := []struct {
		version string
		stale   bool
	}{
		{"2025.01.15", true},
		{"2025.01.15.232815", true},
		{"2026.08.01", false},
		{"nightly", false},    // sem data: nunca falso positivo
		{"2025.13.99", false}, // data inválida
		{"", false},
	}
	for _, tc := range cases {
		if got, _ := staleYtdlp(tc.version); got != tc.stale {
			t.Errorf("staleYtdlp(%q) = %v, quer %v", tc.version, got, tc.stale)
		}
	}
}

func TestSanitizeLogLineRemovesURLQueriesAndSecrets(t *testing.T) {
	input := "falha https://media.example/video?token=abc password=segredo cookie=sessao"
	got := sanitizeLogLine(input)
	if got != "falha https://media.example/video?<redacted> password=<redacted> cookie=<redacted>" {
		t.Fatalf("linha sanitizada inesperada: %q", got)
	}
}
