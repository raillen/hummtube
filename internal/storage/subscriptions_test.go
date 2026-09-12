package storage

import (
	"context"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestSubscriptionVideosFiltersDatesWatchStateAndPaginates(t *testing.T) {
	repository := newTestRepo(t)
	ctx := context.Background()
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.Local)
	if err := repository.UpsertChannels(ctx, []domain.Channel{
		{ID: "sub", Title: "Inscrito", Subscribed: true},
		{ID: "other", Title: "Outro", Subscribed: false},
	}); err != nil {
		t.Fatal(err)
	}
	videos := []domain.Video{
		{ID: "today-unwatched", ChannelID: "sub", Title: "Hoje", Category: "Tecnologia", PublishedAt: now.Add(-time.Hour)},
		{ID: "yesterday-watched", ChannelID: "sub", Title: "Ontem", Category: "Tecnologia", PublishedAt: now.Add(-25 * time.Hour)},
		{ID: "not-subscribed", ChannelID: "other", Title: "Fora", PublishedAt: now.Add(-time.Hour)},
	}
	if err := repository.UpsertVideos(ctx, videos); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkProgress(ctx, videos[1].ID, domain.PlaybackProgress{
		VideoID: videos[1].ID, Position: time.Minute, Duration: 10 * time.Minute, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	page, err := repository.SubscriptionVideos(ctx, domain.SubscriptionVideoQuery{
		Limit: 1, From: now.Add(-48 * time.Hour), To: now.Add(time.Hour),
		Watch: domain.SubscriptionWatchUnwatched,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Videos) != 1 || page.Videos[0].ID != videos[0].ID || page.Videos[0].ChannelTitle != "Inscrito" {
		t.Fatalf("página inesperada: %+v", page)
	}
	if len(page.Categories) != 1 || page.Categories[0] != "Tecnologia" {
		t.Fatalf("categorias inesperadas: %+v", page.Categories)
	}
}

func TestManagedChannelsSupportsTagsAndBulkUnsubscribe(t *testing.T) {
	repository := newTestRepo(t)
	ctx := context.Background()
	if err := repository.SubscribeChannel(ctx, "channel-a", "Canal A"); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertChannels(ctx, []domain.Channel{{ID: "channel-a", Title: "Canal A", ThumbnailURL: "https://img/channel-a.jpg", Subscribed: true}}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetChannelTags(ctx, "channel-a", []string{"Música", "Favoritos", "música"}); err != nil {
		t.Fatal(err)
	}
	channels, err := repository.ManagedChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || len(channels[0].Tags) != 2 || channels[0].SubscribedAt.IsZero() || channels[0].Channel.ThumbnailURL != "https://img/channel-a.jpg" {
		t.Fatalf("canal gerenciado inesperado: %+v", channels)
	}
	if err := repository.BulkUnsubscribeChannels(ctx, []string{"channel-a", "channel-a"}); err != nil {
		t.Fatal(err)
	}
	remaining, err := repository.ManagedChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("desinscrição em lote falhou: %+v", remaining)
	}
}
