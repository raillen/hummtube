// Package search implements public YouTube discovery through yt-dlp and
// coordinates it with the official filtered-search adapter.
//
// Documento canônico: docs/03-implementation/SEARCH.md.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/playback"
)

// DefaultTimeout limita uma busca. O yt-dlp resolve o desafio JS antes de
// responder, então o teto é generoso de propósito.
const DefaultTimeout = 45 * time.Second

const maxPublicSearchOffset = 500

const maxRemoteListingOffset = 5000

// YtDlp busca no YouTube executando o yt-dlp.
type YtDlp struct {
	// Config shares only the installed binary/runtime policy with playback.
	// Cookies, PO providers and remote components are stripped from searches.
	Config playback.YtdlConfig
	// Timeout sobrescreve DefaultTimeout quando não for zero.
	Timeout time.Duration
	// Binary permite injetar um executável falso nos testes.
	Binary string
}

// flatEntry é o subconjunto do JSON de `--flat-playlist` que a UI usa.
type flatEntry struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Channel          string  `json:"channel"`
	Uploader         string  `json:"uploader"`
	ChannelID        string  `json:"channel_id"`
	Duration         float64 `json:"duration"`
	Description      string  `json:"description"`
	ViewCount        int64   `json:"view_count"`
	LiveStatus       string  `json:"live_status"`
	Timestamp        int64   `json:"timestamp"`
	ReleaseTimestamp int64   `json:"release_timestamp"`
	URL              string  `json:"url"`
	Thumbnails       []struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"thumbnails"`
}

type flatPlaylist struct {
	Entries []flatEntry `json:"entries"`
}

// Search consulta o YouTube e devolve resultados prontos para virar card.
func (y YtDlp) Search(ctx context.Context, opts domain.SearchOptions) ([]domain.Video, error) {
	opts = NormalizeOptions(opts)
	if err := ValidateOptions(opts); err != nil {
		return nil, fmt.Errorf("busca: %w", err)
	}
	if RequiresOfficialAPI(opts) {
		return nil, fmt.Errorf("busca: %s", noAccountHint(opts))
	}
	query := QueryString(opts)
	if query == "" {
		return nil, nil
	}

	binary := y.Binary
	if binary == "" {
		resolved, err := exec.LookPath("yt-dlp")
		if err != nil {
			return nil, fmt.Errorf("busca: yt-dlp não encontrado")
		}
		binary = resolved
	}

	timeout := y.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	offset, err := publicSearchOffset(opts.PageToken)
	if err != nil {
		return nil, err
	}
	requestedResults := offset + opts.MaxResults

	// A consulta entra como argumento separado, nunca concatenada em linha de
	// shell (docs/03-implementation/PLAYBACK_BACKENDS.md).
	args := append(publicSearchConfig(y.Config).CommandArgs(),
		"--flat-playlist", "--dump-single-json", "--no-warnings",
		"--", "ytsearch"+strconv.Itoa(requestedResults)+":"+query)

	output, _, err := playback.RunBoundedExtractor(ctx, binary, args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("busca: tempo esgotado")
		}
		return nil, fmt.Errorf("busca: %w", err)
	}
	videos, err := parseFlatPlaylist(output)
	if err != nil {
		return nil, err
	}
	if offset >= len(videos) {
		return nil, nil
	}
	end := offset + opts.MaxResults
	if end > len(videos) {
		end = len(videos)
	}
	return videos[offset:end], nil
}

func publicSearchOffset(pageToken string) (int, error) {
	if strings.TrimSpace(pageToken) == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(pageToken)
	if err != nil || offset < 0 || offset > maxPublicSearchOffset {
		return 0, fmt.Errorf("busca: token de página pública inválido")
	}
	return offset, nil
}

// PlaylistItems lists a public YouTube playlist without requiring OAuth. The
// token is a local offset because flat extraction has no API page token.
func (y YtDlp) PlaylistItems(ctx context.Context, playlistID string, opts domain.PageOptions) ([]domain.Video, string, error) {
	playlistID = strings.TrimSpace(playlistID)
	if !simpleIDPattern.MatchString(playlistID) {
		return nil, "", fmt.Errorf("playlist: ID inválido")
	}
	limit := opts.MaxResults
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	start := 1
	if opts.PageToken != "" {
		offset, err := strconv.Atoi(opts.PageToken)
		if err != nil || offset < 0 || offset > maxRemoteListingOffset {
			return nil, "", fmt.Errorf("playlist: token inválido")
		}
		start = offset + 1
	}
	binary := y.Binary
	if binary == "" {
		resolved, err := exec.LookPath("yt-dlp")
		if err != nil {
			return nil, "", fmt.Errorf("playlist: yt-dlp não encontrado")
		}
		binary = resolved
	}
	timeout := y.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	target := "https://www.youtube.com/playlist?list=" + url.QueryEscape(playlistID)
	args := append(publicSearchConfig(y.Config).CommandArgs(),
		"--flat-playlist", "--dump-single-json", "--no-warnings",
		"--playlist-start", strconv.Itoa(start), "--playlist-end", strconv.Itoa(start+limit-1),
		"--", target)
	output, _, err := playback.RunBoundedExtractor(ctx, binary, args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", fmt.Errorf("playlist: tempo esgotado")
		}
		return nil, "", fmt.Errorf("playlist: %w", err)
	}
	videos, err := parseFlatPlaylist(output)
	if err != nil {
		return nil, "", err
	}
	if len(videos) > limit {
		videos = videos[:limit]
	}
	next := ""
	if len(videos) == limit {
		next = strconv.Itoa(start - 1 + limit)
	}
	return videos, next, nil
}

// ChannelVideos lists the recent uploads of a public channel without OAuth.
// The token is a local offset, same convention as PlaylistItems.
func (y YtDlp) ChannelVideos(ctx context.Context, channelID string, opts domain.PageOptions) ([]domain.Video, string, error) {
	channelID = strings.TrimSpace(channelID)
	if !simpleIDPattern.MatchString(channelID) {
		return nil, "", fmt.Errorf("canal: ID inválido")
	}
	limit := opts.MaxResults
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	start := 1
	if opts.PageToken != "" {
		offset, err := strconv.Atoi(opts.PageToken)
		if err != nil || offset < 0 || offset > maxRemoteListingOffset {
			return nil, "", fmt.Errorf("canal: token inválido")
		}
		start = offset + 1
	}
	binary := y.Binary
	if binary == "" {
		resolved, err := exec.LookPath("yt-dlp")
		if err != nil {
			return nil, "", fmt.Errorf("canal: yt-dlp não encontrado")
		}
		binary = resolved
	}
	timeout := y.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	target := "https://www.youtube.com/channel/" + url.QueryEscape(channelID) + "/videos"
	args := append(publicSearchConfig(y.Config).CommandArgs(),
		"--flat-playlist", "--dump-single-json", "--no-warnings",
		"--playlist-start", strconv.Itoa(start), "--playlist-end", strconv.Itoa(start+limit-1),
		"--", target)
	output, _, err := playback.RunBoundedExtractor(ctx, binary, args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", fmt.Errorf("canal: tempo esgotado")
		}
		return nil, "", fmt.Errorf("canal: %w", err)
	}
	videos, err := parseFlatPlaylist(output)
	if err != nil {
		return nil, "", err
	}
	if len(videos) > limit {
		videos = videos[:limit]
	}
	next := ""
	if len(videos) == limit {
		next = strconv.Itoa(start - 1 + limit)
	}
	return videos, next, nil
}

func publicSearchConfig(cfg playback.YtdlConfig) playback.YtdlConfig {
	cfg.PlayerClient = playback.AutoPlayerClient
	cfg.POTProvider = ""
	cfg.POTMode = ""
	cfg.CookiesFromBrowser = ""
	cfg.CookiesFile = ""
	cfg.AllowRemoteComponents = false
	cfg.MaxHeight = 0
	return cfg
}

// parseFlatPlaylist converte a saída do yt-dlp em vídeos do domínio.
func parseFlatPlaylist(data []byte) ([]domain.Video, error) {
	var playlist flatPlaylist
	if err := json.Unmarshal(data, &playlist); err != nil {
		return nil, fmt.Errorf("busca: resposta ilegível: %w", err)
	}

	now := time.Now()
	videos := make([]domain.Video, 0, len(playlist.Entries))
	for _, entry := range playlist.Entries {
		if entry.ID == "" {
			continue
		}
		channel := entry.Channel
		if channel == "" {
			channel = entry.Uploader
		}
		videos = append(videos, domain.Video{
			ID:        entry.ID,
			ChannelID: entry.ChannelID,
			// O nome do canal viaja no próprio vídeo: resultado de busca não
			// tem linha em `channels` para consultar.
			ChannelTitle:       channel,
			Title:              entry.Title,
			Description:        entry.Description,
			DescriptionExcerpt: entry.Description,
			Duration:           time.Duration(entry.Duration) * time.Second,
			ViewCount:          entry.ViewCount,
			LiveStatus:         entry.LiveStatus,
			ResourceType:       domain.SearchResourceVideo,
			ExternalURL:        searchEntryURL(entry),
			PublishedAt:        searchEntryTime(entry),
			ThumbnailURL:       bestThumbnail(entry),
			FirstSeenAt:        now,
			LastSeenAt:         now,
		})
	}
	return videos, nil
}

func searchEntryURL(entry flatEntry) string {
	if strings.HasPrefix(entry.URL, "http://") || strings.HasPrefix(entry.URL, "https://") {
		return entry.URL
	}
	return "https://www.youtube.com/watch?v=" + entry.ID
}

func searchEntryTime(entry flatEntry) time.Time {
	timestamp := entry.Timestamp
	if timestamp == 0 {
		timestamp = entry.ReleaseTimestamp
	}
	if timestamp <= 0 {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

// bestThumbnail escolhe a maior imagem que ainda seja razoável para um card.
// O yt-dlp devolve de 32 px a 1280 px; pegar a maior desperdiça banda e
// memória de decode em hardware modesto.
func bestThumbnail(entry flatEntry) string {
	const ideal = 480
	best, bestScore := "", 1<<30
	for _, thumb := range entry.Thumbnails {
		if thumb.URL == "" {
			continue
		}
		score := thumb.Width - ideal
		if score < 0 {
			score = -score
		}
		if score < bestScore {
			best, bestScore = thumb.URL, score
		}
	}
	if best == "" {
		return "https://i.ytimg.com/vi/" + entry.ID + "/mqdefault.jpg"
	}
	return best
}
