// Domain models shared across NanoTube boundaries
// (docs/03-implementation/STORAGE.md).
package domain

import "time"

// Channel is a YouTube channel known locally.
type Channel struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	ThumbnailURL      string    `json:"thumbnail_url,omitempty"`
	Subscribed        bool      `json:"subscribed"`
	UploadsPlaylistID string    `json:"uploads_playlist_id,omitempty"`
	LastSyncAt        time.Time `json:"last_sync_at,omitempty"`
	LastKnownVideoID  string    `json:"last_known_video_id,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
}

// ChannelFolder is a local folder grouping subscriptions (M9/QOL-03). It is
// purely local state: nothing is synced to YouTube.
type ChannelFolder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Video is locally known metadata about a video.
type Video struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	// ResourceType/ExternalURL are populated only by remote search when the
	// result is a channel or playlist. Catalog rows remain ordinary videos.
	ResourceType      SearchResourceType `json:"resource_type,omitempty"`
	ExternalURL       string             `json:"external_url,omitempty"`
	ViewCount         int64              `json:"view_count,omitempty"`
	ResourceItemCount int64              `json:"resource_item_count,omitempty"`
	LiveStatus        string             `json:"live_status,omitempty"`
	// ChannelTitle é a projeção pronta para UI. Em vídeos locais vem da tabela
	// channels; em resultados remotos vem diretamente do provedor.
	ChannelTitle string `json:"channel_title,omitempty"`
	Title        string `json:"title"`
	// Description guarda o texto completo quando o provedor o disponibiliza.
	// DescriptionExcerpt continua sendo a projeção curta usada em cards/FTS.
	Description           string        `json:"description,omitempty"`
	DescriptionExcerpt    string        `json:"description_excerpt,omitempty"`
	PublishedAt           time.Time     `json:"published_at"`
	Duration              time.Duration `json:"duration"`
	Category              string        `json:"category,omitempty"`
	ThumbnailURL          string        `json:"thumbnail_url"`
	FirstSeenAt           time.Time     `json:"first_seen_at,omitempty"`
	LastSeenAt            time.Time     `json:"last_seen_at,omitempty"`
	RecommendationReasons []string      `json:"recommendation_reasons,omitempty"`
}

// ShortsMaxDuration é o teto de duração usado para classificar um vídeo como
// Short (mesma convenção da busca local; docs/03-implementation/SEARCH.md).
const ShortsMaxDuration = 3 * time.Minute

// IsShort informa se o vídeo se enquadra no formato Short, pela duração. O
// catálogo não guarda orientação nem flag do YouTube, então a duração é a
// única heurística determinística disponível.
func (v Video) IsShort() bool {
	return v.Duration > 0 && v.Duration <= ShortsMaxDuration
}

// PlaybackProgress is the resume state for a video.
type PlaybackProgress struct {
	VideoID   string
	Position  time.Duration
	Duration  time.Duration
	UpdatedAt time.Time
	Completed bool
}

// VideoFilter narrows and pages a Recent query.
type VideoFilter struct {
	ChannelID      string
	OnlySubscribed bool
	Limit          int
	Offset         int
}

// SearchHit is a lightweight FTS5 result for the UI.
type SearchHit struct {
	VideoID     string `json:"video_id"`
	Title       string `json:"title"`
	ChannelName string `json:"channel_name"`
}

// ThumbnailKey identifies a remote thumbnail by video and content version
// (docs/03-implementation/THUMBNAILS_AND_CACHE.md).
type ThumbnailKey struct {
	// VideoID of the video owning the thumbnail.
	VideoID string
	// URL is the remote thumbnail source. A change forces a refetch because
	// the disk cache is content-addressed from it.
	URL string
}

// ImageSize is the target display size for a decoded thumbnail.
type ImageSize struct {
	Width  int
	Height int
}

// CachedFile is the compressed thumbnail stored on disk.
type CachedFile struct {
	Path string
}

// DisplayImage is a decoded, ready-to-render thumbnail. It is an interface so
// the domain stays toolkit-free; the concrete implementation wraps a
// toolkit pixbuf (e.g. *gdk.Pixbuf).
type DisplayImage interface {
	Width() int
	Height() int
}
