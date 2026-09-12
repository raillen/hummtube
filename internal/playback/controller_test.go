package playback

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

type fakePlayer struct {
	mu          sync.Mutex
	plans       []domain.PlaybackPlan
	paused      bool
	volume      int
	gain        float64
	speed       float64
	audioTracks []domain.AudioTrack
	subTracks   []domain.SubtitleTrackInfo
	audioSet    []int
	subSet      []int
	subEnabled  bool
	tracksErr   error
	closed      bool
	loadErr     error
	playErr     error
	pauseErr    error
	stopErr     error
	seekErr     error
	volumeErr   error
	gainErr     error
	speedErr    error
	seekCalls   []time.Duration
	volumeSet   []int
	gainSet     []float64
	loadIDs     []uint64
	events      chan domain.PlayerEvent
}

func newFakePlayer() *fakePlayer {
	return &fakePlayer{events: make(chan domain.PlayerEvent, 16)}
}

func (f *fakePlayer) Load(_ context.Context, plan domain.PlaybackPlan, _ time.Duration, generation uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.loadErr != nil {
		return f.loadErr
	}
	f.plans = append(f.plans, plan)
	f.loadIDs = append(f.loadIDs, generation)
	return nil
}

func (f *fakePlayer) Pause() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pauseErr != nil {
		return f.pauseErr
	}
	f.paused = true
	return nil
}

func (f *fakePlayer) Play() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.playErr != nil {
		return f.playErr
	}
	f.paused = false
	return nil
}

func (f *fakePlayer) Seek(position time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.seekErr != nil {
		return f.seekErr
	}
	f.seekCalls = append(f.seekCalls, position)
	return nil
}

func (f *fakePlayer) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopErr != nil {
		return f.stopErr
	}
	return nil
}

func (f *fakePlayer) Volume() (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.volumeErr != nil {
		return 0, f.volumeErr
	}
	return f.volume, nil
}

func (f *fakePlayer) SetVolume(volume int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.volumeErr != nil {
		return f.volumeErr
	}
	f.volume = volume
	f.volumeSet = append(f.volumeSet, volume)
	return nil
}

func (f *fakePlayer) Gain() (float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gainErr != nil {
		return 0, f.gainErr
	}
	return f.gain, nil
}

func (f *fakePlayer) SetGain(gain float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gainErr != nil {
		return f.gainErr
	}
	f.gain = gain
	f.gainSet = append(f.gainSet, gain)
	return nil
}

func (f *fakePlayer) Speed() (float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.speedErr != nil {
		return 0, f.speedErr
	}
	return f.speed, nil
}

func (f *fakePlayer) SetSpeed(speed float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.speedErr != nil {
		return f.speedErr
	}
	f.speed = speed
	return nil
}

func (f *fakePlayer) AudioTracks() ([]domain.AudioTrack, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tracksErr != nil {
		return nil, f.tracksErr
	}
	return append([]domain.AudioTrack(nil), f.audioTracks...), nil
}

func (f *fakePlayer) SubtitleTracks() ([]domain.SubtitleTrackInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tracksErr != nil {
		return nil, f.tracksErr
	}
	return append([]domain.SubtitleTrackInfo(nil), f.subTracks...), nil
}

func (f *fakePlayer) AudioDevices() ([]domain.AudioDevice, error) {
	return []domain.AudioDevice{
		{Name: "auto", Description: "Autoselect", Selected: true},
	}, nil
}

func (f *fakePlayer) SetAudioDevice(device string) error {
	return nil
}

func (f *fakePlayer) AudioNormalization() (bool, error) {
	return false, nil
}

func (f *fakePlayer) SetAudioNormalization(enabled bool) error {
	return nil
}

func (f *fakePlayer) AudioChannels() (string, error) {
	return "auto", nil
}

func (f *fakePlayer) SetAudioChannels(layout string) error {
	return nil
}

func (f *fakePlayer) SetAudioTrack(id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tracksErr != nil {
		return f.tracksErr
	}
	f.audioSet = append(f.audioSet, id)
	return nil
}

func (f *fakePlayer) SetSubtitleTrack(id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tracksErr != nil {
		return f.tracksErr
	}
	f.subSet = append(f.subSet, id)
	return nil
}

func (f *fakePlayer) SubtitlesEnabled() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tracksErr != nil {
		return false, f.tracksErr
	}
	return f.subEnabled, nil
}

func (f *fakePlayer) SetSubtitlesEnabled(enabled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tracksErr != nil {
		return f.tracksErr
	}
	f.subEnabled = enabled
	return nil
}

func (f *fakePlayer) Snapshot() domain.PlaybackSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return domain.PlaybackSnapshot{Paused: f.paused}
}

func (f *fakePlayer) Events() <-chan domain.PlayerEvent { return f.events }

func (f *fakePlayer) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	close(f.events)
	return nil
}

func (f *fakePlayer) loadedPlans() []domain.PlaybackPlan {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.PlaybackPlan(nil), f.plans...)
}

type fakeResolver struct {
	plan domain.PlaybackPlan
	err  error
}

func (r fakeResolver) Resolve(_ context.Context, _ domain.PlaybackRequest) (domain.PlaybackPlan, error) {
	return r.plan, r.err
}

func waitEvent(t *testing.T, events <-chan domain.PlayerEvent, timeout time.Duration) domain.PlayerEvent {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(timeout):
		t.Fatal("timeout esperando evento do player")
		return domain.PlayerEvent{}
	}
}

func TestControllerPlayResolvesAndLoads(t *testing.T) {
	fp := newFakePlayer()
	c := NewController(fp, fakeResolver{plan: domain.PlaybackPlan{Mode: domain.PlaybackModeDirect, LoadTarget: "/tmp/v.mp4"}})
	defer c.Close()

	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "/tmp/v.mp4"}); err != nil {
		t.Fatalf("Play: %v", err)
	}
	ev := waitEvent(t, c.Events(), 2*time.Second)
	if ev.Kind != domain.EventLoadStarted || ev.Request.SourceURL != "/tmp/v.mp4" {
		t.Fatalf("evento antes do load = %#v", ev)
	}
	if ev.Generation == 0 {
		t.Fatal("evento de load sem geração")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if plans := fp.loadedPlans(); len(plans) == 1 {
			if plans[0].LoadTarget != "/tmp/v.mp4" {
				t.Fatalf("loadTarget = %q", plans[0].LoadTarget)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("player não recebeu Load")
}

func TestControllerPlayResolverErrorEmitsEvent(t *testing.T) {
	fp := newFakePlayer()
	c := NewController(fp, fakeResolver{err: errors.New("boom")})
	defer c.Close()

	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "x"}); err != nil {
		t.Fatalf("Play: %v", err)
	}
	ev := waitEvent(t, c.Events(), 2*time.Second)
	if ev.Kind != domain.EventError {
		t.Fatalf("kind = %v, want EventError", ev.Kind)
	}
	if len(fp.loadedPlans()) != 0 {
		t.Fatal("player não deveria ter recebido Load")
	}
}

func TestControllerPlayLoadErrorEmitsEvent(t *testing.T) {
	fp := newFakePlayer()
	fp.loadErr = errors.New("load failed")
	c := NewController(fp, fakeResolver{plan: domain.PlaybackPlan{LoadTarget: "x"}})
	defer c.Close()

	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "x"}); err != nil {
		t.Fatalf("Play: %v", err)
	}
	started := waitEvent(t, c.Events(), 2*time.Second)
	if started.Kind != domain.EventLoadStarted {
		t.Fatalf("kind = %v, want EventLoadStarted", started.Kind)
	}
	ev := waitEvent(t, c.Events(), 2*time.Second)
	if ev.Kind != domain.EventError {
		t.Fatalf("kind = %v, want EventError", ev.Kind)
	}
	if ev.Request.SourceURL != "x" {
		t.Fatalf("request = %#v", ev.Request)
	}
}

func TestControllerTogglePause(t *testing.T) {
	fp := newFakePlayer()
	c := NewController(fp, fakeResolver{})
	defer c.Close()

	if err := c.TogglePause(); err != nil {
		t.Fatalf("TogglePause: %v", err)
	}
	if !fp.Snapshot().Paused {
		t.Fatal("esperava pausado após primeiro TogglePause")
	}
	if err := c.TogglePause(); err != nil {
		t.Fatalf("TogglePause: %v", err)
	}
	if fp.Snapshot().Paused {
		t.Fatal("esperava reproduzindo após segundo TogglePause")
	}
}

func TestControllerSeekAndStop(t *testing.T) {
	fp := newFakePlayer()
	c := NewController(fp, fakeResolver{})
	defer c.Close()

	if err := c.Seek(90 * time.Second); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(fp.seekCalls) != 1 || fp.seekCalls[0] != 90*time.Second {
		t.Fatalf("seekCalls = %v", fp.seekCalls)
	}
}

func TestControllerVolumeForwarding(t *testing.T) {
	fp := newFakePlayer()
	fp.volume = 42
	c := NewController(fp, fakeResolver{})
	defer c.Close()

	v, err := c.Volume()
	if err != nil {
		t.Fatalf("Volume: %v", err)
	}
	if v != 42 {
		t.Fatalf("volume = %d, want 42", v)
	}
	if err := c.SetVolume(70); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	fp.mu.Lock()
	if len(fp.volumeSet) != 1 || fp.volumeSet[0] != 70 {
		fp.mu.Unlock()
		t.Fatalf("volumeSet = %v", fp.volumeSet)
	}
	fp.mu.Unlock()
}

func TestControllerRelaysPlayerEvents(t *testing.T) {
	fp := newFakePlayer()
	c := NewController(fp, fakeResolver{})
	defer c.Close()

	fp.events <- domain.PlayerEvent{Kind: domain.EventEOF}
	ev := waitEvent(t, c.Events(), 2*time.Second)
	if ev.Kind != domain.EventEOF {
		t.Fatalf("kind = %v, want EventEOF", ev.Kind)
	}
}

func TestControllerCurrentAfterLoad(t *testing.T) {
	fp := newFakePlayer()
	c := NewController(fp, fakeResolver{plan: domain.PlaybackPlan{LoadTarget: "v"}})
	defer c.Close()

	_, ok := c.Current()
	if ok {
		t.Fatal("Current deveria estar vazio antes do load")
	}
	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "v"}); err != nil {
		t.Fatalf("Play: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if req, ok := c.Current(); ok && req.SourceURL == "v" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Current não atualizou após load")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestControllerClose(t *testing.T) {
	fp := newFakePlayer()
	c := NewController(fp, fakeResolver{})

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close idempotente: %v", err)
	}
	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "x"}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Play após Close = %v, want ErrClosed", err)
	}
	fp.mu.Lock()
	defer fp.mu.Unlock()
	if !fp.closed {
		t.Fatal("player subjacente não foi fechado")
	}
}

type blockingResolver struct {
	started chan struct{}
}

type outOfOrderResolver struct {
	firstStarted chan struct{}
	releaseFirst chan struct{}
}

func (r outOfOrderResolver) Resolve(_ context.Context, req domain.PlaybackRequest) (domain.PlaybackPlan, error) {
	if req.SourceURL == "first" {
		close(r.firstStarted)
		<-r.releaseFirst
	}
	return domain.PlaybackPlan{Mode: domain.PlaybackModeDirect, LoadTarget: req.SourceURL}, nil
}

func TestControllerIgnoresStaleResolutionThatDisregardsCancellation(t *testing.T) {
	fp := newFakePlayer()
	resolver := outOfOrderResolver{firstStarted: make(chan struct{}), releaseFirst: make(chan struct{})}
	c := NewController(fp, resolver)
	defer c.Close()

	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "first"}); err != nil {
		t.Fatalf("primeiro Play: %v", err)
	}
	<-resolver.firstStarted
	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "second"}); err != nil {
		t.Fatalf("segundo Play: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(fp.loadedPlans()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	close(resolver.releaseFirst)
	time.Sleep(25 * time.Millisecond)

	plans := fp.loadedPlans()
	if len(plans) != 1 || plans[0].LoadTarget != "second" {
		t.Fatalf("loads = %+v; resolução antiga não poderia sobrescrever a nova", plans)
	}
}

func (r blockingResolver) Resolve(ctx context.Context, _ domain.PlaybackRequest) (domain.PlaybackPlan, error) {
	close(r.started)
	<-ctx.Done()
	return domain.PlaybackPlan{}, ctx.Err()
}

func TestControllerCancelsPriorResolution(t *testing.T) {
	fp := newFakePlayer()
	br := blockingResolver{started: make(chan struct{})}
	c := NewController(fp, br)
	defer c.Close()

	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "first"}); err != nil {
		t.Fatalf("Play 1: %v", err)
	}

	<-br.started // Wait until first resolution starts blocking

	// Switch to instant resolver and play second
	c.SetResolver(fakeResolver{plan: domain.PlaybackPlan{Mode: domain.PlaybackModeDirect, LoadTarget: "second"}})
	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "second"}); err != nil {
		t.Fatalf("Play 2: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		if plans := fp.loadedPlans(); len(plans) == 1 && plans[0].LoadTarget == "second" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected 1 loaded plan with 'second', got %+v", fp.loadedPlans())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestControllerStopInvalidatesResolutionThatIgnoresCancellation(t *testing.T) {
	fp := newFakePlayer()
	resolver := outOfOrderResolver{firstStarted: make(chan struct{}), releaseFirst: make(chan struct{})}
	c := NewController(fp, resolver)
	defer c.Close()

	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "first"}); err != nil {
		t.Fatalf("Play first: %v", err)
	}
	<-resolver.firstStarted
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	c.SetResolver(fakeResolver{plan: domain.PlaybackPlan{Mode: domain.PlaybackModeDirect, LoadTarget: "second"}})
	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "second"}); err != nil {
		t.Fatalf("Play second: %v", err)
	}
	close(resolver.releaseFirst)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		plans := fp.loadedPlans()
		if len(plans) == 1 && plans[0].LoadTarget == "second" {
			if req, ok := c.Current(); !ok || req.SourceURL != "second" {
				t.Fatalf("Current = %#v, %v", req, ok)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("Stop permitiu load antigo ou impediu o novo: %+v", fp.loadedPlans())
}

func TestControllerResolvedTitle(t *testing.T) {
	fp := newFakePlayer()
	c := NewController(fp, fakeResolver{plan: domain.PlaybackPlan{
		Mode:     domain.PlaybackModeResolvedMedia,
		Primary:  domain.ResolvedStream{URL: "https://example.com/video"},
		Metadata: domain.PlaybackMetadata{Title: "Resolved Sample Title"},
	}})
	defer c.Close()

	if err := c.Play(context.Background(), domain.PlaybackRequest{SourceURL: "https://example.com/video"}); err != nil {
		t.Fatalf("Play: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		s := c.Snapshot()
		if s.MediaTitle == "Resolved Sample Title" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected title 'Resolved Sample Title', got %q", s.MediaTitle)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
