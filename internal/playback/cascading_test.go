package playback

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

type mockResolver struct {
	plan  domain.PlaybackPlan
	err   error
	calls atomic.Int32
}

func (m *mockResolver) Resolve(_ context.Context, _ domain.PlaybackRequest) (domain.PlaybackPlan, error) {
	m.calls.Add(1)
	return m.plan, m.err
}

func TestCascadingResolverFallsBackOnFailure(t *testing.T) {
	primary := &mockResolver{err: errors.New("primary failed: bot detection")}
	secondary := &mockResolver{plan: domain.PlaybackPlan{
		Mode:       domain.PlaybackModeDirect,
		LoadTarget: "https://fallback.example.com/video.mp4",
	}}

	cascading := NewCascadingResolver(
		NamedResolver{Name: "primary", Resolver: primary},
		NamedResolver{Name: "secondary", Resolver: secondary},
	)

	plan, err := cascading.Resolve(context.Background(), domain.PlaybackRequest{SourceURL: "https://youtube.com/watch?v=xyz"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if plan.LoadTarget != "https://fallback.example.com/video.mp4" {
		t.Errorf("LoadTarget = %q, want secondary target", plan.LoadTarget)
	}
	if cascading.LastUsed() != "secondary" {
		t.Errorf("LastUsed = %q, want secondary", cascading.LastUsed())
	}
}

func TestCascadingResolverAllFail(t *testing.T) {
	r1 := &mockResolver{err: errors.New("err 1")}
	r2 := &mockResolver{err: errors.New("err 2")}

	cascading := NewCascadingResolver(
		NamedResolver{Name: "r1", Resolver: r1},
		NamedResolver{Name: "r2", Resolver: r2},
	)

	_, err := cascading.Resolve(context.Background(), domain.PlaybackRequest{SourceURL: "https://youtube.com/watch?v=xyz"})
	if err == nil {
		t.Fatal("esperava erro quando todos falham")
	}

	var multiErr *MultiProviderError
	if !errors.As(err, &multiErr) {
		t.Fatalf("tipo de erro = %T, want *MultiProviderError", err)
	}
	if len(multiErr.Errors) != 2 {
		t.Errorf("erros coletados = %d, want 2", len(multiErr.Errors))
	}
}

func TestCascadingResolverPrefersActionableRestrictedFailure(t *testing.T) {
	restricted := describe(FailureRestricted, YtdlConfig{})
	invidious := &mockResolver{err: errors.New("invidious http status 403")}
	resolver := NewCascadingResolver(
		NamedResolver{Name: "ytdlp", Resolver: &mockResolver{err: restricted}},
		NamedResolver{Name: "invidious", Resolver: invidious},
	)

	_, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{SourceURL: "https://youtube.com/watch?v=restricted"})
	if err == nil || !strings.Contains(err.Error(), "Entrar → Cookies") {
		t.Fatalf("erro agregado não preservou a orientação acionável: %v", err)
	}
	if strings.Contains(err.Error(), "invidious http status") || strings.Contains(err.Error(), "watch?v=") {
		t.Fatalf("erro agregado expôs detalhe técnico desnecessário: %v", err)
	}
	if invidious.calls.Load() != 0 {
		t.Fatalf("Invidious foi chamado após falha terminal %d vez(es)", invidious.calls.Load())
	}
}

func TestCascadingResolverStopsAfterDRMAndUnavailable(t *testing.T) {
	for _, kind := range []FailureKind{FailureDRM, FailureUnavailable} {
		t.Run(describe(kind, YtdlConfig{}).Summary, func(t *testing.T) {
			fallback := &mockResolver{plan: resolvedTestPlan("https://fallback.invalid/video")}
			resolver := NewCascadingResolver(
				NamedResolver{Name: "primary", Resolver: &mockResolver{err: describe(kind, YtdlConfig{})}},
				NamedResolver{Name: "fallback", Resolver: fallback},
			)
			_, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{VideoID: "terminal"})
			if err == nil || fallback.calls.Load() != 0 {
				t.Fatalf("falha terminal %v permitiu fallback: err=%v calls=%d", kind, err, fallback.calls.Load())
			}
		})
	}
}

func TestCascadingResolverUsesAtMostOneAuthenticatedFallback(t *testing.T) {
	publicFallback := &mockResolver{plan: resolvedTestPlan("https://public-fallback.invalid/video")}
	authenticated := &mockResolver{err: describe(FailureRestricted, YtdlConfig{})}
	resolver := NewCascadingResolver(
		NamedResolver{Name: "public", Resolver: &mockResolver{err: describe(FailureRestricted, YtdlConfig{})}},
		NamedResolver{Name: "must-not-run", Resolver: publicFallback},
	)
	resolver.authenticatedFallback = &NamedResolver{Name: "authenticated", Resolver: authenticated}

	_, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{VideoID: "members"})
	if err == nil {
		t.Fatal("esperava falha autenticada")
	}
	if authenticated.calls.Load() != 1 || publicFallback.calls.Load() != 0 {
		t.Fatalf("tentativas auth=%d fallback público=%d", authenticated.calls.Load(), publicFallback.calls.Load())
	}
}

func TestCascadingResolverTriesPublicFallbackBeforeCookiesForBotCheck(t *testing.T) {
	publicFallback := &mockResolver{plan: resolvedTestPlan("https://public.invalid/video")}
	authenticated := &mockResolver{plan: resolvedTestPlan("https://authenticated.invalid/video")}
	resolver := NewCascadingResolver(
		NamedResolver{Name: "bot-check", Resolver: &mockResolver{err: describe(FailureBotCheck, YtdlConfig{})}},
		NamedResolver{Name: "public-fallback", Resolver: publicFallback},
	)
	resolver.authenticatedFallback = &NamedResolver{Name: "authenticated", Resolver: authenticated}

	plan, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{VideoID: "public-after-bot-check"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Primary.URL != "https://public.invalid/video" || authenticated.calls.Load() != 0 {
		t.Fatalf("cookies foram usados antes de esgotar fallback público: plan=%+v auth=%d", plan, authenticated.calls.Load())
	}
}

func TestCascadingResolverUsesCookiesOnceAfterPublicBotCheckFallbacksFail(t *testing.T) {
	authenticated := &mockResolver{plan: resolvedTestPlan("https://authenticated.invalid/video")}
	resolver := NewCascadingResolver(
		NamedResolver{Name: "bot-check", Resolver: &mockResolver{err: describe(FailureBotCheck, YtdlConfig{})}},
		NamedResolver{Name: "public-fallback", Resolver: &mockResolver{err: errors.New("public fallback indisponível")}},
	)
	resolver.authenticatedFallback = &NamedResolver{Name: "authenticated", Resolver: authenticated}

	plan, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{VideoID: "auth-after-public"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Primary.URL != "https://authenticated.invalid/video" || authenticated.calls.Load() != 1 {
		t.Fatalf("fallback autenticado = %+v, calls=%d", plan, authenticated.calls.Load())
	}
}

func TestWebResolverStartsWithoutCookies(t *testing.T) {
	resolver := NewWebCascadingResolver(YtdlConfig{
		CookiesFromBrowser: "firefox",
		CookiesFile:        "/tmp/cookies.txt",
	}, "")
	if resolver.authenticatedFallback == nil {
		t.Fatal("fallback autenticado ausente")
	}
	for _, candidate := range resolver.resolvers {
		explicit, ok := candidate.Resolver.(*ExplicitYtDlpResolver)
		if !ok {
			continue
		}
		if explicit.Config.hasCookies() {
			t.Fatalf("resolvedor público %q herdou cookies", candidate.Name)
		}
	}
	authenticated := resolver.authenticatedFallback.Resolver.(*ExplicitYtDlpResolver)
	if !authenticated.Config.hasCookies() {
		t.Fatal("fallback autenticado perdeu os cookies configurados")
	}
	if authenticated.Config.PlayerClient != "web" {
		t.Fatalf("cliente autenticado = %q, esperado web", authenticated.Config.PlayerClient)
	}
}

func TestCascadingResolverCoalescesAndCachesResolution(t *testing.T) {
	upstream := &blockingCountingResolver{
		started: make(chan struct{}),
		release: make(chan struct{}),
		plan:    resolvedTestPlan("https://media.invalid/video?expire=4102444800"),
	}
	resolver := NewCascadingResolver(NamedResolver{Name: "upstream", Resolver: upstream})
	req := domain.PlaybackRequest{VideoID: "same-video", SourceURL: "https://youtube.invalid/watch?v=same-video"}

	var wait sync.WaitGroup
	wait.Add(2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			defer wait.Done()
			_, err := resolver.Resolve(context.Background(), req)
			errs <- err
		}()
	}
	<-upstream.started
	close(upstream.release)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
	}
	if _, err := resolver.Resolve(context.Background(), req); err != nil {
		t.Fatalf("cache Resolve: %v", err)
	}
	if upstream.calls.Load() != 1 {
		t.Fatalf("extractor executou %d vezes, esperado 1", upstream.calls.Load())
	}
}

func TestCascadingResolverRenewsExpiredCacheEntry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	upstream := &mockResolver{plan: resolvedTestPlan("https://media.invalid/video")}
	resolver := NewCascadingResolver(NamedResolver{Name: "upstream", Resolver: upstream})
	resolver.now = func() time.Time { return now }
	req := domain.PlaybackRequest{VideoID: "expiring"}

	if _, err := resolver.Resolve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	now = now.Add(defaultPlaybackCacheTTL)
	if _, err := resolver.Resolve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if upstream.calls.Load() != 2 {
		t.Fatalf("plano expirado não foi renovado: calls=%d", upstream.calls.Load())
	}
}

func TestCascadingResolverBoundsCacheMemory(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	resolver := NewCascadingResolver()
	resolver.now = func() time.Time { return now }
	for index := range maximumPlaybackCacheEntries + 10 {
		key := fmt.Sprintf("video-%d", index)
		resolver.storePlan(key, resolvedTestPlan("https://media.invalid/"+key+"?expire=4102444800"))
	}
	if len(resolver.cache) > maximumPlaybackCacheEntries {
		t.Fatalf("cache cresceu para %d entradas", len(resolver.cache))
	}
}

func TestCascadingResolverRateLimitCircuitBreaker(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	limited := &mockResolver{err: describe(FailureRateLimited, YtdlConfig{})}
	fallback := &mockResolver{plan: resolvedTestPlan("https://fallback.invalid/video")}
	resolver := NewCascadingResolver(
		NamedResolver{Name: "youtube", Resolver: limited},
		NamedResolver{Name: "fallback", Resolver: fallback},
	)
	resolver.now = func() time.Time { return now }

	for range 2 {
		if _, err := resolver.Resolve(context.Background(), domain.PlaybackRequest{VideoID: "limited"}); err == nil {
			t.Fatal("esperava rate limit")
		}
	}
	if limited.calls.Load() != 1 || fallback.calls.Load() != 0 {
		t.Fatalf("circuit breaker falhou: limited=%d fallback=%d", limited.calls.Load(), fallback.calls.Load())
	}
}

type blockingCountingResolver struct {
	started chan struct{}
	release chan struct{}
	plan    domain.PlaybackPlan
	calls   atomic.Int32
	once    sync.Once
}

func (r *blockingCountingResolver) Resolve(ctx context.Context, _ domain.PlaybackRequest) (domain.PlaybackPlan, error) {
	r.calls.Add(1)
	r.once.Do(func() { close(r.started) })
	select {
	case <-ctx.Done():
		return domain.PlaybackPlan{}, ctx.Err()
	case <-r.release:
		return r.plan, nil
	}
}

func resolvedTestPlan(rawURL string) domain.PlaybackPlan {
	return domain.PlaybackPlan{
		Mode:    domain.PlaybackModeResolvedMedia,
		Primary: domain.ResolvedStream{URL: rawURL},
	}
}

func TestWebCascadingResolverDoesNotReturnMpvHookPlans(t *testing.T) {
	resolver := NewWebCascadingResolver(YtdlConfig{}, "")
	if len(resolver.resolvers) == 0 {
		t.Fatal("cadeia web vazia")
	}
	for _, candidate := range resolver.resolvers {
		if candidate.Name == "mpv_ytdl_hook" {
			t.Fatal("player HTML5 não pode receber plano exclusivo do mpv")
		}
	}
}

func TestWebCascadingResolverPrioritizesMWebWhenPOTProviderExists(t *testing.T) {
	pluginRoot := t.TempDir()
	pluginFile := filepath.Join(pluginRoot, "yt-dlp-plugins", "extractor", "pot.py")
	if err := os.MkdirAll(filepath.Dir(pluginFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginFile, []byte("# fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	previousPluginDirs := potPluginDirs
	potPluginDirs = func() []string { return []string{pluginRoot} }
	t.Cleanup(func() { potPluginDirs = previousPluginDirs })

	resolver := NewWebCascadingResolver(YtdlConfig{POTProvider: "provider-fixture"}, "")
	if len(resolver.resolvers) != 4 || resolver.resolvers[0].Name != "ytdlp_mweb" {
		t.Fatalf("cadeia web com PO Token = %+v", resolver.resolvers)
	}
}
