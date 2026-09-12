package domain

import (
	"testing"
	"time"
)

func TestPlaylistFilterIsZero(t *testing.T) {
	cases := []struct {
		name   string
		filter PlaylistFilter
		want   bool
	}{
		{"vazio", PlaylistFilter{}, true},
		{"canal", PlaylistFilter{Channel: "c1"}, false},
		{"duracao", PlaylistFilter{Duration: SearchDurationShort}, false},
		{"duracao custom vazia", PlaylistFilter{Duration: SearchDurationCustom}, true},
		{"min duracao", PlaylistFilter{MinDuration: time.Minute}, false},
		{"idade", PlaylistFilter{Age: FeedAgeWeek}, false},
		{"categoria", PlaylistFilter{Category: "X"}, false},
		{"assistido", PlaylistFilter{Watched: SearchWatchWatched}, false},
		{"favorito", PlaylistFilter{Favorite: true}, false},
		{"conteudo", PlaylistFilter{Content: FeedLive}, false},
		{"shorts", PlaylistFilter{ShortsOnly: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.filter.IsZero(); got != tc.want {
				t.Errorf("IsZero = %v, quer %v (%+v)", got, tc.want, tc.filter)
			}
		})
	}
}

func TestPlaylistFilterActiveCount(t *testing.T) {
	cases := []struct {
		name   string
		filter PlaylistFilter
		want   int
	}{
		{"vazio", PlaylistFilter{}, 0},
		{"um", PlaylistFilter{Channel: "c1"}, 1},
		{"quatro", PlaylistFilter{Channel: "c1", Duration: SearchDurationLong, Age: FeedAgeMonth, Favorite: true}, 4},
		{"custom conta como um", PlaylistFilter{Duration: SearchDurationCustom, MinDuration: time.Minute, MaxDuration: time.Hour}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.filter.ActiveCount(); got != tc.want {
				t.Errorf("ActiveCount = %d, quer %d", got, tc.want)
			}
		})
	}
}
