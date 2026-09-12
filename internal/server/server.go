// Package server provê o servidor HTTP local e ponte RPC para o frontend SPA.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/iptv"
	"github.com/nanotube/nanotube-web/internal/product"
	"github.com/nanotube/nanotube-web/internal/services"
)

// Server encapsulates the HTTP API and static file serving for NanoTube Web.
type Server struct {
	services   *services.AppServices
	httpServer *http.Server
	listener   net.Listener
	staticFS   fs.FS
	staticDir  string
	allowedRPC map[string]map[string]struct{}
}

// Config controls server listener and static assets.
type Config struct {
	Addr      string
	StaticDir string
	StaticFS  fs.FS
	// AllowedRPCMethods restringe a superfície pública por produto. Vazio
	// mantém todos os pares conhecidos para testes e ferramentas internas.
	AllowedRPCMethods map[string][]string
}

// NewServer creates a server instance bound to the application services.
func NewServer(svc *services.AppServices, cfg Config) *Server {
	s := &Server{
		services:   svc,
		staticFS:   cfg.StaticFS,
		staticDir:  cfg.StaticDir,
		allowedRPC: compileRPCPolicy(cfg.AllowedRPCMethods),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/rpc", s.handleRPC)
	mux.HandleFunc("/api/media/", s.handleMedia)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/", s.handleStatic)

	s.httpServer = &http.Server{
		Addr:        cfg.Addr,
		Handler:     s.Handler(),
		ReadTimeout: 30 * time.Second,
		// Streams de mídia podem durar horas; um WriteTimeout global cortaria
		// playback contínuo. O proxy limita headers/upstream e o ReadTimeout
		// continua protegendo a leitura lenta das requisições.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Handler returns the HTTP handler for embedding into custom servers or Wails AssetOptions.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/rpc", s.handleRPC)
	mux.HandleFunc("/api/media/", s.handleMedia)
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/", s.handleStatic)
	return s.corsMiddleware(mux)
}

// Start begins listening on the configured address.
func (s *Server) Start(ctx context.Context) (string, error) {
	addr := s.httpServer.Addr
	if addr == "" {
		addr = "127.0.0.1:0"
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("listen %s: %w", addr, err)
	}
	s.listener = l

	go func() {
		_ = s.httpServer.Serve(l)
	}()

	url := fmt.Sprintf("http://%s", l.Addr().String())
	return url, nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// URL returns the bound listener URL, or empty if not started.
func (s *Server) URL() string {
	if s.listener != nil {
		return fmt.Sprintf("http://%s", s.listener.Addr().String())
	}
	return ""
}

type rpcRequest struct {
	Service string        `json:"service"`
	Method  string        `json:"method"`
	Args    []interface{} `json:"args"`
}

type rpcResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, rpcResponse{Error: "JSON inválido: " + err.Error()})
		return
	}

	ctx := r.Context()
	if !product.ServiceAllowsMethod(req.Service, req.Method) {
		writeJSON(w, http.StatusOK, rpcResponse{Error: fmt.Sprintf("RPC não reconhecida: %s.%s", req.Service, req.Method)})
		return
	}
	if !s.rpcAllowed(req.Service, req.Method) {
		writeJSON(w, http.StatusOK, rpcResponse{Error: fmt.Sprintf("RPC indisponível neste aplicativo: %s.%s", req.Service, req.Method)})
		return
	}
	result, err := s.dispatch(ctx, req.Method, req.Args)
	if err != nil {
		writeJSON(w, http.StatusOK, rpcResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, rpcResponse{Result: result})
}

func compileRPCPolicy(policy map[string][]string) map[string]map[string]struct{} {
	if len(policy) == 0 {
		return nil
	}
	compiled := make(map[string]map[string]struct{}, len(policy))
	for service, methods := range policy {
		compiled[service] = make(map[string]struct{}, len(methods))
		for _, method := range methods {
			compiled[service][method] = struct{}{}
		}
	}
	return compiled
}

func (s *Server) rpcAllowed(service, method string) bool {
	if s.allowedRPC == nil {
		return true
	}
	_, allowed := s.allowedRPC[service][method]
	return allowed
}

func (s *Server) dispatch(ctx context.Context, method string, args []interface{}) (interface{}, error) {
	switch method {
	// Catalog
	case "GetHome":
		profileID := getStringArg(args, 0)
		return s.services.GetHome(ctx, profileID)
	case "RefreshSubscriptions":
		return s.services.RefreshSubscriptions(ctx)
	case "GetChannels":
		return s.services.GetChannels(ctx)
	case "GetRemoteChannelVideosPage":
		channelID := getStringArg(args, 0)
		pageToken := getStringArg(args, 1)
		limit := getIntArg(args, 2, 24)
		return s.services.GetRemoteChannelVideosPage(ctx, channelID, pageToken, limit)
	case "RememberVideo":
		var video domain.Video
		if len(args) == 0 {
			return nil, fmt.Errorf("vídeo ausente")
		}
		raw, err := json.Marshal(args[0])
		if err != nil {
			return nil, fmt.Errorf("codificar vídeo: %w", err)
		}
		if err := json.Unmarshal(raw, &video); err != nil {
			return nil, fmt.Errorf("vídeo inválido: %w", err)
		}
		return nil, s.services.RememberVideo(ctx, video)
	case "SubscribeChannel":
		id := getStringArg(args, 0)
		title := getStringArg(args, 1)
		return nil, s.services.SubscribeChannel(ctx, id, title)
	case "UnsubscribeChannel":
		id := getStringArg(args, 0)
		return nil, s.services.UnsubscribeChannel(ctx, id)
	case "GetChannelFolders":
		return s.services.GetChannelFolders(ctx)
	case "CreateChannelFolder":
		name := getStringArg(args, 0)
		return s.services.CreateChannelFolder(ctx, name)
	case "RenameChannelFolder":
		id := getStringArg(args, 0)
		name := getStringArg(args, 1)
		return nil, s.services.RenameChannelFolder(ctx, id, name)
	case "DeleteChannelFolder":
		id := getStringArg(args, 0)
		return nil, s.services.DeleteChannelFolder(ctx, id)
	case "AddChannelFavorite":
		id := getStringArg(args, 0)
		return nil, s.services.AddChannelFavorite(ctx, id)
	case "RemoveChannelFavorite":
		id := getStringArg(args, 0)
		return nil, s.services.RemoveChannelFavorite(ctx, id)
	case "GetFavoriteChannelIDs":
		return s.services.GetFavoriteChannelIDs(ctx)
	case "GetFolderMembership":
		return s.services.GetFolderMembership(ctx)
	case "ListSubscriptionVideos":
		var query domain.SubscriptionVideoQuery
		if err := decodeRPCArg(args, 0, &query); err != nil {
			return nil, fmt.Errorf("inscrições: consulta inválida: %w", err)
		}
		return s.services.ListSubscriptionVideos(ctx, query)
	case "ListManagedChannels":
		return s.services.ListManagedChannels(ctx)
	case "BulkUnsubscribeChannels":
		var channelIDs []string
		if err := decodeRPCArg(args, 0, &channelIDs); err != nil {
			return nil, fmt.Errorf("canais: seleção inválida: %w", err)
		}
		return nil, s.services.BulkUnsubscribeChannels(ctx, channelIDs)
	case "BulkFavoriteChannels":
		var channelIDs []string
		if err := decodeRPCArg(args, 0, &channelIDs); err != nil {
			return nil, fmt.Errorf("canais: seleção inválida: %w", err)
		}
		return nil, s.services.BulkFavoriteChannels(ctx, channelIDs)
	case "BulkAddChannelsToFolder":
		folderID := getStringArg(args, 0)
		var channelIDs []string
		if err := decodeRPCArg(args, 1, &channelIDs); err != nil {
			return nil, fmt.Errorf("canais: seleção inválida: %w", err)
		}
		return nil, s.services.BulkAddChannelsToFolder(ctx, folderID, channelIDs)
	case "SetChannelTags":
		channelID := getStringArg(args, 0)
		var tags []string
		if err := decodeRPCArg(args, 1, &tags); err != nil {
			return nil, fmt.Errorf("canais: tags inválidas: %w", err)
		}
		return nil, s.services.SetChannelTags(ctx, channelID, tags)

	// Player
	case "ResolveMedia":
		videoID := getStringArg(args, 0)
		sourceURL := getStringArg(args, 1)
		return s.services.ResolveMedia(ctx, videoID, sourceURL)
	case "SaveProgress":
		videoID := getStringArg(args, 0)
		posMs := getInt64Arg(args, 1)
		durMs := getInt64Arg(args, 2)
		completed := getBoolArg(args, 3)
		return nil, s.services.SaveProgress(ctx, videoID, posMs, durMs, completed)

	// Queue
	case "GetQueue":
		return s.services.GetQueue(ctx)
	case "EnqueueVideo":
		var video domain.Video
		if err := decodeRPCArg(args, 0, &video); err != nil {
			return nil, fmt.Errorf("fila: vídeo inválido: %w", err)
		}
		return s.services.EnqueueVideo(ctx, video)
	case "ReorderQueue":
		var itemIDs []string
		if err := decodeRPCArg(args, 0, &itemIDs); err != nil {
			return nil, fmt.Errorf("fila: ordem inválida: %w", err)
		}
		return nil, s.services.ReorderQueue(ctx, itemIDs)
	case "RemoveQueueItem":
		return nil, s.services.RemoveQueueItem(ctx, getStringArg(args, 0))
	case "MarkQueueItemPlayed":
		return nil, s.services.MarkQueueItemPlayed(ctx, getStringArg(args, 0), getBoolArg(args, 1))
	case "ClearQueue":
		return nil, s.services.ClearQueue(ctx, getStringArg(args, 0))
	case "SaveQueuePreferences":
		var preferences domain.QueuePreferences
		if err := decodeRPCArg(args, 0, &preferences); err != nil {
			return nil, fmt.Errorf("fila: preferências inválidas: %w", err)
		}
		return nil, s.services.SaveQueuePreferences(ctx, preferences)
	case "SaveQueueAsPlaylist":
		return s.services.SaveQueueAsPlaylist(ctx, getStringArg(args, 0))

	// Search
	case "Search":
		if len(args) > 0 {
			if _, isLegacy := args[0].(string); !isLegacy {
				var request domain.SearchOptions
				if err := decodeRPCArg(args, 0, &request); err != nil {
					return nil, fmt.Errorf("busca: request inválido: %w", err)
				}
				return s.services.SearchAdvanced(ctx, request)
			}
		}
		query := getStringArg(args, 0)
		limit := getIntArg(args, 1, 20)
		offset := getIntArg(args, 2, 0)
		return s.services.Search(ctx, query, limit, offset)
	case "SearchLocalFTS":
		query := getStringArg(args, 0)
		limit := getIntArg(args, 1, 20)
		return s.services.SearchLocalFTS(ctx, query, limit)

	// Playlists
	case "ListPlaylists":
		return s.services.ListPlaylists(ctx)
	case "GetPlaylist":
		id := getStringArg(args, 0)
		return s.services.GetPlaylist(ctx, id)
	case "CreatePlaylist":
		name := getStringArg(args, 0)
		desc := getStringArg(args, 1)
		color := getStringArg(args, 2)
		return s.services.CreatePlaylist(ctx, name, desc, color)
	case "UpdatePlaylist":
		return nil, s.services.UpdatePlaylist(ctx, getStringArg(args, 0), getStringArg(args, 1), getStringArg(args, 2), getStringArg(args, 3))
	case "MergePlaylist":
		return nil, s.services.MergePlaylist(ctx, getStringArg(args, 0), getStringArg(args, 1))
	case "DeletePlaylist":
		id := getStringArg(args, 0)
		return nil, s.services.DeletePlaylist(ctx, id)
	case "AddVideoToPlaylist":
		plID := getStringArg(args, 0)
		videoID := getStringArg(args, 1)
		return nil, s.services.AddVideoToPlaylist(ctx, plID, videoID)
	case "RemoveVideoFromPlaylist":
		plID := getStringArg(args, 0)
		videoID := getStringArg(args, 1)
		return nil, s.services.RemoveVideoFromPlaylist(ctx, plID, videoID)
	case "GetRemotePlaylistPage":
		playlistID := getStringArg(args, 0)
		pageToken := getStringArg(args, 1)
		limit := getIntArg(args, 2, 24)
		return s.services.GetRemotePlaylistPage(ctx, playlistID, pageToken, limit)
	case "ImportRemotePlaylist":
		playlistID := getStringArg(args, 0)
		name := getStringArg(args, 1)
		description := getStringArg(args, 2)
		maxItems := getIntArg(args, 3, 200)
		return s.services.ImportRemotePlaylist(ctx, playlistID, name, description, maxItems)
	case "AddRemotePlaylistToPlaylist":
		return s.services.AddRemotePlaylistToPlaylist(ctx, getStringArg(args, 0), getStringArg(args, 1), getIntArg(args, 2, 200))

	// Library
	case "GetFavorites":
		return s.services.GetFavorites(ctx)
	case "AddFavorite":
		id := getStringArg(args, 0)
		return nil, s.services.AddFavorite(ctx, id)
	case "RemoveFavorite":
		id := getStringArg(args, 0)
		return nil, s.services.RemoveFavorite(ctx, id)
	case "GetHistory":
		limit := getIntArg(args, 0, 50)
		offset := getIntArg(args, 1, 0)
		return s.services.GetHistory(ctx, limit, offset)
	case "ClearHistory":
		return nil, s.services.ClearHistory(ctx)
	case "RemoveHistoryItem":
		return nil, s.services.RemoveHistoryItem(ctx, getStringArg(args, 0))
	case "GetNote":
		id := getStringArg(args, 0)
		return s.services.GetNote(ctx, id)
	case "SaveNote":
		id := getStringArg(args, 0)
		content := getStringArg(args, 1)
		return nil, s.services.SaveNote(ctx, id, content)
	case "GetBookmarks":
		id := getStringArg(args, 0)
		return s.services.GetBookmarks(ctx, id)
	case "AddBookmark":
		id := getStringArg(args, 0)
		posMs := getInt64Arg(args, 1)
		label := getStringArg(args, 2)
		return s.services.AddBookmark(ctx, id, posMs, label)
	case "DeleteBookmark":
		id := getStringArg(args, 0)
		return nil, s.services.DeleteBookmark(ctx, id)
	case "GetLocalStats":
		return s.services.GetLocalStats(ctx)

	// IPTV
	case "ListIPTVSources":
		return s.services.ListIPTVSources(ctx)
	case "SaveIPTVSource":
		id := getStringArg(args, 0)
		name := getStringArg(args, 1)
		playlistURL := getStringArg(args, 2)
		guideURL := getStringArg(args, 3)
		enabled := getBoolArg(args, 4)
		return nil, s.services.SaveIPTVSource(ctx, id, name, playlistURL, guideURL, enabled)
	case "SaveIPTVSourceWithCredentials":
		id := getStringArg(args, 0)
		name := getStringArg(args, 1)
		playlistURL := getStringArg(args, 2)
		guideURL := getStringArg(args, 3)
		enabled := getBoolArg(args, 4)
		username := getStringArg(args, 5)
		password := getStringArg(args, 6)
		clearCredentials := getBoolArg(args, 7)
		outputMode := getStringArg(args, 8)
		return nil, s.services.SaveIPTVSourceWithCredentials(ctx, id, name, playlistURL, guideURL, enabled, username, password, clearCredentials, outputMode)
	case "DiagnoseIPTVEndpoint":
		endpoint := getStringArg(args, 0)
		return s.services.DiagnoseIPTVEndpoint(ctx, endpoint)
	case "DeleteIPTVSource":
		id := getStringArg(args, 0)
		return nil, s.services.DeleteIPTVSource(ctx, id)
	case "SyncIPTVSource":
		id := getStringArg(args, 0)
		return s.services.SyncIPTVSource(ctx, id)
	case "ListIPTVItemsPaginated":
		var filter iptv.ItemFilter
		if len(args) > 0 {
			raw, _ := json.Marshal(args[0])
			_ = json.Unmarshal(raw, &filter)
		}
		return s.services.ListIPTVItemsPaginated(ctx, filter)
	case "ListIPTVGroups":
		kind := getStringArg(args, 0)
		return s.services.ListIPTVGroups(ctx, kind)
	case "ListIPTVGuide":
		sourceID := getStringArg(args, 0)
		limit := getIntArg(args, 1, 100)
		return s.services.ListIPTVGuide(ctx, sourceID, limit)
	case "SetIPTVItemSaved":
		id := getStringArg(args, 0)
		saved := getBoolArg(args, 1)
		return nil, s.services.SetIPTVItemSaved(ctx, id, saved)
	case "ListIPTVSavedItems":
		limit := getIntArg(args, 0, 100)
		return s.services.ListIPTVSavedItems(ctx, limit)
	case "ResolveIPTVStream":
		sourceID := getStringArg(args, 0)
		itemID := getStringArg(args, 1)
		return s.services.ResolveIPTVStream(ctx, sourceID, itemID)
	case "SaveIPTVPlaybackProgress":
		id := getStringArg(args, 0)
		posMs := getInt64Arg(args, 1)
		durMs := getInt64Arg(args, 2)
		completed := getBoolArg(args, 3)
		return nil, s.services.SaveIPTVPlaybackProgress(ctx, id, posMs, durMs, completed)
	case "ListIPTVResume":
		limit := getIntArg(args, 0, 20)
		return s.services.ListIPTVResume(ctx, limit)

	// Account & Settings
	case "GetAccount":
		acc, ok, err := s.services.GetAccount(ctx)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		return acc, nil
	case "GetLoginStatus":
		return s.services.GetLoginStatus(ctx), nil
	case "DisconnectAccount":
		return nil, s.services.DisconnectAccount(ctx)
	case "ListProfiles":
		return s.services.ListProfiles(ctx)
	case "GetActiveProfile":
		return s.services.GetActiveProfile(ctx)
	case "CreateProfile":
		return s.services.CreateProfile(ctx, getStringArg(args, 0))
	case "CreateGuestProfile":
		return s.services.CreateGuestProfile(ctx, getStringArg(args, 0))
	case "SetActiveProfile":
		return nil, s.services.SetActiveProfile(ctx, getStringArg(args, 0))
	case "DeleteProfile":
		return nil, s.services.DeleteProfile(ctx, getStringArg(args, 0))
	case "GetResourceUsage":
		return s.services.GetResourceUsage(ctx), nil
	case "GetDiagnostics":
		return s.services.GetDiagnostics(ctx), nil
	case "GetPlaybackRuntime":
		return s.services.GetPlaybackRuntime(ctx)
	case "ActivatePlaybackRuntime":
		return nil, s.services.ActivatePlaybackRuntime(ctx, getStringArg(args, 0))
	case "RollbackPlaybackRuntime":
		return nil, s.services.RollbackPlaybackRuntime(ctx)
	case "GetSettings":
		return s.services.GetSettings(ctx)
	case "SaveSetting":
		key := getStringArg(args, 0)
		val := getStringArg(args, 1)
		return nil, s.services.SaveSetting(ctx, key, val)
	case "GetSecretInventory":
		return s.services.GetSecretInventory(ctx)
	case "GetLastFMStatus":
		return s.services.GetLastFMStatus(ctx)
	case "StartLastFMAuthorization":
		return s.services.StartLastFMAuthorization(ctx)
	case "CompleteLastFMAuthorization":
		return s.services.CompleteLastFMAuthorization(ctx)
	case "DisconnectLastFM":
		return nil, s.services.DisconnectLastFM(ctx)
	case "ScrobbleLastFM":
		return nil, s.services.ScrobbleLastFM(
			ctx,
			getStringArg(args, 0),
			getStringArg(args, 1),
			getInt64Arg(args, 2),
			getInt64Arg(args, 3),
		)
	case "ExportPersonalData":
		return s.services.ExportPersonalData(ctx)
	case "ImportPersonalData":
		jsonData := getStringArg(args, 0)
		strategy := getStringArg(args, 1)
		return nil, s.services.ImportPersonalData(ctx, jsonData, strategy)
	case "StartGoogleLogin":
		return s.services.StartGoogleLogin(ctx)
	case "StartDeviceLogin":
		return s.services.StartDeviceLogin(ctx)
	case "ImportSubscriptions":
		content := getStringArg(args, 0)
		return s.services.ImportSubscriptions(ctx, content)
	case "SetBrowserCookies":
		browserName := getStringArg(args, 0)
		return nil, s.services.SetBrowserCookies(ctx, browserName)

	default:
		return nil, fmt.Errorf("método RPC desconhecido: %s", method)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	if s.services == nil || s.services.MediaProxy == nil {
		http.Error(w, "proxy de mídia indisponível", http.StatusServiceUnavailable)
		return
	}
	s.services.MediaProxy.ServeHTTP(w, r)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// 1. Server/dev mode explicitly supplies frontend/dist. Prefer it so E2E
	// and web fallback never serve stale assets embedded in an older binary.
	if s.staticDir != "" {
		filePath := filepath.Join(s.staticDir, path)
		if fi, err := os.Stat(filePath); err == nil && !fi.IsDir() {
			http.ServeFile(w, r, filePath)
			return
		}
		indexFilePath := filepath.Join(s.staticDir, "index.html")
		if _, err := os.Stat(indexFilePath); err == nil {
			http.ServeFile(w, r, indexFilePath)
			return
		}
	}

	// 2. Packaged desktop/server fallback uses the assets embedded at build time.
	if s.staticFS != nil {
		f, err := s.staticFS.Open(path)
		if err == nil {
			_ = f.Close()
			http.FileServer(http.FS(s.staticFS)).ServeHTTP(w, r)
			return
		}
		// Fallback to index.html for SPA client-side routing.
		indexFile, err := s.staticFS.Open("index.html")
		if err == nil {
			_ = indexFile.Close()
			r.URL.Path = "/"
			http.FileServer(http.FS(s.staticFS)).ServeHTTP(w, r)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && !allowedOrigin(origin, r.Host) {
			http.Error(w, "Origem não permitida", http.StatusForbidden)
			return
		}
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedOrigin(origin, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if strings.EqualFold(parsed.Host, host) {
		return true
	}
	hostname := strings.ToLower(parsed.Hostname())
	return (hostname == "localhost" || hostname == "127.0.0.1") && parsed.Port() == "5173"
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func getStringArg(args []interface{}, index int) string {
	if index < len(args) {
		if s, ok := args[index].(string); ok {
			return s
		}
	}
	return ""
}

func getIntArg(args []interface{}, index int, defaultVal int) int {
	if index < len(args) {
		switch v := args[index].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return defaultVal
}

func getInt64Arg(args []interface{}, index int) int64 {
	if index < len(args) {
		switch v := args[index].(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		}
	}
	return 0
}

func getBoolArg(args []interface{}, index int) bool {
	if index < len(args) {
		if b, ok := args[index].(bool); ok {
			return b
		}
	}
	return false
}

func decodeRPCArg(args []interface{}, index int, target interface{}) error {
	if index < 0 || index >= len(args) {
		return fmt.Errorf("argumento %d ausente", index)
	}
	raw, err := json.Marshal(args[index])
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	return nil
}
