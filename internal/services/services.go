// Package services expõe a ponte de serviços Go para o frontend Svelte via Wails v3.
package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	stdsync "sync"
	"time"

	"github.com/nanotube/nanotube-web/internal/auth"
	"github.com/nanotube/nanotube-web/internal/backup"
	"github.com/nanotube/nanotube-web/internal/diagnostics"
	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/iptv"
	"github.com/nanotube/nanotube-web/internal/lastfm"
	"github.com/nanotube/nanotube-web/internal/playback"
	"github.com/nanotube/nanotube-web/internal/search"
	"github.com/nanotube/nanotube-web/internal/storage"
	"github.com/nanotube/nanotube-web/internal/suggestions"
	"github.com/nanotube/nanotube-web/internal/sync"
	"github.com/nanotube/nanotube-web/internal/youtubeapi"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// PlaylistDetailModel agrega metadados e os vídeos contidos na playlist.
type PlaylistDetailModel struct {
	Playlist domain.Playlist `json:"playlist"`
	Videos   []domain.Video  `json:"videos"`
}

// RemotePlaylistPageModel é uma projeção paginada e somente leitura.
// Ela não confunde uma playlist do YouTube com uma coleção local SQLite.
type RemotePlaylistPageModel struct {
	PlaylistID    string         `json:"playlist_id"`
	Videos        []domain.Video `json:"videos"`
	NextPageToken string         `json:"next_page_token,omitempty"`
}

// RemoteChannelVideoPageModel is a public, non-persisted page of channel
// uploads. Persistence happens only when the user plays or queues an item.
type RemoteChannelVideoPageModel struct {
	ChannelID     string         `json:"channel_id"`
	Videos        []domain.Video `json:"videos"`
	NextPageToken string         `json:"next_page_token,omitempty"`
}

// HomeSectionModel is the frontend projection of a recommendation section.
// Scores and explanations remain domain concerns; cards consume plain videos.
type HomeSectionModel struct {
	ID     string         `json:"id"`
	Title  string         `json:"title"`
	Videos []domain.Video `json:"videos"`
}

// HomeModel is the stable frontend contract for the local recommendation feed.
type HomeModel struct {
	ContinueWatching    []domain.Video     `json:"continue_watching"`
	ForYou              []domain.Video     `json:"for_you"`
	TopicSections       []HomeSectionModel `json:"topic_sections"`
	RecentSubscriptions []domain.Video     `json:"recent_subscriptions"`
	Rediscovery         []domain.Video     `json:"rediscovery"`
	Favorites           []domain.Video     `json:"favorites"`
	Unwatched           []domain.Video     `json:"unwatched"`
	LongVideos          []domain.Video     `json:"long_videos"`
}

// LoginStatus exposes asynchronous OAuth completion without exposing tokens.
type LoginStatus struct {
	State     string `json:"state"`
	ProfileID string `json:"profile_id,omitempty"`
	Warning   string `json:"warning,omitempty"`
	Error     string `json:"error,omitempty"`
}

// DeviceCodeInfo é a projeção pública do fluxo de dispositivo. O device_code
// usado para polling permanece exclusivamente no backend.
type DeviceCodeInfo struct {
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
}

// AppServices agrega todos os serviços do backend NanoTube Web.
type AppServices struct {
	resources     diagnostics.ResourceSampler
	Repo          *storage.Repository
	Resolver      *playback.CascadingResolver
	SearchService search.Service
	Engine        *suggestions.Engine
	SyncService   *sync.Service
	SecretStore   domain.SecretStore
	LastFM        *lastfm.Client
	MediaProxy    *playback.MediaProxy
	Runtime       *playback.RuntimeManager
	OAuthSession  *auth.Session
	// IPTVM3USource is replaceable by integration tests and trusted adapters.
	// The zero value is the production implementation with SSRF validation.
	IPTVM3USource       iptv.HTTPM3USource
	resolverMu          stdsync.RWMutex
	loginMu             stdsync.RWMutex
	loginCommitMu       stdsync.Mutex
	loginStatuses       map[string]LoginStatus
	loginGenerations    map[string]uint64
	profileSessions     map[string]*auth.Session
	identityFetcher     func(context.Context, *http.Client) (auth.ProviderIdentity, error)
	profileBootstrapErr error
	lastFMMu            stdsync.Mutex
	lastFMPending       map[string]string
	settingListenersMu  stdsync.RWMutex
	settingListeners    map[uint64]func(string, string)
	nextSettingListener uint64
}

// NewAppServices instancia o container de serviços com as dependências integradas.
func NewAppServices(repo *storage.Repository, secretStore domain.SecretStore) *AppServices {
	bootstrapCtx := context.Background()
	var bootstrapWarnings []string
	var profileBootstrapErrors []error
	if err := repo.CleanupEphemeralProfiles(bootstrapCtx); err != nil {
		bootstrapWarnings = append(bootstrapWarnings, "não foi possível limpar perfis guest abandonados")
		profileBootstrapErrors = append(profileBootstrapErrors, err)
	}
	if err := storage.NewAccountRepository(repo.DB()).ClearTransientAccounts(bootstrapCtx); err != nil {
		bootstrapWarnings = append(bootstrapWarnings, "não foi possível limpar sessões apenas em memória")
		profileBootstrapErrors = append(profileBootstrapErrors, err)
	}
	if err := auth.MigrateLegacyRefreshToken(bootstrapCtx, secretStore, storage.DefaultProfileID); err != nil {
		if errors.Is(err, domain.ErrSecretStoreUnavailable) {
			bootstrapWarnings = append(bootstrapWarnings, auth.KeyringUnavailableWarning)
		} else {
			bootstrapWarnings = append(bootstrapWarnings, "não foi possível migrar a sessão OAuth legada")
		}
	}
	activeProfileID := storage.DefaultProfileID
	if profileID, err := repo.ActiveProfileID(bootstrapCtx); err == nil && strings.TrimSpace(profileID) != "" {
		activeProfileID = profileID
	} else if err != nil {
		bootstrapWarnings = append(bootstrapWarnings, "não foi possível determinar o perfil ativo para o playback")
	}
	config, invidiousInstance := playbackConfigFromSettings(bootstrapCtx, repo, activeProfileID, &bootstrapWarnings)
	cascadingResolver := playback.NewWebCascadingResolver(config, invidiousInstance)

	searchSvc := search.Service{
		Public: search.YtDlp{Config: config},
	}
	engine := suggestions.NewEngine()
	syncSvc, _ := sync.NewLocalService(repo, sync.DefaultOptions())

	services := &AppServices{
		Repo:          repo,
		Resolver:      cascadingResolver,
		SearchService: searchSvc,
		Engine:        engine,
		SyncService:   syncSvc,
		SecretStore:   secretStore,
		LastFM:        lastfm.NewClient(lastfm.LoadConfig()),
		MediaProxy:    playback.NewMediaProxy(),
		Runtime:       managedRuntimeManager(),
		loginStatuses: map[string]LoginStatus{
			storage.DefaultProfileID: {
				State: "idle", ProfileID: storage.DefaultProfileID,
				Warning: strings.Join(bootstrapWarnings, "; "),
			},
		},
		loginGenerations:    make(map[string]uint64),
		profileSessions:     make(map[string]*auth.Session),
		identityFetcher:     auth.FetchIdentity,
		profileBootstrapErr: errors.Join(profileBootstrapErrors...),
		lastFMPending:       make(map[string]string),
		settingListeners:    make(map[uint64]func(string, string)),
	}
	services.SearchService.Official = youtubeapi.Provider{NewService: services.newYouTubeService}
	return services
}

func managedRuntimeManager() *playback.RuntimeManager {
	manager, err := playback.NewManagedRuntimeManager()
	if err != nil {
		return nil
	}
	return manager
}

func (s *AppServices) newYouTubeService(ctx context.Context) (*youtube.Service, error) {
	if err := s.ensureProfileStorageReady(); err != nil {
		return nil, err
	}
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return nil, fmt.Errorf("determinar perfil ativo: %w", err)
	}
	if profile.IsGuest() {
		return nil, errors.New("o perfil visitante não possui uma conta do YouTube; conecte uma conta para usar esta busca")
	}
	account, connected, err := storage.NewAccountRepository(s.Repo.DB(), profile.ID).Account(ctx)
	if err != nil {
		return nil, fmt.Errorf("verificar conta ativa: %w", err)
	}
	if !connected {
		return nil, errors.New("conecte uma conta do YouTube para usar filtros oficiais, canais e playlists")
	}
	s.assessAccountCredential(ctx, &account)
	if account.CredentialState != domain.CredentialStateAvailable && account.CredentialState != domain.CredentialStateMemory {
		return nil, errors.New(firstNonEmpty(account.Warning, "a sessão da conta ativa não está disponível; autentique novamente"))
	}

	session := s.existingSession(profile.ID)
	if session == nil {
		cfg, configErr := auth.LoadConfig(ctx)
		if configErr != nil {
			return nil, configErr
		}
		candidate := s.sessionForProfile(cfg, profile)
		s.loginMu.Lock()
		if current := s.profileSessions[profile.ID]; current != nil {
			session = current
		} else {
			s.profileSessions[profile.ID] = candidate
			session = candidate
		}
		s.loginMu.Unlock()
	}
	source, err := session.Source(ctx)
	if err != nil {
		return nil, fmt.Errorf("restaurar sessão da conta ativa: %w", err)
	}
	client := auth.AuthenticatedClient(context.WithoutCancel(ctx), source)
	if client == nil {
		return nil, errors.New("criar cliente autenticado: sessão indisponível")
	}
	client.Timeout = 30 * time.Second
	return youtube.NewService(ctx, option.WithHTTPClient(client))
}

// StartDeviceLogin inicia o fluxo OAuth Device Code (ideal para TV e outros dispositivos).
func (s *AppServices) StartDeviceLogin(ctx context.Context) (DeviceCodeInfo, error) {
	if err := s.ensureProfileStorageReady(); err != nil {
		return DeviceCodeInfo{}, err
	}
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return DeviceCodeInfo{}, err
	}
	cfg, err := auth.LoadConfig(ctx)
	if err != nil {
		s.auditCredential(ctx, profile.ID, "google", "verification_failed", "Configuração OAuth indisponível")
		return DeviceCodeInfo{}, err
	}

	dcr, err := auth.RequestDeviceCode(ctx, cfg)
	if err != nil {
		s.auditCredential(ctx, profile.ID, "google", "verification_failed", "Código de dispositivo não solicitado")
		return DeviceCodeInfo{}, err
	}
	generation := s.beginLoginAttempt(profile.ID)

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), time.Duration(dcr.ExpiresIn)*time.Second)
		defer cancel()

		tok, err := auth.PollDeviceToken(bgCtx, cfg, dcr.DeviceCode, dcr.Interval)
		if err != nil {
			s.setLoginStatusForAttempt("error", profile.ID, generation, "", fmt.Errorf("autorização por código falhou"))
			s.auditCredential(bgCtx, profile.ID, "google", "verification_failed", "Autorização por código falhou")
			return
		}
		s.completeLogin(bgCtx, cfg, profile, generation, tok)
	}()

	return DeviceCodeInfo{
		UserCode: dcr.UserCode, VerificationURL: dcr.VerificationURL, ExpiresIn: dcr.ExpiresIn,
	}, nil
}

// ImportSubscriptions importa inscrições de OPML, CSV Takeout ou NewPipe.
func (s *AppServices) ImportSubscriptions(ctx context.Context, content string) (sync.ImportResult, error) {
	return sync.ImportSubscriptions(ctx, s.Repo, []byte(content))
}

// SetBrowserCookies define o navegador para importação de cookies no yt-dlp.
func (s *AppServices) SetBrowserCookies(ctx context.Context, browserName string) error {
	if err := s.ensureProfileStorageReady(); err != nil {
		return err
	}
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return err
	}
	if profile.IsGuest() {
		return errors.New("o perfil visitante não pode usar uma sessão persistida de cookies")
	}
	browserName, err = playback.NormalizeBrowserCookieSource(browserName)
	if err != nil {
		return err
	}
	if err := s.Repo.SetProfileSetting(ctx, profile.ID, storage.SettingCookiesFrom, browserName); err != nil {
		return err
	}
	return s.reloadPlaybackResolver(ctx)
}

// playbackConfigFromSettings aplica somente preferências de playback salvas;
// variáveis de ambiente continuam tendo precedência para operações de suporte.
func playbackConfigFromSettings(ctx context.Context, repo *storage.Repository, profileID string, warnings *[]string) (playback.YtdlConfig, string) {
	config := playback.LoadYtdlConfig()
	read := func(key string) string {
		value, ok, err := repo.Setting(ctx, key)
		if err != nil {
			if warnings != nil {
				*warnings = append(*warnings, "não foi possível ler a configuração de playback")
			}
			return ""
		}
		if !ok {
			return ""
		}
		return strings.TrimSpace(value)
	}
	if os.Getenv(playback.EnvRemoteEJS) == "" {
		config.AllowRemoteComponents = read(storage.SettingRemoteEJS) == "1"
	}
	if os.Getenv(playback.EnvPlayerClient) == "" {
		if value := read(storage.SettingPlayerClient); value != "" {
			config.PlayerClient = playback.NormalizePlayerClientPreference(value)
		}
	}
	if os.Getenv(playback.EnvMaxHeight) == "" {
		if value, err := strconv.Atoi(read(storage.SettingMaxHeight)); err == nil && value >= 0 && value <= 4320 {
			config.MaxHeight = value
		}
	}
	// Cookie sources are profile-scoped. Environment variables remain an
	// explicit support override, but a stored session is never inherited by a
	// different profile (especially guest).
	if os.Getenv(playback.EnvCookiesFrom) == "" && strings.TrimSpace(profileID) != "" {
		profile, profileErr := repo.Profile(ctx, profileID)
		if profileErr == nil && !profile.IsGuest() {
			if browser, ok, settingErr := repo.ProfileSetting(ctx, profileID, storage.SettingCookiesFrom); settingErr != nil {
				if warnings != nil {
					*warnings = append(*warnings, "não foi possível ler a configuração de cookies do perfil ativo")
				}
			} else if ok && strings.TrimSpace(browser) != "" {
				if normalized, normalizeErr := playback.NormalizeBrowserCookieSource(browser); normalizeErr == nil {
					config.CookiesFromBrowser = normalized
				} else if warnings != nil {
					*warnings = append(*warnings, "a configuração salva de cookies do navegador foi ignorada por ser inválida")
				}
			}
		}
	}
	return config, read(storage.SettingInvidiousInstance)
}

func (s *AppServices) reloadPlaybackResolver(ctx context.Context) error {
	profileID, err := s.Repo.ActiveProfileID(ctx)
	if err != nil {
		return fmt.Errorf("determinar perfil do playback: %w", err)
	}
	config, invidiousInstance := playbackConfigFromSettings(ctx, s.Repo, profileID, nil)
	s.resolverMu.Lock()
	s.Resolver = playback.NewWebCascadingResolver(config, invidiousInstance)
	s.resolverMu.Unlock()
	return nil
}

// --- CATALOG SERVICE ---

// GetHome constrói a Home local determinística.
func (s *AppServices) GetHome(ctx context.Context, profileID string) (HomeModel, error) {
	recent, err := s.Repo.Recent(ctx, domain.VideoFilter{Limit: 100})
	if err != nil {
		return HomeModel{}, err
	}
	profile, err := s.Repo.BuildProfile(ctx)
	if err != nil {
		profile = domain.InterestProfile{}
	}
	home, err := s.Engine.BuildHome(ctx, profile, recent)
	if err != nil {
		return HomeModel{}, err
	}
	favorites, err := s.Repo.FavoriteVideos(ctx)
	if err != nil {
		return HomeModel{}, err
	}
	unwatched := make([]domain.Video, 0, len(recent))
	longVideos := make([]domain.Video, 0, len(recent))
	for _, video := range recent {
		progress, watched := profile.WatchState[video.ID]
		if !watched || (!progress.Completed && progress.Position == 0) {
			unwatched = append(unwatched, video)
		}
		if video.Duration > 20*time.Minute {
			longVideos = append(longVideos, video)
		}
	}
	model := projectHomeModel(home)
	model.Favorites = favorites
	model.Unwatched = unwatched
	model.LongVideos = longVideos
	return model, nil
}

func projectHomeModel(home domain.HomeModel) HomeModel {
	sections := make([]HomeSectionModel, 0, len(home.TopicSections))
	for _, section := range home.TopicSections {
		sections = append(sections, HomeSectionModel{
			ID: section.ID, Title: section.Title, Videos: projectHomeVideos(section.Videos),
		})
	}
	return HomeModel{
		ContinueWatching:    projectHomeVideos(home.ContinueWatching),
		ForYou:              projectHomeVideos(home.ForYou),
		TopicSections:       sections,
		RecentSubscriptions: projectHomeVideos(home.RecentSubscriptions),
		Rediscovery:         projectHomeVideos(home.Rediscovery),
	}
}

func projectHomeVideos(recommendations []domain.HomeVideo) []domain.Video {
	videos := make([]domain.Video, 0, len(recommendations))
	for _, recommendation := range recommendations {
		video := recommendation.Video
		video.RecommendationReasons = append([]string(nil), recommendation.Reasons...)
		videos = append(videos, video)
	}
	return videos
}

// RememberVideo persists metadata before local actions reference a remote
// search result. This keeps history, favorites and playlist joins visible
// after the application restarts.
func (s *AppServices) RememberVideo(ctx context.Context, video domain.Video) error {
	video.ID = strings.TrimSpace(video.ID)
	video.Title = strings.TrimSpace(video.Title)
	if video.ID == "" || video.Title == "" {
		return fmt.Errorf("salvar metadados: vídeo sem ID ou título")
	}
	return s.Repo.UpsertVideos(ctx, []domain.Video{video})
}

// RefreshSubscriptions executa a sincronização incremental dos canais.
func (s *AppServices) RefreshSubscriptions(ctx context.Context) (sync.Stats, error) {
	if s.SyncService == nil {
		return sync.Stats{}, fmt.Errorf("sync service não inicializado")
	}
	return s.SyncService.RefreshLocal(ctx, nil)
}

// GetChannels lista os canais inscritos conhecidos localmente.
func (s *AppServices) GetChannels(ctx context.Context) ([]domain.Channel, error) {
	return s.Repo.SubscribedChannels(ctx)
}

// GetRemoteChannelVideosPage lists a channel's public uploads through the
// bounded yt-dlp catalog adapter. It does not require or expose OAuth tokens.
func (s *AppServices) GetRemoteChannelVideosPage(ctx context.Context, channelID, pageToken string, limit int) (RemoteChannelVideoPageModel, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return RemoteChannelVideoPageModel{}, fmt.Errorf("canal remoto: ID vazio")
	}
	if limit <= 0 || limit > 50 {
		limit = 24
	}
	videos, nextPageToken, err := s.SearchService.Public.ChannelVideos(ctx, channelID, domain.PageOptions{
		MaxResults: limit,
		PageToken:  strings.TrimSpace(pageToken),
	})
	if err != nil {
		return RemoteChannelVideoPageModel{}, err
	}
	return RemoteChannelVideoPageModel{ChannelID: channelID, Videos: videos, NextPageToken: nextPageToken}, nil
}

// GetChannelFolders lista as pastas de canais.
func (s *AppServices) GetChannelFolders(ctx context.Context) ([]domain.ChannelFolder, error) {
	return s.Repo.Folders(ctx)
}

// CreateChannelFolder cria uma pasta local de inscrições.
func (s *AppServices) CreateChannelFolder(ctx context.Context, name string) (domain.ChannelFolder, error) {
	return s.Repo.CreateFolder(ctx, name)
}

// DeleteChannelFolder remove uma pasta local de inscrições.
func (s *AppServices) DeleteChannelFolder(ctx context.Context, id string) error {
	return s.Repo.DeleteFolder(ctx, id)
}

// AddChannelToFolder adiciona um canal a uma pasta.
func (s *AppServices) AddChannelToFolder(ctx context.Context, folderID, channelID string) error {
	return s.Repo.AddChannelToFolder(ctx, folderID, channelID)
}

// RemoveChannelFromFolder remove um canal de uma pasta.
func (s *AppServices) RemoveChannelFromFolder(ctx context.Context, folderID, channelID string) error {
	return s.Repo.RemoveChannelFromFolder(ctx, folderID, channelID)
}

// AddChannelFavorite adiciona um canal aos favoritos.
func (s *AppServices) AddChannelFavorite(ctx context.Context, channelID string) error {
	return s.Repo.AddChannelFavorite(ctx, channelID)
}

// RemoveChannelFavorite remove um canal dos favoritos.
func (s *AppServices) RemoveChannelFavorite(ctx context.Context, channelID string) error {
	return s.Repo.RemoveChannelFavorite(ctx, channelID)
}

// SubscribeChannel inscreve em um canal localmente.
func (s *AppServices) SubscribeChannel(ctx context.Context, id, title string) error {
	return s.Repo.SubscribeChannel(ctx, id, title)
}

// UnsubscribeChannel remove a inscrição de um canal localmente.
func (s *AppServices) UnsubscribeChannel(ctx context.Context, id string) error {
	return s.Repo.UnsubscribeChannel(ctx, id)
}

// RenameChannelFolder renomeia uma pasta de canais.
func (s *AppServices) RenameChannelFolder(ctx context.Context, folderID, name string) error {
	return s.Repo.RenameFolder(ctx, folderID, name)
}

// GetFavoriteChannelIDs devolve o mapa de canais favoritos.
func (s *AppServices) GetFavoriteChannelIDs(ctx context.Context) (map[string]bool, error) {
	return s.Repo.FavoriteChannelIDs(ctx)
}

// GetFolderMembership devolve o mapeamento de canais por pasta.
func (s *AppServices) GetFolderMembership(ctx context.Context) (map[string][]string, error) {
	return s.Repo.FolderMembership(ctx)
}

// --- PLAYER SERVICE ---

// ResolveMedia resolve a URL de vídeo para um PlaybackPlan consumível pelo player web.
func (s *AppServices) ResolveMedia(ctx context.Context, videoID, sourceURL string) (domain.PlaybackPlan, error) {
	req := domain.PlaybackRequest{
		VideoID:   videoID,
		SourceURL: sourceURL,
	}
	if req.SourceURL == "" && req.VideoID != "" {
		req.SourceURL = fmt.Sprintf("https://www.youtube.com/watch?v=%s", req.VideoID)
	}
	s.resolverMu.RLock()
	resolver := s.Resolver
	s.resolverMu.RUnlock()
	if resolver == nil {
		return domain.PlaybackPlan{}, fmt.Errorf("resolver de reprodução indisponível")
	}
	plan, err := resolver.Resolve(ctx, req)
	if err != nil {
		log.Printf("playback: resolução falhou para video_id=%s: %v", diagnosticVideoID(req.VideoID), err)
		return domain.PlaybackPlan{}, err
	}
	if strings.TrimSpace(plan.Primary.URL) == "" {
		log.Printf("playback: resolvedor devolveu plano sem mídia para video_id=%s", diagnosticVideoID(req.VideoID))
		return domain.PlaybackPlan{}, fmt.Errorf("resolver retornou plano incompatível com o player web")
	}
	if s.MediaProxy != nil {
		plan, err = s.MediaProxy.WrapPlan(ctx, plan)
		if err != nil {
			log.Printf("playback: proxy rejeitou plano para video_id=%s: %v", diagnosticVideoID(req.VideoID), err)
			return domain.PlaybackPlan{}, fmt.Errorf("proteger streams de reprodução: %w", err)
		}
	}
	log.Printf(
		"playback: plano pronto para video_id=%s variants=%d adaptive_audio=%t audio_only=%t loopback_http=%t",
		diagnosticVideoID(req.VideoID),
		len(plan.Variants),
		plan.Audio != nil,
		plan.AudioOnly != nil,
		strings.HasPrefix(plan.Primary.URL, "http://127.0.0.1:"),
	)
	return plan, nil
}

func diagnosticVideoID(videoID string) string {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" || len(videoID) > 64 {
		return "indisponível"
	}
	for _, character := range videoID {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' {
			continue
		}
		return "inválido"
	}
	return videoID
}

// SaveProgress persiste a posição atual e término de um vídeo.
func (s *AppServices) SaveProgress(ctx context.Context, videoID string, positionMs, durationMs int64, completed bool) error {
	return s.Repo.MarkProgress(ctx, videoID, domain.PlaybackProgress{
		VideoID:   videoID,
		Position:  time.Duration(positionMs) * time.Millisecond,
		Duration:  time.Duration(durationMs) * time.Millisecond,
		UpdatedAt: time.Now(),
		Completed: completed,
	})
}

// --- SEARCH SERVICE ---

// Search preserva o contrato RPC legado query/limit/offset.
func (s *AppServices) Search(ctx context.Context, query string, limit, offset int) (domain.SearchPage, error) {
	opts := domain.SearchOptions{
		Query:      query,
		MaxResults: limit,
	}
	if offset > 0 {
		opts.PageToken = fmt.Sprintf("%d", offset)
	}
	return s.SearchAdvanced(ctx, opts)
}

// SearchAdvanced recebe o request completo e monta os sinais locais usados
// por filtros e NanoRank. A cópia do serviço evita data race entre buscas.
func (s *AppServices) SearchAdvanced(ctx context.Context, options domain.SearchOptions) (domain.SearchPage, error) {
	guestFallback, err := s.shouldUseGuestVideoSearch(ctx, options)
	if err != nil {
		return domain.SearchPage{}, fmt.Errorf("busca: verificar conta ativa: %w", err)
	}
	if guestFallback {
		options.ResourceTypes = []domain.SearchResourceType{domain.SearchResourceVideo}
	}
	personal, err := s.localSearchState(ctx)
	if err != nil {
		return domain.SearchPage{}, fmt.Errorf("busca: carregar estado local: %w", err)
	}
	searchService := s.SearchService
	searchService.Personal = personal
	page, err := searchService.Search(ctx, options)
	if err == nil && guestFallback {
		page.Notices = append(page.Notices, "Modo visitante: a busca pública retornou vídeos; conecte uma conta para incluir canais e playlists.")
	}
	return page, err
}

func (s *AppServices) shouldUseGuestVideoSearch(ctx context.Context, options domain.SearchOptions) (bool, error) {
	normalized := search.NormalizeOptions(options)
	if len(normalized.ResourceTypes) != 3 {
		return false, nil
	}
	resources := make(map[domain.SearchResourceType]bool, len(normalized.ResourceTypes))
	for _, resource := range normalized.ResourceTypes {
		resources[resource] = true
	}
	if !resources[domain.SearchResourceVideo] || !resources[domain.SearchResourceChannel] || !resources[domain.SearchResourcePlaylist] {
		return false, nil
	}
	videoOnly := normalized
	videoOnly.ResourceTypes = []domain.SearchResourceType{domain.SearchResourceVideo}
	if search.RequiresOfficialAPI(videoOnly) {
		return false, nil
	}
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return false, err
	}
	if profile.IsGuest() {
		return true, nil
	}
	_, connected, err := storage.NewAccountRepository(s.Repo.DB(), profile.ID).Account(ctx)
	return !connected, err
}

func (s *AppServices) localSearchState(ctx context.Context) (search.PersonalState, error) {
	profile, err := s.Repo.BuildProfile(ctx)
	if err != nil {
		return search.PersonalState{}, err
	}
	favorites, err := s.Repo.FavoriteIDs(ctx)
	if err != nil {
		return search.PersonalState{}, err
	}
	queuedIDs, err := s.Repo.QueuedIDs(ctx)
	if err != nil {
		return search.PersonalState{}, err
	}
	queued := make(map[string]bool, len(queuedIDs))
	for _, videoID := range queuedIDs {
		queued[videoID] = true
	}
	subscribed := make(map[string]bool)
	for channelID, affinity := range profile.Channels {
		if affinity.Subscribed {
			subscribed[channelID] = true
		}
	}
	return search.PersonalState{
		Progress:           profile.WatchState,
		Favorites:          favorites,
		Queued:             queued,
		SubscribedChannels: subscribed,
		RejectedVideos:     profile.ExcludedVideoIDs,
		RejectedChannels:   profile.ExcludedChannelIDs,
	}, nil
}

// SearchLocalFTS busca diretamente no índice local FTS5.
func (s *AppServices) SearchLocalFTS(ctx context.Context, query string, limit int) ([]domain.SearchHit, error) {
	return s.Repo.Search(ctx, query, limit)
}

// --- PLAYLIST SERVICE ---

// ListPlaylists lista todas as playlists locais.
func (s *AppServices) ListPlaylists(ctx context.Context) ([]domain.Playlist, error) {
	return s.Repo.Playlists(ctx)
}

// GetPlaylist obtém os detalhes e vídeos de uma playlist.
func (s *AppServices) GetPlaylist(ctx context.Context, id string) (PlaylistDetailModel, error) {
	pl, err := s.Repo.Playlist(ctx, id)
	if err != nil {
		return PlaylistDetailModel{}, err
	}
	videos, err := s.Repo.PlaylistVideos(ctx, id)
	if err != nil {
		return PlaylistDetailModel{}, err
	}
	return PlaylistDetailModel{
		Playlist: pl,
		Videos:   videos,
	}, nil
}

// CreatePlaylist cria uma nova playlist manual.
func (s *AppServices) CreatePlaylist(ctx context.Context, name, description, color string) (domain.Playlist, error) {
	return s.Repo.CreatePlaylist(ctx, name, description, color)
}

func (s *AppServices) UpdatePlaylist(ctx context.Context, id, name, description, color string) error {
	return s.Repo.UpdatePlaylist(ctx, id, name, description, color)
}

func (s *AppServices) MergePlaylist(ctx context.Context, sourceID, targetID string) error {
	if sourceID == targetID {
		return fmt.Errorf("playlists: escolha uma playlist de destino diferente")
	}
	videos, err := s.Repo.PlaylistVideos(ctx, sourceID)
	if err != nil {
		return err
	}
	videoIDs := make([]string, 0, len(videos))
	for _, video := range videos {
		videoIDs = append(videoIDs, video.ID)
	}
	if len(videoIDs) == 0 {
		return nil
	}
	return s.Repo.CopyPlaylistItems(ctx, sourceID, targetID, videoIDs)
}

// DeletePlaylist remove uma playlist local.
func (s *AppServices) DeletePlaylist(ctx context.Context, id string) error {
	return s.Repo.DeletePlaylist(ctx, id)
}

// AddVideoToPlaylist adiciona um vídeo à playlist.
func (s *AppServices) AddVideoToPlaylist(ctx context.Context, playlistID, videoID string) error {
	return s.Repo.AddPlaylistItem(ctx, playlistID, videoID)
}

// RemoveVideoFromPlaylist remove um vídeo da playlist.
func (s *AppServices) RemoveVideoFromPlaylist(ctx context.Context, playlistID, videoID string) error {
	return s.Repo.RemovePlaylistItem(ctx, playlistID, videoID)
}

// GetRemotePlaylistPage lista uma página sem persistir nem baixar mídia.
func (s *AppServices) GetRemotePlaylistPage(ctx context.Context, playlistID, pageToken string, limit int) (RemotePlaylistPageModel, error) {
	playlistID = strings.TrimSpace(playlistID)
	if playlistID == "" {
		return RemotePlaylistPageModel{}, fmt.Errorf("playlist remota: ID vazio")
	}
	if limit <= 0 || limit > 50 {
		limit = 24
	}
	videos, nextPageToken, err := s.SearchService.Public.PlaylistItems(ctx, playlistID, domain.PageOptions{
		MaxResults: limit,
		PageToken:  strings.TrimSpace(pageToken),
	})
	if err != nil {
		return RemotePlaylistPageModel{}, err
	}
	return RemotePlaylistPageModel{PlaylistID: playlistID, Videos: videos, NextPageToken: nextPageToken}, nil
}

// ImportRemotePlaylist materializa metadados de uma playlist remota em uma
// playlist local. O limite evita uma importação ilimitada em hardware modesto.
func (s *AppServices) ImportRemotePlaylist(ctx context.Context, playlistID, name, description string, maxItems int) (PlaylistDetailModel, error) {
	playlistID = strings.TrimSpace(playlistID)
	name = strings.TrimSpace(name)
	if playlistID == "" {
		return PlaylistDetailModel{}, fmt.Errorf("importar playlist remota: ID vazio")
	}
	if name == "" {
		name = "Playlist importada " + playlistID
	}
	if maxItems <= 0 {
		maxItems = 200
	} else if maxItems > 500 {
		maxItems = 500
	}

	videos, err := s.fetchRemotePlaylistItems(ctx, playlistID, maxItems)
	if err != nil {
		return PlaylistDetailModel{}, err
	}
	if len(videos) == 0 {
		return PlaylistDetailModel{}, fmt.Errorf("importar playlist remota: playlist vazia")
	}
	if err := s.Repo.UpsertVideos(ctx, videos); err != nil {
		return PlaylistDetailModel{}, fmt.Errorf("importar playlist remota: salvar metadados: %w", err)
	}
	playlist, err := s.Repo.CreatePlaylist(ctx, name, strings.TrimSpace(description), "")
	if err != nil {
		return PlaylistDetailModel{}, err
	}
	for _, video := range videos {
		if err := s.Repo.AddPlaylistItem(ctx, playlist.ID, video.ID); err != nil {
			_ = s.Repo.DeletePlaylist(context.Background(), playlist.ID)
			return PlaylistDetailModel{}, fmt.Errorf("importar playlist remota: adicionar item: %w", err)
		}
	}
	playlist.ItemCount = len(videos)
	return PlaylistDetailModel{Playlist: playlist, Videos: videos}, nil
}

func (s *AppServices) AddRemotePlaylistToPlaylist(ctx context.Context, remoteID, targetID string, maxItems int) (int, error) {
	if strings.TrimSpace(remoteID) == "" || strings.TrimSpace(targetID) == "" {
		return 0, fmt.Errorf("playlist remota e destino são obrigatórios")
	}
	if maxItems <= 0 || maxItems > 500 {
		maxItems = 200
	}
	videos, err := s.fetchRemotePlaylistItems(ctx, strings.TrimSpace(remoteID), maxItems)
	if err != nil {
		return 0, err
	}
	if err := s.Repo.UpsertVideos(ctx, videos); err != nil {
		return 0, err
	}
	existingVideos, err := s.Repo.PlaylistVideos(ctx, targetID)
	if err != nil {
		return 0, err
	}
	existing := make(map[string]bool, len(existingVideos))
	for _, video := range existingVideos {
		existing[video.ID] = true
	}
	videoIDs := make([]string, 0, len(videos))
	added := 0
	for _, video := range videos {
		videoIDs = append(videoIDs, video.ID)
		if !existing[video.ID] {
			added++
		}
	}
	if len(videoIDs) == 0 {
		return 0, nil
	}
	if err := s.Repo.CopyPlaylistItems(ctx, "remote:"+strings.TrimSpace(remoteID), targetID, videoIDs); err != nil {
		return 0, err
	}
	return added, nil
}

func (s *AppServices) fetchRemotePlaylistItems(ctx context.Context, playlistID string, maxItems int) ([]domain.Video, error) {
	videos := make([]domain.Video, 0, min(maxItems, 50))
	seen := make(map[string]bool, maxItems)
	pageToken := ""
	for len(videos) < maxItems {
		pageLimit := min(50, maxItems-len(videos))
		page, err := s.GetRemotePlaylistPage(ctx, playlistID, pageToken, pageLimit)
		if err != nil {
			return nil, fmt.Errorf("importar playlist remota: %w", err)
		}
		countBeforePage := len(videos)
		for _, video := range page.Videos {
			video.ID = strings.TrimSpace(video.ID)
			video.Title = strings.TrimSpace(video.Title)
			if video.ID == "" || video.Title == "" || seen[video.ID] {
				continue
			}
			seen[video.ID] = true
			video.ResourceType = domain.SearchResourceVideo
			videos = append(videos, video)
			if len(videos) == maxItems {
				break
			}
		}
		if len(videos) == countBeforePage || page.NextPageToken == "" || page.NextPageToken == pageToken {
			break
		}
		pageToken = page.NextPageToken
	}
	return videos, nil
}

// --- LIBRARY SERVICE ---

// GetFavorites lista todos os vídeos marcados como favoritos.
func (s *AppServices) GetFavorites(ctx context.Context) ([]domain.Video, error) {
	return s.Repo.FavoriteVideos(ctx)
}

// AddFavorite adiciona um vídeo aos favoritos.
func (s *AppServices) AddFavorite(ctx context.Context, videoID string) error {
	return s.Repo.AddFavorite(ctx, videoID)
}

// RemoveFavorite remove um vídeo dos favoritos.
func (s *AppServices) RemoveFavorite(ctx context.Context, videoID string) error {
	return s.Repo.RemoveFavorite(ctx, videoID)
}

// GetHistory obtém a lista de vídeos assistidos.
func (s *AppServices) GetHistory(ctx context.Context, limit, offset int) ([]domain.Video, error) {
	return s.Repo.HistoryPaged(ctx, limit, offset)
}

// ClearHistory limpa o histórico de reprodução.
func (s *AppServices) ClearHistory(ctx context.Context) error {
	return s.Repo.ClearHistory(ctx)
}

func (s *AppServices) RemoveHistoryItem(ctx context.Context, videoID string) error {
	return s.Repo.MarkUnwatched(ctx, videoID)
}

// GetNote recupera anotações de um vídeo.
func (s *AppServices) GetNote(ctx context.Context, videoID string) (string, error) {
	return s.Repo.Note(ctx, videoID)
}

// SaveNote salva a anotação textual de um vídeo.
func (s *AppServices) SaveNote(ctx context.Context, videoID, content string) error {
	return s.Repo.SaveNote(ctx, videoID, content)
}

// GetBookmarks recupera marcadores de um vídeo.
func (s *AppServices) GetBookmarks(ctx context.Context, videoID string) ([]domain.VideoBookmark, error) {
	return s.Repo.Bookmarks(ctx, videoID)
}

// AddBookmark adiciona um marcador temporal no vídeo.
func (s *AppServices) AddBookmark(ctx context.Context, videoID string, positionMs int64, label string) (domain.VideoBookmark, error) {
	return s.Repo.AddBookmark(ctx, videoID, time.Duration(positionMs)*time.Millisecond, label)
}

// DeleteBookmark remove um marcador.
func (s *AppServices) DeleteBookmark(ctx context.Context, bookmarkID string) error {
	return s.Repo.DeleteBookmark(ctx, bookmarkID)
}

// GetLocalStats calcula estatísticas de uso local.
func (s *AppServices) GetLocalStats(ctx context.Context) (domain.LocalStats, error) {
	return s.Repo.LocalStats(ctx)
}

// --- SETTINGS & DIAGNOSTICS ---

// GetResourceUsage retorna métricas leves de uso de recursos do processo.
func (s *AppServices) GetResourceUsage(ctx context.Context) *diagnostics.ResourceUsage {
	usage := s.resources.Sample(ctx)
	return &usage
}

// GetDiagnostics retorna o relatório de diagnósticos do sistema.
func (s *AppServices) GetDiagnostics(ctx context.Context) *diagnostics.Report {
	return diagnostics.Collect(ctx)
}

// GetPlaybackRuntime expõe somente metadados do runtime instalado; caminhos,
// hashes e qualquer credencial permanecem fora do contrato de UI.
func (s *AppServices) GetPlaybackRuntime(ctx context.Context) (playback.RuntimeStatus, error) {
	if s.Runtime == nil {
		return playback.RuntimeStatus{}, errors.New("gerenciador de runtime indisponível")
	}
	return s.Runtime.Status()
}

func (s *AppServices) ActivatePlaybackRuntime(ctx context.Context, version string) error {
	if s.Runtime == nil {
		return errors.New("gerenciador de runtime indisponível")
	}
	return s.Runtime.Activate(version)
}

func (s *AppServices) RollbackPlaybackRuntime(ctx context.Context) error {
	if s.Runtime == nil {
		return errors.New("gerenciador de runtime indisponível")
	}
	return s.Runtime.Rollback()
}

// GetSettings carrega todas as preferências salvas.
func (s *AppServices) GetSettings(ctx context.Context) (map[string]string, error) {
	settings, err := s.Repo.AllSettings(ctx)
	if err != nil {
		return nil, err
	}
	// The UI keeps one flat map for compatibility. Merge only the active
	// profile's settings into that projection; persistence itself remains
	// profile-scoped and therefore cannot leak on profile switches.
	if profileID, profileErr := s.Repo.ActiveProfileID(ctx); profileErr == nil {
		profileSettings, profileErr := s.Repo.ProfileSettings(ctx, profileID)
		if profileErr != nil {
			return nil, profileErr
		}
		for key, value := range profileSettings {
			settings[key] = value
		}
	}
	// Caminhos de arquivos de cookies são configuração operacional sensível e
	// não pertencem ao contrato genérico consumido pelo frontend.
	delete(settings, storage.SettingCookiesFile)
	return settings, nil
}

// SaveSetting persiste uma preferência.
func (s *AppServices) SaveSetting(ctx context.Context, key, value string) error {
	if !publicSettingKeys[key] {
		return fmt.Errorf("configuração não pode ser alterada por este contrato: %s", key)
	}
	if err := validatePublicSetting(key, value); err != nil {
		return err
	}
	if err := s.Repo.SetSetting(ctx, key, value); err != nil {
		return err
	}
	if isPlaybackRuntimeSetting(key) {
		if err := s.reloadPlaybackResolver(ctx); err != nil {
			return err
		}
	}
	s.notifySettingChanged(key, value)
	return nil
}

// RevokePlayerSession invalida imediatamente os tokens de mídia da sessão atual
// e cancela downloads em curso. Chamado quando o frontend fecha o player ou sai
// do miniplayer, liberando recursos no backend.
func (s *AppServices) RevokePlayerSession(ctx context.Context) error {
	if s.MediaProxy == nil {
		return nil
	}
	s.MediaProxy.Revoke()
	return nil
}

func isPlaybackRuntimeSetting(key string) bool {
	switch key {
	case storage.SettingInvidiousInstance, storage.SettingMaxHeight,
		storage.SettingPlayerClient, storage.SettingRemoteEJS:
		return true
	default:
		return false
	}
}

func (s *AppServices) SubscribeSettingChanges(listener func(string, string)) func() {
	if listener == nil {
		return func() {}
	}
	s.settingListenersMu.Lock()
	s.nextSettingListener++
	id := s.nextSettingListener
	s.settingListeners[id] = listener
	s.settingListenersMu.Unlock()
	return func() {
		s.settingListenersMu.Lock()
		delete(s.settingListeners, id)
		s.settingListenersMu.Unlock()
	}
}

func (s *AppServices) notifySettingChanged(key, value string) {
	s.settingListenersMu.RLock()
	listeners := make([]func(string, string), 0, len(s.settingListeners))
	for _, listener := range s.settingListeners {
		listeners = append(listeners, listener)
	}
	s.settingListenersMu.RUnlock()
	for _, listener := range listeners {
		listener(key, value)
	}
}

func validatePublicSetting(key, value string) error {
	if len(value) > 16*1024 {
		return fmt.Errorf("configuração excede o limite permitido")
	}
	switch key {
	case "locale":
		if value != "pt-BR" && value != "en-US" {
			return fmt.Errorf("idioma inválido")
		}
	case storage.SettingTrayEnabled, storage.SettingRemoteEJS, storage.SettingTVMode:
		if value != "0" && value != "1" {
			return fmt.Errorf("valor booleano inválido para %s", key)
		}
	case storage.SettingMaxHeight:
		height, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || height < 0 || height > 4320 {
			return fmt.Errorf("altura máxima inválida: use 0, 360, 720, 1080, 1440 ou 2160")
		}
	case storage.SettingPlayerClient:
		client := playback.NormalizePlayerClientPreference(value)
		if client != playback.AutoPlayerClient && client != "android" && client != "ios" && client != "mweb" {
			return fmt.Errorf("cliente de reprodução inválido")
		}
	}
	return nil
}

var publicSettingKeys = map[string]bool{
	"accent_color":                   true,
	"close_behavior":                 true,
	"custom_shortcuts":               true,
	"home_page_size":                 true,
	"home_filters":                   true,
	"subscriptions_page_size":        true,
	"subscriptions_filters":          true,
	"search_filters":                 true,
	"sidebar_collapsed":              true,
	"playlist_view_mode":             true,
	"channel_view_mode":              true,
	"channel_filters":                true,
	"library_view_mode":              true,
	"locale":                         true,
	"hide_shorts":                    true,
	"hide_lives":                     true,
	"hide_upcoming":                  true,
	"hide_mixes":                     true,
	"hide_members":                   true,
	"theme":                          true,
	"thumbnail_quality":              true,
	"thumbnail_size":                 true,
	"tray_seek_seconds":              true,
	storage.SettingAudioChannels:     true,
	storage.SettingAudioDevice:       true,
	storage.SettingAudioNorm:         true,
	storage.SettingColorScheme:       true,
	storage.SettingDensity:           true,
	storage.SettingGain:              true,
	storage.SettingInvidiousInstance: true,
	storage.SettingMaxHeight:         true,
	storage.SettingPeriodicRefresh:   true,
	storage.SettingPlaybackProvider:  true,
	storage.SettingPlayerClient:      true,
	storage.SettingRefreshOnStartup:  true,
	storage.SettingRemoteEJS:         true,
	storage.SettingShortcuts:         true,
	storage.SettingSpeed:             true,
	storage.SettingTrayEnabled:       true,
	storage.SettingTrayHideOnClose:   true,
	storage.SettingTrayRefresh:       true,
	storage.SettingTVMode:            true,
}

// ExportPersonalData exporta dados pessoais em formato JSON seguro.
func (s *AppServices) ExportPersonalData(ctx context.Context) (string, error) {
	data, err := backup.ExportJSON(ctx, s.Repo)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ImportPersonalData restaura dados pessoais a partir de um snapshot JSON.
func (s *AppServices) ImportPersonalData(ctx context.Context, jsonData, strategy string) error {
	strat := backup.ImportStrategy(strategy)
	if strat == "" {
		strat = backup.StrategyMerge
	}
	return backup.ImportJSON(ctx, s.Repo, []byte(jsonData), strat)
}

// --- IPTV SERVICE ---

// ListIPTVSources lista todas as fontes IPTV configuradas e seus status.
func (s *AppServices) ListIPTVSources(ctx context.Context) ([]iptv.SourceState, error) {
	return s.Repo.ListIPTVSources(ctx)
}

// SaveIPTVSource salva ou atualiza uma fonte IPTV M3U/M3U8.
func (s *AppServices) SaveIPTVSource(ctx context.Context, id, name, playlistURL, guideURL string, enabled bool) error {
	return s.SaveIPTVSourceWithCredentials(ctx, id, name, playlistURL, guideURL, enabled, "", "", false, "preserve")
}

// SaveIPTVSourceWithCredentials atualiza metadados e credenciais sem expor o
// segredo salvo. Campos de credencial vazios preservam o keyring atual; a
// remoção precisa ser solicitada explicitamente.
func (s *AppServices) SaveIPTVSourceWithCredentials(
	ctx context.Context,
	id, name, playlistURL, guideURL string,
	enabled bool,
	username, password string,
	clearCredentials bool,
	outputMode string,
) error {
	id = strings.TrimSpace(id)
	playlistURL = strings.TrimSpace(playlistURL)
	guideURL = strings.TrimSpace(guideURL)
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if id == "" {
		return fmt.Errorf("salvar fonte IPTV: ID vazio")
	}

	existing, exists, err := s.Repo.GetIPTVSource(ctx, id)
	if err != nil {
		return err
	}
	credentialRef := ""
	if exists {
		credentialRef = existing.CredentialRef
	}

	playlistURL, inlineUsername, inlinePassword, err := prepareEditablePlaylistURL(playlistURL, outputMode)
	if err != nil {
		return err
	}
	if username == "" && password == "" {
		username, password = inlineUsername, inlinePassword
	} else if inlineUsername != "" || inlinePassword != "" {
		return fmt.Errorf("salvar fonte IPTV: use credenciais na URL ou nos campos separados, não nos dois")
	}
	if (username == "") != (password == "") {
		return fmt.Errorf("salvar fonte IPTV: usuário e senha devem ser informados juntos")
	}
	if clearCredentials && (username != "" || password != "") {
		return fmt.Errorf("salvar fonte IPTV: não é possível remover e substituir credenciais ao mesmo tempo")
	}

	var credentialRollback func()
	if username != "" {
		credentialRef = "iptv_cred_" + id
		rollback, err := s.replaceIPTVCredential(ctx, credentialRef, iptv.PlaylistCredentials{
			Username: username,
			Password: password,
		})
		if err != nil {
			return err
		}
		credentialRollback = rollback
	} else if clearCredentials && credentialRef != "" {
		rollback, err := s.removeIPTVCredential(ctx, credentialRef)
		if err != nil {
			return err
		}
		credentialRollback = rollback
		credentialRef = ""
	}

	err = s.Repo.SaveIPTVSource(ctx, iptv.SourceConfig{
		ID:            id,
		Name:          strings.TrimSpace(name),
		Format:        iptv.SourceFormatM3U,
		PlaylistURL:   playlistURL,
		GuideURL:      guideURL,
		CredentialRef: credentialRef,
		Enabled:       enabled,
	})
	if err != nil && credentialRollback != nil {
		credentialRollback()
	}
	return err
}

func prepareEditablePlaylistURL(value, outputMode string) (string, string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", "", fmt.Errorf("salvar fonte IPTV: URL HTTP(S) inválida")
	}
	query := parsed.Query()
	username := firstNonEmpty(query.Get("username"), query.Get("user"))
	password := firstNonEmpty(query.Get("password"), query.Get("pass"))
	if parsed.User != nil {
		userInfoPassword, hasPassword := parsed.User.Password()
		if username == "" {
			username = parsed.User.Username()
		}
		if password == "" && hasPassword {
			password = userInfoPassword
		}
	}
	if (strings.TrimSpace(username) == "") != (strings.TrimSpace(password) == "") {
		return "", "", "", fmt.Errorf("salvar fonte IPTV: credenciais incompletas na URL")
	}

	for key := range query {
		switch strings.ToLower(key) {
		case "username", "user", "password", "pass":
			query.Del(key)
		case "token", "auth", "key", "api_key", "apikey", "credential":
			return "", "", "", fmt.Errorf("salvar fonte IPTV: credencial por token ainda não é suportada; use usuário e senha")
		}
	}
	switch strings.ToLower(strings.TrimSpace(outputMode)) {
	case "", "preserve":
	case "hls":
		query.Set("output", "m3u8")
	default:
		return "", "", "", fmt.Errorf("salvar fonte IPTV: opção de saída inválida")
	}
	parsed.User = nil
	parsed.RawQuery = query.Encode()
	return parsed.String(), strings.TrimSpace(username), strings.TrimSpace(password), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *AppServices) replaceIPTVCredential(ctx context.Context, reference string, credentials iptv.PlaylistCredentials) (func(), error) {
	var previous []byte
	if s.SecretStore != nil {
		previous, _ = s.SecretStore.Get(ctx, domain.SecretKey(reference))
	}
	if err := iptv.StorePlaylistCredentials(ctx, s.SecretStore, reference, credentials); err != nil {
		return nil, err
	}
	return func() {
		if len(previous) > 0 {
			_ = s.SecretStore.Set(context.Background(), domain.SecretKey(reference), previous)
			return
		}
		_ = s.SecretStore.Delete(context.Background(), domain.SecretKey(reference))
	}, nil
}

func (s *AppServices) removeIPTVCredential(ctx context.Context, reference string) (func(), error) {
	var previous []byte
	if s.SecretStore != nil {
		previous, _ = s.SecretStore.Get(ctx, domain.SecretKey(reference))
	}
	if err := iptv.DeletePlaylistCredentials(ctx, s.SecretStore, reference); err != nil {
		return nil, err
	}
	return func() {
		if len(previous) > 0 {
			_ = s.SecretStore.Set(context.Background(), domain.SecretKey(reference), previous)
		}
	}, nil
}

// DiagnoseIPTVEndpoint inspeciona URL, DNS e conectividade sem baixar a lista.
func (s *AppServices) DiagnoseIPTVEndpoint(ctx context.Context, endpoint string) (iptv.EndpointDiagnostic, error) {
	if ctx == nil {
		return iptv.EndpointDiagnostic{}, fmt.Errorf("diagnosticar IPTV: contexto nil")
	}
	return iptv.DiagnoseEndpoint(ctx, endpoint), nil
}

// DeleteIPTVSource remove uma fonte IPTV.
func (s *AppServices) DeleteIPTVSource(ctx context.Context, id string) error {
	source, exists, err := s.Repo.GetIPTVSource(ctx, id)
	if err != nil {
		return err
	}
	var credentialRollback func()
	if exists && source.CredentialRef != "" {
		credentialRollback, err = s.removeIPTVCredential(ctx, source.CredentialRef)
		if err != nil {
			return err
		}
	}
	if err := s.Repo.DeleteIPTVSource(ctx, id); err != nil {
		if credentialRollback != nil {
			credentialRollback()
		}
		return err
	}
	return nil
}

// SyncIPTVSource sincroniza a playlist M3U e o guia EPG de uma fonte IPTV.
func (s *AppServices) SyncIPTVSource(ctx context.Context, sourceID string) (int, error) {
	source, ok, err := s.Repo.GetIPTVSource(ctx, sourceID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("fonte IPTV %q não encontrada", sourceID)
	}

	m3uSource := s.configuredIPTVM3USource()
	catalogSync := iptv.CatalogSync{
		Source: m3uSource,
		Store:  s.Repo,
	}

	_, err = catalogSync.SyncM3U(ctx, source)
	if err != nil {
		_ = s.Repo.SetIPTVSourceError(ctx, sourceID, err.Error())
		return 0, err
	}

	// Sincroniza EPG XMLTV se configurado
	if source.GuideURL != "" {
		guideSync := iptv.GuideSync{
			Source: iptv.HTTPXMLTVSource{},
			Store:  s.Repo,
		}
		_ = guideSync.SyncXMLTV(ctx, source)
	}

	page, err := s.Repo.ListIPTVItemsPaginated(ctx, iptv.ItemFilter{
		SourceID: sourceID,
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		return 0, fmt.Errorf("contar catálogo IPTV sincronizado: %w", err)
	}
	return page.TotalCount, nil
}

// ListIPTVItemsPaginated retorna itens paginados do catálogo IPTV com filtros.
func (s *AppServices) ListIPTVItemsPaginated(ctx context.Context, filter iptv.ItemFilter) (iptv.PageResult, error) {
	return s.Repo.ListIPTVItemsPaginated(ctx, filter)
}

// ListIPTVGroups lista categorias e grupos presentes no catálogo IPTV.
func (s *AppServices) ListIPTVGroups(ctx context.Context, kind string) ([]string, error) {
	return s.Repo.ListIPTVGroups(ctx, iptv.ContentKind(kind))
}

// ListIPTVGuide lista programas do guia EPG para a janela de tempo atual.
func (s *AppServices) ListIPTVGuide(ctx context.Context, sourceID string, limit int) ([]iptv.GuideEntry, error) {
	now := time.Now().UTC()
	return s.Repo.ListIPTVGuide(ctx, sourceID, now, now.Add(24*time.Hour), limit)
}

// SetIPTVItemSaved salva ou desfaz o salvamento de um canal ou título IPTV.
func (s *AppServices) SetIPTVItemSaved(ctx context.Context, itemID string, saved bool) error {
	return s.Repo.SetIPTVItemSaved(ctx, itemID, saved)
}

// ListIPTVSavedItems lista itens marcados como favoritos no IPTV.
func (s *AppServices) ListIPTVSavedItems(ctx context.Context, limit int) ([]iptv.Item, error) {
	return s.Repo.ListIPTVSavedItems(ctx, limit)
}

// ResolveIPTVStream resolve o stream de um item IPTV e devolve um PlaybackPlan para o player web.
func (s *AppServices) ResolveIPTVStream(ctx context.Context, sourceID, itemID string) (domain.PlaybackPlan, error) {
	source, ok, err := s.Repo.GetIPTVSource(ctx, sourceID)
	if err != nil {
		return domain.PlaybackPlan{}, err
	}
	if !ok {
		return domain.PlaybackPlan{}, fmt.Errorf("fonte IPTV %q não encontrada", sourceID)
	}
	item, ok, err := s.Repo.GetIPTVItem(ctx, sourceID, itemID)
	if err != nil {
		return domain.PlaybackPlan{}, err
	}
	if !ok {
		return domain.PlaybackPlan{}, fmt.Errorf("item IPTV %q não encontrado", itemID)
	}

	resolver := iptv.M3UStreamResolver{Source: s.configuredIPTVM3USource()}

	streamURL, err := resolver.ResolveFirst(ctx, source, iptv.ParseOptions{}, func(candidate iptv.Item) bool {
		return iptv.SameCatalogIdentity(candidate, item)
	})
	if err != nil {
		return domain.PlaybackPlan{}, fmt.Errorf("resolver stream IPTV: %w", err)
	}

	return domain.PlaybackPlan{
		Mode: domain.PlaybackModeResolvedMedia,
		Primary: domain.ResolvedStream{
			URL: streamURL,
		},
		Metadata: domain.PlaybackMetadata{
			VideoID: itemID,
		},
	}, nil
}

func (s *AppServices) configuredIPTVM3USource() iptv.HTTPM3USource {
	source := s.IPTVM3USource
	if source.Credentials == nil {
		source.Credentials = s.SecretStore
	}
	return source
}

// SaveIPTVPlaybackProgress salva o progresso de visualização de conteúdo VOD IPTV.
func (s *AppServices) SaveIPTVPlaybackProgress(ctx context.Context, itemID string, positionMs, durationMs int64, completed bool) error {
	return s.Repo.SetIPTVProgress(ctx, iptv.PlaybackPosition{
		ItemID:    itemID,
		Position:  time.Duration(positionMs) * time.Millisecond,
		Duration:  time.Duration(durationMs) * time.Millisecond,
		Completed: completed,
		UpdatedAt: time.Now().UTC(),
	})
}

// ListIPTVResume lista títulos IPTV em andamento para a linha Continuar Assistindo.
func (s *AppServices) ListIPTVResume(ctx context.Context, limit int) ([]iptv.ResumeEntry, error) {
	return s.Repo.ListIPTVResumeItems(ctx, limit)
}

// --- ACCOUNT / AUTH SERVICE ---

// GetAccount recupera os dados da conta conectada.
func (s *AppServices) GetAccount(ctx context.Context) (domain.AccountInfo, bool, error) {
	if err := s.ensureProfileStorageReady(); err != nil {
		return domain.AccountInfo{}, false, err
	}
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return domain.AccountInfo{}, false, err
	}
	account, ok, err := storage.NewAccountRepository(s.Repo.DB(), profile.ID).Account(ctx)
	if err != nil || !ok {
		return account, ok, err
	}
	s.assessAccountCredential(ctx, &account)
	return account, true, nil
}

func (s *AppServices) ListProfiles(ctx context.Context) ([]domain.Profile, error) {
	if err := s.ensureProfileStorageReady(); err != nil {
		return nil, err
	}
	return s.Repo.Profiles(ctx)
}

func (s *AppServices) GetActiveProfile(ctx context.Context) (domain.Profile, error) {
	if err := s.ensureProfileStorageReady(); err != nil {
		return domain.Profile{}, err
	}
	return s.Repo.ActiveProfile(ctx)
}

func (s *AppServices) CreateProfile(ctx context.Context, name string) (domain.Profile, error) {
	if err := s.ensureProfileStorageReady(); err != nil {
		return domain.Profile{}, err
	}
	profile, err := s.Repo.CreateProfile(ctx, name)
	if err != nil {
		return domain.Profile{}, err
	}
	if err := s.SetActiveProfile(ctx, profile.ID); err != nil {
		_ = s.Repo.DeleteProfile(ctx, profile.ID)
		return domain.Profile{}, err
	}
	return profile, nil
}

func (s *AppServices) CreateGuestProfile(ctx context.Context, name string) (domain.Profile, error) {
	if err := s.ensureProfileStorageReady(); err != nil {
		return domain.Profile{}, err
	}
	profile, err := s.Repo.CreateGuestProfile(ctx, name)
	if err != nil {
		return domain.Profile{}, err
	}
	if err := s.SetActiveProfile(ctx, profile.ID); err != nil {
		_ = s.Repo.DeleteProfile(ctx, profile.ID)
		return domain.Profile{}, err
	}
	return profile, nil
}

func (s *AppServices) SetActiveProfile(ctx context.Context, profileID string) error {
	if err := s.ensureProfileStorageReady(); err != nil {
		return err
	}
	if err := s.Repo.SetActiveProfile(ctx, profileID); err != nil {
		return err
	}
	// Rebuild the resolver immediately so a profile switch cannot retain the
	// previous profile's browser-cookie source in the active playback chain.
	if err := s.reloadPlaybackResolver(ctx); err != nil {
		return err
	}
	account, connected, err := storage.NewAccountRepository(s.Repo.DB(), profileID).Account(ctx)
	if err != nil {
		return err
	}
	status := LoginStatus{State: "idle", ProfileID: profileID}
	if connected {
		s.assessAccountCredential(ctx, &account)
		status.Warning = account.Warning
		switch account.CredentialState {
		case domain.CredentialStateAvailable, domain.CredentialStateMemory:
			status.State = "connected"
		default:
			status.State = "error"
			status.Error = account.Warning
		}
	}
	s.loginMu.Lock()
	s.OAuthSession = s.profileSessions[profileID]
	s.loginStatuses[profileID] = status
	s.loginMu.Unlock()
	return nil
}

func (s *AppServices) assessAccountCredential(ctx context.Context, account *domain.AccountInfo) {
	if account == nil {
		return
	}
	if account.SessionPersistence == domain.SessionPersistenceMemory {
		account.CredentialState = domain.CredentialStateMemory
		account.Warning = auth.MemorySessionWarning
		return
	}
	if s.SecretStore == nil {
		account.CredentialState = domain.CredentialStateUnavailable
		account.Warning = auth.KeyringUnavailableWarning
		return
	}
	value, err := s.SecretStore.Get(ctx, auth.RefreshTokenKey(account.ProfileID))
	switch {
	case err == nil && len(value) > 0:
		account.CredentialState = domain.CredentialStateAvailable
	case err == nil || errors.Is(err, domain.ErrSecretNotFound):
		account.CredentialState = domain.CredentialStateMissing
		account.Warning = auth.MissingSessionWarning
	case errors.Is(err, domain.ErrSecretStoreUnavailable):
		account.CredentialState = domain.CredentialStateUnavailable
		account.Warning = auth.KeyringUnavailableWarning
	default:
		account.CredentialState = domain.CredentialStateUnavailable
		account.Warning = "Não foi possível verificar a credencial salva desta conta."
	}
}

func (s *AppServices) DeleteProfile(ctx context.Context, profileID string) error {
	if err := s.ensureProfileStorageReady(); err != nil {
		return err
	}
	if profileID == storage.DefaultProfileID {
		return fmt.Errorf("o perfil padrão não pode ser removido")
	}
	profile, err := s.Repo.Profile(ctx, profileID)
	if err != nil {
		return err
	}
	s.loginCommitMu.Lock()
	defer s.loginCommitMu.Unlock()
	s.loginMu.Lock()
	s.loginGenerations[profileID]++
	s.loginMu.Unlock()
	session := s.existingSession(profile.ID)
	if session == nil {
		session = s.sessionForProfile(auth.Config{}, profile)
	}
	if err := session.Clear(ctx); err != nil {
		return fmt.Errorf("remover credencial do perfil: %w", err)
	}
	if err := s.Repo.DeleteProfile(ctx, profileID); err != nil {
		return err
	}
	s.loginMu.Lock()
	delete(s.profileSessions, profileID)
	delete(s.loginStatuses, profileID)
	if s.OAuthSession != nil && s.OAuthSession.ProfileID() == profileID {
		s.OAuthSession = nil
	}
	s.loginMu.Unlock()
	active, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return err
	}
	return s.SetActiveProfile(ctx, active.ID)
}

// GetLoginStatus returns the state of the current asynchronous login attempt.
func (s *AppServices) GetLoginStatus(ctx context.Context) LoginStatus {
	if err := s.ensureProfileStorageReady(); err != nil {
		return LoginStatus{
			State: "error", Warning: "Dados temporários de perfil estão em quarentena.",
			Error: "a limpeza segura de perfis não foi concluída",
		}
	}
	profileID, err := s.Repo.ActiveProfileID(ctx)
	if err != nil {
		return LoginStatus{State: "error", Error: "não foi possível determinar o perfil ativo"}
	}
	s.loginMu.RLock()
	defer s.loginMu.RUnlock()
	status, ok := s.loginStatuses[profileID]
	if !ok {
		return LoginStatus{State: "idle", ProfileID: profileID}
	}
	return status
}

func (s *AppServices) beginLoginAttempt(profileID string) uint64 {
	s.loginCommitMu.Lock()
	defer s.loginCommitMu.Unlock()
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	s.loginGenerations[profileID]++
	generation := s.loginGenerations[profileID]
	s.loginStatuses[profileID] = LoginStatus{State: "pending", ProfileID: profileID}
	return generation
}

func (s *AppServices) setLoginStatusForAttempt(state, profileID string, generation uint64, warning string, err error) {
	status := LoginStatus{State: state, ProfileID: profileID, Warning: warning}
	if err != nil {
		status.Error = err.Error()
	}
	s.loginMu.Lock()
	if s.loginGenerations[profileID] == generation {
		s.loginStatuses[profileID] = status
	}
	s.loginMu.Unlock()
}

// DisconnectAccount remove as credenciais e desconecta a conta.
func (s *AppServices) DisconnectAccount(ctx context.Context) error {
	if err := s.ensureProfileStorageReady(); err != nil {
		return err
	}
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return err
	}
	s.loginCommitMu.Lock()
	defer s.loginCommitMu.Unlock()
	s.loginMu.Lock()
	s.loginGenerations[profile.ID]++
	s.loginMu.Unlock()
	session := s.existingSession(profile.ID)
	if session == nil {
		session = s.sessionForProfile(auth.Config{}, profile)
	}
	if err := session.Clear(ctx); err != nil {
		return fmt.Errorf("remover credencial da conta: %w", err)
	}
	accountRepo := storage.NewAccountRepository(s.Repo.DB(), profile.ID)
	if err := accountRepo.ClearAccount(ctx); err != nil {
		return fmt.Errorf("remover metadados da conta: %w", err)
	}
	s.loginMu.Lock()
	delete(s.profileSessions, profile.ID)
	if s.OAuthSession != nil && s.OAuthSession.ProfileID() == profile.ID {
		s.OAuthSession = nil
	}
	s.loginStatuses[profile.ID] = LoginStatus{State: "idle", ProfileID: profile.ID}
	s.loginMu.Unlock()
	s.auditCredential(ctx, profile.ID, "google", "revoked", "Conta desconectada pelo usuário")
	return nil
}

// StartGoogleLogin inicia o fluxo OAuth2 loopback e abre o navegador para autenticação.
func (s *AppServices) StartGoogleLogin(ctx context.Context) (string, error) {
	if err := s.ensureProfileStorageReady(); err != nil {
		return "", err
	}
	profile, err := s.Repo.ActiveProfile(ctx)
	if err != nil {
		return "", err
	}
	cfg, err := auth.LoadConfig(ctx)
	if err != nil {
		s.auditCredential(ctx, profile.ID, "google", "verification_failed", "Configuração OAuth indisponível")
		return "", err
	}
	flow, err := auth.StartFlow(cfg)
	if err != nil {
		return "", fmt.Errorf("iniciar fluxo OAuth: %w", err)
	}

	authURL := flow.AuthURL()
	if err := auth.OpenBrowser(context.Background(), authURL); err != nil {
		flow.Cancel()
		return "", fmt.Errorf("abrir navegador para autenticação: %w", err)
	}
	generation := s.beginLoginAttempt(profile.ID)

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		tok, err := flow.AwaitToken(bgCtx)
		if err != nil {
			s.setLoginStatusForAttempt("error", profile.ID, generation, "", fmt.Errorf("autorização no navegador não foi concluída"))
			s.auditCredential(bgCtx, profile.ID, "google", "verification_failed", "Autorização OAuth não concluída")
			return
		}
		s.completeLogin(bgCtx, cfg, profile, generation, tok)
	}()

	return authURL, nil
}

func (s *AppServices) ensureProfileStorageReady() error {
	if s.profileBootstrapErr != nil {
		return fmt.Errorf("dados de perfil em quarentena: limpeza segura pendente: %w", s.profileBootstrapErr)
	}
	return nil
}

func (s *AppServices) sessionForProfile(cfg auth.Config, profile domain.Profile) *auth.Session {
	return auth.NewSessionForProfile(
		cfg,
		s.SecretStore,
		storage.NewAccountRepository(s.Repo.DB(), profile.ID),
		profile.ID,
		profile.IsGuest(),
	)
}

func (s *AppServices) existingSession(profileID string) *auth.Session {
	s.loginMu.RLock()
	defer s.loginMu.RUnlock()
	return s.profileSessions[profileID]
}

func (s *AppServices) completeLogin(ctx context.Context, cfg auth.Config, profile domain.Profile, generation uint64, tok *oauth2.Token) {
	identity, err := s.identityFetcher(ctx, auth.AuthenticatedClient(ctx, oauth2.StaticTokenSource(tok)))
	if err != nil {
		s.setLoginStatusForAttempt("error", profile.ID, generation, "", fmt.Errorf("não foi possível confirmar a identidade da conta"))
		s.auditCredential(ctx, profile.ID, "google", "verification_failed", "Identidade OAuth não confirmada")
		return
	}

	s.loginCommitMu.Lock()
	defer s.loginCommitMu.Unlock()
	s.loginMu.RLock()
	currentGeneration := s.loginGenerations[profile.ID]
	s.loginMu.RUnlock()
	if currentGeneration != generation {
		return
	}

	previousToken, previousTokenExists, err := s.snapshotRefreshToken(ctx, profile)
	if err != nil {
		s.setLoginStatusForAttempt("error", profile.ID, generation, "", fmt.Errorf("não foi possível preparar a atualização segura da sessão"))
		s.auditCredential(ctx, profile.ID, "google", "verification_failed", "Preparação segura da sessão falhou")
		return
	}
	session := s.sessionForProfile(cfg, profile)
	persistence, err := session.StoreRefreshToken(ctx, tok)
	if err != nil {
		s.setLoginStatusForAttempt("error", profile.ID, generation, "", fmt.Errorf("não foi possível armazenar a sessão OAuth"))
		s.auditCredential(ctx, profile.ID, "google", "verification_failed", "Persistência segura da sessão falhou")
		return
	}
	warning := ""
	if persistence == domain.SessionPersistenceMemory {
		warning = auth.MemorySessionWarning
	}
	if err := session.SaveAccount(ctx, domain.AccountInfo{
		Provider:           identity.Provider,
		ProviderSubject:    identity.Subject,
		Email:              identity.Email,
		ConnectedAt:        auth.NowRFC3339(),
		SessionPersistence: persistence,
	}); err != nil {
		rollbackErr := s.rollbackRefreshToken(ctx, profile, persistence, previousToken, previousTokenExists)
		if rollbackErr != nil {
			s.setLoginStatusForAttempt("error", profile.ID, generation, warning, fmt.Errorf("não foi possível salvar a conta nem restaurar a sessão anterior"))
			s.auditCredential(ctx, profile.ID, "google", "verification_failed", "Persistência e rollback da sessão falharam")
			return
		}
		s.setLoginStatusForAttempt("error", profile.ID, generation, warning, fmt.Errorf("não foi possível salvar o estado da conta"))
		s.auditCredential(ctx, profile.ID, "google", "verification_failed", "Metadados da conta não foram persistidos")
		return
	}
	s.loginMu.Lock()
	activeProfileID, _ := s.Repo.ActiveProfileID(ctx)
	s.profileSessions[profile.ID] = session
	if activeProfileID == profile.ID {
		s.OAuthSession = session
	}
	s.loginStatuses[profile.ID] = LoginStatus{State: "connected", ProfileID: profile.ID, Warning: warning}
	s.loginMu.Unlock()
	action := "connected"
	if previousTokenExists {
		action = "rotated"
	}
	s.auditCredential(ctx, profile.ID, "google", action, "Sessão OAuth autorizada")
}

func (s *AppServices) snapshotRefreshToken(ctx context.Context, profile domain.Profile) ([]byte, bool, error) {
	if profile.IsGuest() || s.SecretStore == nil {
		return nil, false, nil
	}
	value, err := s.SecretStore.Get(ctx, auth.RefreshTokenKey(profile.ID))
	if err == nil {
		return append([]byte(nil), value...), len(value) > 0, nil
	}
	if errors.Is(err, domain.ErrSecretNotFound) || errors.Is(err, domain.ErrSecretStoreUnavailable) {
		return nil, false, nil
	}
	return nil, false, err
}

func (s *AppServices) rollbackRefreshToken(
	ctx context.Context,
	profile domain.Profile,
	persistence domain.SessionPersistence,
	previousToken []byte,
	previousTokenExists bool,
) error {
	if profile.IsGuest() || persistence == domain.SessionPersistenceMemory || s.SecretStore == nil {
		return nil
	}
	key := auth.RefreshTokenKey(profile.ID)
	if previousTokenExists {
		return s.SecretStore.Set(ctx, key, previousToken)
	}
	return s.SecretStore.Delete(ctx, key)
}
