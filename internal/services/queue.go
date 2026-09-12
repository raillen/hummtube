package services

import (
	"context"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func (s *AppServices) GetQueue(ctx context.Context) (domain.QueueSnapshot, error) {
	return s.Repo.PlaybackQueue(ctx)
}

func (s *AppServices) EnqueueVideo(ctx context.Context, video domain.Video) (domain.QueueItem, error) {
	if err := s.RememberVideo(ctx, video); err != nil {
		return domain.QueueItem{}, err
	}
	return s.Repo.EnqueueVideo(ctx, video.ID)
}

func (s *AppServices) ReorderQueue(ctx context.Context, itemIDs []string) error {
	return s.Repo.ReorderPlaybackQueue(ctx, itemIDs)
}

func (s *AppServices) RemoveQueueItem(ctx context.Context, itemID string) error {
	return s.Repo.RemoveQueueItem(ctx, itemID)
}

func (s *AppServices) MarkQueueItemPlayed(ctx context.Context, itemID string, played bool) error {
	return s.Repo.MarkQueueItemPlayed(ctx, itemID, played)
}

func (s *AppServices) ClearQueue(ctx context.Context, scope string) error {
	return s.Repo.ClearPlaybackQueue(ctx, scope)
}

func (s *AppServices) SaveQueuePreferences(ctx context.Context, preferences domain.QueuePreferences) error {
	return s.Repo.SaveQueuePreferences(ctx, preferences)
}

func (s *AppServices) SaveQueueAsPlaylist(ctx context.Context, name string) (domain.Playlist, error) {
	return s.Repo.SavePlaybackQueueAsPlaylist(ctx, name)
}
