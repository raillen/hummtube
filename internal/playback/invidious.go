// Package playback implements the Invidious fallback resolver
// (docs/03-implementation/PLAYBACK_BACKENDS.md).
package playback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const (
	defaultInvidiousInstance = "https://inv.nadeko.net"
	invidiousHTTPTimeout     = 8 * time.Second
	maxInvidiousBodyBytes    = 1024 * 1024 // 1 MB
)

// InvidiousFormat represents a media format stream returned by Invidious API.
type InvidiousFormat struct {
	URL          string `json:"url"`
	ITag         string `json:"itag"`
	Type         string `json:"type"`
	Quality      string `json:"quality"`
	Bitrate      string `json:"bitrate"`
	Container    string `json:"container"`
	Resolution   string `json:"resolution"`
	QualityLabel string `json:"qualityLabel"`
	VCodec       string `json:"vcodec"`
	ACodec       string `json:"acodec"`
	AudioTrack   string `json:"audioTrack"`
}

// InvidiousVideoJSON represents the structured response from /api/v1/videos/<id>.
type InvidiousVideoJSON struct {
	Title           string            `json:"title"`
	VideoID         string            `json:"videoId"`
	LengthSeconds   int64             `json:"lengthSeconds"`
	HlsURL          string            `json:"hlsUrl"`
	FormatStreams   []InvidiousFormat `json:"formatStreams"`
	AdaptiveFormats []InvidiousFormat `json:"adaptiveFormats"`
	Error           string            `json:"error"`
}

// InvidiousResolver resolves YouTube streams through public/custom Invidious instances.
type InvidiousResolver struct {
	InstanceURL   string
	MaxHeight     int
	Client        *http.Client
	AllowLoopback bool
	RequireMuxed  bool
}

// NewInvidiousResolver creates a new Invidious resolver with SSRF protection.
func NewInvidiousResolver(instanceURL string, maxHeight int) *InvidiousResolver {
	if instanceURL == "" {
		instanceURL = defaultInvidiousInstance
	}
	return &InvidiousResolver{
		InstanceURL: strings.TrimRight(instanceURL, "/"),
		MaxHeight:   maxHeight,
		Client: &http.Client{
			Timeout: invidiousHTTPTimeout,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 5 * time.Second,
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

// Resolve processes a playback request via Invidious API into a PlaybackPlan.
func (r *InvidiousResolver) Resolve(ctx context.Context, req domain.PlaybackRequest) (domain.PlaybackPlan, error) {
	if err := ctx.Err(); err != nil {
		return domain.PlaybackPlan{}, err
	}
	videoID := req.VideoID
	if videoID == "" {
		videoID = extractVideoID(req.SourceURL)
	}
	if videoID == "" {
		return domain.PlaybackPlan{}, fmt.Errorf("invidious: video ID ausente para %q", req.SourceURL)
	}

	apiURL := fmt.Sprintf("%s/api/v1/videos/%s", r.InstanceURL, url.PathEscape(videoID))
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return domain.PlaybackPlan{}, fmt.Errorf("invidious: url inválida: %w", err)
	}
	if err := validateInvidiousURL(parsed, r.AllowLoopback); err != nil {
		return domain.PlaybackPlan{}, fmt.Errorf("invidious: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return domain.PlaybackPlan{}, fmt.Errorf("invidious request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "NanoTube/1.0 (Linux; x86_64)")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := r.Client.Do(httpReq)
	if err != nil {
		return domain.PlaybackPlan{}, fmt.Errorf("invidious fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.PlaybackPlan{}, fmt.Errorf("invidious http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxInvidiousBodyBytes))
	if err != nil {
		return domain.PlaybackPlan{}, fmt.Errorf("invidious body: %w", err)
	}

	var data InvidiousVideoJSON
	if err := json.Unmarshal(body, &data); err != nil {
		return domain.PlaybackPlan{}, fmt.Errorf("invidious parse json: %w", err)
	}
	if data.Error != "" {
		return domain.PlaybackPlan{}, fmt.Errorf("invidious api error: %s", data.Error)
	}

	return r.buildPlan(data)
}

func (r *InvidiousResolver) buildPlan(data InvidiousVideoJSON) (domain.PlaybackPlan, error) {
	audioOnly := selectInvidiousWebAudio(data.AdaptiveFormats)
	// Formatos progressivos são o fallback mais compatível. Formatos
	// adaptativos liberam resoluções maiores quando o player combina a faixa de
	// vídeo com o áudio compartilhado.
	var bestCombined InvidiousFormat
	var bestCombinedHeight int
	combinedByHeight := make(map[int]InvidiousFormat)
	for _, f := range data.FormatStreams {
		if f.URL == "" {
			continue
		}
		h := parseResolutionHeight(f.Resolution, f.QualityLabel)
		if r.MaxHeight > 0 && h > r.MaxHeight {
			continue
		}
		if h > 0 {
			combinedByHeight[h] = f
		}
		if h > bestCombinedHeight || bestCombined.URL == "" {
			bestCombined = f
			bestCombinedHeight = h
		}
	}

	var bestVideo InvidiousFormat
	var bestVideoHeight int
	var bestAudio InvidiousFormat
	adaptiveByHeight := make(map[int]InvidiousFormat)
	for _, f := range data.AdaptiveFormats {
		if f.URL == "" {
			continue
		}
		if strings.HasPrefix(f.Type, "video/") {
			h := parseResolutionHeight(f.Resolution, f.QualityLabel)
			if r.MaxHeight > 0 && h > r.MaxHeight {
				continue
			}
			if h > 0 {
				adaptiveByHeight[h] = f
			}
			if h > bestVideoHeight || bestVideo.URL == "" {
				bestVideo = f
				bestVideoHeight = h
			}
		} else if strings.HasPrefix(f.Type, "audio/") {
			if bestAudio.URL == "" {
				bestAudio = f
			}
		}
	}

	if bestVideo.URL != "" && bestAudio.URL != "" && !r.RequireMuxed && bestVideoHeight > bestCombinedHeight {
		plan := domain.PlaybackPlan{
			Mode: domain.PlaybackModeResolvedMedia,
			Primary: domain.ResolvedStream{
				URL: bestVideo.URL,
			},
			Metadata: domain.PlaybackMetadata{
				VideoID:  data.VideoID,
				Title:    data.Title,
				Duration: time.Duration(data.LengthSeconds) * time.Second,
			},
			ExpiresAt: time.Now().Add(4 * time.Hour),
		}
		plan.Audio = &domain.ResolvedStream{URL: bestAudio.URL}
		plan.AudioOnly = audioOnly
		plan.Variants = buildInvidiousVariants(combinedByHeight, adaptiveByHeight)
		return plan, nil
	}

	if bestCombined.URL != "" {
		return domain.PlaybackPlan{
			Mode:    domain.PlaybackModeResolvedMedia,
			Primary: domain.ResolvedStream{URL: bestCombined.URL},
			Metadata: domain.PlaybackMetadata{
				VideoID: data.VideoID, Title: data.Title,
				Duration: time.Duration(data.LengthSeconds) * time.Second,
			},
			Variants:  buildInvidiousVariants(combinedByHeight, nil),
			AudioOnly: audioOnly,
			ExpiresAt: time.Now().Add(4 * time.Hour),
		}, nil
	}

	// HLS continua sendo o último fallback porque sua disponibilidade varia
	// entre clientes e instâncias.
	if data.HlsURL != "" {
		return domain.PlaybackPlan{
			Mode: domain.PlaybackModeResolvedMedia,
			Primary: domain.ResolvedStream{
				URL: data.HlsURL,
			},
			Metadata: domain.PlaybackMetadata{
				VideoID:  data.VideoID,
				Title:    data.Title,
				Duration: time.Duration(data.LengthSeconds) * time.Second,
			},
			AudioOnly: audioOnly,
			ExpiresAt: time.Now().Add(4 * time.Hour),
		}, nil
	}

	return domain.PlaybackPlan{}, errors.New("invidious: nenhum stream reproduzível encontrado")
}

func buildInvidiousVariants(combined, adaptive map[int]InvidiousFormat) []domain.PlaybackVariant {
	variants := make([]domain.PlaybackVariant, 0, len(combined)+len(adaptive))
	for height, format := range adaptive {
		if _, hasCombined := combined[height]; hasCombined {
			continue
		}
		variants = append(variants, invidiousVariant(format, height, false))
	}
	for height, format := range combined {
		variants = append(variants, invidiousVariant(format, height, true))
	}
	sort.SliceStable(variants, func(i, j int) bool { return variants[i].Height > variants[j].Height })
	return variants
}

func invidiousVariant(format InvidiousFormat, height int, hasAudio bool) domain.PlaybackVariant {
	id := strings.TrimSpace(format.ITag)
	if id == "" {
		id = fmt.Sprintf("%dp", height)
	}
	label := strings.TrimSpace(format.QualityLabel)
	if label == "" {
		label = fmt.Sprintf("%dp", height)
	}
	return domain.PlaybackVariant{
		ID: id, Label: label, Height: height, HasAudio: hasAudio,
		Stream: domain.ResolvedStream{URL: format.URL},
	}
}

func selectInvidiousWebAudio(formats []InvidiousFormat) *domain.ResolvedStream {
	for _, format := range formats {
		mediaType := strings.ToLower(strings.TrimSpace(strings.Split(format.Type, ";")[0]))
		switch mediaType {
		case "audio/mp4", "audio/webm", "audio/mpeg", "audio/ogg":
			if format.URL != "" {
				return &domain.ResolvedStream{URL: format.URL}
			}
		}
	}
	return nil
}

func parseResolutionHeight(resolution, qualityLabel string) int {
	var h int
	if resolution != "" {
		var w int
		if _, err := fmt.Sscanf(resolution, "%dx%d", &w, &h); err == nil && h > 0 {
			return h
		}
	}
	if qualityLabel != "" {
		if _, err := fmt.Sscanf(qualityLabel, "%dp", &h); err == nil && h > 0 {
			return h
		}
	}
	return 0
}

func extractVideoID(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	if u.Host == "youtu.be" {
		return strings.TrimPrefix(u.Path, "/")
	}
	if strings.Contains(u.Host, "youtube.com") {
		if v := u.Query().Get("v"); v != "" {
			return v
		}
		if strings.HasPrefix(u.Path, "/embed/") {
			return strings.TrimPrefix(u.Path, "/embed/")
		}
		if strings.HasPrefix(u.Path, "/v/") {
			return strings.TrimPrefix(u.Path, "/v/")
		}
		if strings.HasPrefix(u.Path, "/shorts/") {
			return strings.TrimPrefix(u.Path, "/shorts/")
		}
	}
	return ""
}

func validateInvidiousURL(u *url.URL, allowLoopback bool) error {
	if u == nil || u.Scheme != "https" && u.Scheme != "http" {
		return errors.New("esquema inválido")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return errors.New("host ausente")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !allowLoopback || isPrivateOrReservedIP(ip) {
			if !allowLoopback || !ip.IsLoopback() {
				return errors.New("ip privado bloqueado por segurança")
			}
		}
	}
	if !allowLoopback {
		for _, local := range []string{"localhost", "broadcasthost", ".local", ".internal", ".lan", ".home", ".corp", ".arpa", "metadata.google.internal", "instance-data"} {
			if host == strings.TrimPrefix(local, ".") || strings.HasSuffix(host, local) {
				return errors.New("host local bloqueado por segurança")
			}
		}
	}
	return nil
}

func isPrivateOrReservedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// InvidiousInstanceList is a curated list of reliable public instances.
var InvidiousInstanceList = []string{
	"https://inv.nadeko.net",
	"https://invidious.nerdvpn.de",
	"https://invidious.protokolla.fi",
	"https://yt.artemislena.eu",
}

func init() {
	sort.Strings(InvidiousInstanceList)
}
