// Package domain defines the ports used by NanoTube (docs/01-architecture/CONTRACTS.md).
package domain

import (
	"context"
	"errors"
	"time"
)

// PlaybackMode identifies how a PlaybackPlan must be handled by the player.
type PlaybackMode string

const (
	PlaybackModeDirect        PlaybackMode = "direct"
	PlaybackModeMpvYtdlHook   PlaybackMode = "mpv-ytdl-hook"
	PlaybackModeResolvedMedia PlaybackMode = "resolved-media"
)

// ResolvedStream represents a resolved media stream with necessary headers.
type ResolvedStream struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// PlaybackVariant is an alternative video stream. HasAudio distinguishes a
// self-contained stream from an adaptive video-only stream that must use the
// shared PlaybackPlan.Audio track.
type PlaybackVariant struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Height   int            `json:"height,omitempty"`
	HasAudio bool           `json:"has_audio"`
	Stream   ResolvedStream `json:"stream"`
}

// SubtitleTrack represents an external or embedded subtitle track.
type SubtitleTrack struct {
	Language string `json:"language"`
	URL      string `json:"url"`
	Format   string `json:"format,omitempty"`
}

// AudioTrack é uma faixa de áudio do conteúdo carregado, exposta pelo libmpv
// via track-list (QOL-02).
type AudioTrack struct {
	ID       int
	Language string
	Title    string
	Default  bool
	Selected bool
}

// AudioDevice descreve um dispositivo de saída de áudio detectado pelo player (ex: HDMI, Analog, Bluetooth, PipeWire).
type AudioDevice struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Selected    bool   `json:"selected"`
}

// SubtitleTrackInfo é uma faixa de legenda do conteúdo carregado. O nome evita
// colidir com SubtitleTrack, que descreve legendas de um plano de mídia.
type SubtitleTrackInfo struct {
	ID       int
	Language string
	Title    string
	Default  bool
	Forced   bool
	Selected bool
	External bool
}

// PlaybackMetadata holds metadata extracted along with the media streams.
type PlaybackMetadata struct {
	VideoID  string        `json:"video_id,omitempty"`
	Title    string        `json:"title,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
}

// PlaybackRequest describes the source the user wants to play.
type PlaybackRequest struct {
	SourceURL string
	VideoID   string
}

// PlaybackPlan is the resolver output consumed by the player.
type PlaybackPlan struct {
	Mode PlaybackMode `json:"mode"`

	// LoadTarget and Options are used by direct load and mpv-ytdl-hook modes.
	LoadTarget string            `json:"load_target,omitempty"`
	Options    map[string]string `json:"options,omitempty"`

	// Primary and Audio are used by resolved-media mode.
	Primary   ResolvedStream    `json:"primary"`
	Audio     *ResolvedStream   `json:"audio,omitempty"`
	AudioOnly *ResolvedStream   `json:"audio_only,omitempty"`
	Variants  []PlaybackVariant `json:"variants,omitempty"`
	Subtitles []SubtitleTrack   `json:"subtitles,omitempty"`
	Metadata  PlaybackMetadata  `json:"metadata"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

// PlaybackResolver prepares a video reference for the player.
type PlaybackResolver interface {
	Resolve(ctx context.Context, req PlaybackRequest) (PlaybackPlan, error)
}

// PlaybackSnapshot is cheap player state for the UI.
type PlaybackSnapshot struct {
	VideoID string
	// MediaTitle é o título que o player resolveu para a mídia corrente. Para
	// vídeo remoto vem do extrator, e é o que evita mostrar a URL crua quando
	// o vídeo não está no catálogo local.
	MediaTitle string
	Position   time.Duration
	Duration   time.Duration
	Paused     bool
	CoreIdle   bool
	Hwdec      string
	VideoCodec string
	AudioCodec string
}

// PlayerEventKind identifies the kind of a player event.
type PlayerEventKind int

const (
	EventFileLoaded PlayerEventKind = iota
	// EventLoadStarted marks the boundary between stream resolution and the
	// native player opening the resolved tracks.
	EventLoadStarted
	EventEOF
	EventPauseChanged
	EventError
)

// PlayerEvent is an internal, toolkit-independent player event.
type PlayerEvent struct {
	Kind PlayerEventKind
	// Generation identifies one concrete playback attempt. It is monotonic and
	// distinguishes retries of the same request as well as different URLs.
	Generation uint64
	Request    PlaybackRequest
	Error      error
	Paused     bool
}

// Player is the port implemented by the mpv adapter.
type Player interface {
	Load(ctx context.Context, plan PlaybackPlan, resumeAt time.Duration, generation uint64) error
	Pause() error
	Play() error
	Stop() error
	Seek(position time.Duration) error
	Volume() (int, error)
	SetVolume(volume int) error
	// Gain devolve o ganho digital atual em dB (0 = sem ganho).
	Gain() (float64, error)
	// SetGain define o ganho digital em dB (-12..+12, 0 desliga o filtro af).
	SetGain(gain float64) error
	// Speed devolve o fator de velocidade atual (1.0 = normal).
	Speed() (float64, error)
	// SetSpeed define o fator de velocidade (0.25 = um quarto, 2.0 = dobro).
	// É uma propriedade de instância: vale para o próximo carregamento.
	SetSpeed(speed float64) error
	// AudioTracks lista as faixas de áudio do conteúdo corrente (QOL-02).
	// Vazio quando não há mídia ou o conteúdo não expõe track-list.
	AudioTracks() ([]AudioTrack, error)
	// AudioDevices lista os dispositivos de saída de áudio disponíveis no sistema.
	AudioDevices() ([]AudioDevice, error)
	// SetAudioDevice seleciona o dispositivo de saída de áudio (ex: "auto", "pulse/...", "alsa/...").
	SetAudioDevice(device string) error
	// AudioNormalization informa se a normalização de volume (loudnorm/EBU R128) está ativa.
	AudioNormalization() (bool, error)
	// SetAudioNormalization liga ou desliga o filtro de normalização de volume.
	SetAudioNormalization(enabled bool) error
	// AudioChannels devolve o layout de canais atual (ex: "auto", "stereo", "5.1", "mono").
	AudioChannels() (string, error)
	// SetAudioChannels define a matriz de canais para transcodificação/downmix.
	SetAudioChannels(layout string) error
	// SubtitleTracks lista as faixas de legenda do conteúdo corrente.
	SubtitleTracks() ([]SubtitleTrackInfo, error)
	// SetAudioTrack seleciona a faixa de áudio pelo id do track-list. Zero ou
	// negativo devolve à escolha automática (aid=auto).
	SetAudioTrack(id int) error
	// SetSubtitleTrack seleciona a faixa de legenda pelo id do track-list.
	// Zero desliga a legenda; negativo devolve ao padrão do arquivo (sid=auto).
	SetSubtitleTrack(id int) error
	// SubtitlesEnabled informa se a legenda está visível (sub-visibility).
	SubtitlesEnabled() (bool, error)
	// SetSubtitlesEnabled liga ou desliga a exibição de legenda.
	SetSubtitlesEnabled(enabled bool) error
	Snapshot() PlaybackSnapshot
	Events() <-chan PlayerEvent
	Close() error
}

// UIDispatcher runs functions on the GTK main thread.
type UIDispatcher interface {
	Post(func())
}

// SecretKey identifies a secret in a SecretStore.
type SecretKey string

var (
	// ErrSecretNotFound distingue ausência normal de uma falha no cofre.
	ErrSecretNotFound = errors.New("segredo não encontrado")
	// ErrSecretStoreUnavailable permite fallback controlado sem acoplar auth ao
	// adaptador de keyring concreto.
	ErrSecretStoreUnavailable = errors.New("armazenamento seguro indisponível")
)

// SecretStore is the keyring-backed secret storage.
type SecretStore interface {
	Get(ctx context.Context, key SecretKey) ([]byte, error)
	Set(ctx context.Context, key SecretKey, value []byte) error
	Delete(ctx context.Context, key SecretKey) error
}

// AccountStore persists non-sensitive account state locally.
type AccountStore interface {
	SaveAccount(ctx context.Context, info AccountInfo) error
	Account(ctx context.Context) (AccountInfo, bool, error)
	ClearAccount(ctx context.Context) error
}

// VideoRepository persists video metadata and playback progress.
type VideoRepository interface {
	UpsertVideos(ctx context.Context, videos []Video) error
	MarkProgress(ctx context.Context, videoID string, progress PlaybackProgress) error
	Recent(ctx context.Context, filter VideoFilter) ([]Video, error)
}

// SearchIndex runs local FTS5 text search over videos.
type SearchIndex interface {
	Search(ctx context.Context, query string, limit int) ([]SearchHit, error)
	ReindexVideos(ctx context.Context, ids []string) error
}

// SearchService coordinates remote providers and local-only personal filters.
// The UI depends on this contract, never on yt-dlp or Google API types.
type SearchService interface {
	Search(ctx context.Context, opts SearchOptions) (SearchPage, error)
}

// ThumbnailStore downloads, caches and decodes video thumbnails
// (docs/03-implementation/THUMBNAILS_AND_CACHE.md).
type ThumbnailStore interface {
	// GetCompressed returns the compressed thumbnail on disk, downloading it
	// once if missing. The cache is content-addressed from the URL.
	GetCompressed(ctx context.Context, key ThumbnailKey) (CachedFile, error)
	// GetDisplay returns a decoded thumbnail scaled to size, cached in a LRU.
	GetDisplay(ctx context.Context, key ThumbnailKey, size ImageSize) (DisplayImage, error)
	// Prefetch downloads compressed thumbnails for nearby keys without decoding.
	Prefetch(ctx context.Context, keys []ThumbnailKey)
	// Invalidate drops both the disk copy and any decoded display image.
	Invalidate(ctx context.Context, key ThumbnailKey) error
}

// RecommendationEngine builds the local Home from an InterestProfile and
// explains its deterministic choices
// (docs/03-implementation/RECOMMENDATIONS.md).
type RecommendationEngine interface {
	BuildHome(ctx context.Context, profile InterestProfile, candidates []Video) (HomeModel, error)
	Explain(videoID string) RecommendationExplanation
}

// PageOptions pages a provider listing request.
type PageOptions struct {
	MaxResults int
	PageToken  string
}

// YouTubeProvider supplies account-scoped YouTube catalog data
// (docs/03-implementation/YOUTUBE_AND_AUTH.md). The UI never imports
// youtube/v3 types; conversion happens inside the provider adapter.
type YouTubeProvider interface {
	// Subscriptions lists the account's subscribed channels and the next
	// page token (empty when done).
	Subscriptions(ctx context.Context, opts PageOptions) ([]Channel, string, error)
	// Channels resolves authoritative titles and uploads playlist ids for
	// the given channel ids (batched by the provider).
	Channels(ctx context.Context, ids []string) ([]Channel, error)
	// Uploads lists the recent uploads of an uploads playlist and the next
	// page token (empty when done).
	Uploads(ctx context.Context, uploadsPlaylistID string, opts PageOptions) ([]Video, string, error)
	// Search runs a remote catalog search and the next page token. É o
	// complemento da busca local FTS5: cobre o que ainda não foi sincronizado
	// (FR-011). Consome quota da Data API, então só roda sob ação explícita.
	Search(ctx context.Context, opts SearchOptions) ([]Video, string, error)
}

// RemotePlaylistProvider lists metadata from a remote YouTube playlist. It is
// separate from YouTubeProvider so sync providers do not need playlist UI
// capabilities. Implementations never download media through this port.
type RemotePlaylistProvider interface {
	PlaylistItems(ctx context.Context, playlistID string, opts PageOptions) ([]Video, string, error)
	// ChannelVideos lists the recent uploads of a channel and the next page
	// token (empty when done). Used by the in-app channel page.
	ChannelVideos(ctx context.Context, channelID string, opts PageOptions) ([]Video, string, error)
}
