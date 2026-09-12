package services

import (
	"context"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func (s *AppServices) ListSubscriptionVideos(ctx context.Context, query domain.SubscriptionVideoQuery) (domain.SubscriptionVideoPage, error) {
	return s.Repo.SubscriptionVideos(ctx, query)
}

func (s *AppServices) ListManagedChannels(ctx context.Context) ([]domain.ManagedChannel, error) {
	return s.Repo.ManagedChannels(ctx)
}

func (s *AppServices) BulkUnsubscribeChannels(ctx context.Context, channelIDs []string) error {
	return s.Repo.BulkUnsubscribeChannels(ctx, channelIDs)
}

func (s *AppServices) BulkFavoriteChannels(ctx context.Context, channelIDs []string) error {
	return s.Repo.BulkFavoriteChannels(ctx, channelIDs)
}

func (s *AppServices) BulkAddChannelsToFolder(ctx context.Context, folderID string, channelIDs []string) error {
	return s.Repo.BulkAddChannelsToFolder(ctx, folderID, channelIDs)
}

func (s *AppServices) SetChannelTags(ctx context.Context, channelID string, tags []string) error {
	return s.Repo.SetChannelTags(ctx, channelID, tags)
}
