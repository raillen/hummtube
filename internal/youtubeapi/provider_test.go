package youtubeapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func TestSearchMapsFiltersAndMixedResults(t *testing.T) {
	t.Parallel()
	queries := make(chan url.Values, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries <- r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/search"):
			_, _ = fmt.Fprint(w, `{
				"nextPageToken":"NEXT","items":[
					{"id":{"kind":"youtube#video","videoId":"video-1"},"snippet":{"title":"Video","channelId":"channel-1","channelTitle":"Canal","description":"Descrição","publishedAt":"2025-01-02T03:04:05Z","liveBroadcastContent":"live","thumbnails":{"high":{"url":"https://img/video.jpg"}}}},
					{"id":{"kind":"youtube#channel","channelId":"channel-2"},"snippet":{"title":"Outro canal","channelId":"channel-2","thumbnails":{"medium":{"url":"https://img/channel.jpg"}}}},
					{"id":{"kind":"youtube#playlist","playlistId":"playlist-1"},"snippet":{"title":"Playlist","channelId":"channel-1"}}
				]}`)
		case strings.HasSuffix(r.URL.Path, "/videos"):
			_, _ = fmt.Fprint(w, `{"items":[{"id":"video-1","snippet":{"title":"Video completo","channelId":"channel-1","channelTitle":"Canal","categoryId":"10","publishedAt":"2025-01-02T03:04:05Z","liveBroadcastContent":"live","thumbnails":{"maxres":{"url":"https://img/max.jpg"}}},"contentDetails":{"duration":"PT1H2M3.5S"},"statistics":{"viewCount":"9223372036854775808"}}]}`)
		case strings.HasSuffix(r.URL.Path, "/playlists"):
			_, _ = fmt.Fprint(w, `{"items":[{"id":"playlist-1","contentDetails":{"itemCount":42}}]}`)
		default:
			http.Error(w, "rota inesperada", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	provider := testProvider(t, server)
	items, next, err := provider.Search(context.Background(), domain.SearchOptions{
		Query: "linux", ExactPhrase: "desktop leve", ExcludeTerms: "shorts propaganda",
		MaxResults: 24, PageToken: "PAGE", ResourceTypes: []domain.SearchResourceType{
			domain.SearchResourceVideo, domain.SearchResourceChannel, domain.SearchResourcePlaylist,
		}, Order: domain.SearchOrderDate, PublishedAfter: time.Date(2025, 1, 1, 0, 0, 0, 0, time.FixedZone("BRT", -3*60*60)),
		RelevanceLanguage: "pt-BR", RegionCode: "BR", SafeSearch: domain.SearchSafeStrict,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if next != "NEXT" || len(items) != 3 {
		t.Fatalf("resultado = %#v next=%q", items, next)
	}
	if items[0].Title != "Video completo" || items[0].Duration != time.Hour+2*time.Minute+3500*time.Millisecond ||
		items[0].ViewCount != int64(^uint64(0)>>1) || items[0].ThumbnailURL != "https://img/max.jpg" {
		t.Fatalf("vídeo enriquecido = %+v", items[0])
	}
	if items[1].ResourceType != domain.SearchResourceChannel || items[1].ExternalURL != "https://www.youtube.com/channel/channel-2" {
		t.Fatalf("canal = %+v", items[1])
	}
	if items[2].ResourceType != domain.SearchResourcePlaylist || items[2].ExternalURL != "https://www.youtube.com/playlist?list=playlist-1" {
		t.Fatalf("playlist = %+v", items[2])
	}
	if items[2].ResourceItemCount != 42 {
		t.Fatalf("quantidade da playlist = %d", items[2].ResourceItemCount)
	}

	searchParams := <-queries
	if got := searchParams.Get("q"); got != `linux "desktop leve" -shorts -propaganda` {
		t.Errorf("q = %q", got)
	}
	for key, want := range map[string]string{
		"maxResults": "24", "pageToken": "PAGE", "order": "date", "publishedAfter": "2025-01-01T03:00:00Z",
		"relevanceLanguage": "pt-BR", "regionCode": "BR", "safeSearch": "strict",
	} {
		if got := searchParams.Get(key); got != want {
			t.Errorf("%s = %q, esperado %q", key, got, want)
		}
	}
	if got := strings.Join(searchParams["type"], ","); got != "video,channel,playlist" {
		t.Errorf("type = %q", got)
	}
	videoParams := <-queries
	if got := videoParams.Get("id"); got != "video-1" {
		t.Errorf("videos.id = %q", got)
	}
	playlistParams := <-queries
	if got := playlistParams.Get("id"); got != "playlist-1" {
		t.Errorf("playlists.id = %q", got)
	}
}

func TestAccountCatalogOperationsMapAndPage(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/subscriptions"):
			if r.URL.Query().Get("mine") != "true" || r.URL.Query().Get("pageToken") != "S" {
				t.Errorf("subscriptions query = %v", r.URL.Query())
			}
			_, _ = fmt.Fprint(w, `{"nextPageToken":"SN","items":[{"snippet":{"title":"Canal A","resourceId":{"channelId":"UC-A"}}}]}`)
		case strings.HasSuffix(r.URL.Path, "/channels"):
			_, _ = fmt.Fprint(w, `{"items":[{"id":"UC-A","snippet":{"title":"Canal A oficial","thumbnails":{"high":{"url":"https://img/channel-a.jpg"}}},"contentDetails":{"relatedPlaylists":{"uploads":"UU-A"}}}]}`)
		case strings.HasSuffix(r.URL.Path, "/playlistItems"):
			if r.URL.Query().Get("playlistId") != "UU-A" {
				t.Errorf("playlistId = %q", r.URL.Query().Get("playlistId"))
			}
			_, _ = fmt.Fprint(w, `{"nextPageToken":"UN","items":[{"snippet":{"title":"Upload","description":"Desc","publishedAt":"2025-02-01T00:00:00Z","videoOwnerChannelId":"UC-A","videoOwnerChannelTitle":"Canal A","resourceId":{"videoId":"v-a"},"thumbnails":{"high":{"url":"https://img/a.jpg"}}}}]}`)
		case strings.HasSuffix(r.URL.Path, "/videos"):
			_, _ = fmt.Fprint(w, `{"items":[{"id":"v-a","snippet":{"title":"Upload","channelId":"UC-A","channelTitle":"Canal A","publishedAt":"2025-02-01T00:00:00Z"},"contentDetails":{"duration":"PT45S"}}]}`)
		default:
			http.Error(w, "rota inesperada", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	provider := testProvider(t, server)

	subscriptions, next, err := provider.Subscriptions(context.Background(), domain.PageOptions{MaxResults: 12, PageToken: "S"})
	if err != nil || next != "SN" || len(subscriptions) != 1 || !subscriptions[0].Subscribed {
		t.Fatalf("Subscriptions = %+v next=%q err=%v", subscriptions, next, err)
	}
	channels, err := provider.Channels(context.Background(), []string{"UC-A", "UC-A", ""})
	if err != nil || len(channels) != 1 || channels[0].UploadsPlaylistID != "UU-A" || channels[0].ThumbnailURL != "https://img/channel-a.jpg" {
		t.Fatalf("Channels = %+v err=%v", channels, err)
	}
	uploads, next, err := provider.Uploads(context.Background(), "UU-A", domain.PageOptions{MaxResults: 10})
	if err != nil || next != "UN" || len(uploads) != 1 || uploads[0].Duration != 45*time.Second {
		t.Fatalf("Uploads = %+v next=%q err=%v", uploads, next, err)
	}
}

func TestProviderRejectsMissingClientAndEmptyUploadsID(t *testing.T) {
	t.Parallel()
	provider := Provider{}
	if _, _, err := provider.Search(context.Background(), domain.SearchOptions{}); err == nil {
		t.Fatal("Search deveria rejeitar factory ausente")
	}
	if _, _, err := provider.Uploads(context.Background(), " ", domain.PageOptions{}); err == nil {
		t.Fatal("Uploads deveria rejeitar ID vazio")
	}
}

func testProvider(t *testing.T, server *httptest.Server) Provider {
	t.Helper()
	return Provider{NewService: func(ctx context.Context) (*youtube.Service, error) {
		return youtube.NewService(ctx,
			option.WithHTTPClient(server.Client()),
			option.WithEndpoint(server.URL+"/youtube/v3/"),
		)
	}}
}
