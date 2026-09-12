package iptv

import (
	"context"
	"fmt"
)

// GuideSource is the network boundary for XMLTV providers.
type GuideSource interface {
	Fetch(context.Context, SourceConfig) ([]GuideChannel, []GuideProgram, error)
}

// GuideStore is the persistence boundary for an atomic EPG snapshot.
type GuideStore interface {
	ReplaceIPTVGuide(context.Context, string, []GuideChannel, []GuideProgram) error
}

// GuideSync coordinates fetch and persistence outside GTK.
type GuideSync struct {
	Source GuideSource
	Store  GuideStore
}

func (sync GuideSync) SyncXMLTV(ctx context.Context, config SourceConfig) error {
	if sync.Source == nil || sync.Store == nil {
		return fmt.Errorf("sincronizar XMLTV: dependência ausente")
	}
	if config.GuideURL == "" {
		return nil
	}
	channels, programs, err := sync.Source.Fetch(ctx, config)
	if err != nil {
		return fmt.Errorf("sincronizar XMLTV: fetch: %w", err)
	}
	if err := sync.Store.ReplaceIPTVGuide(ctx, config.ID, channels, programs); err != nil {
		return fmt.Errorf("sincronizar XMLTV: persistir: %w", err)
	}
	return nil
}
