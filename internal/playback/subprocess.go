package playback

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

var (
	extractorAttemptTimeout = 45 * time.Second
	extractorStdoutLimit    = 8 << 20
	extractorStderrLimit    = 512 << 10
)

var (
	errExtractorOutputLimit = errors.New("saída do extractor excedeu o limite seguro")
	errExtractorTimeout     = errors.New("extractor excedeu o tempo limite")
)

type boundedBuffer struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	overflow bool
	cancel   context.CancelFunc
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 || len(data) > remaining {
		if remaining > 0 {
			_, _ = b.buffer.Write(data[:remaining])
		}
		b.overflow = true
		if b.cancel != nil {
			b.cancel()
		}
		return len(data), errExtractorOutputLimit
	}
	return b.buffer.Write(data)
}

func (b *boundedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buffer.Bytes()...)
}

func (b *boundedBuffer) String() string { return string(b.Bytes()) }

func (b *boundedBuffer) exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.overflow
}

func runExtractorCommand(
	ctx context.Context,
	commandContext func(context.Context, string, ...string) *exec.Cmd,
	binary string,
	args ...string,
) ([]byte, []byte, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, extractorAttemptTimeout)
	defer cancel()

	stdout := &boundedBuffer{limit: extractorStdoutLimit, cancel: cancel}
	stderr := &boundedBuffer{limit: extractorStderrLimit, cancel: cancel}
	cmd := commandContext(attemptCtx, binary, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	protectProcessTree(cmd)

	err := cmd.Run()
	if stdout.exceeded() || stderr.exceeded() {
		return stdout.Bytes(), stderr.Bytes(), errExtractorOutputLimit
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stdout.Bytes(), stderr.Bytes(), ctxErr
	}
	if errors.Is(attemptCtx.Err(), context.DeadlineExceeded) {
		return stdout.Bytes(), stderr.Bytes(), errExtractorTimeout
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

// RunBoundedExtractor executes an extractor used by another adapter with the
// same timeout, output limits and process-group cleanup as playback. Keeping a
// single runner prevents search/diagnostics from becoming a weaker subprocess
// boundary than media resolution.
func RunBoundedExtractor(ctx context.Context, binary string, args ...string) ([]byte, []byte, error) {
	return runExtractorCommand(ctx, exec.CommandContext, binary, args...)
}
