export interface Video {
  id: string;
  channel_id: string;
  channel_title?: string;
  resource_type?: 'video' | 'channel' | 'playlist';
  external_url?: string;
  view_count?: number;
  resource_item_count?: number;
  live_status?: string;
  title: string;
  description?: string;
  description_excerpt?: string;
  published_at: string;
  duration: number; // nanoseconds in Go
  category?: string;
  thumbnail_url: string;
  recommendation_reasons?: string[];
}

export interface Channel {
  id: string;
  title: string;
  thumbnail_url?: string;
  subscribed: boolean;
  uploads_playlist_id?: string;
  last_sync_at?: string;
  last_known_video_id?: string;
  last_error?: string;
}

export interface ChannelFolder {
  id: string;
  name: string;
  created_at: string;
}

export interface ManagedChannel {
  channel: Channel;
  category?: string;
  tags: string[];
  subscribed_at?: string;
  last_watched_at?: string;
}

export interface SubscriptionVideoQuery {
  offset?: number;
  limit?: number;
  content?: 'all' | 'regular' | 'live';
  category?: string;
  channel_id?: string;
  watch?: 'any' | 'watched' | 'unwatched';
  from?: string;
  to?: string;
}

export interface SubscriptionVideoPage {
  videos: Video[];
  categories: string[];
  total: number;
  offset: number;
  limit: number;
  has_more: boolean;
}

export interface Playlist {
  id: string;
  name: string;
  description?: string;
  color?: string;
  tags?: string[];
  is_smart: boolean;
  item_count: number;
  created_at: string;
  updated_at: string;
}

export type SearchResourceType = 'video' | 'channel' | 'playlist';
export type SearchOrder = 'relevance' | 'date' | 'viewCount' | 'rating' | 'title' | 'videoCount';
export type SearchDuration = 'any' | 'short' | 'medium' | 'long' | 'custom';
export type SearchEventType = '' | 'live' | 'completed' | 'upcoming';
export type SearchCaption = 'any' | 'closedCaption' | 'none';
export type SearchDefinition = 'any' | 'high' | 'standard';
export type SearchDimension = 'any' | '2d' | '3d';
export type SearchLicense = 'any' | 'youtube' | 'creativeCommon';
export type SearchTriState = 'any' | 'yes' | 'no';
export type SearchVideoType = 'any' | 'movie' | 'episode';
export type SearchSafe = 'any' | 'moderate' | 'none' | 'strict';
export type SearchWatchState = 'any' | 'unwatched' | 'watched' | 'continue';
export type SearchSavedState = 'any' | 'favorites' | 'queue' | 'not-saved';

export interface SearchRequest {
  query: string;
  exact_phrase?: string;
  include_terms?: string;
  exclude_terms?: string;
  max_results?: number;
  page_token?: string;
  resource_types?: SearchResourceType[];
  order?: SearchOrder;
  published_after?: string;
  published_before?: string;
  duration?: SearchDuration;
  min_duration?: number;
  max_duration?: number;
  shorts_only?: boolean;
  regular_only?: boolean;
  event_type?: SearchEventType;
  channel_id?: string;
  only_subscribed?: boolean;
  caption?: SearchCaption;
  watch_state?: SearchWatchState;
  saved_state?: SearchSavedState;
  hide_rejected?: boolean;
  definition?: SearchDefinition;
  dimension?: SearchDimension;
  license?: SearchLicense;
  embeddable?: SearchTriState;
  syndicated?: SearchTriState;
  paid_promotion?: SearchTriState;
  video_type?: SearchVideoType;
  category_id?: string;
  topic_id?: string;
  relevance_language?: string;
  region_code?: string;
  safe_search?: SearchSafe;
  location?: string;
  location_radius?: string;
}

export interface SearchPage {
  items: Video[];
  next_page_token?: string;
  source: 'yt-dlp' | 'youtube-api';
  notices?: string[];
}

export interface SearchHit {
  video_id: string;
  title: string;
  channel_name: string;
}

export interface LastFMStatus {
  configured: boolean;
  connected: boolean;
  username?: string;
  warning?: string;
}

export interface SecretMetadata {
  id: string;
  provider: 'google' | 'lastfm' | string;
  label: string;
  profile_id: string;
  state: 'available' | 'memory' | 'unavailable' | 'missing';
  storage: 'keyring' | 'memory' | string;
  rotated_at?: string;
  expires_at?: string;
  can_rotate: boolean;
  can_revoke: boolean;
  warning?: string;
}

export interface CredentialAuditEvent {
  id: number;
  profile_id: string;
  provider: string;
  action: 'connected' | 'rotated' | 'revoked' | 'verification_failed';
  occurred_at: string;
  detail?: string;
}

export interface SecretInventory {
  secrets: SecretMetadata[];
  audit: CredentialAuditEvent[];
}

export interface RemotePlaylistPage {
  playlist_id: string;
  videos: Video[];
  next_page_token?: string;
}

export interface RemoteChannelVideoPage {
  channel_id: string;
  videos: Video[];
  next_page_token?: string;
}

export interface PlaylistDetail {
  playlist: Playlist;
  videos: Video[];
}

export interface VideoBookmark {
  id: string;
  video_id: string;
  position: number;
  label: string;
  created_at: string;
}

export interface FeedSection {
  id: string;
  title: string;
  description?: string;
  videos: Video[];
}

export interface HomeModel {
  continue_watching: Video[];
  for_you: Video[];
  topic_sections: FeedSection[];
  recent_subscriptions: Video[];
  rediscovery: Video[];
  favorites: Video[];
  unwatched: Video[];
  long_videos: Video[];
}

export interface PlaybackPlan {
  mode: 'direct' | 'resolved-media';
  primary: {
    url: string;
    headers?: Record<string, string>;
  };
  audio?: {
    url: string;
    headers?: Record<string, string>;
  };
  audio_only?: {
    url: string;
    headers?: Record<string, string>;
  };
  variants?: Array<{
    id: string;
    label: string;
    height?: number;
    has_audio?: boolean;
    stream: {
      url: string;
      headers?: Record<string, string>;
    };
  }>;
  subtitles?: Array<{
    language: string;
    label?: string;
    url: string;
    format?: string;
  }>;
  audio_tracks?: Array<{
    id: string;
    label: string;
    language?: string;
    stream: {
      url: string;
      headers?: Record<string, string>;
    };
  }>;
  metadata: {
    video_id?: string;
    title?: string;
    duration?: number;
  };
  expires_at?: string;
}

export interface QueueItem {
  id: string;
  video: Video;
  position: number;
  state: 'pending' | 'played';
  added_at: string;
  played_at?: string;
}

export interface QueuePreferences {
  autoplay: boolean;
  remove_played: boolean;
}

export interface QueueSnapshot {
  items: QueueItem[];
  preferences: QueuePreferences;
}

export interface LocalStats {
  videos_watched: number;
  watch_time: number;
  history_count: number;
  favorites_count: number;
  playlists_count: number;
  playlist_items: number;
  subscriptions_count: number;
  channel_favorites_count: number;
  folders_count: number;
  feedback_count: number;
}

export interface ResourceUsage {
  rss_bytes: number | null;
  cpu_percent: number | null;
  receive_bytes_per_second: number | null;
  transmit_bytes_per_second: number | null;
}

export interface DiagnosticsReport {
  version: string;
  commit?: string;
  go_version: string;
  framework: string;
  resolver_strategy: string;
  ytdlp_path: string;
  ytdlp_version: string;
  ytdlp_source: string;
  js_runtime: string;
  ejs_status: string;
  pot_provider: string;
  pot_mode: string;
  playback_manifest?: string;
  player_client: string;
  extraction_config: string;
  extraction_hint?: string;
  sqlite_version: string;
  keyring: string;
  rss_bytes: number;
  checks?: {
    yt_dlp?: { ok: boolean; detail?: string };
    pot_provider?: { ok: boolean; detail?: string };
    api?: { ok: boolean; detail?: string };
    [key: string]: { ok: boolean; detail?: string } | undefined;
  };
  metrics?: {
    playback_requests: number;
    playback_cache_hits: number;
    playback_errors: number;
    playback_latency_total_us: number;
    playback_latency_max_us: number;
    search_requests: number;
    search_errors: number;
    search_latency_total_us: number;
    search_latency_max_us: number;
  };
  errors?: string[];
  warnings?: string[];
	log_file?: string;
	recent_logs?: string[];
}

export interface PlaybackRuntimeStatus {
  active_version?: string;
  previous_version?: string;
  installed_versions: string[];
}

export interface IPTVSourceConfig {
  id: string;
  name: string;
  format: string;
  playlist_url?: string;
  guide_url?: string;
  credential_ref?: string;
  enabled: boolean;
}

export interface IPTVSourceState {
  config: IPTVSourceConfig;
  last_sync_at?: string;
  last_error?: string;
}

export interface IPTVEndpointDiagnostic {
  base_url: string;
  scheme: string;
  host: string;
  port: string;
  resolved_addresses: string[];
  url_valid: boolean;
  dns_resolved: boolean;
  public_target: boolean;
  tcp_reachable: boolean;
  credential_hint: boolean;
  error?: string;
}

export interface IPTVEpisodeRef {
  season: number;
  episode: number;
  has_season: boolean;
  has_episode: boolean;
}

export interface IPTVItem {
  id: string;
  source_id: string;
  source_name?: string;
  kind: 'tv' | 'movie' | 'series' | 'unknown';
  classification?: string;
  title: string;
  raw_title?: string;
  group?: string;
  logo_url?: string;
  stream_url?: string;
  epg_id?: string;
  channel_number?: string;
  language?: string;
  country?: string;
  episode?: IPTVEpisodeRef;
}

export interface IPTVPageResult {
  items: IPTVItem[];
  total_count: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface IPTVItemFilter {
  source_id?: string;
  kind?: IPTVItem['kind'];
  group?: string;
  query?: string;
  page?: number;
  page_size?: number;
  saved_only?: boolean;
}

export interface IPTVGuideProgram {
  channel_id: string;
  title: string;
  description: string;
  start: string;
  end: string;
}

export interface IPTVGuideEntry {
  source_id: string;
  channel_id: string;
  channel_name: string;
  logo_url?: string;
  program: IPTVGuideProgram;
}

export interface IPTVResumeEntry {
  item: IPTVItem;
  position: number;
  duration: number;
  updated_at: string;
}

export interface AccountInfo {
  profile_id: string;
  provider: string;
  provider_subject: string;
  email: string;
  connected_at: string;
  session_persistence: 'keyring' | 'memory';
  credential_state: 'available' | 'memory' | 'unavailable' | 'missing';
  warning?: string;
}

export interface Profile {
  id: string;
  name: string;
  kind: 'persistent' | 'guest';
  created_at: string;
}

export interface LoginStatus {
  state: 'idle' | 'pending' | 'connected' | 'error';
  profile_id?: string;
  warning?: string;
  error?: string;
}

export interface DeviceCodeInfo {
  user_code: string;
  verification_url: string;
  expires_in: number;
}

export interface ImportSubscriptionsResult {
  total_channels: number;
  imported: number;
  failed: number;
  channel_ids: string[];
}
