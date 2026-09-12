package playback

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestBoundedBufferStopsAtLimit(t *testing.T) {
	buffer := &boundedBuffer{limit: 4}
	if _, err := buffer.Write([]byte("12345")); !errors.Is(err, errExtractorOutputLimit) {
		t.Fatalf("Write = %v, esperado limite", err)
	}
	if got := buffer.String(); got != "1234" {
		t.Fatalf("conteúdo limitado = %q", got)
	}
}

func TestRunExtractorCommandHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := runExtractorCommand(ctx, exec.CommandContext, "sleep", "5")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("erro = %v, esperado cancelamento", err)
	}
}

func TestRunExtractorCommandCancelsOnOutputOverflow(t *testing.T) {
	originalLimit := extractorStdoutLimit
	extractorStdoutLimit = 1024
	t.Cleanup(func() { extractorStdoutLimit = originalLimit })

	_, _, err := runExtractorCommand(context.Background(), helperCommand("overflow"), "ignored")
	if !errors.Is(err, errExtractorOutputLimit) {
		t.Fatalf("erro = %v, esperado limite de saída", err)
	}
}

func TestRunExtractorCommandTimesOutRunningProcess(t *testing.T) {
	originalTimeout := extractorAttemptTimeout
	extractorAttemptTimeout = 50 * time.Millisecond
	t.Cleanup(func() { extractorAttemptTimeout = originalTimeout })

	started := time.Now()
	_, _, err := runExtractorCommand(context.Background(), helperCommand("hang"), "ignored")
	if !errors.Is(err, errExtractorTimeout) {
		t.Fatalf("erro = %v, esperado timeout", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cancelamento demorou %v", elapsed)
	}
}

func helperCommand(mode string) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestPlaybackHelperProcess")
		cmd.Env = append(os.Environ(), "NANOTUBE_HELPER_PROCESS=1", "NANOTUBE_HELPER_MODE="+mode)
		return cmd
	}
}

func TestPlaybackHelperProcess(t *testing.T) {
	if os.Getenv("NANOTUBE_HELPER_PROCESS") != "1" {
		return
	}
	switch os.Getenv("NANOTUBE_HELPER_MODE") {
	case "overflow":
		_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), 4096))
	case "hang":
		time.Sleep(5 * time.Second)
	}
	os.Exit(0)
}
