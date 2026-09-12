// Package youtubeapi adapts the official YouTube Data API client to NanoTube's
// provider-independent domain ports. OAuth credentials never cross this package
// boundary and are supplied through a per-request service factory.
package youtubeapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

const maxPageSize = 50

var isoDurationPattern = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)

// ServiceFactory creates an authenticated SDK client for the active profile.
// A fresh service is intentionally resolved per operation so profile switching
// cannot accidentally reuse another account's OAuth transport.
type ServiceFactory func(context.Context) (*youtube.Service, error)

type Provider struct {
	NewService ServiceFactory
}

func (p Provider) service(ctx context.Context) (*youtube.Service, error) {
	if ctx == nil {
		return nil, errors.New("YouTube Data API: contexto nulo")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.NewService == nil {
		return nil, errors.New("YouTube Data API: cliente autenticado indisponível")
	}
	service, err := p.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("YouTube Data API: %w", err)
	}
	if service == nil {
		return nil, errors.New("YouTube Data API: cliente autenticado indisponível")
	}
	return service, nil
}

func (p Provider) Search(ctx context.Context, opts domain.SearchOptions) ([]domain.Video, string, error) {
	service, err := p.service(ctx)
	if err != nil {
		return nil, "", err
	}

	call := service.Search.List([]string{"snippet"}).
		MaxResults(int64(pageSize(opts.MaxResults))).
		Q(searchQuery(opts)).
		Type(resourceTypes(opts.ResourceTypes)...)
	if opts.PageToken != "" {
		call = call.PageToken(opts.PageToken)
	}
	if opts.Order != "" {
		call = call.Order(string(opts.Order))
	}
	if !opts.PublishedAfter.IsZero() {
		call = call.PublishedAfter(opts.PublishedAfter.UTC().Format(time.RFC3339))
	}
	if !opts.PublishedBefore.IsZero() {
		call = call.PublishedBefore(opts.PublishedBefore.UTC().Format(time.RFC3339))
	}
	if opts.ChannelID != "" {
		call = call.ChannelId(opts.ChannelID)
	}
	if opts.EventType != "" {
		call = call.EventType(string(opts.EventType))
	}
	if opts.Caption != "" && opts.Caption != domain.SearchCaptionAny {
		call = call.VideoCaption(string(opts.Caption))
	}
	if opts.Definition != "" && opts.Definition != domain.SearchDefinitionAny {
		call = call.VideoDefinition(string(opts.Definition))
	}
	if opts.Dimension != "" && opts.Dimension != domain.SearchDimensionAny {
		call = call.VideoDimension(string(opts.Dimension))
	}
	if opts.Duration != "" && opts.Duration != domain.SearchDurationAny && opts.Duration != domain.SearchDurationCustom {
		call = call.VideoDuration(string(opts.Duration))
	}
	if opts.License != "" && opts.License != domain.SearchLicenseAny {
		call = call.VideoLicense(string(opts.License))
	}
	if opts.VideoType != "" && opts.VideoType != domain.SearchVideoAny {
		call = call.VideoType(string(opts.VideoType))
	}
	if opts.Embeddable == domain.SearchYes {
		call = call.VideoEmbeddable("true")
	}
	if opts.Syndicated == domain.SearchYes {
		call = call.VideoSyndicated("true")
	}
	if opts.PaidPromotion == domain.SearchYes {
		call = call.VideoPaidProductPlacement("true")
	}
	if opts.CategoryID != "" {
		call = call.VideoCategoryId(opts.CategoryID)
	}
	if opts.TopicID != "" {
		call = call.TopicId(opts.TopicID)
	}
	if opts.RelevanceLanguage != "" {
		call = call.RelevanceLanguage(opts.RelevanceLanguage)
	}
	if opts.RegionCode != "" {
		call = call.RegionCode(opts.RegionCode)
	}
	if opts.SafeSearch == "" || opts.SafeSearch == domain.SearchSafeAny {
		// The API default is "moderate". NanoTube's "any" means no remote
		// filtering, so it must be explicit rather than silently narrowed.
		call = call.SafeSearch(string(domain.SearchSafeNone))
	} else {
		call = call.SafeSearch(string(opts.SafeSearch))
	}
	if opts.Location != "" {
		call = call.Location(opts.Location).LocationRadius(opts.LocationRadius)
	}

	response, err := call.Context(ctx).Do()
	if err != nil {
		return nil, "", apiError("buscar catálogo", err)
	}
	items := make([]domain.Video, 0, len(response.Items))
	videoIndexes := make(map[string]int)
	playlistIndexes := make(map[string]int)
	for _, item := range response.Items {
		result, ok := searchResult(item)
		if !ok {
			continue
		}
		items = append(items, result)
		if result.ResourceType == domain.SearchResourceVideo {
			videoIndexes[result.ID] = len(items) - 1
		} else if result.ResourceType == domain.SearchResourcePlaylist {
			playlistIndexes[result.ID] = len(items) - 1
		}
	}
	if err := enrichVideos(ctx, service, items, videoIndexes); err != nil {
		return nil, "", err
	}
	if err := enrichPlaylists(ctx, service, items, playlistIndexes); err != nil {
		return nil, "", err
	}
	return items, response.NextPageToken, nil
}

func enrichPlaylists(ctx context.Context, service *youtube.Service, items []domain.Video, indexes map[string]int) error {
	if len(indexes) == 0 {
		return nil
	}
	ids := make([]string, 0, len(indexes))
	for id := range indexes {
		ids = append(ids, id)
	}
	for start := 0; start < len(ids); start += maxPageSize {
		end := min(start+maxPageSize, len(ids))
		response, err := service.Playlists.List([]string{"contentDetails"}).Id(ids[start:end]...).Context(ctx).Do()
		if err != nil {
			return apiError("completar metadados das playlists", err)
		}
		for _, detail := range response.Items {
			if detail == nil || detail.ContentDetails == nil {
				continue
			}
			if index, ok := indexes[detail.Id]; ok {
				items[index].ResourceItemCount = int64(detail.ContentDetails.ItemCount)
			}
		}
	}
	return nil
}

func (p Provider) Subscriptions(ctx context.Context, opts domain.PageOptions) ([]domain.Channel, string, error) {
	service, err := p.service(ctx)
	if err != nil {
		return nil, "", err
	}
	call := service.Subscriptions.List([]string{"snippet"}).Mine(true).Order("alphabetical").MaxResults(int64(pageSize(opts.MaxResults)))
	if opts.PageToken != "" {
		call = call.PageToken(opts.PageToken)
	}
	response, err := call.Context(ctx).Do()
	if err != nil {
		return nil, "", apiError("listar inscrições", err)
	}
	channels := make([]domain.Channel, 0, len(response.Items))
	for _, item := range response.Items {
		if item == nil || item.Snippet == nil || item.Snippet.ResourceId == nil || item.Snippet.ResourceId.ChannelId == "" {
			continue
		}
		channels = append(channels, domain.Channel{
			ID: item.Snippet.ResourceId.ChannelId, Title: item.Snippet.Title,
			ThumbnailURL: thumbnailURL(item.Snippet.Thumbnails), Subscribed: true,
		})
	}
	return channels, response.NextPageToken, nil
}

func (p Provider) Channels(ctx context.Context, ids []string) ([]domain.Channel, error) {
	service, err := p.service(ctx)
	if err != nil {
		return nil, err
	}
	ids = cleanIDs(ids)
	channels := make([]domain.Channel, 0, len(ids))
	for start := 0; start < len(ids); start += maxPageSize {
		end := min(start+maxPageSize, len(ids))
		response, requestErr := service.Channels.List([]string{"snippet", "contentDetails"}).Id(ids[start:end]...).Context(ctx).Do()
		if requestErr != nil {
			return nil, apiError("resolver canais", requestErr)
		}
		for _, item := range response.Items {
			if item == nil || item.Id == "" {
				continue
			}
			channel := domain.Channel{ID: item.Id}
			if item.Snippet != nil {
				channel.Title = item.Snippet.Title
				channel.ThumbnailURL = thumbnailURL(item.Snippet.Thumbnails)
			}
			if item.ContentDetails != nil && item.ContentDetails.RelatedPlaylists != nil {
				channel.UploadsPlaylistID = item.ContentDetails.RelatedPlaylists.Uploads
			}
			channels = append(channels, channel)
		}
	}
	return channels, nil
}

func (p Provider) Uploads(ctx context.Context, uploadsPlaylistID string, opts domain.PageOptions) ([]domain.Video, string, error) {
	uploadsPlaylistID = strings.TrimSpace(uploadsPlaylistID)
	if uploadsPlaylistID == "" {
		return nil, "", errors.New("listar uploads: ID da playlist vazio")
	}
	service, err := p.service(ctx)
	if err != nil {
		return nil, "", err
	}
	call := service.PlaylistItems.List([]string{"snippet", "contentDetails"}).
		PlaylistId(uploadsPlaylistID).
		MaxResults(int64(pageSize(opts.MaxResults)))
	if opts.PageToken != "" {
		call = call.PageToken(opts.PageToken)
	}
	response, err := call.Context(ctx).Do()
	if err != nil {
		return nil, "", apiError("listar uploads", err)
	}
	items := make([]domain.Video, 0, len(response.Items))
	videoIndexes := make(map[string]int)
	for _, item := range response.Items {
		if item == nil || item.Snippet == nil || item.Snippet.ResourceId == nil || item.Snippet.ResourceId.VideoId == "" {
			continue
		}
		snippet := item.Snippet
		video := domain.Video{
			ID: snippet.ResourceId.VideoId, ChannelID: firstNonEmpty(snippet.VideoOwnerChannelId, snippet.ChannelId),
			ResourceType: domain.SearchResourceVideo, ExternalURL: videoURL(snippet.ResourceId.VideoId),
			ChannelTitle: firstNonEmpty(snippet.VideoOwnerChannelTitle, snippet.ChannelTitle), Title: snippet.Title,
			Description: snippet.Description, DescriptionExcerpt: excerpt(snippet.Description), PublishedAt: parseTime(snippet.PublishedAt),
			ThumbnailURL: thumbnailURL(snippet.Thumbnails),
		}
		items = append(items, video)
		videoIndexes[video.ID] = len(items) - 1
	}
	if err := enrichVideos(ctx, service, items, videoIndexes); err != nil {
		return nil, "", err
	}
	return items, response.NextPageToken, nil
}

func searchResult(item *youtube.SearchResult) (domain.Video, bool) {
	if item == nil || item.Id == nil || item.Snippet == nil {
		return domain.Video{}, false
	}
	resourceType, id := resourceIdentity(item.Id)
	if id == "" {
		return domain.Video{}, false
	}
	snippet := item.Snippet
	channelID := snippet.ChannelId
	if resourceType == domain.SearchResourceChannel {
		channelID = id
	}
	return domain.Video{
		ID: id, ChannelID: channelID, ResourceType: resourceType, ExternalURL: externalURL(resourceType, id),
		ChannelTitle: snippet.ChannelTitle, Title: snippet.Title, Description: snippet.Description, DescriptionExcerpt: excerpt(snippet.Description),
		PublishedAt: parseTime(snippet.PublishedAt), ThumbnailURL: thumbnailURL(snippet.Thumbnails),
		LiveStatus: normalizeLiveStatus(snippet.LiveBroadcastContent),
	}, true
}

func enrichVideos(ctx context.Context, service *youtube.Service, items []domain.Video, indexes map[string]int) error {
	if len(indexes) == 0 {
		return nil
	}
	ids := make([]string, 0, len(indexes))
	for id := range indexes {
		ids = append(ids, id)
	}
	for start := 0; start < len(ids); start += maxPageSize {
		end := min(start+maxPageSize, len(ids))
		response, err := service.Videos.List([]string{"snippet", "contentDetails", "statistics"}).Id(ids[start:end]...).Context(ctx).Do()
		if err != nil {
			return apiError("completar metadados dos vídeos", err)
		}
		for _, detail := range response.Items {
			if detail == nil {
				continue
			}
			index, ok := indexes[detail.Id]
			if !ok {
				continue
			}
			video := &items[index]
			if detail.Snippet != nil {
				video.ChannelID = detail.Snippet.ChannelId
				video.ChannelTitle = detail.Snippet.ChannelTitle
				video.Title = detail.Snippet.Title
				video.Description = detail.Snippet.Description
				video.DescriptionExcerpt = excerpt(detail.Snippet.Description)
				video.PublishedAt = parseTime(detail.Snippet.PublishedAt)
				video.Category = detail.Snippet.CategoryId
				video.LiveStatus = normalizeLiveStatus(detail.Snippet.LiveBroadcastContent)
				if thumbnail := thumbnailURL(detail.Snippet.Thumbnails); thumbnail != "" {
					video.ThumbnailURL = thumbnail
				}
			}
			if detail.ContentDetails != nil {
				video.Duration = parseISODuration(detail.ContentDetails.Duration)
			}
			if detail.Statistics != nil {
				video.ViewCount = saturatingInt64(detail.Statistics.ViewCount)
			}
		}
	}
	return nil
}

func resourceIdentity(id *youtube.ResourceId) (domain.SearchResourceType, string) {
	switch {
	case id.VideoId != "":
		return domain.SearchResourceVideo, id.VideoId
	case id.ChannelId != "":
		return domain.SearchResourceChannel, id.ChannelId
	case id.PlaylistId != "":
		return domain.SearchResourcePlaylist, id.PlaylistId
	default:
		return "", ""
	}
}

func searchQuery(opts domain.SearchOptions) string {
	parts := make([]string, 0, 4)
	if value := strings.TrimSpace(opts.Query); value != "" {
		parts = append(parts, value)
	}
	if value := strings.Trim(strings.TrimSpace(opts.ExactPhrase), `"`); value != "" {
		parts = append(parts, `"`+value+`"`)
	}
	if value := strings.TrimSpace(opts.IncludeTerms); value != "" {
		parts = append(parts, value)
	}
	for _, value := range strings.Fields(opts.ExcludeTerms) {
		if value = strings.TrimPrefix(value, "-"); value != "" {
			parts = append(parts, "-"+value)
		}
	}
	return strings.Join(parts, " ")
}

func resourceTypes(values []domain.SearchResourceType) []string {
	if len(values) == 0 {
		return []string{string(domain.SearchResourceVideo)}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, string(value))
	}
	return result
}

func pageSize(value int) int {
	if value <= 0 {
		return 20
	}
	return min(value, maxPageSize)
}

func cleanIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	return clean
}

func externalURL(resourceType domain.SearchResourceType, id string) string {
	switch resourceType {
	case domain.SearchResourceVideo:
		return videoURL(id)
	case domain.SearchResourceChannel:
		return "https://www.youtube.com/channel/" + id
	case domain.SearchResourcePlaylist:
		return "https://www.youtube.com/playlist?list=" + id
	default:
		return ""
	}
}

func videoURL(id string) string { return "https://www.youtube.com/watch?v=" + id }

func thumbnailURL(thumbnails *youtube.ThumbnailDetails) string {
	if thumbnails == nil {
		return ""
	}
	for _, thumbnail := range []*youtube.Thumbnail{
		thumbnails.Uhd, thumbnails.Qhd, thumbnails.Fhd, thumbnails.Maxres,
		thumbnails.Standard, thumbnails.High, thumbnails.Medium, thumbnails.Default,
	} {
		if thumbnail != nil && thumbnail.Url != "" {
			return thumbnail.Url
		}
	}
	return ""
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func parseISODuration(value string) time.Duration {
	parts := isoDurationPattern.FindStringSubmatch(value)
	if parts == nil {
		return 0
	}
	days, _ := strconv.ParseFloat(parts[1], 64)
	hours, _ := strconv.ParseFloat(parts[2], 64)
	minutes, _ := strconv.ParseFloat(parts[3], 64)
	seconds, _ := strconv.ParseFloat(parts[4], 64)
	return time.Duration((days*24*float64(time.Hour) + hours*float64(time.Hour) + minutes*float64(time.Minute) + seconds*float64(time.Second)))
}

func excerpt(value string) string {
	value = strings.TrimSpace(value)
	const maxRunes = 500
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "…"
}

func normalizeLiveStatus(value string) string {
	if value == "none" {
		return ""
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func saturatingInt64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}

func apiError(action string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", action, err)
	}
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("%s: falha de comunicação com o YouTube", action)
	}
	switch apiErr.Code {
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: sessão expirada; autentique a conta novamente", action)
	case http.StatusForbidden:
		return fmt.Errorf("%s: acesso negado ou quota da YouTube Data API esgotada", action)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s: limite temporário da YouTube Data API atingido", action)
	default:
		return fmt.Errorf("%s: YouTube Data API retornou HTTP %d", action, apiErr.Code)
	}
}
