package domain

import (
	"testing"
	"time"
)

func TestLiveKindOf(t *testing.T) {
	tests := []struct {
		raw  string
		want LiveKind
	}{
		{"", LiveUnknown},
		{"none", NotLive},
		{"not_live", NotLive},
		{"is_live", LiveNow},
		{"live", LiveNow},
		{"was_live", LiveCompleted},
		{"post_live", LiveCompleted},
		{"completed", LiveCompleted},
		{"is_upcoming", LiveUpcoming},
		{"upcoming", LiveUpcoming},
		{"IS_LIVE", LiveNow},
		{" is_upcoming ", LiveUpcoming},
		{"qualquer_coisa", LiveUnknown},
	}
	for _, tt := range tests {
		if got := LiveKindOf(tt.raw); got != tt.want {
			t.Errorf("LiveKindOf(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestVideoLiveHelpers(t *testing.T) {
	v := func(status string) Video { return Video{LiveStatus: status} }
	if !v("is_live").IsLiveNow() {
		t.Error("is_live deveria ser LiveNow")
	}
	if !v("live").IsLiveNow() {
		t.Error("live deveria ser LiveNow")
	}
	if v("was_live").IsLiveNow() {
		t.Error("was_live não é LiveNow")
	}
	if !v("upcoming").IsLiveUpcoming() {
		t.Error("upcoming deveria ser LiveUpcoming")
	}
	if !v("post_live").IsLiveCompleted() {
		t.Error("post_live deveria ser LiveCompleted")
	}
	if !v("completed").IsLive() {
		t.Error("completed deveria ser IsLive")
	}
	if v("not_live").IsLive() {
		t.Error("not_live não é IsLive")
	}
	if v("").IsLive() {
		t.Error("sem status não é IsLive")
	}
}

func TestFeedFilterMatches(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	recent := Video{ID: "r", LiveStatus: "not_live", Category: "Technology", PublishedAt: now.Add(-2 * time.Hour)}
	old := Video{ID: "o", LiveStatus: "not_live", Category: "Technology", PublishedAt: now.Add(-40 * 24 * time.Hour)}
	live := Video{ID: "l", LiveStatus: "is_live", Category: "Gaming", PublishedAt: now.Add(-30 * time.Minute)}
	vod := Video{ID: "v", LiveStatus: "was_live", Category: "Music", PublishedAt: now.Add(-10 * 24 * time.Hour)}

	tests := []struct {
		name   string
		filter FeedFilter
		want   map[string]bool
	}{
		{"sem filtro", FeedFilter{}, map[string]bool{"r": true, "o": true, "l": true, "v": true}},
		{"só vídeos comuns", FeedFilter{Content: FeedRegular}, map[string]bool{"r": true, "o": true, "l": false, "v": false}},
		{"só lives", FeedFilter{Content: FeedLive}, map[string]bool{"r": false, "o": false, "l": true, "v": true}},
		{"hoje", FeedFilter{Age: FeedAgeToday}, map[string]bool{"r": true, "o": false, "l": true, "v": false}},
		{"semana", FeedFilter{Age: FeedAgeWeek}, map[string]bool{"r": true, "o": false, "l": true, "v": false}},
		{"mês", FeedFilter{Age: FeedAgeMonth}, map[string]bool{"r": true, "o": false, "l": true, "v": true}},
		{"ano", FeedFilter{Age: FeedAgeYear}, map[string]bool{"r": true, "o": true, "l": true, "v": true}},
		{"categoria", FeedFilter{Category: "technology"}, map[string]bool{"r": true, "o": true, "l": false, "v": false}},
		{"live + semana", FeedFilter{Content: FeedLive, Age: FeedAgeWeek}, map[string]bool{"r": false, "o": false, "l": true, "v": false}},
		{"regular + tecnologia + semana", FeedFilter{Content: FeedRegular, Age: FeedAgeWeek, Category: "Technology"}, map[string]bool{"r": true, "o": false, "l": false, "v": false}},
	}
	videos := []Video{recent, old, live, vod}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyFeedFilter(tt.filter, videos, now)
			seen := make(map[string]bool, len(got))
			for _, v := range got {
				seen[v.ID] = true
			}
			for id, want := range tt.want {
				if seen[id] != want {
					t.Errorf("filtro %+v: vídeo %s presente=%v, want %v", tt.filter, id, seen[id], want)
				}
			}
		})
	}
}

func TestFeedFilterMatchesZeroPublished(t *testing.T) {
	now := time.Now()
	v := Video{ID: "sem-data", LiveStatus: "not_live"}
	if !(FeedFilter{Age: FeedAgeYear}).Matches(v, now) {
		t.Error("vídeo sem data publicada passa no filtro de idade")
	}
}

func TestFeedFilterIsZero(t *testing.T) {
	if !(FeedFilter{}).IsZero() {
		t.Error("filtro vazio deveria ser zero")
	}
	if (FeedFilter{Content: FeedLive}).IsZero() {
		t.Error("filtro com conteúdo não é zero")
	}
	if (FeedFilter{Category: "X"}).IsZero() {
		t.Error("filtro com categoria não é zero")
	}
}

func TestFeedCategories(t *testing.T) {
	videos := []Video{
		{Category: "Technology"},
		{Category: "technology"},
		{Category: "Music"},
		{Category: ""},
	}
	got := FeedCategories(videos)
	want := []string{"Technology", "Music"}
	if len(got) != len(want) {
		t.Fatalf("FeedCategories = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("FeedCategories[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if got := FeedCategories([]Video{{Category: ""}}); len(got) != 0 {
		t.Errorf("sem categorias deveria devolver lista vazia, veio %v", got)
	}
}

func TestApplyFeedFilterKeepsOrder(t *testing.T) {
	now := time.Now()
	videos := []Video{
		{ID: "a", LiveStatus: "is_live", Category: "Gaming", PublishedAt: now},
		{ID: "b", LiveStatus: "not_live", Category: "Music", PublishedAt: now},
		{ID: "c", LiveStatus: "upcoming", Category: "Gaming", PublishedAt: now},
	}
	got := ApplyFeedFilter(FeedFilter{Content: FeedLive, Category: "gaming"}, videos, now)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("ordem perdida: %v", got)
	}
}
