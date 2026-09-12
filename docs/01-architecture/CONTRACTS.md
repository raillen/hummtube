---
id: contracts
status: canonical
---

# Contratos e Interfaces — NanoTube Web

## 1. Wails v3 Service Contracts (Frontend ↔ Backend)

`Settings.GetResourceUsage` retorna `rss_bytes`, `cpu_percent`,
`receive_bytes_per_second` e `transmit_bytes_per_second`, todos anuláveis.
RSS mede o processo Go; CPU e rede medem o sistema Linux, sem loopback na rede.
Não há valores simulados: plataformas sem suporte, falhas e taxas ainda sem
segunda amostra retornam `null`. O sampler reutiliza leituras por 3 segundos;
o frontend consulta sequencialmente com intervalo de 5 segundos após conclusão,
somente com o cartão visível. Detalhes em `DIAGNOSTICS_SPEC.md`.

### CatalogService
```go
type CatalogService interface {
    GetHome(ctx context.Context, profileID string) (HomeModel, error)
    RefreshSubscriptions(ctx context.Context) (SyncSummary, error)
    GetChannels(ctx context.Context) ([]Channel, error)
    RememberVideo(ctx context.Context, video Video) error
    GetChannelFolders(ctx context.Context) ([]ChannelFolder, error)
    CreateChannelFolder(ctx context.Context, name string) (ChannelFolder, error)
    DeleteChannelFolder(ctx context.Context, id string) error
    SetChannelFolder(ctx context.Context, channelID, folderID string) error
    ToggleFavoriteChannel(ctx context.Context, channelID string) (bool, error)
}
```

### PlayerService
```go
type PlayerService interface {
    ResolveMedia(ctx context.Context, req PlaybackRequest) (PlaybackPlan, error)
    SaveProgress(ctx context.Context, videoID string, positionMs int64, durationMs int64, completed bool) error
    GetProgress(ctx context.Context, videoID string) (PlaybackProgress, error)
    GetAudioDevices(ctx context.Context) ([]AudioDevice, error)
}
```

Em planos `resolved-media`, `ResolveMedia` devolve referências locais opacas
(`/api/media/<token>`). O backend mantém a URL assinada, os headers permitidos e
o prazo de expiração; o endpoint aceita somente `GET`/`HEAD`, Range e headers
de negociação necessários. O token expira em poucos minutos e nunca carrega
cookies ou `Authorization` do perfil.

### SearchService
```go
type SearchService interface {
    Search(ctx context.Context, opts SearchOptions) (SearchPage, error)
    GetSuggestions(ctx context.Context, query string) ([]string, error)
}
```

No RPC HTTP/Wails, `Search` aceita `SearchOptions` como primeiro argumento. A assinatura histórica `Search(query, limit, offset)` permanece suportada; clientes novos devem enviar o request tipado e tratar `next_page_token` como opaco.

### PlaylistService
```go
type PlaylistService interface {
    ListPlaylists(ctx context.Context) ([]Playlist, error)
    GetPlaylist(ctx context.Context, id string) (PlaylistDetail, error)
    CreatePlaylist(ctx context.Context, name, description, color string) (Playlist, error)
    DeletePlaylist(ctx context.Context, id string) error
    AddVideoToPlaylist(ctx context.Context, playlistID, videoID string) error
    RemoveVideoFromPlaylist(ctx context.Context, playlistID, videoID string) error
    ReorderPlaylistItems(ctx context.Context, playlistID string, videoIDs []string) error
    SaveSmartRule(ctx context.Context, playlistID string, rule SmartRule) error
    GetRemotePlaylistPage(ctx context.Context, playlistID, pageToken string, limit int) (RemotePlaylistPage, error)
    ImportRemotePlaylist(ctx context.Context, playlistID, name, description string, maxItems int) (PlaylistDetail, error)
    AddRemotePlaylistToPlaylist(ctx context.Context, remoteID, targetID string, maxItems int) (int, error)
    UpdatePlaylist(ctx context.Context, playlistID, name, description, color string) error
    MergePlaylist(ctx context.Context, sourceID, targetID string) error
}
```

`GetRemotePlaylistPage` não persiste a lista. `ImportRemotePlaylist` é a conversão explícita para uma playlist local e persiste os metadados necessários antes das chaves estrangeiras. `AddRemotePlaylistToPlaylist` e `MergePlaylist` descartam IDs duplicados transacionalmente.

### QueueService
```go
type QueueService interface {
    GetQueue(ctx context.Context) (QueueSnapshot, error)
    EnqueueVideo(ctx context.Context, video Video) (QueueItem, error)
    ReorderQueue(ctx context.Context, itemIDs []string) error
    RemoveQueueItem(ctx context.Context, itemID string) error
    MarkQueueItemPlayed(ctx context.Context, itemID string, played bool) error
    ClearQueue(ctx context.Context, scope string) error
    SaveQueuePreferences(ctx context.Context, preferences QueuePreferences) error
    SaveQueueAsPlaylist(ctx context.Context, name string) (Playlist, error)
}
```

`ReorderQueue` exige a lista completa de IDs da fila ativa e rejeita IDs ausentes, duplicados ou externos ao perfil. `ClearQueue` aceita somente `all` ou `played`. A conversão em playlist é transacional.

### Feed de inscrições e gerenciamento de canais

`CatalogService.ListSubscriptionVideos` recebe `SubscriptionVideoQuery` com paginação, intervalo de data, conteúdo, categoria, canal e estado assistido. O gerenciamento separado expõe `ListManagedChannels`, ações em lote de desinscrição/favorito/pasta e `SetChannelTags`; todas as mutações são limitadas ao perfil ativo.

### LibraryService
```go
type LibraryService interface {
    GetFavorites(ctx context.Context) ([]Video, error)
    ToggleFavorite(ctx context.Context, videoID string) (bool, error)
    GetHistory(ctx context.Context, limit, offset int) ([]Video, error)
    ClearHistory(ctx context.Context) error
    GetNotes(ctx context.Context, videoID string) (VideoNotes, error)
    SaveNotes(ctx context.Context, videoID, notes string, bookmarks []Bookmark) error
}
```

### IPTVService
```go
type IPTVService interface {
    ListIPTVSources(ctx context.Context) ([]IPTVSourceState, error)
    SaveIPTVSource(ctx context.Context, id, name, playlistURL, guideURL string, enabled bool) error
    SaveIPTVSourceWithCredentials(ctx context.Context, id, name, playlistURL, guideURL string, enabled bool, username, password string, clearCredentials bool, outputMode string) error
    DiagnoseIPTVEndpoint(ctx context.Context, endpoint string) (IPTVEndpointDiagnostic, error)
    DeleteIPTVSource(ctx context.Context, id string) error
    SyncIPTVSource(ctx context.Context, sourceID string) (int, error)
    ListIPTVItemsPaginated(ctx context.Context, filter IPTVItemFilter) (IPTVPageResult, error)
    ListIPTVGroups(ctx context.Context, kind string) ([]string, error)
    ListIPTVGuide(ctx context.Context, sourceID string, limit int) ([]IPTVGuideEntry, error)
    SetIPTVItemSaved(ctx context.Context, itemID string, saved bool) error
    ListIPTVSavedItems(ctx context.Context, limit int) ([]IPTVItem, error)
    ResolveIPTVStream(ctx context.Context, sourceID, itemID string) (PlaybackPlan, error)
    SaveIPTVPlaybackProgress(ctx context.Context, itemID string, positionMs, durationMs int64, completed bool) error
    ListIPTVResume(ctx context.Context, limit int) ([]IPTVResumeEntry, error)
}
```

### AccountService
```go
type AccountService interface {
    GetAccount(ctx context.Context) (AccountInfo, bool, error)
    GetLoginStatus(ctx context.Context) LoginStatus
    ListProfiles(ctx context.Context) ([]Profile, error)
    GetActiveProfile(ctx context.Context) (Profile, error)
    CreateProfile(ctx context.Context, name string) (Profile, error)
    CreateGuestProfile(ctx context.Context, name string) (Profile, error)
    SetActiveProfile(ctx context.Context, profileID string) error
    DeleteProfile(ctx context.Context, profileID string) error
    StartGoogleLogin(ctx context.Context) (string, error)
    StartDeviceLogin(ctx context.Context) (DeviceCodeInfo, error)
    DisconnectAccount(ctx context.Context) error
}
```

`AccountInfo` expõe `profile_id`, `provider`, `provider_subject`, `email`, `connected_at`, `session_persistence`, `credential_state` (`available|memory|unavailable|missing`) e, quando necessário, `warning`. Nenhum token atravessa o RPC. `DeviceCodeInfo` expõe somente `user_code`, `verification_url` e `expires_in`; o `device_code` permanece no backend. `Repository.ActiveProfileID(ctx)` é o contrato interno para serviços que persistem dados pessoais e sempre devolve ao menos `default`.

`Settings.GetSecretInventory` devolve apenas `SecretMetadata` e os últimos eventos de auditoria do perfil ativo. Datas ausentes significam que o provedor não publica expiração/rotação; nunca são inferidas a partir do valor secreto.

`Settings.GetPlaybackRuntime` devolve somente `active_version`, `previous_version` e `installed_versions`. `ActivatePlaybackRuntime` e `RollbackPlaybackRuntime` operam sobre versões locais já instaladas e validadas por manifesto/hash; nenhum caminho, hash bruto, token ou conteúdo executável atravessa o RPC. A ausência de um runtime gerenciado não impede o fallback controlado para o sistema.

### LastFMService (Beta)

`GetLastFMStatus`, `StartLastFMAuthorization`, `CompleteLastFMAuthorization`, `DisconnectLastFM` e `ScrobbleLastFM` formam um fluxo opt-in. O token temporário e a session key nunca atravessam o RPC; somente URL pública de autorização, status e nome do usuário são expostos. O player dispara uma única tentativa ao atingir 50% ou quatro minutos, o que ocorrer primeiro; o backend revalida o limiar e ignora chamadas quando a integração não está configurada ou conectada.

### Contrato JSON do frontend

O RPC HTTP/Wails usa `snake_case` em todos os DTOs consumidos pelo Svelte. Tipos de domínio expostos diretamente precisam declarar tags `json` explícitas; a capitalização padrão do `encoding/json` não faz parte do contrato. Operações remotas que criam referências locais devem chamar `CatalogService.RememberVideo` antes de salvar progresso, favorito ou item de playlist.

Falhas de transporte ou do backend são erros visíveis. Respostas simuladas só são permitidas no modo Vite quando `VITE_ENABLE_MOCKS=true`; nunca existe fallback silencioso para mocks em uma aplicação empacotada.

---

## 2. Core Domain Ports (Backend Internal)

### PlaybackResolver
```go
type PlaybackResolver interface {
    Resolve(ctx context.Context, req PlaybackRequest) (PlaybackPlan, error)
}
```

### RecommendationEngine
```go
type RecommendationEngine interface {
    BuildHome(ctx context.Context, profile InterestProfile, candidates []Video) (HomeModel, error)
    Explain(videoID string) RecommendationExplanation
}
```

### VideoRepository & Storage
```go
type VideoRepository interface {
    UpsertVideos(ctx context.Context, videos []Video) error
    MarkProgress(ctx context.Context, videoID string, progress PlaybackProgress) error
    Recent(ctx context.Context, filter VideoFilter) ([]Video, error)
    SearchFTS(ctx context.Context, query string, limit int) ([]SearchHit, error)
}
```
