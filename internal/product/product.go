// Package product define os limites executáveis dos produtos NanoSuite.
package product

// Spec descreve identidade, persistência e superfície RPC de um aplicativo.
// Os mapas são declarativos para facilitar a extração futura em repositórios.
type Spec struct {
	ID                string
	Name              string
	Description       string
	Identifier        string
	DataNamespace     string
	DatabaseName      string
	KeyringService    string
	FrontendTarget    string
	DefaultWidth      int
	DefaultHeight     int
	QueueEnabled      bool
	AllowedRPCMethods map[string][]string
}

var HummTube = Spec{
	ID:             "hummtube",
	Name:           "HummTube",
	Description:    "Cliente desktop YouTube leve para Linux",
	Identifier:     "io.hummtube.desktop",
	DataNamespace:  "hummtube",
	DatabaseName:   "hummtube.db",
	KeyringService: "hummtube",
	FrontendTarget: "hummtube",
	DefaultWidth:   1180,
	DefaultHeight:  780,
	QueueEnabled:   true,
	AllowedRPCMethods: allowServices(
		"Catalog", "Player", "Queue", "Search", "Playlist", "Library", "Account", "Settings",
	),
}

var HummIPTV = Spec{
	ID:             "hummiptv",
	Name:           "HummIPTV",
	Description:    "Player desktop dedicado a IPTV, M3U e EPG",
	Identifier:     "io.hummtube.hummiptv",
	DataNamespace:  "hummiptv",
	DatabaseName:   "hummiptv.db",
	KeyringService: "hummiptv",
	FrontendTarget: "hummiptv",
	DefaultWidth:   1280,
	DefaultHeight:  800,
	QueueEnabled:   false,
	AllowedRPCMethods: map[string][]string{
		"IPTV": {
			"ListIPTVSources", "SaveIPTVSource", "SaveIPTVSourceWithCredentials",
			"DiagnoseIPTVEndpoint", "DeleteIPTVSource", "SyncIPTVSource",
			"ListIPTVItemsPaginated", "ListIPTVGroups", "ListIPTVGuide",
			"SetIPTVItemSaved", "ListIPTVSavedItems", "ResolveIPTVStream",
			"SaveIPTVPlaybackProgress", "ListIPTVResume",
		},
		"Settings": {"GetSettings", "SaveSetting"},
	},
}

var HummMusic = Spec{
	ID:             "hummmusic",
	Name:           "HummMusic",
	Description:    "Player desktop dedicado a música e podcasts",
	Identifier:     "io.hummtube.hummmusic",
	DataNamespace:  "hummmusic",
	DatabaseName:   "hummmusic.db",
	KeyringService: "hummmusic",
	FrontendTarget: "hummmusic",
	DefaultWidth:   1180,
	DefaultHeight:  780,
	QueueEnabled:   true,
	AllowedRPCMethods: map[string][]string{
		"Player":   append([]string(nil), serviceMethods["Player"]...),
		"Queue":    append([]string(nil), serviceMethods["Queue"]...),
		"Search":   append([]string(nil), serviceMethods["Search"]...),
		"Playlist": append([]string(nil), serviceMethods["Playlist"]...),
		"Library":  append([]string(nil), serviceMethods["Library"]...),
		"Account": {
			"GetAccount", "GetLoginStatus", "DisconnectAccount", "ListProfiles", "GetActiveProfile",
			"CreateProfile", "CreateGuestProfile", "SetActiveProfile", "DeleteProfile",
			"StartGoogleLogin", "StartDeviceLogin", "SetBrowserCookies",
		},
		"Settings": append([]string(nil), serviceMethods["Settings"]...),
		"LastFM":   append([]string(nil), serviceMethods["LastFM"]...),
		"Catalog":  {"RememberVideo"},
	},
}

// Aliases para manter retrocompatibilidade
var (
	NanoTube  = HummTube
	NanoIPTV  = HummIPTV
	NanoMusic = HummMusic
)

var serviceMethods = map[string][]string{
	"Catalog": {
		"GetHome", "RefreshSubscriptions", "GetChannels", "GetRemoteChannelVideosPage",
		"RememberVideo", "SubscribeChannel", "UnsubscribeChannel", "GetChannelFolders",
		"CreateChannelFolder", "RenameChannelFolder", "DeleteChannelFolder", "AddChannelFavorite",
		"RemoveChannelFavorite", "GetFavoriteChannelIDs", "GetFolderMembership", "ListSubscriptionVideos",
		"ListManagedChannels", "BulkUnsubscribeChannels", "BulkFavoriteChannels", "BulkAddChannelsToFolder",
		"SetChannelTags",
	},
	"Player":   {"ResolveMedia", "SaveProgress"},
	"Queue":    {"GetQueue", "EnqueueVideo", "ReorderQueue", "RemoveQueueItem", "MarkQueueItemPlayed", "ClearQueue", "SaveQueuePreferences", "SaveQueueAsPlaylist"},
	"Search":   {"Search", "SearchLocalFTS"},
	"Playlist": {"ListPlaylists", "GetPlaylist", "CreatePlaylist", "UpdatePlaylist", "MergePlaylist", "DeletePlaylist", "AddVideoToPlaylist", "RemoveVideoFromPlaylist", "GetRemotePlaylistPage", "ImportRemotePlaylist", "AddRemotePlaylistToPlaylist"},
	"Library":  {"GetFavorites", "AddFavorite", "RemoveFavorite", "GetHistory", "ClearHistory", "RemoveHistoryItem", "GetNote", "SaveNote", "GetBookmarks", "AddBookmark", "DeleteBookmark", "GetLocalStats"},
	"IPTV":     {"ListIPTVSources", "SaveIPTVSource", "SaveIPTVSourceWithCredentials", "DiagnoseIPTVEndpoint", "DeleteIPTVSource", "SyncIPTVSource", "ListIPTVItemsPaginated", "ListIPTVGroups", "ListIPTVGuide", "SetIPTVItemSaved", "ListIPTVSavedItems", "ResolveIPTVStream", "SaveIPTVPlaybackProgress", "ListIPTVResume"},
	"Account":  {"GetAccount", "GetLoginStatus", "DisconnectAccount", "ListProfiles", "GetActiveProfile", "CreateProfile", "CreateGuestProfile", "SetActiveProfile", "DeleteProfile", "StartGoogleLogin", "StartDeviceLogin", "ImportSubscriptions", "SetBrowserCookies"},
	"Settings": {"GetResourceUsage", "GetDiagnostics", "GetSettings", "SaveSetting", "GetSecretInventory", "ExportPersonalData", "ImportPersonalData", "GetPlaybackRuntime", "ActivatePlaybackRuntime", "RollbackPlaybackRuntime"},
	"LastFM":   {"GetLastFMStatus", "StartLastFMAuthorization", "CompleteLastFMAuthorization", "DisconnectLastFM", "ScrobbleLastFM"},
}

func allowServices(names ...string) map[string][]string {
	allowed := make(map[string][]string, len(names))
	for _, name := range names {
		allowed[name] = append([]string(nil), serviceMethods[name]...)
	}
	return allowed
}

// ServiceAllowsMethod valida o par público serviço/método do contrato RPC.
func ServiceAllowsMethod(service, method string) bool {
	for _, candidate := range serviceMethods[service] {
		if candidate == method {
			return true
		}
	}
	return false
}
