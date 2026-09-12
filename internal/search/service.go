package search

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/telemetry"
)

// Service selects the cheapest capable remote backend and then applies
// personal filters locally. Official may be nil when there is no account.
type Service struct {
	Public   YtDlp
	Official domain.YouTubeProvider
	Personal PersonalState
}

func (s Service) Search(ctx context.Context, opts domain.SearchOptions) (page domain.SearchPage, err error) {
	started := time.Now()
	defer func() {
		telemetry.Process().ObserveSearch(time.Since(started), err)
	}()

	opts = NormalizeOptions(opts)
	if validationErr := ValidateOptions(opts); validationErr != nil {
		err = fmt.Errorf("busca: %w", validationErr)
		return domain.SearchPage{}, err
	}

	remoteOpts := remoteSearchOptions(opts)
	if RequiresOfficialAPI(opts) {
		if s.Official == nil {
			err = fmt.Errorf("busca: %s", noAccountHint(opts))
			return page, err
		}
		items, next, providerErr := s.Official.Search(ctx, remoteOpts)
		if providerErr != nil {
			err = providerErr
			return page, err
		}
		page.Items, page.NextPageToken, page.Source = items, next, domain.SearchSourceOfficial
	} else {
		items, providerErr := s.Public.Search(ctx, remoteOpts)
		if providerErr != nil {
			err = providerErr
			return page, err
		}
		page.Items, page.Source = items, domain.SearchSourcePublic
		if len(items) == opts.MaxResults {
			offset, _ := strconv.Atoi(opts.PageToken)
			page.NextPageToken = strconv.Itoa(offset + len(items))
		}
	}
	page.Items = ApplyLocalFilters(page.Items, opts, s.Personal)
	RankWithNanoRank(page.Items, opts, s.Personal, time.Now())
	if opts.ShortsOnly || opts.RegularOnly {
		page.Notices = append(page.Notices, "Shorts são aproximados pela duração; o YouTube não expõe um marcador oficial estável.")
	}
	if opts.OnlySubscribed {
		page.Notices = append(page.Notices, "O filtro de inscrições usa os canais sincronizados neste dispositivo.")
	}
	return page, nil
}

// remoteSearchOptions enforces the privacy boundary in code: account-local
// state must never reach yt-dlp or the Google adapter, even as unused fields.
func remoteSearchOptions(opts domain.SearchOptions) domain.SearchOptions {
	opts.WatchState = domain.SearchWatchAny
	opts.SavedState = domain.SearchSavedAny
	opts.OnlySubscribed = false
	opts.HideRejected = false
	return opts
}

// noAccountHint explica por que um conjunto de opções exige conta. O yt-dlp
// estável só raspa a tab de vídeos da busca (mesmo com sp de canal/playlist),
// então tipos de recurso não-vídeo e filtros avançados dependem da Data API.
func noAccountHint(opts domain.SearchOptions) string {
	if len(opts.ResourceTypes) != 1 || opts.ResourceTypes[0] != domain.SearchResourceVideo {
		return "buscar canais e playlists exige uma conta conectada (YouTube Data API)"
	}
	return "estes filtros específicos exigem uma conta conectada"
}
