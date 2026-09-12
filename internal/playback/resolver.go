// Package playback implements PlaybackResolver strategies
// (docs/03-implementation/PLAYBACK_BACKENDS.md).
package playback

import (
	"context"
	"net/url"
	"strings"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// MpvYtdlHookResolver is the MVP resolver: it delegates remote URLs to
// mpv's built-in ytdl_hook (which runs yt-dlp) and local files to direct load.
//
// O Ytdl carrega a política de extração (runtime JS, cliente, cookies, teto de
// resolução). Ela vira `Options` no plano, e é o adapter do mpv que as aplica —
// o domínio continua sem conhecer flags de yt-dlp.
type MpvYtdlHookResolver struct {
	Ytdl YtdlConfig
}

// NewMpvYtdlHookResolver builds the default resolver from the environment.
func NewMpvYtdlHookResolver() MpvYtdlHookResolver {
	return MpvYtdlHookResolver{Ytdl: LoadYtdlConfig()}
}

// Resolve classifies the source URL into a PlaybackPlan.
func (r MpvYtdlHookResolver) Resolve(ctx context.Context, req domain.PlaybackRequest) (domain.PlaybackPlan, error) {
	if err := ctx.Err(); err != nil {
		return domain.PlaybackPlan{}, err
	}
	if !isRemote(req.SourceURL) {
		// Arquivo local não passa pelo yt-dlp, mas ainda precisa limpar as
		// opções deixadas pelo vídeo remoto anterior.
		return domain.PlaybackPlan{
			Mode:       domain.PlaybackModeDirect,
			LoadTarget: req.SourceURL,
			Options:    map[string]string{"ytdl-raw-options": ""},
		}, nil
	}
	options, err := r.Ytdl.MpvOptions()
	if err != nil {
		return domain.PlaybackPlan{}, err
	}
	return domain.PlaybackPlan{
		Mode:       domain.PlaybackModeMpvYtdlHook,
		LoadTarget: req.SourceURL,
		Options:    options,
	}, nil
}

func isRemote(source string) bool {
	u, err := url.Parse(strings.TrimSpace(source))
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
