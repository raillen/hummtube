// Package sync orchestrates incremental catalog refresh from the YouTube
// provider into the local repository
// (docs/03-implementation/YOUTUBE_AND_AUTH.md).
package sync

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/nanotube/nanotube-web/internal/domain"
	"golang.org/x/sync/errgroup"
)

// CatalogRepo is the persistence surface needed by the sync service.
type CatalogRepo interface {
	UpsertChannels(ctx context.Context, channels []domain.Channel) error
	UpsertVideos(ctx context.Context, videos []domain.Video) error
	ChannelsByIDs(ctx context.Context, ids []string) ([]domain.Channel, error)
	SubscribedChannels(ctx context.Context) ([]domain.Channel, error)
	SetChannelSync(ctx context.Context, channelID, lastKnownVideoID, lastError string) error
}

// Options bounds the sync work (quota-conscious defaults).
type Options struct {
	SubscriptionsMaxPages int
	UploadsMaxPages       int
	Workers               int
}

// DefaultOptions are safe defaults for the reference hardware.
func DefaultOptions() Options {
	return Options{
		SubscriptionsMaxPages: 5,
		UploadsMaxPages:       1,
		Workers:               3,
	}
}

// Stats summarizes a Refresh run. Numbers are factual (fetched/synced), not
// inferred, to avoid performance/correctness claims without evidence.
type Stats struct {
	Subscriptions  int
	ChannelsSynced int
	VideosFetched  int
	ChannelErrors  []string
}

// Service refreshes the local catalog from a YouTubeProvider.
type Service struct {
	provider domain.YouTubeProvider
	repo     CatalogRepo
	opts     Options
}

// NewService builds a sync service.
func NewService(provider domain.YouTubeProvider, repo CatalogRepo, opts Options) (*Service, error) {
	if provider == nil {
		return nil, errors.New("sync: provider nulo")
	}
	return newService(repo, opts, provider)
}

// NewLocalService builds the RSS/yt-dlp service, which deliberately has no
// YouTube API provider.
func NewLocalService(repo CatalogRepo, opts Options) (*Service, error) {
	return newService(repo, opts, nil)
}

func newService(repo CatalogRepo, opts Options, provider domain.YouTubeProvider) (*Service, error) {
	if repo == nil {
		return nil, errors.New("sync: repository nulo")
	}
	if opts.SubscriptionsMaxPages <= 0 || opts.UploadsMaxPages <= 0 || opts.Workers <= 0 {
		return nil, fmt.Errorf("sync: opções inválidas: %+v", opts)
	}
	return &Service{provider: provider, repo: repo, opts: opts}, nil
}

// Refresh performs an incremental sync: subscriptions → channel details →
// uploads per channel → local upserts. It stops paginating a channel once a
// previously known video is found.
func (s *Service) Refresh(ctx context.Context, progress func(string)) (Stats, error) {
	var st Stats
	if s == nil || s.repo == nil {
		return st, errors.New("sync: serviço ou repository nulo")
	}
	if s.provider == nil {
		return st, errors.New("sync: provider nulo")
	}
	if ctx == nil {
		return st, errors.New("sync: contexto nulo")
	}
	if progress == nil {
		progress = func(string) {}
	}

	progress("buscando inscrições…")
	var subscribed []domain.Channel
	pageToken := ""
	for page := 0; page < s.opts.SubscriptionsMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return st, err
		}
		chs, next, err := s.provider.Subscriptions(ctx,
			domain.PageOptions{MaxResults: 50, PageToken: pageToken})
		if err != nil {
			return st, fmt.Errorf("subscriptions: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return st, err
		}
		subscribed = append(subscribed, chs...)
		if next == "" {
			break
		}
		pageToken = next
	}
	st.Subscriptions = len(subscribed)

	ids := make([]string, 0, len(subscribed))
	for _, ch := range subscribed {
		ids = append(ids, ch.ID)
	}

	progress("resolvendo canais…")
	detail, err := s.provider.Channels(ctx, ids)
	if err != nil {
		return st, fmt.Errorf("channels: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return st, err
	}
	byID := make(map[string]domain.Channel, len(detail))
	for _, ch := range detail {
		byID[ch.ID] = ch
	}

	resolved := make([]domain.Channel, 0, len(subscribed))
	for _, ch := range subscribed {
		if d, ok := byID[ch.ID]; ok {
			ch.Title = d.Title
			ch.UploadsPlaylistID = d.UploadsPlaylistID
		}
		resolved = append(resolved, ch)
	}

	progress("gravando inscrições…")
	if err := s.repo.UpsertChannels(ctx, resolved); err != nil {
		return st, fmt.Errorf("upsert channels: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return st, err
	}

	// Recarrega para obter os marcadores de sync atuais (last_known_video_id).
	current, err := s.repo.ChannelsByIDs(ctx, ids)
	if err != nil {
		return st, fmt.Errorf("load sync state: %w", err)
	}
	syncState := make(map[string]domain.Channel, len(current))
	for _, ch := range current {
		syncState[ch.ID] = ch
	}

	// Fan-out de uploads por canal, limitado por Workers.
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.opts.Workers)
	var (
		mu        sync.Mutex
		allVideos []domain.Video
	)
	for _, ch := range resolved {
		ch := ch
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			state := syncState[ch.ID]
			vids, werr := s.syncChannel(gctx, ch, state, progress)
			mu.Lock()
			allVideos = append(allVideos, vids...)
			st.ChannelsSynced++
			if werr != nil {
				st.ChannelErrors = append(st.ChannelErrors, ch.ID+": "+werr.Error())
			}
			mu.Unlock()

			if len(vids) > 0 {
				if err := s.repo.UpsertVideos(gctx, vids); err != nil {
					return fmt.Errorf("channel %s upsert videos: %w", ch.ID, err)
				}
			}
			var markerErr error
			if werr != nil {
				markerErr = s.repo.SetChannelSync(gctx, ch.ID, state.LastKnownVideoID, werr.Error())
			} else if len(vids) > 0 {
				markerErr = s.repo.SetChannelSync(gctx, ch.ID, vids[0].ID, "")
			}
			if markerErr != nil {
				return fmt.Errorf("channel %s sync marker: %w", ch.ID, markerErr)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return st, fmt.Errorf("sync channels: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return st, err
	}

	dedup := make(map[string]domain.Video, len(allVideos))
	for _, v := range allVideos {
		if v.ID != "" {
			dedup[v.ID] = v
		}
	}
	st.VideosFetched = len(dedup)
	if err := ctx.Err(); err != nil {
		return st, err
	}
	return st, nil
}

// RefreshLocal atualiza inscrições locais sem OAuth: RSS + yt-dlp.
func (s *Service) RefreshLocal(ctx context.Context, progress func(string)) (Stats, error) {
	var st Stats
	if s == nil || s.repo == nil {
		return st, errors.New("sync: serviço ou repository nulo")
	}
	if ctx == nil {
		return st, errors.New("sync: contexto nulo")
	}
	if progress == nil {
		progress = func(string) {}
	}
	channels, err := s.repo.SubscribedChannels(ctx)
	if err != nil {
		return st, fmt.Errorf("subscribed: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return st, err
	}
	if len(channels) == 0 {
		return st, nil
	}
	st.Subscriptions = len(channels)
	ids := make([]string, 0, len(channels))
	for _, ch := range channels {
		ids = append(ids, ch.ID)
	}
	current, err := s.repo.ChannelsByIDs(ctx, ids)
	if err != nil {
		return st, fmt.Errorf("load sync state: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return st, err
	}
	syncState := make(map[string]domain.Channel, len(current))
	for _, ch := range current {
		syncState[ch.ID] = ch
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.opts.Workers)
	var (
		mu        sync.Mutex
		allVideos []domain.Video
	)
	for _, ch := range channels {
		ch := ch
		g.Go(func() error {
			state := syncState[ch.ID]
			vids, werr := s.syncChannelLocal(gctx, ch, state, progress)
			mu.Lock()
			allVideos = append(allVideos, vids...)
			st.ChannelsSynced++
			if werr != nil {
				st.ChannelErrors = append(st.ChannelErrors, ch.ID+": "+werr.Error())
			}
			mu.Unlock()

			if len(vids) > 0 {
				if err := s.repo.UpsertVideos(gctx, vids); err != nil {
					return fmt.Errorf("channel %s upsert videos: %w", ch.ID, err)
				}
			}
			var markerErr error
			if werr != nil {
				markerErr = s.repo.SetChannelSync(gctx, ch.ID, state.LastKnownVideoID, werr.Error())
			} else if len(vids) > 0 {
				markerErr = s.repo.SetChannelSync(gctx, ch.ID, vids[0].ID, "")
			}
			if markerErr != nil {
				return fmt.Errorf("channel %s sync marker: %w", ch.ID, markerErr)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return st, fmt.Errorf("sync local: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return st, err
	}
	dedup := make(map[string]domain.Video, len(allVideos))
	for _, v := range allVideos {
		if v.ID != "" {
			dedup[v.ID] = v
		}
	}
	st.VideosFetched = len(dedup)
	return st, nil
}

func (s *Service) syncChannelLocal(ctx context.Context, ch domain.Channel, state domain.Channel, progress func(string)) ([]domain.Video, error) {
	progress("local: " + ch.Title)
	vids, err := FetchChannelFeed(ctx, ch.ID)
	if err != nil {
		return nil, err
	}
	var out []domain.Video
	for _, v := range vids {
		if state.LastKnownVideoID != "" && v.ID == state.LastKnownVideoID {
			return out, nil
		}
		out = append(out, v)
	}
	return out, nil
}

// syncChannel fetches uploads for one channel, stopping at the last known
// video when present.
func (s *Service) syncChannel(ctx context.Context, ch domain.Channel, state domain.Channel, progress func(string)) ([]domain.Video, error) {
	if ch.UploadsPlaylistID == "" {
		return nil, fmt.Errorf("sem uploads playlist id")
	}
	progress("inscrições: " + ch.Title)
	var out []domain.Video
	pageToken := ""
	for page := 0; page < s.opts.UploadsMaxPages; page++ {
		vids, next, err := s.provider.Uploads(ctx, ch.UploadsPlaylistID,
			domain.PageOptions{MaxResults: 50, PageToken: pageToken})
		if err != nil {
			return nil, fmt.Errorf("uploads: %w", err)
		}
		for _, v := range vids {
			if state.LastKnownVideoID != "" && v.ID == state.LastKnownVideoID {
				return out, nil
			}
			out = append(out, v)
		}
		if next == "" {
			break
		}
		pageToken = next
	}
	return out, nil
}
