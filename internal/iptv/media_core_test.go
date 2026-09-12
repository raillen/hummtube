package iptv

import (
	"context"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestDomainMediaCoreAdapterMapsEphemeralStream(t *testing.T) {
	port := &recordingDomainPlayback{}
	adapter := DomainMediaCoreAdapter{Player: port}
	err := adapter.Play(context.Background(), MediaCoreRequest{
		ContentID: "item", StreamURL: "https://stream.invalid/movie.m3u8",
	})
	if err != nil {
		t.Fatalf("Play: %v", err)
	}
	if port.request.VideoID != "item" || port.request.SourceURL == "" {
		t.Fatalf("request=%+v", port.request)
	}
}

func TestDomainMediaCoreAdapterRejectsMissingPort(t *testing.T) {
	if err := (DomainMediaCoreAdapter{}).Play(context.Background(), MediaCoreRequest{}); err != ErrMediaCoreUnavailable {
		t.Fatalf("err=%v, want ErrMediaCoreUnavailable", err)
	}
}

type recordingDomainPlayback struct{ request domain.PlaybackRequest }

func (port *recordingDomainPlayback) Play(_ context.Context, request domain.PlaybackRequest) error {
	port.request = request
	return nil
}
