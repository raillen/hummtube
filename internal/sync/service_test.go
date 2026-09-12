package sync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/storage"
	"go.uber.org/goleak"
)

type fakeProvider struct {
	mu          sync.Mutex
	subs        []domain.Channel
	detail      []domain.Channel
	uploads     map[string][]domain.Video
	uploadsErr  map[string]error
	uploadsCall map[string]int
}

func (f *fakeProvider) Subscriptions(ctx context.Context, opts domain.PageOptions) ([]domain.Channel, string, error) {
	return f.subs, "", nil
}

func (f *fakeProvider) Channels(ctx context.Context, ids []string) ([]domain.Channel, error) {
	return f.detail, nil
}

func (f *fakeProvider) Uploads(ctx context.Context, playlistID string, opts domain.PageOptions) ([]domain.Video, string, error) {
	f.mu.Lock()
	f.uploadsCall[playlistID]++
	err := f.uploadsErr[playlistID]
	f.mu.Unlock()
	if err != nil {
		return nil, "", err
	}
	if opts.PageToken != "" {
		return nil, "", nil
	}
	return f.uploads[playlistID], "", nil
}

// Search não participa da sincronização: o refresh usa inscrições e uploads.
func (f *fakeProvider) Search(ctx context.Context, opts domain.SearchOptions) ([]domain.Video, string, error) {
	return nil, "", nil
}

func newDB(t *testing.T) *storage.Repository {
	t.Helper()
	db, err := storage.OpenMemory(context.Background())
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return storage.NewRepository(db)
}

func videos(channelID string, ids ...string) []domain.Video {
	out := make([]domain.Video, 0, len(ids))
	for i, id := range ids {
		out = append(out, domain.Video{
			ID:          id,
			ChannelID:   channelID,
			Title:       "Video " + id,
			PublishedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}
	return out
}

type failingVideoRepo struct {
	*storage.Repository
	fail bool
}

func (r *failingVideoRepo) UpsertVideos(ctx context.Context, videos []domain.Video) error {
	if r.fail {
		return fmt.Errorf("injected write failure")
	}
	return r.Repository.UpsertVideos(ctx, videos)
}

func TestRefreshRetriesVideosAfterWriteFailure(t *testing.T) {
	ctx := context.Background()
	repo := &failingVideoRepo{Repository: newDB(t), fail: true}
	provider := &fakeProvider{
		subs:        []domain.Channel{{ID: "UC1", Subscribed: true}},
		detail:      []domain.Channel{{ID: "UC1", UploadsPlaylistID: "UU1"}},
		uploads:     map[string][]domain.Video{"UU1": videos("UC1", "v1")},
		uploadsCall: map[string]int{},
	}
	service, err := NewService(provider, repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(ctx, nil); err == nil {
		t.Fatal("expected write failure")
	}
	channels, err := repo.ChannelsByIDs(ctx, []string{"UC1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].LastKnownVideoID != "" {
		t.Fatalf("marker advanced: %+v", channels)
	}
	repo.fail = false
	stats, err := service.Refresh(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.VideosFetched != 1 {
		t.Fatalf("retry skipped video: %+v", stats)
	}
	stored, err := repo.Recent(ctx, domain.VideoFilter{Limit: 10})
	if err != nil || len(stored) != 1 {
		t.Fatalf("videos=%+v err=%v", stored, err)
	}
}

func TestRefreshFull(t *testing.T) {
	repo := newDB(t)
	fp := &fakeProvider{
		subs: []domain.Channel{{ID: "UC1", Subscribed: true}, {ID: "UC2", Subscribed: true}},
		detail: []domain.Channel{
			{ID: "UC1", Title: "Canal Um", UploadsPlaylistID: "UUP1"},
			{ID: "UC2", Title: "Canal Dois", UploadsPlaylistID: "UUP2"},
		},
		uploads: map[string][]domain.Video{
			"UUP1": videos("UC1", "v1", "v2"),
			"UUP2": videos("UC2", "v3"),
		},
		uploadsCall: map[string]int{},
	}

	svc, err := NewService(fp, repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.Refresh(context.Background(), nil)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if st.Subscriptions != 2 || st.ChannelsSynced != 2 || st.VideosFetched != 3 {
		t.Fatalf("stats = %+v", st)
	}
	if len(st.ChannelErrors) != 0 {
		t.Fatalf("errors = %v", st.ChannelErrors)
	}

	chs, err := repo.SubscribedChannels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 2 {
		t.Fatalf("inscrições = %d", len(chs))
	}
	if chs[0].UploadsPlaylistID != "UUP1" && chs[0].UploadsPlaylistID != "UUP2" {
		t.Fatalf("uploads playlist não resolvido: %+v", chs[0])
	}

	rec, err := repo.Recent(context.Background(), domain.VideoFilter{OnlySubscribed: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rec) != 3 {
		t.Fatalf("recent = %d", len(rec))
	}
}

func TestRefreshIncrementalStopsAtKnown(t *testing.T) {
	repo := newDB(t)
	fp := &fakeProvider{
		subs: []domain.Channel{{ID: "UC1", Subscribed: true}},
		detail: []domain.Channel{
			{ID: "UC1", Title: "Canal Um", UploadsPlaylistID: "UUP1"},
		},
		uploads: map[string][]domain.Video{
			"UUP1": videos("UC1", "v3", "v2", "v1"),
		},
		uploadsCall: map[string]int{},
	}

	svc, err := NewService(fp, repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(context.Background(), nil); err != nil {
		t.Fatalf("refresh 1: %v", err)
	}

	// Novo upload chega; a página agora tem v4 antes de v3.
	fp.mu.Lock()
	fp.uploads["UUP1"] = videos("UC1", "v4", "v3", "v2", "v1")
	fp.mu.Unlock()

	if _, err := svc.Refresh(context.Background(), nil); err != nil {
		t.Fatalf("refresh 2: %v", err)
	}

	fp.mu.Lock()
	calls := fp.uploadsCall["UUP1"]
	fp.mu.Unlock()
	if calls != 2 {
		t.Errorf("chamadas de uploads = %d, esperado 2 (parou no vídeo conhecido)", calls)
	}

	chs, err := repo.ChannelsByIDs(context.Background(), []string{"UC1"})
	if err != nil || len(chs) != 1 {
		t.Fatalf("ChannelsByIDs: %v %d", err, len(chs))
	}
	if chs[0].LastKnownVideoID != "v4" {
		t.Errorf("last_known_video_id = %q, esperado v4", chs[0].LastKnownVideoID)
	}
	if chs[0].LastError != "" {
		t.Errorf("last_error = %q", chs[0].LastError)
	}
}

func TestRefreshChannelErrorRecorded(t *testing.T) {
	repo := newDB(t)
	fp := &fakeProvider{
		subs: []domain.Channel{{ID: "UC1", Subscribed: true}},
		detail: []domain.Channel{
			{ID: "UC1", Title: "Canal Um", UploadsPlaylistID: "UUP1"},
		},
		uploads:     map[string][]domain.Video{},
		uploadsErr:  map[string]error{"UUP1": errFakeQuota},
		uploadsCall: map[string]int{},
	}

	svc, err := NewService(fp, repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.Refresh(context.Background(), nil)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(st.ChannelErrors) != 1 {
		t.Fatalf("esperado 1 erro de canal, got %v", st.ChannelErrors)
	}
	chs, _ := repo.ChannelsByIDs(context.Background(), []string{"UC1"})
	if chs[0].LastError == "" {
		t.Error("last_error deveria estar preenchido")
	}
}

var errFakeQuota = &quotaError{}

type quotaError struct{}

func (*quotaError) Error() string { return "quota excedida (fake)" }

func TestRefreshNoGoleak(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"))

	repo := newDB(t)
	fp := &fakeProvider{
		subs: []domain.Channel{{ID: "UC1", Subscribed: true}},
		detail: []domain.Channel{
			{ID: "UC1", Title: "Canal Um", UploadsPlaylistID: "UUP1"},
		},
		uploads: map[string][]domain.Video{
			"UUP1": videos("UC1", "v1"),
		},
		uploadsCall: map[string]int{},
	}

	svc, err := NewService(fp, repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = svc.Refresh(ctx, nil)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
}

func TestNewServiceValidatesDependenciesAndOptions(t *testing.T) {
	repo := newDB(t)
	fp := &fakeProvider{}
	if _, err := NewService(nil, repo, DefaultOptions()); err == nil {
		t.Fatal("provider nulo deveria ser rejeitado")
	}
	if _, err := NewService(fp, nil, DefaultOptions()); err == nil {
		t.Fatal("repository nulo deveria ser rejeitado")
	}
	if _, err := NewService(fp, repo, Options{Workers: 1}); err == nil {
		t.Fatal("opções incompletas deveriam ser rejeitadas")
	}
}

type markerErrorRepo struct {
	*storage.Repository
}

func (r markerErrorRepo) SetChannelSync(context.Context, string, string, string) error {
	return errMarkerWrite
}

var errMarkerWrite = &quotaError{}

func TestRefreshPropagatesSyncMarkerError(t *testing.T) {
	dbRepo := newDB(t)
	repo := markerErrorRepo{Repository: dbRepo}
	fp := &fakeProvider{
		subs:        []domain.Channel{{ID: "UC1", Subscribed: true}},
		detail:      []domain.Channel{{ID: "UC1", UploadsPlaylistID: "UUP1"}},
		uploads:     map[string][]domain.Video{"UUP1": videos("UC1", "v1")},
		uploadsCall: map[string]int{},
	}
	svc, err := NewService(fp, repo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "sync marker") {
		t.Fatalf("erro de marcador não propagado: %v", err)
	}
}

func TestRefreshLocalEmptyGraceful(t *testing.T) {
	dbRepo := newDB(t)
	svc, err := NewLocalService(dbRepo, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	stats, err := svc.RefreshLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("RefreshLocal com 0 canais não deveria retornar erro, retornou: %v", err)
	}
	if stats.Subscriptions != 0 || stats.VideosFetched != 0 {
		t.Fatalf("stats inesperado para base vazia: %+v", stats)
	}
}
