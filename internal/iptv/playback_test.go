package iptv

import (
	"context"
	"testing"
)

func TestPlaybackServiceResolvesEphemeralURLAndCallsMediaCore(t *testing.T) {
	core := &recordingMediaCore{}
	service := PlaybackService{
		Sources:  recordingSourceLookup{source: SourceConfig{ID: "source", Name: "Fixture"}},
		Resolver: recordingStreamResolver{url: "https://stream.invalid/live.m3u8"},
		Core:     core,
	}
	item := Item{ID: "item", SourceID: "source", Title: "Canal Fixture", Kind: ContentKindTV}
	if err := service.Play(context.Background(), item); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if core.request.StreamURL != "https://stream.invalid/live.m3u8" || core.request.Application != "nanoiptv" {
		t.Fatalf("request=%+v", core.request)
	}
}

func TestPlaybackServiceRejectsIncompleteContract(t *testing.T) {
	err := (PlaybackService{}).Play(context.Background(), Item{ID: "item", SourceID: "source"})
	if err == nil {
		t.Fatal("Play() error = nil, want incomplete contract")
	}
}

type recordingSourceLookup struct{ source SourceConfig }

func (lookup recordingSourceLookup) GetIPTVSource(context.Context, string) (SourceConfig, bool, error) {
	return lookup.source, true, nil
}

type recordingStreamResolver struct{ url string }

func (resolver recordingStreamResolver) Resolve(context.Context, SourceConfig, Item) (string, error) {
	return resolver.url, nil
}

type recordingMediaCore struct{ request MediaCoreRequest }

func (core *recordingMediaCore) Play(_ context.Context, request MediaCoreRequest) error {
	core.request = request
	return nil
}
