// Package playback implements the Cascading multi-provider playback resolver
// (docs/03-implementation/PLAYBACK_BACKENDS.md).
package playback

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/telemetry"
	"golang.org/x/sync/singleflight"
)

// NamedResolver associates a human-readable identifier with a PlaybackResolver.
type NamedResolver struct {
	Name     string
	Resolver domain.PlaybackResolver
}

// MultiProviderError aggregates errors when all configured resolvers fail.
type MultiProviderError struct {
	Request domain.PlaybackRequest
	Errors  map[string]error
}

func (e *MultiProviderError) Error() string {
	names := make([]string, 0, len(e.Errors))
	attempts := make([]error, 0, len(e.Errors))
	for name := range e.Errors {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		attempts = append(attempts, e.Errors[name])
	}
	if selected := selectFinalFailure(attempts); selected != nil {
		var failure Failure
		if errors.As(selected, &failure) && failure.Kind != FailureUnknown {
			return fmt.Sprintf("%s Os outros fallbacks também não conseguiram abrir este vídeo.", selected)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("todos os %d provedores de reprodução falharam:", len(e.Errors)))
	for _, name := range names {
		err := e.Errors[name]
		sb.WriteString(fmt.Sprintf("\n  - [%s]: %v", name, err))
	}
	return sb.String()
}

// CascadingResolver attempts resolution across a prioritized chain of resolvers.
type CascadingResolver struct {
	resolvers             []NamedResolver
	authenticatedFallback *NamedResolver
	mu                    sync.Mutex
	lastUsed              string
	cache                 map[string]playbackCacheEntry
	resolutions           singleflight.Group
	now                   func() time.Time
	rateLimitedUntil      time.Time
}

// NewCascadingResolver creates a cascading resolver from an ordered list of providers.
func NewCascadingResolver(resolvers ...NamedResolver) *CascadingResolver {
	return &CascadingResolver{
		resolvers: resolvers,
		cache:     make(map[string]playbackCacheEntry),
		now:       time.Now,
	}
}

// Resolve tries each resolver in order until one produces a valid PlaybackPlan.
func (c *CascadingResolver) Resolve(ctx context.Context, req domain.PlaybackRequest) (plan domain.PlaybackPlan, err error) {
	started := time.Now()
	cacheHit := false
	defer func() {
		telemetry.Process().ObservePlayback(time.Since(started), cacheHit, err)
	}()
	if err = ctx.Err(); err != nil {
		return domain.PlaybackPlan{}, err
	}
	key := playbackRequestKey(req)
	if cached, ok := c.cachedPlan(key); ok {
		cacheHit = true
		return cached, nil
	}

	result := c.resolutions.DoChan(key, func() (any, error) {
		if cached, ok := c.cachedPlan(key); ok {
			return cached, nil
		}
		plan, err := c.resolveUncached(ctx, req)
		if err == nil {
			c.storePlan(key, plan)
		}
		return plan, err
	})
	select {
	case <-ctx.Done():
		err = ctx.Err()
		return domain.PlaybackPlan{}, err
	case resolved := <-result:
		if resolved.Err != nil {
			err = resolved.Err
			return domain.PlaybackPlan{}, err
		}
		return clonePlaybackPlan(resolved.Val.(domain.PlaybackPlan)), nil
	}
}

func (c *CascadingResolver) resolveUncached(ctx context.Context, req domain.PlaybackRequest) (domain.PlaybackPlan, error) {
	if len(c.resolvers) == 0 {
		return domain.PlaybackPlan{}, errors.New("nenhum provedor de reprodução configurado")
	}
	if c.rateLimitActive() {
		return domain.PlaybackPlan{}, describe(FailureRateLimited, YtdlConfig{})
	}

	errs := make(map[string]error, len(c.resolvers)+1)
	for _, nr := range c.resolvers {
		if err := ctx.Err(); err != nil {
			return domain.PlaybackPlan{}, err
		}
		plan, err := nr.Resolver.Resolve(ctx, req)
		if err == nil {
			c.recordSuccess(nr.Name)
			return plan, nil
		}
		errs[nr.Name] = err
		if canTryAuthenticatedFallback(err) && c.authenticatedFallback != nil {
			return c.resolveAuthenticated(ctx, req, errs)
		}
		if stopsProviderFallback(err) {
			c.recordRateLimit(err)
			return domain.PlaybackPlan{}, err
		}
	}
	if c.authenticatedFallback != nil && failuresMayBenefitFromAuthentication(errs) {
		return c.resolveAuthenticated(ctx, req, errs)
	}

	return domain.PlaybackPlan{}, &MultiProviderError{
		Request: req,
		Errors:  errs,
	}
}

func (c *CascadingResolver) resolveAuthenticated(
	ctx context.Context,
	req domain.PlaybackRequest,
	errs map[string]error,
) (domain.PlaybackPlan, error) {
	fallback := *c.authenticatedFallback
	plan, err := fallback.Resolver.Resolve(ctx, req)
	if err == nil {
		c.recordSuccess(fallback.Name)
		return plan, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return domain.PlaybackPlan{}, err
	}
	errs[fallback.Name] = err
	c.recordRateLimit(err)
	return domain.PlaybackPlan{}, &MultiProviderError{Request: req, Errors: errs}
}

func (c *CascadingResolver) recordSuccess(name string) {
	c.mu.Lock()
	c.lastUsed = name
	c.mu.Unlock()
}

func (c *CascadingResolver) recordRateLimit(err error) {
	var failure Failure
	if !errors.As(err, &failure) || failure.Kind != FailureRateLimited {
		return
	}
	c.mu.Lock()
	c.rateLimitedUntil = c.clockNow().Add(time.Minute)
	c.mu.Unlock()
}

func (c *CascadingResolver) rateLimitActive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clockNow().Before(c.rateLimitedUntil)
}

func (c *CascadingResolver) clockNow() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// LastUsed returns the name of the provider that successfully resolved the last request.
func (c *CascadingResolver) LastUsed() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastUsed
}

// NewDefaultCascadingResolver builds the default multi-provider chain for NanoTube:
// 1. mpv built-in ytdl_hook (fastest path)
// 2. Explicit yt-dlp with android client
// 3. Explicit yt-dlp with ios client
// 4. Invidious fallback API
func NewDefaultCascadingResolver(cfg YtdlConfig, invidiousInstance string) *CascadingResolver {
	hookResolver := NewMpvYtdlHookResolver()
	hookResolver.Ytdl = cfg

	androidCfg := cfg
	androidCfg.PlayerClient = "android"
	androidResolver := NewExplicitYtDlpResolver(androidCfg)

	iosCfg := cfg
	iosCfg.PlayerClient = "ios"
	iosResolver := NewExplicitYtDlpResolver(iosCfg)

	invidiousResolver := NewInvidiousResolver(invidiousInstance, cfg.MaxHeight)

	return NewCascadingResolver(
		NamedResolver{Name: "mpv_ytdl_hook", Resolver: hookResolver},
		NamedResolver{Name: "ytdlp_android", Resolver: androidResolver},
		NamedResolver{Name: "ytdlp_ios", Resolver: iosResolver},
		NamedResolver{Name: "invidious", Resolver: invidiousResolver},
	)
}

// NewWebCascadingResolver builds plans consumable by an HTML5/HLS player.
// The mpv ytdl_hook resolver is intentionally excluded because it returns a
// LoadTarget for libmpv instead of a resolved Primary stream URL.
func NewWebCascadingResolver(cfg YtdlConfig, invidiousInstance string) *CascadingResolver {
	publicCfg := cfg.withoutCookies()
	androidCfg := publicCfg
	androidCfg.PlayerClient = "android"

	iosCfg := publicCfg
	iosCfg.PlayerClient = "ios"

	androidResolver := NewExplicitYtDlpResolver(androidCfg)
	iosResolver := NewExplicitYtDlpResolver(iosCfg)
	invidiousResolver := NewInvidiousResolver(invidiousInstance, publicCfg.MaxHeight)

	resolvers := make([]NamedResolver, 0, 4)
	if potProviderCandidateAvailable(publicCfg) {
		mwebCfg := publicCfg
		mwebCfg.PlayerClient = "mweb"
		resolvers = append(resolvers, NamedResolver{Name: "ytdlp_mweb", Resolver: NewExplicitYtDlpResolver(mwebCfg)})
	}
	resolvers = append(resolvers,
		NamedResolver{Name: "ytdlp_android", Resolver: androidResolver},
		NamedResolver{Name: "ytdlp_ios", Resolver: iosResolver},
		NamedResolver{Name: "invidious", Resolver: invidiousResolver},
	)
	cascade := NewCascadingResolver(resolvers...)
	if cfg.hasCookies() {
		authenticatedCfg := cfg
		if authenticatedCfg.PlayerClient == "" || authenticatedCfg.PlayerClient == AutoPlayerClient {
			// O cliente android é deliberadamente ignorado pelo yt-dlp quando
			// cookies são fornecidos. O cliente web aceita a sessão autenticada e
			// permite abrir vídeos que dispararam verificação anti-bot, de idade ou
			// de membros sem desperdiçar o único fallback com cookies.
			authenticatedCfg.PlayerClient = "web"
		}
		cascade.authenticatedFallback = &NamedResolver{
			Name:     "ytdlp_authenticated",
			Resolver: NewExplicitYtDlpResolver(authenticatedCfg),
		}
	}
	return cascade
}
