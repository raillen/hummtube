package search

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const MaxResults = 50

// NormalizeOptions applies safe defaults shared by every search adapter.
func NormalizeOptions(opts domain.SearchOptions) domain.SearchOptions {
	opts.Query = strings.TrimSpace(opts.Query)
	opts.ExactPhrase = strings.TrimSpace(opts.ExactPhrase)
	opts.IncludeTerms = strings.TrimSpace(opts.IncludeTerms)
	opts.ExcludeTerms = strings.TrimSpace(opts.ExcludeTerms)
	opts.ChannelID = strings.TrimSpace(opts.ChannelID)
	opts.CategoryID = strings.TrimSpace(opts.CategoryID)
	opts.TopicID = strings.TrimSpace(opts.TopicID)
	opts.RelevanceLanguage = strings.TrimSpace(opts.RelevanceLanguage)
	opts.RegionCode = strings.ToUpper(strings.TrimSpace(opts.RegionCode))
	opts.Location = strings.TrimSpace(opts.Location)
	opts.LocationRadius = strings.TrimSpace(opts.LocationRadius)
	if opts.MaxResults <= 0 {
		opts.MaxResults = 20
	} else if opts.MaxResults > MaxResults {
		opts.MaxResults = MaxResults
	}
	if len(opts.ResourceTypes) == 0 {
		opts.ResourceTypes = []domain.SearchResourceType{domain.SearchResourceVideo}
	}
	if opts.Order == "" {
		opts.Order = domain.SearchOrderRelevance
	}
	if opts.Duration == "" {
		opts.Duration = domain.SearchDurationAny
	}
	if opts.Caption == "" {
		opts.Caption = domain.SearchCaptionAny
	}
	if opts.Definition == "" {
		opts.Definition = domain.SearchDefinitionAny
	}
	if opts.Dimension == "" {
		opts.Dimension = domain.SearchDimensionAny
	}
	if opts.License == "" {
		opts.License = domain.SearchLicenseAny
	}
	if opts.VideoType == "" {
		opts.VideoType = domain.SearchVideoAny
	}
	if opts.SafeSearch == "" {
		opts.SafeSearch = domain.SearchSafeAny
	}
	if opts.WatchState == "" {
		opts.WatchState = domain.SearchWatchAny
	}
	if opts.SavedState == "" {
		opts.SavedState = domain.SearchSavedAny
	}
	if opts.Embeddable == "" {
		opts.Embeddable = domain.SearchAny
	}
	if opts.Syndicated == "" {
		opts.Syndicated = domain.SearchAny
	}
	if opts.PaidPromotion == "" {
		opts.PaidPromotion = domain.SearchAny
	}
	return opts
}

// ValidateOptions rejects combinations that the YouTube API itself rejects.
func ValidateOptions(opts domain.SearchOptions) error {
	opts = NormalizeOptions(opts)
	for label, value := range map[string]string{
		"consulta": opts.Query, "frase exata": opts.ExactPhrase,
		"termos incluídos": opts.IncludeTerms, "termos excluídos": opts.ExcludeTerms,
	} {
		if utf8.RuneCountInString(value) > 500 {
			return fmt.Errorf("%s excede 500 caracteres", label)
		}
		if strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return fmt.Errorf("%s contém caractere de controle", label)
		}
	}
	if opts.Query == "" && opts.ExactPhrase == "" && opts.IncludeTerms == "" && opts.ChannelID == "" && opts.TopicID == "" && opts.Location == "" {
		return fmt.Errorf("informe um termo, canal, tópico ou localização")
	}
	if len(opts.PageToken) > 2048 || strings.IndexFunc(opts.PageToken, unicode.IsControl) >= 0 {
		return fmt.Errorf("token de paginação inválido")
	}
	if !opts.PublishedAfter.IsZero() && !opts.PublishedBefore.IsZero() && !opts.PublishedAfter.Before(opts.PublishedBefore) {
		return fmt.Errorf("a data inicial precisa ser anterior à final")
	}
	if opts.MinDuration < 0 || opts.MaxDuration < 0 {
		return fmt.Errorf("a duração não pode ser negativa")
	}
	if opts.MaxDuration > 0 && opts.MinDuration > opts.MaxDuration {
		return fmt.Errorf("a duração mínima precisa ser menor que a máxima")
	}
	if opts.ShortsOnly && opts.RegularOnly {
		return fmt.Errorf("shorts e vídeos comuns são opções exclusivas")
	}
	if (opts.Location == "") != (opts.LocationRadius == "") {
		return fmt.Errorf("localização e raio precisam ser informados juntos")
	}
	if len(opts.RegionCode) != 0 && len(opts.RegionCode) != 2 {
		return fmt.Errorf("região deve usar código ISO de duas letras")
	}
	if len(opts.RelevanceLanguage) > 0 && len(opts.RelevanceLanguage) > 12 {
		return fmt.Errorf("idioma inválido")
	}
	if !validSearchEnums(opts) {
		return fmt.Errorf("uma opção de filtro possui valor inválido")
	}
	seenResources := map[domain.SearchResourceType]bool{}
	for _, resource := range opts.ResourceTypes {
		if seenResources[resource] {
			return fmt.Errorf("tipo de resultado duplicado")
		}
		seenResources[resource] = true
	}
	if opts.ChannelID != "" && !simpleIDPattern.MatchString(opts.ChannelID) {
		return fmt.Errorf("ID do canal inválido")
	}
	if opts.CategoryID != "" && !categoryPattern.MatchString(opts.CategoryID) {
		return fmt.Errorf("ID da categoria inválido")
	}
	if opts.TopicID != "" && (len(opts.TopicID) > 64 || !topicPattern.MatchString(opts.TopicID)) {
		return fmt.Errorf("ID do tópico inválido")
	}
	if opts.RelevanceLanguage != "" && !languagePattern.MatchString(opts.RelevanceLanguage) {
		return fmt.Errorf("idioma deve usar um código como pt ou pt-BR")
	}
	if opts.Location != "" {
		if err := validateLocation(opts.Location, opts.LocationRadius); err != nil {
			return err
		}
	}
	if hasVideoOnlyFilter(opts) && (len(opts.ResourceTypes) != 1 || opts.ResourceTypes[0] != domain.SearchResourceVideo) {
		return fmt.Errorf("filtros de vídeo exigem selecionar somente Vídeos")
	}
	if opts.Order == domain.SearchOrderVideoCount && (len(opts.ResourceTypes) != 1 || opts.ResourceTypes[0] != domain.SearchResourceChannel) {
		return fmt.Errorf("ordem por quantidade de vídeos exige selecionar somente Canais")
	}
	if opts.Embeddable == domain.SearchNo || opts.Syndicated == domain.SearchNo {
		return fmt.Errorf("a API oficial só suporta exigir conteúdo incorporável ou distribuível")
	}
	if opts.PaidPromotion == domain.SearchNo {
		return fmt.Errorf("a API oficial só suporta exigir vídeos com promoção paga")
	}
	return nil
}

// QueryString converts the structured text controls to YouTube's documented
// q syntax. Arguments still travel as a single argv value, never through a shell.
func QueryString(opts domain.SearchOptions) string {
	parts := make([]string, 0, 4)
	if q := strings.TrimSpace(opts.Query); q != "" {
		parts = append(parts, q)
	}
	if exact := strings.Trim(strings.TrimSpace(opts.ExactPhrase), `"`); exact != "" {
		parts = append(parts, `"`+exact+`"`)
	}
	if include := strings.TrimSpace(opts.IncludeTerms); include != "" {
		parts = append(parts, include)
	}
	for _, term := range strings.Fields(opts.ExcludeTerms) {
		term = strings.TrimPrefix(term, "-")
		if term != "" {
			parts = append(parts, "-"+term)
		}
	}
	return strings.Join(parts, " ")
}

// RequiresOfficialAPI reports whether an option relies on documented Data API
// parameters unavailable from stable yt-dlp flat-search metadata. Filtros que
// podem ser aplicados localmente (Publish date, duração, ordem, canal, tipo de
// transmissão e estado local) não exigem conta conectada.
func RequiresOfficialAPI(opts domain.SearchOptions) bool {
	opts = NormalizeOptions(opts)
	if len(opts.ResourceTypes) != 1 || opts.ResourceTypes[0] != domain.SearchResourceVideo {
		return true
	}
	if opts.Caption != domain.SearchCaptionAny ||
		opts.Definition != domain.SearchDefinitionAny || opts.Dimension != domain.SearchDimensionAny ||
		opts.License != domain.SearchLicenseAny || opts.Embeddable != domain.SearchAny ||
		opts.Syndicated != domain.SearchAny || opts.PaidPromotion != domain.SearchAny ||
		opts.VideoType != domain.SearchVideoAny || opts.CategoryID != "" || opts.TopicID != "" ||
		opts.RelevanceLanguage != "" || opts.RegionCode != "" ||
		opts.SafeSearch != domain.SearchSafeAny || opts.Location != "" || opts.LocationRadius != "" {
		return true
	}
	if opts.Order == domain.SearchOrderRating || opts.Order == domain.SearchOrderVideoCount {
		return true
	}
	return false
}

// PersonalState is the local-only context used by privacy-preserving filters.
type PersonalState struct {
	Progress           map[string]domain.ProgressInfo
	Favorites          map[string]bool
	Queued             map[string]bool
	SubscribedChannels map[string]bool
	RejectedVideos     map[string]bool
	RejectedChannels   map[string]bool
}

// ApplyLocalFilters applies custom duration, Shorts and personal-state filters
// without sending history, favorites or queue state to a remote provider.
func ApplyLocalFilters(videos []domain.Video, opts domain.SearchOptions, state PersonalState) []domain.Video {
	opts = NormalizeOptions(opts)
	out := make([]domain.Video, 0, len(videos))
	for _, video := range videos {
		if !matchesLocal(video, opts, state) {
			continue
		}
		out = append(out, video)
	}
	sortResults(out, opts.Order)
	return out
}

func matchesLocal(video domain.Video, opts domain.SearchOptions, state PersonalState) bool {
	resource := video.ResourceType
	if resource == "" {
		resource = domain.SearchResourceVideo
	}
	if !containsResource(opts.ResourceTypes, resource) {
		return false
	}
	if opts.ChannelID != "" && video.ChannelID != opts.ChannelID {
		return false
	}
	if opts.OnlySubscribed && !state.SubscribedChannels[video.ChannelID] {
		return false
	}
	if opts.HideRejected && (state.RejectedVideos[video.ID] || state.RejectedChannels[video.ChannelID]) {
		return false
	}
	if !opts.PublishedAfter.IsZero() && video.PublishedAt.Before(opts.PublishedAfter) {
		return false
	}
	if !opts.PublishedBefore.IsZero() && !video.PublishedAt.IsZero() && video.PublishedAt.After(opts.PublishedBefore) {
		return false
	}
	if resource != domain.SearchResourceVideo {
		return opts.WatchState == domain.SearchWatchAny && opts.SavedState == domain.SearchSavedAny && !opts.ShortsOnly
	}
	if opts.MinDuration > 0 && video.Duration < opts.MinDuration {
		return false
	}
	if opts.MaxDuration > 0 && video.Duration > opts.MaxDuration {
		return false
	}
	if opts.ShortsOnly && (video.Duration <= 0 || video.Duration > 3*time.Minute) {
		return false
	}
	if opts.RegularOnly && (video.Duration <= 0 || video.Duration <= 3*time.Minute) {
		return false
	}
	switch opts.Duration {
	case domain.SearchDurationShort:
		if video.Duration <= 0 || video.Duration >= 4*time.Minute {
			return false
		}
	case domain.SearchDurationMedium:
		if video.Duration < 4*time.Minute || video.Duration > 20*time.Minute {
			return false
		}
	case domain.SearchDurationLong:
		if video.Duration <= 20*time.Minute {
			return false
		}
	}
	if opts.EventType != domain.SearchEventAny {
		// Normaliza os dois vocabulários (yt-dlp e API oficial) no mesmo
		// domínio: "is_live"/"live", "was_live"/"completed", "is_upcoming"/"upcoming".
		want := map[domain.SearchEventType]domain.LiveKind{
			domain.SearchEventLive:      domain.LiveNow,
			domain.SearchEventCompleted: domain.LiveCompleted,
			domain.SearchEventUpcoming:  domain.LiveUpcoming,
		}[opts.EventType]
		if domain.LiveKindOf(video.LiveStatus) != want {
			return false
		}
	}
	progress, hasProgress := state.Progress[video.ID]
	meaningful := hasProgress && (progress.Completed || progress.Position >= 15*time.Second)
	switch opts.WatchState {
	case domain.SearchWatchUnwatched:
		if meaningful {
			return false
		}
	case domain.SearchWatchWatched:
		if !meaningful {
			return false
		}
	case domain.SearchWatchContinue:
		if !meaningful || progress.Completed || progress.Duration <= 0 || progress.Duration-progress.Position <= 30*time.Second {
			return false
		}
	}
	switch opts.SavedState {
	case domain.SearchSavedFavorites:
		return state.Favorites[video.ID]
	case domain.SearchSavedQueue:
		return state.Queued[video.ID]
	case domain.SearchSavedNotSaved:
		return !state.Favorites[video.ID] && !state.Queued[video.ID]
	}
	return true
}

func containsResource(resources []domain.SearchResourceType, want domain.SearchResourceType) bool {
	for _, resource := range resources {
		if resource == want {
			return true
		}
	}
	return false
}

func hasVideoOnlyFilter(opts domain.SearchOptions) bool {
	return opts.Duration != domain.SearchDurationAny || opts.MinDuration > 0 || opts.MaxDuration > 0 || opts.ShortsOnly || opts.RegularOnly ||
		opts.EventType != domain.SearchEventAny || opts.Caption != domain.SearchCaptionAny ||
		opts.Definition != domain.SearchDefinitionAny || opts.Dimension != domain.SearchDimensionAny ||
		opts.License != domain.SearchLicenseAny || opts.Embeddable != domain.SearchAny ||
		opts.Syndicated != domain.SearchAny || opts.PaidPromotion != domain.SearchAny || opts.VideoType != domain.SearchVideoAny ||
		opts.CategoryID != "" || opts.Location != "" || opts.WatchState != domain.SearchWatchAny || opts.SavedState != domain.SearchSavedAny
}

var (
	simpleIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	categoryPattern = regexp.MustCompile(`^[0-9]{1,3}$`)
	topicPattern    = regexp.MustCompile(`^/[mg]/[A-Za-z0-9_-]+$`)
	languagePattern = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
	radiusPattern   = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)(m|km|ft|mi)$`)
)

func validateLocation(location, radius string) error {
	latitudeText, longitudeText, ok := strings.Cut(location, ",")
	if !ok {
		return fmt.Errorf("localização deve usar latitude,longitude")
	}
	latitude, latErr := strconv.ParseFloat(strings.TrimSpace(latitudeText), 64)
	longitude, lonErr := strconv.ParseFloat(strings.TrimSpace(longitudeText), 64)
	if latErr != nil || lonErr != nil || latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return fmt.Errorf("coordenadas fora do intervalo válido")
	}
	match := radiusPattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(radius)))
	if match == nil {
		return fmt.Errorf("raio deve incluir unidade m, km, ft ou mi")
	}
	value, _ := strconv.ParseFloat(match[1], 64)
	kilometers := map[string]float64{
		"m": value / 1000, "km": value, "ft": value * 0.0003048, "mi": value * 1.609344,
	}[match[2]]
	if value <= 0 || kilometers > 1000 {
		return fmt.Errorf("raio deve ser maior que zero e no máximo 1000 km")
	}
	return nil
}

func validSearchEnums(opts domain.SearchOptions) bool {
	for _, resource := range opts.ResourceTypes {
		if resource != domain.SearchResourceVideo && resource != domain.SearchResourceChannel && resource != domain.SearchResourcePlaylist {
			return false
		}
	}
	validOrder := opts.Order == domain.SearchOrderRelevance || opts.Order == domain.SearchOrderDate || opts.Order == domain.SearchOrderViewCount || opts.Order == domain.SearchOrderRating || opts.Order == domain.SearchOrderTitle || opts.Order == domain.SearchOrderVideoCount
	validDuration := opts.Duration == domain.SearchDurationAny || opts.Duration == domain.SearchDurationShort || opts.Duration == domain.SearchDurationMedium || opts.Duration == domain.SearchDurationLong || opts.Duration == domain.SearchDurationCustom
	validEvent := opts.EventType == domain.SearchEventAny || opts.EventType == domain.SearchEventLive || opts.EventType == domain.SearchEventCompleted || opts.EventType == domain.SearchEventUpcoming
	validCaption := opts.Caption == domain.SearchCaptionAny || opts.Caption == domain.SearchCaptionClosed || opts.Caption == domain.SearchCaptionNone
	validDefinition := opts.Definition == domain.SearchDefinitionAny || opts.Definition == domain.SearchDefinitionHigh || opts.Definition == domain.SearchDefinitionStandard
	validDimension := opts.Dimension == domain.SearchDimensionAny || opts.Dimension == domain.SearchDimension2D || opts.Dimension == domain.SearchDimension3D
	validLicense := opts.License == domain.SearchLicenseAny || opts.License == domain.SearchLicenseYouTube || opts.License == domain.SearchLicenseCreativeCommon
	validVideoType := opts.VideoType == domain.SearchVideoAny || opts.VideoType == domain.SearchVideoMovie || opts.VideoType == domain.SearchVideoEpisode
	validSafe := opts.SafeSearch == domain.SearchSafeAny || opts.SafeSearch == domain.SearchSafeModerate || opts.SafeSearch == domain.SearchSafeNone || opts.SafeSearch == domain.SearchSafeStrict
	validTri := func(value domain.SearchTriState) bool {
		return value == domain.SearchAny || value == domain.SearchYes || value == domain.SearchNo
	}
	validWatch := opts.WatchState == domain.SearchWatchAny || opts.WatchState == domain.SearchWatchUnwatched || opts.WatchState == domain.SearchWatchWatched || opts.WatchState == domain.SearchWatchContinue
	validSaved := opts.SavedState == domain.SearchSavedAny || opts.SavedState == domain.SearchSavedFavorites || opts.SavedState == domain.SearchSavedQueue || opts.SavedState == domain.SearchSavedNotSaved
	return validOrder && validDuration && validEvent && validCaption && validDefinition && validDimension && validLicense && validVideoType && validSafe && validTri(opts.Embeddable) && validTri(opts.Syndicated) && validTri(opts.PaidPromotion) && validWatch && validSaved
}

func sortResults(videos []domain.Video, order domain.SearchOrder) {
	switch order {
	case domain.SearchOrderDate:
		sort.SliceStable(videos, func(i, j int) bool { return videos[i].PublishedAt.After(videos[j].PublishedAt) })
	case domain.SearchOrderViewCount:
		sort.SliceStable(videos, func(i, j int) bool { return videos[i].ViewCount > videos[j].ViewCount })
	case domain.SearchOrderTitle:
		sort.SliceStable(videos, func(i, j int) bool {
			return strings.ToLower(videos[i].Title) < strings.ToLower(videos[j].Title)
		})
	}
}
