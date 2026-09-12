// Package playback implements PlaybackResolver strategies and the
// PlayerController orchestration layer (docs/03-implementation/PLAYER.md).
package playback

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// ErrClosed is returned when a command is issued after the controller was closed.
var ErrClosed = errors.New("player controller fechado")

// Controller owns the single active player instance, serializes user commands
// and relays player events to the UI. It is toolkit-independent: UI code talks
// to this type, never to go-mpv/libmpv.
type Controller struct {
	player           domain.Player
	resolver         domain.PlaybackResolver
	events           chan domain.PlayerEvent
	done             chan struct{}
	mu               sync.Mutex
	loadMu           sync.Mutex
	current          domain.PlaybackRequest
	cancelResolution context.CancelFunc
	playGeneration   uint64
	resolvedTitle    string
	closed           atomic.Bool
}

// NewController wires the single player instance with the playback resolver.
func NewController(player domain.Player, resolver domain.PlaybackResolver) *Controller {
	c := &Controller{
		player:   player,
		resolver: resolver,
		events:   make(chan domain.PlayerEvent, 64),
		done:     make(chan struct{}),
	}
	go c.relay()
	return c
}

// Play resolves req and loads it into the player asynchronously. Errors are
// delivered as EventError on the Events channel, never blocking the caller.
func (c *Controller) Play(ctx context.Context, req domain.PlaybackRequest) error {
	return c.PlayAt(ctx, req, 0)
}

// PlayAt is Play resuming at an absolute position. A resumeAt of zero (or
// negative) starts from the beginning. Cancels any in-flight resolution for prior requests.
func (c *Controller) PlayAt(ctx context.Context, req domain.PlaybackRequest, resumeAt time.Duration) error {
	_, err := c.PlayAtTracked(ctx, req, resumeAt)
	return err
}

// PlayAtTracked starts playback and returns the opaque generation attached to
// every event from this concrete attempt, including retries of the same URL.
func (c *Controller) PlayAtTracked(ctx context.Context, req domain.PlaybackRequest, resumeAt time.Duration) (uint64, error) {
	if c.closed.Load() {
		return 0, ErrClosed
	}
	if resumeAt < 0 {
		resumeAt = 0
	}

	c.mu.Lock()
	if c.cancelResolution != nil {
		c.cancelResolution()
	}
	resCtx, cancel := context.WithCancel(ctx)
	c.cancelResolution = cancel
	c.playGeneration++
	generation := c.playGeneration
	resolver := c.resolver
	c.mu.Unlock()

	go c.play(resCtx, resolver, req, resumeAt, generation)
	return generation, nil
}

func (c *Controller) play(
	ctx context.Context,
	resolver domain.PlaybackResolver,
	req domain.PlaybackRequest,
	resumeAt time.Duration,
	generation uint64,
) {
	plan, err := resolver.Resolve(ctx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return
		}
		c.emit(domain.PlayerEvent{Kind: domain.EventError, Generation: generation, Request: req, Error: fmt.Errorf("resolver: %w", err)})
		return
	}

	c.loadMu.Lock()
	defer c.loadMu.Unlock()
	c.mu.Lock()
	if generation != c.playGeneration || ctx.Err() != nil || c.closed.Load() {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	c.emit(domain.PlayerEvent{Kind: domain.EventLoadStarted, Generation: generation, Request: req})
	err = c.player.Load(ctx, plan, resumeAt, generation)

	c.mu.Lock()
	active := generation == c.playGeneration && !c.closed.Load()
	if err == nil && active {
		c.current = req
		c.resolvedTitle = plan.Metadata.Title
	}
	c.mu.Unlock()
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return
		}
		if active {
			c.emit(domain.PlayerEvent{Kind: domain.EventError, Generation: generation, Request: req, Error: err})
		}
		return
	}
}

// SetResolver troca a estratégia de resolução em runtime. É o que faz uma
// mudança de política (ex.: qualidade máxima) valer já na próxima reprodução,
// sem reiniciar o app.
func (c *Controller) SetResolver(resolver domain.PlaybackResolver) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resolver = resolver
}

// TogglePause switches between paused and playing based on the player state.
func (c *Controller) TogglePause() error {
	if c.closed.Load() {
		return ErrClosed
	}
	s := c.player.Snapshot()
	if s.Paused {
		return c.player.Play()
	}
	return c.player.Pause()
}

// Pause pauses playback.
func (c *Controller) Pause() error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.Pause()
}

// Resume continues playback.
func (c *Controller) Resume() error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.Play()
}

// Stop unloads the current file.
func (c *Controller) Stop() error {
	if c.closed.Load() {
		return ErrClosed
	}
	c.mu.Lock()
	if c.cancelResolution != nil {
		c.cancelResolution()
		c.cancelResolution = nil
	}
	c.playGeneration++
	c.current = domain.PlaybackRequest{}
	c.resolvedTitle = ""
	c.mu.Unlock()
	c.loadMu.Lock()
	defer c.loadMu.Unlock()
	return c.player.Stop()
}

// Seek jumps to an absolute position.
func (c *Controller) Seek(position time.Duration) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.Seek(position)
}

// Volume returns the current volume in percent (0–100).
func (c *Controller) Volume() (int, error) {
	if c.closed.Load() {
		return 0, ErrClosed
	}
	return c.player.Volume()
}

// SetVolume sets the volume in percent (0–100).
func (c *Controller) SetVolume(volume int) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.SetVolume(volume)
}

// Gain retorna o ganho digital em dB.
func (c *Controller) Gain() (float64, error) {
	if c.closed.Load() {
		return 0, ErrClosed
	}
	return c.player.Gain()
}

// SetGain define o ganho digital em dB.
func (c *Controller) SetGain(gain float64) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.SetGain(gain)
}

// Speed returns the current playback speed factor (1.0 = normal).
func (c *Controller) Speed() (float64, error) {
	if c.closed.Load() {
		return 0, ErrClosed
	}
	return c.player.Speed()
}

// SetSpeed sets the playback speed factor (0.25–2.0).
func (c *Controller) SetSpeed(speed float64) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.SetSpeed(speed)
}

// AudioTracks lista as faixas de áudio do conteúdo corrente (QOL-02).
func (c *Controller) AudioTracks() ([]domain.AudioTrack, error) {
	if c.closed.Load() {
		return nil, ErrClosed
	}
	return c.player.AudioTracks()
}

// SubtitleTracks lista as faixas de legenda do conteúdo corrente.
func (c *Controller) SubtitleTracks() ([]domain.SubtitleTrackInfo, error) {
	if c.closed.Load() {
		return nil, ErrClosed
	}
	return c.player.SubtitleTracks()
}

// SetAudioTrack seleciona a faixa de áudio pelo id do track-list.
func (c *Controller) SetAudioTrack(id int) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.SetAudioTrack(id)
}

// SetSubtitleTrack seleciona a faixa de legenda pelo id do track-list.
func (c *Controller) SetSubtitleTrack(id int) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.SetSubtitleTrack(id)
}

// SubtitlesEnabled informa se a legenda está visível.
func (c *Controller) SubtitlesEnabled() (bool, error) {
	if c.closed.Load() {
		return false, ErrClosed
	}
	return c.player.SubtitlesEnabled()
}

// SetSubtitlesEnabled liga ou desliga a exibição de legenda.
func (c *Controller) SetSubtitlesEnabled(enabled bool) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.SetSubtitlesEnabled(enabled)
}

// AudioDevices lista os dispositivos de saída de áudio disponíveis no sistema.
func (c *Controller) AudioDevices() ([]domain.AudioDevice, error) {
	if c.closed.Load() {
		return nil, ErrClosed
	}
	return c.player.AudioDevices()
}

// SetAudioDevice seleciona o dispositivo de áudio (ex: "auto", "pulse/...", "alsa/...").
func (c *Controller) SetAudioDevice(device string) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.SetAudioDevice(device)
}

// AudioNormalization informa se o filtro de normalização de áudio (loudnorm) está ativo.
func (c *Controller) AudioNormalization() (bool, error) {
	if c.closed.Load() {
		return false, ErrClosed
	}
	return c.player.AudioNormalization()
}

// SetAudioNormalization ativa ou desativa o filtro de normalização de áudio sem reiniciar a reprodução.
func (c *Controller) SetAudioNormalization(enabled bool) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.SetAudioNormalization(enabled)
}

// AudioChannels devolve a configuração de canais ("auto", "stereo", "5.1", etc.).
func (c *Controller) AudioChannels() (string, error) {
	if c.closed.Load() {
		return "auto", ErrClosed
	}
	return c.player.AudioChannels()
}

// SetAudioChannels define a matriz de canais para transcodificação/downmixing no player.
func (c *Controller) SetAudioChannels(layout string) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.player.SetAudioChannels(layout)
}

// Snapshot returns the current cheap player state, augmented with resolved metadata.
func (c *Controller) Snapshot() domain.PlaybackSnapshot {
	s := c.player.Snapshot()
	c.mu.Lock()
	if s.MediaTitle == "" && c.resolvedTitle != "" {
		s.MediaTitle = c.resolvedTitle
	}
	c.mu.Unlock()
	return s
}

// ResolvedTitle returns the title discovered during stream resolution.
func (c *Controller) ResolvedTitle() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resolvedTitle
}

// Current returns the last successfully loaded request, if any.
func (c *Controller) Current() (domain.PlaybackRequest, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current.SourceURL == "" && c.current.VideoID == "" {
		return domain.PlaybackRequest{}, false
	}
	return c.current, true
}

// Events exposes the single serialized player event stream.
func (c *Controller) Events() <-chan domain.PlayerEvent {
	return c.events
}

func (c *Controller) relay() {
	defer close(c.events)
	for {
		select {
		case ev, ok := <-c.player.Events():
			if !ok {
				return
			}
			select {
			case c.events <- ev:
			case <-c.done:
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *Controller) emit(ev domain.PlayerEvent) {
	select {
	case c.events <- ev:
	case <-c.done:
	}
}

// Close stops the relay, cancels any pending resolution, and closes the underlying player. Idempotent.
func (c *Controller) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	c.mu.Lock()
	if c.cancelResolution != nil {
		c.cancelResolution()
	}
	c.mu.Unlock()
	close(c.done)
	c.loadMu.Lock()
	defer c.loadMu.Unlock()
	return c.player.Close()
}
