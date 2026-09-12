// Ponte de chamadas IPC do Wails v3 com suporte a fallback simulado em desenvolvimento web puro.
import type { 
  HomeModel, Video, Channel, ChannelFolder, Playlist, PlaylistDetail,
  SearchRequest, SearchPage, SearchHit, RemotePlaylistPage, RemoteChannelVideoPage,
  VideoBookmark, PlaybackPlan, LocalStats, DiagnosticsReport, ResourceUsage,
  IPTVSourceState, IPTVEndpointDiagnostic, IPTVItem, IPTVItemFilter, IPTVPageResult, IPTVGuideEntry, IPTVResumeEntry, AccountInfo,
  DeviceCodeInfo, ImportSubscriptionsResult, LoginStatus, Profile,
  SubscriptionVideoQuery, SubscriptionVideoPage, ManagedChannel,
  QueueSnapshot, QueuePreferences, QueueItem, LastFMStatus, SecretInventory, PlaybackRuntimeStatus
} from '../types';

interface RPCResponse {
  result?: unknown;
  error?: string;
}

const mocksEnabled = import.meta.env.DEV && import.meta.env.VITE_ENABLE_MOCKS === 'true';

// O mesmo endpoint é servido pelo Asset Handler do Wails e pelo modo servidor.
// Mocks só podem ser habilitados explicitamente; uma falha real nunca vira
// sucesso simulado, especialmente para mutações persistentes.
async function callBackend<T>(service: string, method: string, ...args: unknown[]): Promise<T> {
  let response: Response;
  try {
    response = await fetch('/api/rpc', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ service, method, args })
    });
  } catch (error: unknown) {
    if (mocksEnabled) {
      return mockResponse(method, args) as T;
    }
    const reason = error instanceof Error ? error.message : 'falha de rede';
    throw new Error(`Backend indisponível para ${service}.${method}: ${reason}`);
  }

  if (!response.ok) {
    throw new Error(`Backend respondeu HTTP ${response.status} em ${service}.${method}`);
  }
  const payload = await response.json() as RPCResponse;
  if (payload.error) {
    throw new Error(payload.error);
  }
  return payload.result as T;
}

let catalogRefresh: Promise<unknown> | null = null;
let startupRefreshRequested = false;

export const CatalogService = {
  getHome: (profileID = '') => callBackend<HomeModel>('Catalog', 'GetHome', profileID),
  refreshOnStartup: (settings: Record<string, string>): Promise<unknown> => {
    if (startupRefreshRequested) return Promise.resolve();
    startupRefreshRequested = true;
    if (settings['sync.refresh_on_startup'] === 'false') return Promise.resolve();
    return CatalogService.refreshSubscriptions();
  },
  refreshSubscriptions: (): Promise<unknown> => {
    if (!catalogRefresh) {
      catalogRefresh = callBackend<unknown>('Catalog', 'RefreshSubscriptions')
        .then((stats) => {
          window.dispatchEvent(new Event('nanotube-catalog-refreshed'));
          return stats;
        })
        .finally(() => { catalogRefresh = null; });
    }
    return catalogRefresh;
  },
  getChannels: () => callBackend<Channel[]>('Catalog', 'GetChannels'),
  getRemoteChannelVideosPage: (channelID: string, pageToken = '', limit = 24) =>
    callBackend<RemoteChannelVideoPage>('Catalog', 'GetRemoteChannelVideosPage', channelID, pageToken, limit),
  rememberVideo: (video: Video) => callBackend<void>('Catalog', 'RememberVideo', video),
  subscribeChannel: (id: string, title: string) => callBackend<void>('Catalog', 'SubscribeChannel', id, title),
  unsubscribeChannel: (id: string) => callBackend<void>('Catalog', 'UnsubscribeChannel', id),
  getChannelFolders: () => callBackend<ChannelFolder[]>('Catalog', 'GetChannelFolders'),
  createChannelFolder: (name: string) => callBackend<ChannelFolder>('Catalog', 'CreateChannelFolder', name),
  renameChannelFolder: (id: string, name: string) => callBackend<void>('Catalog', 'RenameChannelFolder', id, name),
  deleteChannelFolder: (id: string) => callBackend<void>('Catalog', 'DeleteChannelFolder', id),
  addChannelToFolder: (folderID: string, channelID: string) => callBackend<void>('Catalog', 'AddChannelToFolder', folderID, channelID),
  removeChannelFromFolder: (folderID: string, channelID: string) => callBackend<void>('Catalog', 'RemoveChannelFromFolder', folderID, channelID),
  getFolderMembership: () => callBackend<Record<string, string[]>>('Catalog', 'GetFolderMembership'),
  addChannelFavorite: (id: string) => callBackend<void>('Catalog', 'AddChannelFavorite', id),
  removeChannelFavorite: (id: string) => callBackend<void>('Catalog', 'RemoveChannelFavorite', id),
  getFavoriteChannelIDs: () => callBackend<Record<string, boolean>>('Catalog', 'GetFavoriteChannelIDs'),
  listSubscriptionVideos: (query: SubscriptionVideoQuery) => callBackend<SubscriptionVideoPage>('Catalog', 'ListSubscriptionVideos', query),
  listManagedChannels: () => callBackend<ManagedChannel[]>('Catalog', 'ListManagedChannels'),
  bulkUnsubscribeChannels: (channelIDs: string[]) => callBackend<void>('Catalog', 'BulkUnsubscribeChannels', channelIDs),
  bulkFavoriteChannels: (channelIDs: string[]) => callBackend<void>('Catalog', 'BulkFavoriteChannels', channelIDs),
  bulkAddChannelsToFolder: (folderID: string, channelIDs: string[]) => callBackend<void>('Catalog', 'BulkAddChannelsToFolder', folderID, channelIDs),
  setChannelTags: (channelID: string, tags: string[]) => callBackend<void>('Catalog', 'SetChannelTags', channelID, tags),
};

export const PlayerService = {
  resolveMedia: (videoID: string, sourceURL = '') => callBackend<PlaybackPlan>('Player', 'ResolveMedia', videoID, sourceURL),
  saveProgress: (videoID: string, posMs: number, durMs: number, completed: boolean) =>
    callBackend<void>('Player', 'SaveProgress', videoID, posMs, durMs, completed),
  revokePlayerSession: () => callBackend<void>('Player', 'RevokePlayerSession'),
};

export const LastFMService = {
  getStatus: () => callBackend<LastFMStatus>('LastFM', 'GetLastFMStatus'),
  startAuthorization: () => callBackend<string>('LastFM', 'StartLastFMAuthorization'),
  completeAuthorization: () => callBackend<LastFMStatus>('LastFM', 'CompleteLastFMAuthorization'),
  disconnect: () => callBackend<void>('LastFM', 'DisconnectLastFM'),
  scrobble: (artist: string, track: string, playedMs: number, durationMs: number) =>
    callBackend<void>('LastFM', 'ScrobbleLastFM', artist, track, playedMs, durationMs),
};

export const QueueService = {
  getQueue: () => callBackend<QueueSnapshot>('Queue', 'GetQueue'),
  enqueueVideo: (video: Video) => callBackend<QueueItem>('Queue', 'EnqueueVideo', video),
  reorderQueue: (itemIDs: string[]) => callBackend<void>('Queue', 'ReorderQueue', itemIDs),
  removeQueueItem: (itemID: string) => callBackend<void>('Queue', 'RemoveQueueItem', itemID),
  markQueueItemPlayed: (itemID: string, played: boolean) => callBackend<void>('Queue', 'MarkQueueItemPlayed', itemID, played),
  clearQueue: (scope: 'all' | 'played') => callBackend<void>('Queue', 'ClearQueue', scope),
  savePreferences: (preferences: QueuePreferences) => callBackend<void>('Queue', 'SaveQueuePreferences', preferences),
  saveAsPlaylist: (name: string) => callBackend<Playlist>('Queue', 'SaveQueueAsPlaylist', name),
};

export const SearchService = {
  search: (request: SearchRequest | string, limit = 20, offset = 0) =>
    typeof request === 'string'
      ? callBackend<SearchPage>('Search', 'Search', request, limit, offset)
      : callBackend<SearchPage>('Search', 'Search', request),
  searchLocalFTS: (query: string, limit = 20) => callBackend<SearchHit[]>('Search', 'SearchLocalFTS', query, limit),
};

export const PlaylistService = {
  listPlaylists: () => callBackend<Playlist[]>('Playlist', 'ListPlaylists'),
  getPlaylist: (id: string) => callBackend<PlaylistDetail>('Playlist', 'GetPlaylist', id),
  createPlaylist: (name: string, desc = '', color = '') => callBackend<Playlist>('Playlist', 'CreatePlaylist', name, desc, color),
  updatePlaylist: (id: string, name: string, desc = '', color = '') => callBackend<void>('Playlist', 'UpdatePlaylist', id, name, desc, color),
  mergePlaylist: (sourceID: string, targetID: string) => callBackend<void>('Playlist', 'MergePlaylist', sourceID, targetID),
  deletePlaylist: (id: string) => callBackend<void>('Playlist', 'DeletePlaylist', id),
  addVideoToPlaylist: (plID: string, videoID: string) => callBackend<void>('Playlist', 'AddVideoToPlaylist', plID, videoID),
  removeVideoFromPlaylist: (plID: string, videoID: string) => callBackend<void>('Playlist', 'RemoveVideoFromPlaylist', plID, videoID),
  getRemotePlaylistPage: (playlistID: string, pageToken = '', limit = 24) =>
    callBackend<RemotePlaylistPage>('Playlist', 'GetRemotePlaylistPage', playlistID, pageToken, limit),
  importRemotePlaylist: (playlistID: string, name: string, description = '', maxItems = 200) =>
    callBackend<PlaylistDetail>('Playlist', 'ImportRemotePlaylist', playlistID, name, description, maxItems),
  addRemotePlaylistToPlaylist: (remoteID: string, targetID: string, maxItems = 200) =>
    callBackend<number>('Playlist', 'AddRemotePlaylistToPlaylist', remoteID, targetID, maxItems),
};

export const LibraryService = {
  getFavorites: () => callBackend<Video[]>('Library', 'GetFavorites'),
  addFavorite: (id: string) => callBackend<void>('Library', 'AddFavorite', id),
  removeFavorite: (id: string) => callBackend<void>('Library', 'RemoveFavorite', id),
  getHistory: (limit = 50, offset = 0) => callBackend<Video[]>('Library', 'GetHistory', limit, offset),
  clearHistory: () => callBackend<void>('Library', 'ClearHistory'),
  removeHistoryItem: (videoID: string) => callBackend<void>('Library', 'RemoveHistoryItem', videoID),
  getNote: (videoID: string) => callBackend<string>('Library', 'GetNote', videoID),
  saveNote: (videoID: string, text: string) => callBackend<void>('Library', 'SaveNote', videoID, text),
  getBookmarks: (videoID: string) => callBackend<VideoBookmark[]>('Library', 'GetBookmarks', videoID),
  addBookmark: (videoID: string, posMs: number, label: string) => callBackend<VideoBookmark>('Library', 'AddBookmark', videoID, posMs, label),
  deleteBookmark: (id: string) => callBackend<void>('Library', 'DeleteBookmark', id),
  getLocalStats: () => callBackend<LocalStats>('Library', 'GetLocalStats'),
};

export const IPTVService = {
  listSources: () => callBackend<IPTVSourceState[]>('IPTV', 'ListIPTVSources'),
  saveSource: (id: string, name: string, playlistUrl: string, guideUrl = '', enabled = true) => 
    callBackend<void>('IPTV', 'SaveIPTVSource', id, name, playlistUrl, guideUrl, enabled),
  saveSourceWithCredentials: (
    id: string,
    name: string,
    playlistUrl: string,
    guideUrl: string,
    enabled: boolean,
    username: string,
    password: string,
    clearCredentials: boolean,
    outputMode: 'preserve' | 'hls',
  ) => callBackend<void>(
    'IPTV',
    'SaveIPTVSourceWithCredentials',
    id,
    name,
    playlistUrl,
    guideUrl,
    enabled,
    username,
    password,
    clearCredentials,
    outputMode,
  ),
  diagnoseEndpoint: (endpoint: string) =>
    callBackend<IPTVEndpointDiagnostic>('IPTV', 'DiagnoseIPTVEndpoint', endpoint),
  deleteSource: (id: string) => callBackend<void>('IPTV', 'DeleteIPTVSource', id),
  syncSource: (id: string) => callBackend<number>('IPTV', 'SyncIPTVSource', id),
  listItemsPaginated: (filter: IPTVItemFilter) => 
    callBackend<IPTVPageResult>('IPTV', 'ListIPTVItemsPaginated', filter),
  listGroups: (kind = '') => callBackend<string[]>('IPTV', 'ListIPTVGroups', kind),
  listGuide: (sourceID = '', limit = 100) => callBackend<IPTVGuideEntry[]>('IPTV', 'ListIPTVGuide', sourceID, limit),
  setItemSaved: (itemID: string, saved: boolean) => callBackend<void>('IPTV', 'SetIPTVItemSaved', itemID, saved),
  listSavedItems: (limit = 100) => callBackend<IPTVItem[]>('IPTV', 'ListIPTVSavedItems', limit),
  resolveIPTVStream: (sourceID: string, itemID: string) => callBackend<PlaybackPlan>('IPTV', 'ResolveIPTVStream', sourceID, itemID),
  saveProgress: (itemID: string, posMs: number, durMs: number, completed: boolean) => 
    callBackend<void>('IPTV', 'SaveIPTVPlaybackProgress', itemID, posMs, durMs, completed),
  listResume: (limit = 20) => callBackend<IPTVResumeEntry[]>('IPTV', 'ListIPTVResume', limit),
};

export const AccountService = {
  getAccount: () => callBackend<AccountInfo | null>('Account', 'GetAccount'),
  getLoginStatus: () => callBackend<LoginStatus>('Account', 'GetLoginStatus'),
  disconnectAccount: () => callBackend<void>('Account', 'DisconnectAccount'),
  startGoogleLogin: () => callBackend<string>('Account', 'StartGoogleLogin'),
  startDeviceLogin: () => callBackend<DeviceCodeInfo>('Account', 'StartDeviceLogin'),
  listProfiles: () => callBackend<Profile[]>('Account', 'ListProfiles'),
  getActiveProfile: () => callBackend<Profile>('Account', 'GetActiveProfile'),
  createProfile: (name: string) => callBackend<Profile>('Account', 'CreateProfile', name),
  createGuestProfile: (name = '') => callBackend<Profile>('Account', 'CreateGuestProfile', name),
  setActiveProfile: (profileID: string) => callBackend<void>('Account', 'SetActiveProfile', profileID),
  deleteProfile: (profileID: string) => callBackend<void>('Account', 'DeleteProfile', profileID),
  importSubscriptions: (content: string) => 
    callBackend<ImportSubscriptionsResult>('Account', 'ImportSubscriptions', content),
  setBrowserCookies: (browserName: string) => 
    callBackend<void>('Account', 'SetBrowserCookies', browserName),
};

export const SettingsService = {
  getResourceUsage: () => callBackend<ResourceUsage>('Settings', 'GetResourceUsage'),
  getDiagnostics: () => callBackend<DiagnosticsReport>('Settings', 'GetDiagnostics'),
  getPlaybackRuntime: () => callBackend<PlaybackRuntimeStatus>('Settings', 'GetPlaybackRuntime'),
  activatePlaybackRuntime: (version: string) => callBackend<void>('Settings', 'ActivatePlaybackRuntime', version),
  rollbackPlaybackRuntime: () => callBackend<void>('Settings', 'RollbackPlaybackRuntime'),
  getSettings: () => callBackend<Record<string, string>>('Settings', 'GetSettings'),
  saveSetting: (key: string, value: string) => callBackend<void>('Settings', 'SaveSetting', key, value),
  getSecretInventory: () => callBackend<SecretInventory>('Settings', 'GetSecretInventory'),
  exportPersonalData: () => callBackend<string>('Settings', 'ExportPersonalData'),
  importPersonalData: (jsonData: string, strategy = 'merge') => callBackend<void>('Settings', 'ImportPersonalData', jsonData, strategy),
};

export const DiagnosticService = {
  getDiagnostics: () => callBackend<DiagnosticsReport>('Settings', 'GetDiagnostics'),
};

// Respostas simuladas para testes de UI no navegador
function mockResponse(method: string, args: unknown[]): unknown {
  const dummyVideo: Video = {
    id: 'dQw4w9WgXcQ',
    channel_id: 'rick_astley',
    channel_title: 'Rick Astley',
    title: 'Rick Astley - Never Gonna Give You Up (Official Music Video)',
    description_excerpt: 'The official video for Never Gonna Give You Up by Rick Astley',
    published_at: new Date().toISOString(),
    duration: 213 * 1e9,
    thumbnail_url: 'https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=480&q=80',
  };

  switch (method) {
    case 'GetHome':
      return {
        for_you: [
          dummyVideo,
          { ...dummyVideo, id: 'v2', title: 'Linux em Hardware Modesto: Otimizações Extremas' },
          { ...dummyVideo, id: 'v3', title: 'Construindo Desktop Apps Ultra Leves com Go e Svelte' },
          { ...dummyVideo, id: 'v4', title: 'Como o YouTube mudou seus desafios de Attestation e PO Tokens' }
        ],
        topic_sections: [
          {
            id: 'tech',
            title: 'Tecnologia & Open Source',
            videos: [
              { ...dummyVideo, id: 'v5', title: 'Aprenda Svelte 5 em 20 Minutos' },
              { ...dummyVideo, id: 'v6', title: 'SQLite e FTS5: Criando Mecanismos de Busca Rápidos' }
            ]
          }
        ]
      };
    case 'GetChannels':
      return [
        { id: 'rick_astley', title: 'Rick Astley', subscribed: true },
        { id: 'tech_channel', title: 'Canal de Tecnologia', subscribed: true }
      ];
    case 'GetRemoteChannelVideosPage':
      return {
        channel_id: String(args[0] || dummyVideo.channel_id),
        videos: [dummyVideo, { ...dummyVideo, id: 'channel-video-2', title: 'Outro vídeo do canal' }],
        next_page_token: '',
      };
    case 'Search':
      return {
        items: [
          dummyVideo,
          { ...dummyVideo, id: 'UC_search', title: 'Canal Linux', resource_type: 'channel', duration: 0 },
          { ...dummyVideo, id: 'PL_search', title: 'Playlist Linux', resource_type: 'playlist', duration: 0 },
        ],
        next_page_token: '24',
        source: 'yt-dlp',
      };
    case 'SearchLocalFTS':
      return [{ video_id: dummyVideo.id, title: dummyVideo.title, channel_name: dummyVideo.channel_title }];
    case 'ListPlaylists':
      return [
        { id: 'pl_1', name: 'Favoritos de Programação', is_smart: false, item_count: 5, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
        { id: 'pl_2', name: 'Vídeos Longos (> 20m)', is_smart: true, item_count: 12, created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
      ];
    case 'GetPlaylist':
      return {
        playlist: { id: String(args[0] || 'pl_1'), name: 'Favoritos de Programação', is_smart: false, item_count: 1, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
        videos: [dummyVideo],
      };
    case 'GetRemotePlaylistPage':
      return { playlist_id: String(args[0] || 'PL_search'), videos: [dummyVideo], next_page_token: '' };
    case 'ImportRemotePlaylist':
      return {
        playlist: { id: 'pl_imported', name: String(args[1] || 'Playlist importada'), is_smart: false, item_count: 1, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
        videos: [dummyVideo],
      };
    case 'GetResourceUsage':
      return { rss_bytes: null, cpu_percent: null, receive_bytes_per_second: null, transmit_bytes_per_second: null };
    case 'GetDiagnostics':
      return {
        version: '0.1.0-dev',
        go_version: 'go1.22.5',
        framework: 'Wails v3 + Svelte 5 + Tailwind CSS',
        resolver_strategy: 'cascading (yt-dlp explícito -> Invidious)',
        ytdlp_path: '/usr/bin/yt-dlp',
        ytdlp_version: '2026.07.04',
        ytdlp_source: 'system',
        js_runtime: 'deno 2.9.5',
        ejs_status: 'remote (ejs:github)',
        pot_provider: 'bgutil-ytdlp-pot-provider (plugin)',
        pot_mode: 'script',
        playback_manifest: 'não configurado',
        player_client: 'auto',
        extraction_config: 'js-runtime=deno player-client=auto pot-provider=configurado cookies=navegador',
        sqlite_version: '3.53.3',
        keyring: 'disponível',
		log_file: '/home/user/.local/state/nanotube-web/nanotube-web.log',
		recent_logs: ['2026/08/31 19:00:00 NanoTube iniciado'],
        rss_bytes: 28 * 1024 * 1024,
        metrics: {
          playback_requests: 0,
          playback_cache_hits: 0,
          playback_errors: 0,
          playback_latency_total_us: 0,
          playback_latency_max_us: 0,
          search_requests: 0,
          search_errors: 0,
          search_latency_total_us: 0,
          search_latency_max_us: 0
        }
      };
    case 'GetPlaybackRuntime':
      return { active_version: '', previous_version: '', installed_versions: [] } as PlaybackRuntimeStatus;
    case 'GetLocalStats':
      return {
        videos_watched: 42,
        watch_time: 14400 * 1e9,
        history_count: 58,
        favorites_count: 14,
        playlists_count: 3,
        playlist_items: 25,
        subscriptions_count: 18,
        channel_favorites_count: 4,
        folders_count: 2,
        feedback_count: 8
      };
    case 'ResolveMedia':
    case 'ResolveIPTVStream':
      return {
        mode: 'resolved-media',
        primary: {
          url: 'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4'
        },
        metadata: {
          title: 'Big Buck Bunny (Sample Stream)'
        }
      };
    case 'ListIPTVSources':
      return [
        {
          config: {
            id: 'src_demo',
            name: 'Canais Abertos & Cultura Brasil',
            format: 'm3u',
            playlist_url: 'https://iptv-org.github.io/iptv/countries/br.m3u',
            guide_url: '',
            enabled: true
          },
          last_sync_at: new Date().toISOString(),
          last_error: ''
        }
      ];
    case 'ListIPTVItemsPaginated':
      return {
        items: [
          {
            id: 'tv_brasil',
            source_id: 'src_demo',
            source_name: 'Canais Abertos',
            kind: 'tv',
            title: 'TV Brasil HD',
            group: 'Cultura & Notícias',
            logo_url: 'https://images.unsplash.com/photo-1598899134739-24c46f58b8c0?w=120&q=80',
            stream_url: 'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4',
            channel_number: '1'
          },
          {
            id: 'tv_cultura',
            source_id: 'src_demo',
            source_name: 'Canais Abertos',
            kind: 'tv',
            title: 'TV Cultura',
            group: 'Cultura & Notícias',
            logo_url: 'https://images.unsplash.com/photo-1578022761797-b8636ac1773c?w=120&q=80',
            stream_url: 'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ElephantsDream.mp4',
            channel_number: '2'
          },
          {
            id: 'movie_sample',
            source_id: 'src_demo',
            source_name: 'Filmes Open Source',
            kind: 'movie',
            title: 'Big Buck Bunny (2008)',
            group: 'Animação',
            logo_url: 'https://images.unsplash.com/photo-1536440136628-849c177e76a1?w=120&q=80',
            stream_url: 'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4'
          }
        ],
        total_count: 3,
        page: 1,
        page_size: 50,
        total_pages: 1
      };
    case 'ListIPTVGroups':
      return ['Cultura & Notícias', 'Animação', 'Documentários', 'Música'];
    case 'ListIPTVGuide':
      return [
        {
          source_id: 'src_demo',
          channel_id: 'tv_brasil',
          channel_name: 'TV Brasil HD',
          logo_url: '',
          program: {
            channel_id: 'tv_brasil',
            title: 'Jornal da Noite',
            description: 'Noticiário ao vivo com os principais acontecimentos do país.',
            start: new Date().toISOString(),
            end: new Date(Date.now() + 3600000).toISOString()
          }
        }
      ];
    case 'GetAccount':
      return null;
    case 'GetLoginStatus':
      return { state: 'idle', profile_id: 'default' };
    case 'ListProfiles':
      return [{ id: 'default', name: 'Padrão', kind: 'persistent', created_at: new Date(0).toISOString() }];
    case 'GetActiveProfile':
    case 'CreateProfile':
      return {
        id: method === 'CreateProfile' ? 'pf_mock' : 'default',
        name: method === 'CreateProfile' ? String(args[0] || 'Perfil') : 'Padrão',
        kind: 'persistent',
        created_at: new Date().toISOString()
      };
    case 'CreateGuestProfile':
      return {
        id: 'guest_mock',
        name: String(args[0] || 'Convidado'),
        kind: 'guest',
        created_at: new Date().toISOString()
      };
    default:
      return [];
  }
}
