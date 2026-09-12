package iptv

import (
	"context"
	"errors"
	"fmt"
)

var ErrMediaCoreUnavailable = errors.New("MediaCore indisponível")

// MediaCoreRequest is the integration contract sent to CalmTV's MediaCore.
// StreamURL is deliberately ephemeral: it is resolved immediately before
// playback and must never be serialized into the catalog database.
type MediaCoreRequest struct {
	Application string
	ContentID   string
	SourceID    string
	Title       string
	Kind        ContentKind
	StreamURL   string
}

// MediaCore is the only playback dependency NanoIPTV needs from the shell.
// CalmTV can implement it with libmpv, a local player process or another
// backend without changing NanoIPTV's catalog/UI code.
type MediaCore interface {
	Play(context.Context, MediaCoreRequest) error
}

// StreamResolver resolves an item's current stream from provider state. It
// may use CredentialRef through a SecretStore, but it does not return data to
// persistent storage.
type StreamResolver interface {
	Resolve(context.Context, SourceConfig, Item) (string, error)
}

// StreamingStreamResolver resolves a stream by streaming the source without
// buffering the entire playlist in memory. It stops at the first valid match.
type StreamingStreamResolver interface {
	ResolveFirst(ctx context.Context, source SourceConfig, options ParseOptions, match func(Item) bool) (string, error)
}

// SourceLookup supplies the public source configuration required for a
// provider resolution.
type SourceLookup interface {
	GetIPTVSource(context.Context, string) (SourceConfig, bool, error)
}

// PlaybackService adapts NanoIPTV items to the CalmTV MediaCore boundary.
type PlaybackService struct {
	Sources           SourceLookup
	Resolver          StreamResolver          // buffered (legacy fallback)
	StreamingResolver StreamingStreamResolver // streaming (preferred)
	Core              MediaCore
}

func (service PlaybackService) Play(ctx context.Context, item Item) error {
	if service.Sources == nil || service.Core == nil {
		return fmt.Errorf("reproduzir IPTV: contrato MediaCore incompleto")
	}
	if service.Resolver == nil && service.StreamingResolver == nil {
		return fmt.Errorf("reproduzir IPTV: nenhum resolver configurado")
	}
	if item.ID == "" || item.SourceID == "" {
		return fmt.Errorf("reproduzir IPTV: item sem identidade")
	}
	source, ok, err := service.Sources.GetIPTVSource(ctx, item.SourceID)
	if err != nil {
		return fmt.Errorf("reproduzir IPTV: obter fonte: %w", err)
	}
	if !ok {
		return fmt.Errorf("reproduzir IPTV: fonte não encontrada")
	}
	var streamURL string
	if service.StreamingResolver != nil {
		streamURL, err = service.StreamingResolver.ResolveFirst(ctx, source, ParseOptions{}, func(candidate Item) bool {
			return candidate.ID == item.ID
		})
	} else {
		streamURL, err = service.Resolver.Resolve(ctx, source, item)
	}
	if err != nil {
		return fmt.Errorf("reproduzir IPTV: resolver: %w", err)
	}
	if streamURL == "" {
		return fmt.Errorf("reproduzir IPTV: resolver retornou URL vazia")
	}
	if err := service.Core.Play(ctx, MediaCoreRequest{
		Application: "nanoiptv",
		ContentID:   item.ID,
		SourceID:    item.SourceID,
		Title:       item.Title,
		Kind:        item.Kind,
		StreamURL:   streamURL,
	}); err != nil {
		return fmt.Errorf("reproduzir IPTV: MediaCore: %w", err)
	}
	return nil
}
