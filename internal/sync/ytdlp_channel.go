// Resolve channel via yt-dlp (handle/URL -> UC ID + title).
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/playback"
)

var ucPattern = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)

// ResolveChannel aceita UC ID, @handle, ou URL do YouTube e devolve o canal canônico.
func ResolveChannel(ctx context.Context, input string, cfg playback.YtdlConfig) (domain.Channel, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return domain.Channel{}, fmt.Errorf("informe a URL, @handle ou ID do canal")
	}
	// UC direto: sem yt-dlp.
	if ucPattern.MatchString(input) {
		vids, err := FetchChannelFeed(ctx, input)
		if err != nil {
			return domain.Channel{ID: input, Title: input, Subscribed: true}, nil
		}
		title := input
		if len(vids) > 0 && vids[0].ChannelTitle != "" {
			title = vids[0].ChannelTitle
		}
		return domain.Channel{ID: input, Title: title, Subscribed: true}, nil
	}
	// Normaliza para URL.
	url := input
	if strings.HasPrefix(input, "@") {
		url = "https://www.youtube.com/" + input + "/videos"
	} else if !strings.HasPrefix(input, "http") {
		url = "https://www.youtube.com/@" + input + "/videos"
	}
	return resolveViaYtDlp(ctx, url, cfg)
}

func resolveViaYtDlp(ctx context.Context, url string, cfg playback.YtdlConfig) (domain.Channel, error) {
	// Usa publicSearchConfig para não vazar cookies/POT.
	publicCfg := publicSearchConfig(cfg)
	args := append(publicCfg.CommandArgs(),
		"--dump-single-json", "--flat-playlist", "--no-warnings", "--", url)

	// Respect context timeout.
	output, _, err := playback.RunBoundedExtractor(ctx, resolveBinary(publicCfg), args...)
	if err != nil {
		return domain.Channel{}, fmt.Errorf("resolver canal: %w", err)
	}
	var payload struct {
		ChannelID  string `json:"channel_id"`
		Channel    string `json:"channel"`
		Uploader   string `json:"uploader"`
		UploaderID string `json:"uploader_id"`
		Title      string `json:"title"`
		Entries    []struct {
			ChannelID string `json:"channel_id"`
			Channel   string `json:"channel"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		// Try flat array
		return domain.Channel{}, fmt.Errorf("resposta yt-dlp ilegível: %w", err)
	}
	cid := payload.ChannelID
	if cid == "" {
		cid = payload.UploaderID
	}
	if cid == "" && len(payload.Entries) > 0 {
		cid = payload.Entries[0].ChannelID
	}
	if cid == "" {
		return domain.Channel{}, fmt.Errorf("não foi possível resolver o canal a partir de %q — verifique se é um @handle ou URL válida", url)
	}
	title := payload.Channel
	if title == "" {
		title = payload.Uploader
	}
	if title == "" {
		title = payload.Title
	}
	if title == "" {
		title = cid
	}
	return domain.Channel{ID: cid, Title: title, Subscribed: true}, nil
}

func resolveBinary(cfg playback.YtdlConfig) string {
	if cfg.YtDlpPath != "" {
		return cfg.YtDlpPath
	}
	return "yt-dlp"
}

func publicSearchConfig(cfg playback.YtdlConfig) playback.YtdlConfig {
	cfg.PlayerClient = playback.AutoPlayerClient
	cfg.POTProvider = ""
	cfg.POTMode = ""
	cfg.CookiesFromBrowser = ""
	cfg.CookiesFile = ""
	cfg.AllowRemoteComponents = false
	cfg.MaxHeight = 0
	return cfg
}
