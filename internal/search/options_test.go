package search

import (
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestNormalizeOptionsClampsLimitWithoutFallingBackToTwenty(t *testing.T) {
	opts := NormalizeOptions(domain.SearchOptions{Query: "linux", MaxResults: 60})
	if opts.MaxResults != 50 {
		t.Fatalf("MaxResults = %d, esperado 50", opts.MaxResults)
	}
	if len(opts.ResourceTypes) != 1 || opts.ResourceTypes[0] != domain.SearchResourceVideo {
		t.Fatalf("ResourceTypes = %#v", opts.ResourceTypes)
	}
}

func TestRequiresOfficialAPIIgnoresResourceTypesOnlyWhenVideo(t *testing.T) {
	base := domain.SearchOptions{Query: "lofi"}
	if RequiresOfficialAPI(base) {
		t.Fatal("sem ResourceTypes não exige API (normalize preenche vídeo)")
	}
	video := domain.SearchOptions{Query: "lofi", ResourceTypes: []domain.SearchResourceType{domain.SearchResourceVideo}}
	if RequiresOfficialAPI(video) {
		t.Fatal("busca só de vídeos sem filtros avançados não exige API")
	}
	any := domain.SearchOptions{Query: "lofi", ResourceTypes: []domain.SearchResourceType{domain.SearchResourceVideo, domain.SearchResourceChannel, domain.SearchResourcePlaylist}}
	if !RequiresOfficialAPI(any) {
		t.Fatal("busca com canais/playlists deveria exigir API")
	}
}

func TestNoAccountHint(t *testing.T) {
	any := domain.SearchOptions{Query: "lofi", ResourceTypes: []domain.SearchResourceType{domain.SearchResourcePlaylist}}
	if got := noAccountHint(any); !strings.Contains(got, "canais e playlists") {
		t.Fatalf("hint para playlist = %q", got)
	}
	video := domain.SearchOptions{Query: "lofi", ResourceTypes: []domain.SearchResourceType{domain.SearchResourceVideo}, RegionCode: "BR"}
	if got := noAccountHint(video); strings.Contains(got, "canais e playlists") {
		t.Fatalf("hint para filtro avançado = %q", got)
	}
}

func TestQueryString(t *testing.T) {
	opts := domain.SearchOptions{
		Query: "linux", ExactPhrase: "pc antigo", IncludeTerms: "gtk|mpv", ExcludeTerms: "electron -browser",
	}
	if got, want := QueryString(opts), `linux "pc antigo" gtk|mpv -electron -browser`; got != want {
		t.Fatalf("QueryString = %q, esperado %q", got, want)
	}
}

func TestValidateOptionsRejectsInvalidRanges(t *testing.T) {
	now := time.Now()
	cases := []domain.SearchOptions{
		{Query: "x", PublishedAfter: now, PublishedBefore: now.Add(-time.Hour)},
		{Query: "x", MinDuration: 10 * time.Minute, MaxDuration: time.Minute},
		{Query: "x", Location: "-23,-46"},
		{Query: "x", ShortsOnly: true, RegularOnly: true},
	}
	for _, opts := range cases {
		if err := ValidateOptions(opts); err == nil {
			t.Fatalf("ValidateOptions(%+v) deveria falhar", opts)
		}
	}
}

func TestRequiresOfficialAPI(t *testing.T) {
	if RequiresOfficialAPI(domain.SearchOptions{Query: "linux"}) {
		t.Fatal("busca simples não deveria exigir API")
	}
	if !RequiresOfficialAPI(domain.SearchOptions{Query: "linux", Caption: domain.SearchCaptionClosed}) {
		t.Fatal("legendas deveriam exigir API")
	}
	if !RequiresOfficialAPI(domain.SearchOptions{Query: "linux", ResourceTypes: []domain.SearchResourceType{domain.SearchResourceChannel}}) {
		t.Fatal("canais deveriam exigir API")
	}
	if !RequiresOfficialAPI(domain.SearchOptions{Query: "linux", SafeSearch: domain.SearchSafeModerate}) {
		t.Fatal("SafeSearch explícito deveria exigir API capaz")
	}
}

func TestValidateOptionsAllowsEmptyQueryWithChannel(t *testing.T) {
	err := ValidateOptions(domain.SearchOptions{ChannelID: "UC1234567890"})
	if err != nil {
		t.Fatalf("canal sem texto deveria ser válido: %v", err)
	}
}

func TestValidateOptionsRejectsPersonalVideoFiltersOnNonVideos(t *testing.T) {
	for _, opts := range []domain.SearchOptions{
		{Query: "x", ResourceTypes: []domain.SearchResourceType{domain.SearchResourceChannel}, WatchState: domain.SearchWatchWatched},
		{Query: "x", ResourceTypes: []domain.SearchResourceType{domain.SearchResourcePlaylist}, SavedState: domain.SearchSavedFavorites},
	} {
		if err := ValidateOptions(opts); err == nil {
			t.Fatalf("combinação deveria falhar antes do provider: %+v", opts)
		}
	}
}

func TestTopicFilterAcceptsNonVideoResources(t *testing.T) {
	err := ValidateOptions(domain.SearchOptions{
		Query: "linux", TopicID: "/m/04rwj", ResourceTypes: []domain.SearchResourceType{domain.SearchResourceChannel},
	})
	if err != nil {
		t.Fatalf("tópico documentado para canais foi recusado: %v", err)
	}
}

func TestValidateOptionsInputBounds(t *testing.T) {
	cases := []domain.SearchOptions{
		{Query: strings.Repeat("x", 501)},
		{Query: "x\ncontrole"},
		{Query: "x", ChannelID: "canal com espaço"},
		{Query: "x", Location: "91,0", LocationRadius: "25km"},
		{Query: "x", Location: "0,181", LocationRadius: "25km"},
		{Query: "x", Location: "0,0", LocationRadius: "1001km"},
		{Query: "x", Location: "0,0", LocationRadius: "25"},
		{Query: "x", PaidPromotion: domain.SearchNo},
	}
	for _, opts := range cases {
		if err := ValidateOptions(opts); err == nil {
			t.Fatalf("entrada deveria falhar: %+v", opts)
		}
	}
	if err := ValidateOptions(domain.SearchOptions{Query: "x", Location: "-23.55,-46.63", LocationRadius: "25km"}); err != nil {
		t.Fatalf("localização válida recusada: %v", err)
	}
}

func TestApplyLocalFiltersPersonalAndDuration(t *testing.T) {
	videos := []domain.Video{
		{ID: "continue", ChannelID: "sub", Duration: 10 * time.Minute},
		{ID: "done", ChannelID: "sub", Duration: 12 * time.Minute},
		{ID: "other", ChannelID: "other", Duration: 8 * time.Minute},
	}
	state := PersonalState{
		Progress: map[string]domain.ProgressInfo{
			"continue": {Position: time.Minute, Duration: 10 * time.Minute},
			"done":     {Completed: true, Position: 12 * time.Minute, Duration: 12 * time.Minute},
		},
		Favorites:          map[string]bool{"continue": true},
		SubscribedChannels: map[string]bool{"sub": true},
	}
	opts := domain.SearchOptions{
		Query: "x", OnlySubscribed: true, WatchState: domain.SearchWatchContinue,
		SavedState: domain.SearchSavedFavorites, MinDuration: 5 * time.Minute, MaxDuration: 11 * time.Minute,
	}
	got := ApplyLocalFilters(videos, opts, state)
	if len(got) != 1 || got[0].ID != "continue" {
		t.Fatalf("ApplyLocalFilters = %+v", got)
	}
}

func TestShortsApproximationBoundaries(t *testing.T) {
	videos := []domain.Video{{ID: "zero"}, {ID: "short", Duration: 3 * time.Minute}, {ID: "regular", Duration: 3*time.Minute + time.Second}}
	got := ApplyLocalFilters(videos, domain.SearchOptions{Query: "x", ShortsOnly: true}, PersonalState{})
	if len(got) != 1 || got[0].ID != "short" {
		t.Fatalf("Shorts = %+v", got)
	}
}

func TestRegularApproximationRejectsUnknownDuration(t *testing.T) {
	videos := []domain.Video{{ID: "unknown"}, {ID: "regular", Duration: 4 * time.Minute}}
	got := ApplyLocalFilters(videos, domain.SearchOptions{Query: "x", RegularOnly: true}, PersonalState{})
	if len(got) != 1 || got[0].ID != "regular" {
		t.Fatalf("RegularOnly = %+v", got)
	}
}

func TestEventTypeFilterNormalizesBothVocabularies(t *testing.T) {
	videos := []domain.Video{
		{ID: "ytdlp-live", LiveStatus: "is_live"},
		{ID: "api-live", LiveStatus: "live"},
		{ID: "ytdlp-done", LiveStatus: "was_live"},
		{ID: "api-done", LiveStatus: "completed"},
		{ID: "ytdlp-up", LiveStatus: "is_upcoming"},
		{ID: "api-up", LiveStatus: "upcoming"},
		{ID: "regular", LiveStatus: "not_live"},
	}
	tests := []struct {
		event domain.SearchEventType
		want  []string
	}{
		{domain.SearchEventLive, []string{"ytdlp-live", "api-live"}},
		{domain.SearchEventCompleted, []string{"ytdlp-done", "api-done"}},
		{domain.SearchEventUpcoming, []string{"ytdlp-up", "api-up"}},
	}
	for _, tt := range tests {
		got := ApplyLocalFilters(videos, domain.SearchOptions{Query: "x", EventType: tt.event}, PersonalState{})
		if len(got) != len(tt.want) {
			t.Fatalf("EventType %s: got %d, want %v", tt.event, len(got), tt.want)
		}
		for i, id := range tt.want {
			if got[i].ID != id {
				t.Errorf("EventType %s: got[%d]=%s, want %s", tt.event, i, got[i].ID, id)
			}
		}
	}
}

func TestNanoRankCombinesProviderOrderTextAndLocalSignals(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	videos := []domain.Video{
		{ID: "provider-first", Title: "Resumo semanal", ChannelID: "other", PublishedAt: now.Add(-300 * 24 * time.Hour)},
		{ID: "local-match", Title: "Linux desktop em PC antigo", ChannelID: "sub", PublishedAt: now.Add(-24 * time.Hour)},
	}
	RankWithNanoRank(videos, NormalizeOptions(domain.SearchOptions{Query: "linux desktop"}), PersonalState{
		SubscribedChannels: map[string]bool{"sub": true},
		Favorites:          map[string]bool{"local-match": true},
	}, now)
	if videos[0].ID != "local-match" {
		t.Fatalf("NanoRank = %v", []string{videos[0].ID, videos[1].ID})
	}
}

func TestNanoRankDoesNotOverrideExplicitOrder(t *testing.T) {
	videos := []domain.Video{{ID: "first", Title: "Outro"}, {ID: "second", Title: "Linux"}}
	RankWithNanoRank(videos, domain.SearchOptions{Query: "linux", Order: domain.SearchOrderDate}, PersonalState{}, time.Now())
	if videos[0].ID != "first" {
		t.Fatalf("ordem explícita foi alterada: %+v", videos)
	}
}
