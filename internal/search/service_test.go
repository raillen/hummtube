package search

import (
	"context"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
)

type officialStub struct {
	called bool
	items  []domain.Video
	opts   domain.SearchOptions
}

func (s *officialStub) Search(_ context.Context, opts domain.SearchOptions) ([]domain.Video, string, error) {
	s.called = true
	s.opts = opts
	return s.items, "NEXT", nil
}
func (*officialStub) Subscriptions(context.Context, domain.PageOptions) ([]domain.Channel, string, error) {
	return nil, "", nil
}
func (*officialStub) Channels(context.Context, []string) ([]domain.Channel, error) { return nil, nil }
func (*officialStub) Uploads(context.Context, string, domain.PageOptions) ([]domain.Video, string, error) {
	return nil, "", nil
}

func TestServiceRequiresAccountForOfficialFilters(t *testing.T) {
	_, err := (Service{}).Search(context.Background(), domain.SearchOptions{Query: "linux", Caption: domain.SearchCaptionClosed})
	if err == nil {
		t.Fatal("era esperado erro de conta")
	}
}

func TestServiceRequiresCapableProviderForExplicitSafeSearch(t *testing.T) {
	_, err := (Service{}).Search(context.Background(), domain.SearchOptions{
		Query: "linux", SafeSearch: domain.SearchSafeModerate,
	})
	if err == nil {
		t.Fatal("SafeSearch explícito não pode ser silenciosamente ignorado pelo backend público")
	}
}

func TestServiceUsesOfficialAndPersonalFilters(t *testing.T) {
	provider := &officialStub{items: []domain.Video{
		{ID: "fav", ChannelID: "sub", ResourceType: domain.SearchResourceVideo},
		{ID: "other", ChannelID: "sub", ResourceType: domain.SearchResourceVideo},
	}}
	service := Service{
		Official: provider,
		Personal: PersonalState{Favorites: map[string]bool{"fav": true}, SubscribedChannels: map[string]bool{"sub": true}},
	}
	page, err := service.Search(context.Background(), domain.SearchOptions{
		Query: "linux", Caption: domain.SearchCaptionClosed, OnlySubscribed: true, SavedState: domain.SearchSavedFavorites,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !provider.called || page.Source != domain.SearchSourceOfficial || page.NextPageToken != "NEXT" {
		t.Fatalf("page = %+v called=%v", page, provider.called)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "fav" {
		t.Fatalf("items = %+v", page.Items)
	}
	if provider.opts.WatchState != domain.SearchWatchAny || provider.opts.SavedState != domain.SearchSavedAny ||
		provider.opts.OnlySubscribed || provider.opts.HideRejected {
		t.Fatalf("estado pessoal atravessou o provider: %+v", provider.opts)
	}
}

func TestServiceRejectsNonVideoPersonalFilterBeforeProvider(t *testing.T) {
	provider := &officialStub{}
	_, err := (Service{Official: provider}).Search(context.Background(), domain.SearchOptions{
		Query: "linux", ResourceTypes: []domain.SearchResourceType{domain.SearchResourceChannel}, WatchState: domain.SearchWatchWatched,
	})
	if err == nil {
		t.Fatal("combinação inválida deveria falhar")
	}
	if provider.called {
		t.Fatal("provider foi chamado antes da validação")
	}
}

func TestServiceRejectsOversizedOrControlPageToken(t *testing.T) {
	for _, token := range []string{string(make([]byte, 2049)), "NEXT\nINJECTED"} {
		_, err := (Service{Official: &officialStub{}}).Search(context.Background(), domain.SearchOptions{
			ChannelID: "UC_valid", PageToken: token, ResourceTypes: []domain.SearchResourceType{domain.SearchResourceChannel},
		})
		if err == nil {
			t.Fatalf("token inválido aceito: %q", token)
		}
	}
}

func TestServiceReturnsTypedVideoChannelAndPlaylistResults(t *testing.T) {
	provider := &officialStub{items: []domain.Video{
		{ID: "video", Title: "Linux video", ResourceType: domain.SearchResourceVideo},
		{ID: "channel", Title: "Linux channel", ResourceType: domain.SearchResourceChannel},
		{ID: "playlist", Title: "Linux playlist", ResourceType: domain.SearchResourcePlaylist},
	}}
	page, err := (Service{Official: provider}).Search(context.Background(), domain.SearchOptions{
		Query: "linux",
		ResourceTypes: []domain.SearchResourceType{
			domain.SearchResourceVideo,
			domain.SearchResourceChannel,
			domain.SearchResourcePlaylist,
		},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(page.Items) != 3 || page.Source != domain.SearchSourceOfficial || page.NextPageToken != "NEXT" {
		t.Fatalf("página mista = %+v", page)
	}
	seen := map[domain.SearchResourceType]bool{}
	for _, result := range page.Items {
		seen[result.ResourceType] = true
	}
	for _, resourceType := range []domain.SearchResourceType{
		domain.SearchResourceVideo, domain.SearchResourceChannel, domain.SearchResourcePlaylist,
	} {
		if !seen[resourceType] {
			t.Fatalf("tipo %q ausente: %+v", resourceType, page.Items)
		}
	}
}
