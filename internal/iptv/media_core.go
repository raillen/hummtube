package iptv

import (
	"context"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// DomainPlaybackPort is the narrow adapter shape already used by NanoTube's
// toolkit-independent playback controller.
type DomainPlaybackPort interface {
	Play(context.Context, domain.PlaybackRequest) error
}

// DomainMediaCoreAdapter bridges the NanoTube playback port to the neutral
// NanoIPTV/CalmTV MediaCore request. CalmTV may use this shape directly or
// provide its own MediaCore implementation.
type DomainMediaCoreAdapter struct {
	Player DomainPlaybackPort
}

func (adapter DomainMediaCoreAdapter) Play(ctx context.Context, request MediaCoreRequest) error {
	if adapter.Player == nil {
		return ErrMediaCoreUnavailable
	}
	return adapter.Player.Play(ctx, domain.PlaybackRequest{
		SourceURL: request.StreamURL,
		VideoID:   request.ContentID,
	})
}
