package storage

import (
	"context"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestPlaybackQueuePersistsOrderPlayedStateAndPlaylist(t *testing.T) {
	repository := newTestRepo(t)
	ctx := context.Background()
	videos := []domain.Video{
		{ID: "queue-1", ChannelID: "channel", Title: "Primeiro", PublishedAt: time.Now()},
		{ID: "queue-2", ChannelID: "channel", Title: "Segundo", PublishedAt: time.Now()},
	}
	if err := repository.UpsertVideos(ctx, videos); err != nil {
		t.Fatal(err)
	}
	first, err := repository.EnqueueVideo(ctx, videos[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.EnqueueVideo(ctx, videos[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.ReorderPlaybackQueue(ctx, []string{second.ID, first.ID}); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkQueueItemPlayed(ctx, second.ID, true); err != nil {
		t.Fatal(err)
	}

	snapshot, err := repository.PlaybackQueue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 2 || snapshot.Items[0].ID != second.ID || snapshot.Items[0].State != domain.QueueItemPlayed {
		t.Fatalf("fila persistida inesperada: %+v", snapshot.Items)
	}
	playlist, err := repository.SavePlaybackQueueAsPlaylist(ctx, "Fila de sábado")
	if err != nil {
		t.Fatal(err)
	}
	playlistVideos, err := repository.PlaylistVideos(ctx, playlist.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(playlistVideos) != 2 || playlistVideos[0].ID != videos[1].ID || playlistVideos[1].ID != videos[0].ID {
		t.Fatalf("playlist não preservou a ordem: %+v", playlistVideos)
	}
}

func TestPlaybackQueueRemovePlayedPreference(t *testing.T) {
	repository := newTestRepo(t)
	ctx := context.Background()
	video := domain.Video{ID: "queue-remove", Title: "Remover depois", PublishedAt: time.Now()}
	if err := repository.UpsertVideos(ctx, []domain.Video{video}); err != nil {
		t.Fatal(err)
	}
	item, err := repository.EnqueueVideo(ctx, video.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveQueuePreferences(ctx, domain.QueuePreferences{Autoplay: true, RemovePlayed: true}); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkQueueItemPlayed(ctx, item.ID, true); err != nil {
		t.Fatal(err)
	}
	snapshot, err := repository.PlaybackQueue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Items) != 0 {
		t.Fatalf("item tocado deveria ter sido removido: %+v", snapshot.Items)
	}
}

func TestPlaybackQueueRejectsPartialReorder(t *testing.T) {
	repository := newTestRepo(t)
	ctx := context.Background()
	video := domain.Video{ID: "queue-partial", Title: "Parcial", PublishedAt: time.Now()}
	if err := repository.UpsertVideos(ctx, []domain.Video{video}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.EnqueueVideo(ctx, video.ID); err != nil {
		t.Fatal(err)
	}
	if err := repository.ReorderPlaybackQueue(ctx, nil); err == nil {
		t.Fatal("reordenação parcial deveria falhar")
	}
}
