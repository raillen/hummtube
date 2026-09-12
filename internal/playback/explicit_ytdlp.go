// Package playback implements the ExplicitYtDlpResolver strategy
// (docs/03-implementation/YOUTUBE_PLAYBACK_MODERNIZATION.md).
package playback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

// YtDlpFormatJSON represents a format stream in yt-dlp JSON dump.
type YtDlpFormatJSON struct {
	FormatID    string            `json:"format_id"`
	URL         string            `json:"url"`
	Ext         string            `json:"ext"`
	Protocol    string            `json:"protocol"`
	VCodec      string            `json:"vcodec"`
	ACodec      string            `json:"acodec"`
	ABR         float64           `json:"abr"`
	Height      int               `json:"height"`
	Width       int               `json:"width"`
	Resolution  string            `json:"resolution"`
	HTTPHeaders map[string]string `json:"http_headers"`
}

// YtDlpVideoJSON represents the structured output of yt-dlp --dump-json.
type YtDlpVideoJSON struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Duration         float64           `json:"duration"`
	Expires          int64             `json:"expires"`
	URL              string            `json:"url"`
	HTTPHeaders      map[string]string `json:"http_headers"`
	RequestedFormats []YtDlpFormatJSON `json:"requested_formats"`
	Formats          []YtDlpFormatJSON `json:"formats"`
}

// ExplicitYtDlpResolver resolves media streams by executing yt-dlp directly,
// parsing structured JSON, and creating a PlaybackPlan with resolved media URLs.
type ExplicitYtDlpResolver struct {
	Config               YtdlConfig
	RequireMuxed         bool
	commandContext       func(context.Context, string, ...string) *exec.Cmd
	potProviderAvailable func(YtdlConfig) bool
}

// NewExplicitYtDlpResolver creates a new resolver instance with the given configuration.
func NewExplicitYtDlpResolver(cfg YtdlConfig) *ExplicitYtDlpResolver {
	return &ExplicitYtDlpResolver{
		Config:               cfg,
		commandContext:       exec.CommandContext,
		potProviderAvailable: potProviderCandidateAvailable,
	}
}

// Resolve processes a PlaybackRequest into a resolved PlaybackPlan.
func (r *ExplicitYtDlpResolver) Resolve(ctx context.Context, req domain.PlaybackRequest) (domain.PlaybackPlan, error) {
	if err := ctx.Err(); err != nil {
		return domain.PlaybackPlan{}, err
	}

	target := strings.TrimSpace(req.SourceURL)
	if target == "" && req.VideoID != "" {
		target = "https://www.youtube.com/watch?v=" + req.VideoID
	}

	if !isRemote(target) {
		// Local media file: direct mode, no yt-dlp
		return domain.PlaybackPlan{
			Mode:       domain.PlaybackModeDirect,
			LoadTarget: target,
			Options:    map[string]string{"ytdl-raw-options": ""},
		}, nil
	}

	binary, _, err := resolveYtDlpBinary(r.Config.YtDlpPath)
	if err != nil {
		return domain.PlaybackPlan{}, ClassifyFailure("yt-dlp not found", r.Config)
	}

	if strings.EqualFold(strings.TrimSpace(r.Config.PlayerClient), "mweb") {
		providerAvailable := r.potProviderAvailable
		if providerAvailable == nil {
			providerAvailable = potProviderCandidateAvailable
		}
		if strings.TrimSpace(r.Config.POTProvider) == "" || !providerAvailable(r.Config) {
			return domain.PlaybackPlan{}, describe(FailurePOTProviderMissing, r.Config)
		}
	}

	clients := []string{r.Config.PlayerClient}
	if r.Config.PlayerClient == "" || r.Config.PlayerClient == AutoPlayerClient {
		// `mweb` is only safe to try when the configured policy can actually
		// provide its required attestation. An unverified name is not enough.
		clients = []string{"android"}
		providerAvailable := r.potProviderAvailable
		if providerAvailable == nil {
			providerAvailable = potProviderCandidateAvailable
		}
		if strings.TrimSpace(r.Config.POTProvider) != "" && providerAvailable(r.Config) {
			clients = append(clients, "mweb")
		}
	}

	var failures []error
	for index, client := range clients {
		cfg := r.Config
		cfg.PlayerClient = client
		plan, err := r.resolveWithConfig(ctx, binary, target, cfg, r.Config)
		if err == nil {
			return plan, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return domain.PlaybackPlan{}, err
		}
		failures = append(failures, err)
		if index == len(clients)-1 || !canTryNextClient(err) {
			return domain.PlaybackPlan{}, selectFinalFailure(failures)
		}
	}

	return domain.PlaybackPlan{}, selectFinalFailure(failures)
}

func selectFinalFailure(attempts []error) error {
	var selected Failure
	selectedScore := -1
	for _, attemptErr := range attempts {
		var failure Failure
		if !errors.As(attemptErr, &failure) {
			continue
		}
		score := failureSelectionScore(failure.Kind)
		if score >= selectedScore {
			selected = failure
			selectedScore = score
		}
	}
	if selectedScore >= 0 {
		return selected
	}
	return errors.Join(attempts...)
}

func failureSelectionScore(kind FailureKind) int {
	switch kind {
	case FailureCookieDecryption:
		return 5
	case FailureRestricted, FailureDRM, FailureUnavailable:
		return 4
	case FailurePOTProviderMissing, FailurePOTGeneration, FailureJSRuntimeMissing, FailureEJSMissing:
		return 3
	case FailureBotCheck, FailureNoFormats, FailureVisitorDataMissing:
		return 2
	case FailureUnknown:
		return 0
	default:
		return 1
	}
}

func canTryNextClient(err error) bool {
	var failure Failure
	if !errors.As(err, &failure) {
		return false
	}
	switch failure.Kind {
	case FailureNoFormats, FailureBotCheck, FailureVisitorDataMissing, FailurePOTProviderMissing, FailurePOTGeneration:
		return true
	default:
		return false
	}
}

// stopsProviderFallback identifica falhas que não podem ser contornadas
// consultando outro provedor público. Continuar após restrição, DRM ou vídeo
// privado só aumenta tráfego e pode transformar uma regra de acesso em uma
// tentativa de bypass. O rate limit também encerra a cadeia para não ampliar o
// bloqueio do IP com outros fingerprints na mesma solicitação.
func stopsProviderFallback(err error) bool {
	var failure Failure
	if !errors.As(err, &failure) {
		return false
	}
	switch failure.Kind {
	case FailureRestricted, FailureDRM, FailureUnavailable, FailureRateLimited, FailureCookieDecryption:
		return true
	default:
		return false
	}
}

func canTryAuthenticatedFallback(err error) bool {
	var failure Failure
	if !errors.As(err, &failure) {
		return false
	}
	return failure.Kind == FailureRestricted
}

func failuresMayBenefitFromAuthentication(failures map[string]error) bool {
	for _, err := range failures {
		var failure Failure
		if errors.As(err, &failure) && failure.Kind == FailureBotCheck {
			return true
		}
	}
	return false
}

func (r *ExplicitYtDlpResolver) resolveWithConfig(
	ctx context.Context,
	binary string,
	target string,
	commandCfg YtdlConfig,
	failureCfg YtdlConfig,
) (domain.PlaybackPlan, error) {
	args := []string{
		"--dump-json",
		"--no-playlist",
		"--no-progress",
	}
	args = append(args, commandCfg.CommandArgs()...)
	args = append(args, "--", target)

	commandContext := r.commandContext
	if commandContext == nil {
		commandContext = exec.CommandContext
	}
	stdout, stderr, err := runExtractorCommand(ctx, commandContext, binary, args...)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return domain.PlaybackPlan{}, ctxErr
		}
		errMsg := string(stderr)
		if errMsg == "" {
			errMsg = err.Error()
		}
		failure := classifyExtractorError(err, errMsg, failureCfg)
		return domain.PlaybackPlan{}, fmt.Errorf("resolução yt-dlp (%s): %w", commandCfg.PlayerClient, failure)
	}
	if commandCfg.PlayerClient == "mweb" {
		if failure := ClassifyFailure(string(stderr), failureCfg); failure.Kind == FailurePOTProviderMissing {
			return domain.PlaybackPlan{}, fmt.Errorf("resolução yt-dlp (%s): %w", commandCfg.PlayerClient, failure)
		}
	}

	var info YtDlpVideoJSON
	if err := json.Unmarshal(stdout, &info); err != nil {
		failure := describe(FailureUnknown, failureCfg)
		failure.Summary = "o yt-dlp devolveu uma resposta incompatível"
		failure.Hint = "Atualize o yt-dlp e tente novamente."
		failure.Detail = sanitize(err.Error(), 300)
		return domain.PlaybackPlan{}, fmt.Errorf("parser json do yt-dlp: %w", failure)
	}

	plan, err := buildPlanFromInfoWithMaxHeight(info, commandCfg.MaxHeight)
	if err != nil {
		failure := ClassifyFailure(err.Error(), failureCfg)
		if failure.Kind == FailureUnknown {
			failure = describe(FailureNoFormats, failureCfg)
			failure.Detail = sanitize(err.Error(), 300)
		}
		return domain.PlaybackPlan{}, fmt.Errorf("resolução yt-dlp (%s): %w", commandCfg.PlayerClient, failure)
	}
	if r.RequireMuxed && plan.Audio != nil {
		muxed := selectMuxedFormat(info.Formats, commandCfg.MaxHeight)
		if muxed == nil {
			return domain.PlaybackPlan{}, fmt.Errorf("resolução yt-dlp (%s): player web exige formato com áudio e vídeo combinados", commandCfg.PlayerClient)
		}
		plan.Primary = resolvedStreamFromFormat(*muxed)
		plan.Audio = nil
		plan.Variants = buildMuxedVariants(info.Formats, commandCfg.MaxHeight)
	}

	return plan, nil
}

func buildPlanFromInfo(info YtDlpVideoJSON) (domain.PlaybackPlan, error) {
	return buildPlanFromInfoWithMaxHeight(info, 0)
}

func buildPlanFromInfoWithMaxHeight(info YtDlpVideoJSON, maxHeight int) (domain.PlaybackPlan, error) {
	availableFormats := mergeYtDlpFormats(info.Formats, info.RequestedFormats)
	plan := domain.PlaybackPlan{
		Mode: domain.PlaybackModeResolvedMedia,
		Metadata: domain.PlaybackMetadata{
			VideoID:  info.ID,
			Title:    info.Title,
			Duration: time.Duration(info.Duration * float64(time.Second)),
		},
		Options:   make(map[string]string),
		Variants:  buildWebVariants(availableFormats, maxHeight),
		AudioOnly: selectWebAudioStream(availableFormats),
	}
	if info.Expires > 0 {
		plan.ExpiresAt = time.Unix(info.Expires, 0)
	}

	// O formato padrão escolhido pelo yt-dlp pode ser AV1/VP9 + Opus. Esses
	// codecs não são reproduzíveis em todas as instalações WebKitGTK e geram
	// MEDIA_ERR_SRC_NOT_SUPPORTED (código 4), sobretudo no hardware de
	// referência. Escolha primeiro o melhor par H.264 + AAC disponível; os
	// formatos pedidos pelo yt-dlp já estão incluídos em availableFormats.
	videoFormat, audioFormat := selectWebPlayableFormats(availableFormats, maxHeight)
	if videoFormat != nil {
		plan.Primary = resolvedStreamFromFormat(*videoFormat)
	}
	if audioFormat != nil {
		audio := resolvedStreamFromFormat(*audioFormat)
		plan.Audio = &audio
	}

	if plan.Primary.URL == "" && info.URL != "" {
		plan.Primary = domain.ResolvedStream{
			URL:     info.URL,
			Headers: sanitizePlaybackHeaders(info.HTTPHeaders),
		}
	}
	if plan.Primary.URL == "" {
		return domain.PlaybackPlan{}, errors.New("nenhuma url de mídia encontrada no json do yt-dlp")
	}

	if plan.Audio != nil && (!sameHTTPHeaders(plan.Primary.Headers, plan.Audio.Headers) ||
		!compatibleCrossOriginHeaders(plan.Primary.URL, plan.Audio.URL, plan.Primary.Headers, plan.Audio.Headers)) {
		muxed := selectMuxedFormat(availableFormats, maxHeight)
		if muxed == nil {
			return domain.PlaybackPlan{}, errors.New("faixas adaptativas exigem cabeçalhos HTTP incompatíveis e não há formato muxado compatível")
		}
		plan.Primary = resolvedStreamFromFormat(*muxed)
		plan.Audio = nil
	}
	for _, streamURL := range playbackPlanURLs(plan) {
		expiresAt := mediaURLExpiry(streamURL)
		if !expiresAt.IsZero() && (plan.ExpiresAt.IsZero() || expiresAt.Before(plan.ExpiresAt)) {
			plan.ExpiresAt = expiresAt
		}
	}

	return plan, nil
}

func mergeYtDlpFormats(formatGroups ...[]YtDlpFormatJSON) []YtDlpFormatJSON {
	seen := make(map[string]bool)
	var merged []YtDlpFormatJSON
	for _, formats := range formatGroups {
		for _, format := range formats {
			key := strings.TrimSpace(format.FormatID) + "\x00" + strings.TrimSpace(format.URL)
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, format)
		}
	}
	return merged
}

func resolvedStreamFromFormat(format YtDlpFormatJSON) domain.ResolvedStream {
	return domain.ResolvedStream{URL: format.URL, Headers: sanitizePlaybackHeaders(format.HTTPHeaders)}
}

func sanitizePlaybackHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	allowed := make(map[string]string, len(headers))
	for name, value := range headers {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "accept", "accept-encoding", "accept-language", "origin", "referer", "user-agent":
		default:
			continue
		}
		if len(value) > 4096 || strings.ContainsAny(value, "\r\n") {
			continue
		}
		allowed[name] = value
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func sameHTTPHeaders(first, second map[string]string) bool {
	if len(first) != len(second) {
		return false
	}
	for firstName, firstValue := range first {
		matched := false
		for secondName, secondValue := range second {
			if strings.EqualFold(firstName, secondName) {
				if firstValue != secondValue {
					return false
				}
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func compatibleCrossOriginHeaders(firstURL, secondURL string, first, second map[string]string) bool {
	if sameHTTPOrigin(firstURL, secondURL) {
		return true
	}
	for name := range first {
		if !safeCrossOriginHeader(name) {
			return false
		}
	}
	for name := range second {
		if !safeCrossOriginHeader(name) {
			return false
		}
	}
	return true
}

func sameHTTPOrigin(first, second string) bool {
	firstURL, firstErr := url.Parse(first)
	secondURL, secondErr := url.Parse(second)
	if firstErr != nil || secondErr != nil || firstURL.Host == "" || secondURL.Host == "" {
		return false
	}
	return strings.EqualFold(firstURL.Scheme, secondURL.Scheme) && strings.EqualFold(firstURL.Host, secondURL.Host)
}

func safeCrossOriginHeader(name string) bool {
	switch strings.ToLower(name) {
	case "accept", "accept-encoding", "accept-language", "user-agent":
		return true
	default:
		return false
	}
}

func selectMuxedFormat(formats []YtDlpFormatJSON, maxHeight int) *YtDlpFormatJSON {
	var selected *YtDlpFormatJSON
	for i := range formats {
		format := &formats[i]
		if format.URL == "" || format.VCodec == "" || format.VCodec == "none" || format.ACodec == "" || format.ACodec == "none" {
			continue
		}
		if maxHeight > 0 && format.Height > maxHeight {
			continue
		}
		if selected == nil || muxedFormatPreferred(*format, *selected) {
			selected = format
		}
	}
	return selected
}

func muxedFormatPreferred(candidate, current YtDlpFormatJSON) bool {
	candidateCompatible := isLegacyWebVideoCodec(candidate) && isLegacyWebAudioCodec(candidate)
	currentCompatible := isLegacyWebVideoCodec(current) && isLegacyWebAudioCodec(current)
	if candidateCompatible != currentCompatible {
		return candidateCompatible
	}
	if candidate.Height != current.Height {
		return candidate.Height > current.Height
	}
	return webVideoPreference(candidate) > webVideoPreference(current)
}

func buildMuxedVariants(formats []YtDlpFormatJSON, maxHeight int) []domain.PlaybackVariant {
	byHeight := make(map[int]YtDlpFormatJSON)
	for _, format := range formats {
		if format.URL == "" || format.Height <= 0 || format.VCodec == "" || format.VCodec == "none" || format.ACodec == "" || format.ACodec == "none" {
			continue
		}
		if maxHeight > 0 && format.Height > maxHeight {
			continue
		}
		current, exists := byHeight[format.Height]
		if !exists || webVideoPreference(format) >= webVideoPreference(current) {
			byHeight[format.Height] = format
		}
	}

	variants := make([]domain.PlaybackVariant, 0, len(byHeight))
	for height, format := range byHeight {
		id := strings.TrimSpace(format.FormatID)
		if id == "" {
			id = fmt.Sprintf("%dp", height)
		}
		variants = append(variants, domain.PlaybackVariant{
			ID:       id,
			Label:    fmt.Sprintf("%dp", height),
			Height:   height,
			HasAudio: true,
			Stream:   resolvedStreamFromFormat(format),
		})
	}
	sort.SliceStable(variants, func(i, j int) bool {
		return variants[i].Height > variants[j].Height
	})
	return variants
}

// buildWebVariants exposes one directly loadable stream per resolution. Above
// the progressive YouTube ceiling (normally 360p), variants are commonly
// video-only and reuse PlaybackPlan.Audio in the web player.
func buildWebVariants(formats []YtDlpFormatJSON, maxHeight int) []domain.PlaybackVariant {
	hasLegacyCompatibleVideo := false
	for _, format := range formats {
		if playableVideoFormat(format, maxHeight) && isDirectWebVideoFormat(format) && isLegacyWebVideoCodec(format) {
			hasLegacyCompatibleVideo = true
			break
		}
	}

	byHeight := make(map[int]YtDlpFormatJSON)
	for _, format := range formats {
		if !playableVideoFormat(format, maxHeight) || format.Height <= 0 || !isDirectWebVideoFormat(format) {
			continue
		}
		// Quando H.264 está disponível, não ofereça AV1/VP9 como se fossem
		// universalmente reproduzíveis. WebViews modernos ainda recebem esses
		// formatos quando forem a única alternativa fornecida pelo provedor.
		if hasLegacyCompatibleVideo && !isLegacyWebVideoCodec(format) {
			continue
		}
		current, exists := byHeight[format.Height]
		if !exists || webVideoPreference(format) >= webVideoPreference(current) {
			byHeight[format.Height] = format
		}
	}

	variants := make([]domain.PlaybackVariant, 0, len(byHeight))
	for height, format := range byHeight {
		id := strings.TrimSpace(format.FormatID)
		if id == "" {
			id = fmt.Sprintf("%dp", height)
		}
		variants = append(variants, domain.PlaybackVariant{
			ID:       id,
			Label:    fmt.Sprintf("%dp", height),
			Height:   height,
			HasAudio: format.ACodec != "" && format.ACodec != "none",
			Stream:   resolvedStreamFromFormat(format),
		})
	}
	sort.SliceStable(variants, func(i, j int) bool {
		return variants[i].Height > variants[j].Height
	})
	return variants
}

func isDirectWebVideoFormat(format YtDlpFormatJSON) bool {
	protocol := strings.ToLower(strings.TrimSpace(format.Protocol))
	switch protocol {
	case "", "http", "https", "m3u8", "m3u8_native":
	default:
		return false
	}
	extension := strings.ToLower(strings.TrimSpace(format.Ext))
	return extension == "" || extension == "mp4" || extension == "webm"
}

func webVideoPreference(format YtDlpFormatJSON) int {
	score := 0
	if isLegacyWebVideoCodec(format) {
		score += 1000
	}
	if isLegacyWebAudioCodec(format) {
		score += 250
	}
	if format.ACodec != "" && format.ACodec != "none" {
		score += 100
	}
	if strings.EqualFold(format.Ext, "mp4") {
		score += 20
	}
	return score
}

func isLegacyWebVideoCodec(format YtDlpFormatJSON) bool {
	codec := strings.ToLower(strings.TrimSpace(format.VCodec))
	return strings.HasPrefix(codec, "avc1") || strings.Contains(codec, "h264")
}

func isLegacyWebAudioCodec(format YtDlpFormatJSON) bool {
	codec := strings.ToLower(strings.TrimSpace(format.ACodec))
	return strings.HasPrefix(codec, "mp4a") || strings.HasPrefix(codec, "aac")
}

func selectWebAudioStream(formats []YtDlpFormatJSON) *domain.ResolvedStream {
	var selected *YtDlpFormatJSON
	for index := range formats {
		format := &formats[index]
		if !isWebAudioFormat(*format) {
			continue
		}
		if selected == nil || webAudioPreference(*format) > webAudioPreference(*selected) {
			selected = format
		}
	}
	if selected == nil {
		return nil
	}
	stream := resolvedStreamFromFormat(*selected)
	return &stream
}

func webAudioPreference(format YtDlpFormatJSON) float64 {
	score := format.ABR
	if isLegacyWebAudioCodec(format) {
		score += 10000
	}
	if strings.EqualFold(format.Ext, "m4a") || strings.EqualFold(format.Ext, "mp4") {
		score += 1000
	}
	return score
}

func isWebAudioFormat(format YtDlpFormatJSON) bool {
	if format.URL == "" || format.VCodec != "none" || format.ACodec == "" || format.ACodec == "none" {
		return false
	}
	protocol := strings.ToLower(strings.TrimSpace(format.Protocol))
	switch protocol {
	case "", "http", "https", "m3u8", "m3u8_native":
		// Protocolos carregáveis diretamente pelo HTML5 ou pelo HLS.js.
	default:
		return false
	}
	extension := strings.ToLower(strings.TrimSpace(format.Ext))
	codec := strings.ToLower(strings.TrimSpace(format.ACodec))
	switch extension {
	case "m4a", "mp4":
		return strings.HasPrefix(codec, "mp4a") || strings.HasPrefix(codec, "aac")
	case "webm":
		return strings.HasPrefix(codec, "opus") || strings.HasPrefix(codec, "vorbis")
	case "mp3":
		return strings.Contains(codec, "mp3")
	case "ogg", "oga":
		return strings.HasPrefix(codec, "opus") || strings.HasPrefix(codec, "vorbis")
	case "":
		// Respostas antigas nem sempre informam `ext`; os codecs abaixo ainda
		// possuem suporte direto nos WebViews alvo.
		return strings.HasPrefix(codec, "mp4a") || strings.HasPrefix(codec, "aac") ||
			strings.HasPrefix(codec, "opus") || strings.HasPrefix(codec, "vorbis") ||
			strings.Contains(codec, "mp3")
	default:
		return false
	}
}

func playableFormat(format YtDlpFormatJSON) bool {
	if format.URL == "" {
		return false
	}
	return (format.VCodec != "" && format.VCodec != "none") || (format.ACodec != "" && format.ACodec != "none")
}

func playableVideoFormat(format YtDlpFormatJSON, maxHeight int) bool {
	if !playableFormat(format) || format.VCodec == "" || format.VCodec == "none" {
		return false
	}
	return maxHeight <= 0 || format.Height <= 0 || format.Height <= maxHeight
}

func selectPlayableFormats(formats []YtDlpFormatJSON, maxHeight int) (video, audio *YtDlpFormatJSON) {
	var muxed, videoOnly, audioOnly *YtDlpFormatJSON
	for i := range formats {
		format := &formats[i]
		if format.URL == "" {
			continue
		}
		if maxHeight > 0 && format.Height > maxHeight {
			continue
		}
		hasVideo := format.VCodec != "" && format.VCodec != "none"
		hasAudio := format.ACodec != "" && format.ACodec != "none"
		if hasVideo && hasAudio {
			if muxed == nil || format.Height > muxed.Height {
				muxed = format
			}
			continue
		}
		if hasVideo && (videoOnly == nil || format.Height > videoOnly.Height) {
			videoOnly = format
		}
		if hasAudio {
			audioOnly = format
		}
	}
	if videoOnly != nil && audioOnly != nil && (muxed == nil || videoOnly.Height > muxed.Height) {
		return videoOnly, audioOnly
	}
	return muxed, nil
}

// selectWebPlayableFormats keeps automatic playback compatible with the
// conservative WebKitGTK/GStreamer baseline. It falls back to the broader
// HTML5 set only when the provider exposes no H.264/AAC alternative at all.
func selectWebPlayableFormats(formats []YtDlpFormatJSON, maxHeight int) (video, audio *YtDlpFormatJSON) {
	var muxed, videoOnly, audioOnly *YtDlpFormatJSON
	for index := range formats {
		format := &formats[index]
		if !playableVideoFormat(*format, maxHeight) || !isDirectWebVideoFormat(*format) || !isLegacyWebVideoCodec(*format) {
			continue
		}
		hasAudio := format.ACodec != "" && format.ACodec != "none"
		if hasAudio && isLegacyWebAudioCodec(*format) {
			if muxed == nil || format.Height > muxed.Height || (format.Height == muxed.Height && webVideoPreference(*format) > webVideoPreference(*muxed)) {
				muxed = format
			}
			continue
		}
		if !hasAudio && (videoOnly == nil || format.Height > videoOnly.Height) {
			videoOnly = format
		}
	}
	for index := range formats {
		format := &formats[index]
		if !isWebAudioFormat(*format) || !isLegacyWebAudioCodec(*format) {
			continue
		}
		if audioOnly == nil || webAudioPreference(*format) > webAudioPreference(*audioOnly) {
			audioOnly = format
		}
	}
	if videoOnly != nil && audioOnly != nil && (muxed == nil || videoOnly.Height > muxed.Height) {
		return videoOnly, audioOnly
	}
	if muxed != nil {
		return muxed, nil
	}
	return selectPlayableFormats(formats, maxHeight)
}
