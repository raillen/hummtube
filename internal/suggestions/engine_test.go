package suggestions

import (
	"context"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func testProfile() domain.InterestProfile {
	now := time.Now()
	return domain.InterestProfile{
		Topics: []domain.TopicScore{
			{Topic: "linux", Score: 1.0},
			{Topic: "golang", Score: 0.6},
		},
		Channels: map[string]domain.ChannelAffinity{
			"ch-linux": {ChannelID: "ch-linux", ChannelName: "Linux Channel", Score: 0.9, Subscribed: true},
			"ch-go":    {ChannelID: "ch-go", ChannelName: "Go Gopher", Score: 0.7, Subscribed: true},
			"ch-cook":  {ChannelID: "ch-cook", ChannelName: "Cooking", Score: 0.1},
		},
		WatchState: map[string]domain.ProgressInfo{
			"v-progress": {Position: 30 * time.Minute, Duration: 60 * time.Minute, UpdatedAt: now.Add(-1 * time.Hour)},
			"v-finished": {Position: 60 * time.Minute, Duration: 60 * time.Minute, UpdatedAt: now.Add(-30 * 24 * time.Hour), Completed: true},
		},
		ExcludedVideoIDs:   map[string]bool{},
		ExcludedChannelIDs: map[string]bool{},
	}
}

func testCandidates() []domain.Video {
	now := time.Now()
	return []domain.Video{
		{ID: "v-linux-1", ChannelID: "ch-linux", Title: "Introdução ao Linux para servidores", Category: "Technology", PublishedAt: now.Add(-1 * 24 * time.Hour)},
		{ID: "v-linux-2", ChannelID: "ch-linux", Title: "Dicas avançadas de Linux", Category: "Technology", PublishedAt: now.Add(-40 * 24 * time.Hour)},
		{ID: "v-go-1", ChannelID: "ch-go", Title: "Golang contextos explicados", Category: "Technology", PublishedAt: now.Add(-3 * 24 * time.Hour)},
		{ID: "v-progress", ChannelID: "ch-go", Title: "Curso de Golang parte 3", Category: "Education", PublishedAt: now.Add(-5 * 24 * time.Hour)},
		{ID: "v-finished", ChannelID: "ch-linux", Title: "História do Linux", Category: "Education", PublishedAt: now.Add(-300 * 24 * time.Hour)},
		{ID: "v-cook-1", ChannelID: "ch-cook", Title: "Receitas rápidas", Category: "Howto", PublishedAt: now.Add(-2 * 24 * time.Hour)},
	}
}

func TestBuildHomeSections(t *testing.T) {
	eng := NewEngine()
	home, err := eng.BuildHome(context.Background(), testProfile(), testCandidates())
	if err != nil {
		t.Fatal(err)
	}

	if len(home.ContinueWatching) != 1 || home.ContinueWatching[0].Video.ID != "v-progress" {
		t.Errorf("ContinueWatching = %+v, want v-progress", ids(home.ContinueWatching))
	}
	if len(home.Rediscovery) != 1 || home.Rediscovery[0].Video.ID != "v-finished" {
		t.Errorf("Rediscovery = %+v, want v-finished", ids(home.Rediscovery))
	}
	if len(home.RecentSubscriptions) == 0 {
		t.Error("RecentSubscriptions vazio")
	}
	for _, s := range home.RecentSubscriptions {
		if !testProfile().Channels[s.Video.ChannelID].Subscribed {
			t.Errorf("RecentSubscriptions inclui canal não assinado: %s", s.Video.ChannelID)
		}
	}
	if len(home.TopicSections) == 0 {
		t.Error("TopicSections vazio")
	}
}

func TestExclusionsHonored(t *testing.T) {
	p := testProfile()
	p.ExcludedVideoIDs["v-linux-1"] = true
	p.ExcludedChannelIDs["ch-cook"] = true
	eng := NewEngine()
	home, err := eng.BuildHome(context.Background(), p, testCandidates())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range home.ForYou {
		if v.Video.ID == "v-linux-1" {
			t.Error("v-linux-1 não deveria aparecer (excluído)")
		}
		if v.Video.ChannelID == "ch-cook" {
			t.Error("ch-cook não deveria aparecer (canal ignorado)")
		}
	}
}

func TestTopicAffinityRanksTopicFirst(t *testing.T) {
	eng := NewEngine()
	home, _ := eng.BuildHome(context.Background(), testProfile(), testCandidates())
	first := home.ForYou[0].Video
	if first.ChannelID != "ch-linux" {
		t.Errorf("ForYou[0] = %s, want canal com maior afinidade de tema (ch-linux)", first.ID)
	}
}

func TestExplain(t *testing.T) {
	eng := NewEngine()
	_, _ = eng.BuildHome(context.Background(), testProfile(), testCandidates())
	exp := eng.Explain("v-linux-1")
	if exp.VideoID != "v-linux-1" {
		t.Fatalf("Explain videoID = %s", exp.VideoID)
	}
	if len(exp.Reasons) == 0 {
		t.Fatal("sem razões")
	}
	found := false
	for _, r := range exp.Reasons {
		if r.Kind == ReasonTopic {
			found = true
		}
	}
	if !found {
		t.Errorf("sem razão de tema: %+v", exp.Reasons)
	}
}

func TestDiversityBreaksChannelMonopoly(t *testing.T) {
	// Vários vídeos do mesmo canal devem ser intercalados com outros canais.
	now := time.Now()
	p := testProfile()
	var cands []domain.Video
	for i := 0; i < 10; i++ {
		cands = append(cands, domain.Video{
			ID: "l" + string(rune('0'+i)), ChannelID: "ch-linux", Title: "Linux tutorial variante",
			PublishedAt: now.Add(-time.Duration(i) * 24 * time.Hour),
		})
	}
	cands = append(cands, domain.Video{ID: "g1", ChannelID: "ch-go", Title: "Golang tutorial", PublishedAt: now.Add(-time.Hour)})

	eng := NewEngine()
	home, _ := eng.BuildHome(context.Background(), p, cands)
	seen := 0
	for _, v := range home.ForYou {
		if v.Video.ChannelID == "ch-go" {
			seen++
		}
	}
	if seen == 0 {
		t.Error("diversidade não promoveu o segundo canal")
	}
}

func TestShortsExcludedFromHome(t *testing.T) {
	now := time.Now()
	cands := []domain.Video{
		{ID: "v-long", ChannelID: "ch-linux", Title: "Vídeo normal", PublishedAt: now.Add(-time.Hour), Duration: 10 * time.Minute},
		{ID: "v-short", ChannelID: "ch-linux", Title: "Short de 2min", PublishedAt: now.Add(-time.Hour), Duration: 2 * time.Minute},
	}
	eng := NewEngine()
	home, err := eng.BuildHome(context.Background(), testProfile(), cands)
	if err != nil {
		t.Fatal(err)
	}
	all := append(append(append(append([]domain.HomeVideo{},
		home.ContinueWatching...), home.ForYou...), home.RecentSubscriptions...), home.Rediscovery...)
	for _, s := range home.TopicSections {
		all = append(all, s.Videos...)
	}
	for _, hv := range all {
		if hv.Video.ID == "v-short" {
			t.Fatal("Short não deveria aparecer na Home")
		}
	}
	if len(home.ForYou) == 0 {
		t.Fatal("vídeo normal deveria aparecer")
	}
}

func TestFavoriteAndFrequentChannelAffinity(t *testing.T) {
	now := time.Now()
	p := domain.InterestProfile{
		Channels: map[string]domain.ChannelAffinity{
			"ch-fav": {
				ChannelID:      "ch-fav",
				ChannelName:    "Canal Favorito",
				Score:          0.95,
				Subscribed:     true,
				Favorite:       true,
				CompletedCount: 5,
			},
			"ch-freq": {
				ChannelID:      "ch-freq",
				ChannelName:    "Canal Frequente",
				Score:          0.80,
				Subscribed:     false,
				CompletedCount: 4,
			},
		},
		WatchState:         map[string]domain.ProgressInfo{},
		ExcludedVideoIDs:   map[string]bool{},
		ExcludedChannelIDs: map[string]bool{},
	}
	cands := []domain.Video{
		{ID: "v-fav", ChannelID: "ch-fav", Title: "Vídeo do canal favorito", PublishedAt: now.Add(-time.Hour)},
		{ID: "v-freq", ChannelID: "ch-freq", Title: "Vídeo do canal frequente", PublishedAt: now.Add(-time.Hour)},
	}
	eng := NewEngine()
	home, err := eng.BuildHome(context.Background(), p, cands)
	if err != nil {
		t.Fatalf("BuildHome: %v", err)
	}

	expFav := eng.Explain("v-fav")
	foundFavReason := false
	for _, r := range expFav.Reasons {
		if r.Label == "canal favorito" {
			foundFavReason = true
			break
		}
	}
	if !foundFavReason {
		t.Errorf("esperava motivo 'canal favorito', obteve: %+v", expFav.Reasons)
	}

	expFreq := eng.Explain("v-freq")
	foundFreqReason := false
	for _, r := range expFreq.Reasons {
		if r.Label == "canal frequentemente assistido" {
			foundFreqReason = true
			break
		}
	}
	if !foundFreqReason {
		t.Errorf("esperava motivo 'canal frequentemente assistido', obteve: %+v", expFreq.Reasons)
	}

	if len(home.ForYou) < 2 || home.ForYou[0].Video.ID != "v-fav" {
		t.Errorf("canal favorito com maior afinidade deveria ser ranqueado em primeiro: %+v", home.ForYou)
	}
}

func ids(vs []domain.HomeVideo) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Video.ID)
	}
	return out
}
