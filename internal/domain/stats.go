// Estatísticas locais (M9/QOL-04): números calculados sob demanda a partir do
// estado local. Nada aqui sai da máquina.
package domain

import "time"

// LocalStats agrega contadores locais de uso. Tudo é derivado do SQLite local
// (docs/00-product/SCOPE_AND_ROADMAP.md, QOL-04).
type LocalStats struct {
	// VideosWatched é quantos vídeos foram assistidos até o fim (completed).
	VideosWatched int `json:"videos_watched"`
	// TotalWatchTime é o tempo total de reprodução: duração dos assistidos
	// mais a posição dos em andamento.
	TotalWatchTime time.Duration `json:"watch_time"`
	// HistoryCount é quantas linhas de progresso existem (assistidos + em
	// andamento).
	HistoryCount int `json:"history_count"`
	// FavoritesCount é quantos vídeos estão salvos como favorito.
	FavoritesCount int `json:"favorites_count"`
	// PlaylistsCount e PlaylistItems são playlists locais e seus itens.
	PlaylistsCount int `json:"playlists_count"`
	PlaylistItems  int `json:"playlist_items"`
	// SubscriptionsCount é quantos canais estão inscritos.
	SubscriptionsCount int `json:"subscriptions_count"`
	// ChannelFavoritesCount é quantos canais foram favoritados localmente.
	ChannelFavoritesCount int `json:"channel_favorites_count"`
	// FoldersCount é quantas pastas locais de canais existem.
	FoldersCount int `json:"folders_count"`
	// FeedbackCount é quantos sinais de feedback de recomendação existem.
	FeedbackCount int `json:"feedback_count"`
}

// PersonalData é o dump de dados pessoais exportáveis (QOL-04). Tem um
// formato versionado para o app conseguir evoluir sem quebrar arquivos antigos.
type PersonalData struct {
	Format     string           `json:"format"`
	Version    int              `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	History    []HistoryEntry   `json:"history,omitempty"`
	Favorites  []string         `json:"favorites,omitempty"`
	Playlists  []PlaylistExport `json:"playlists,omitempty"`
	// ChannelFavorites são os ids de canais favoritados localmente.
	ChannelFavorites []string `json:"channel_favorites,omitempty"`
	// Folders são as pastas locais e os canais que elas contêm.
	Folders []FolderExport `json:"folders,omitempty"`
	// Feedback são os sinais de recomendação registrados.
	Feedback []RecommendationFeedback `json:"feedback,omitempty"`
}

// HistoryEntry é uma linha do histórico de reprodução no formato de exportação.
type HistoryEntry struct {
	VideoID   string    `json:"video_id"`
	Title     string    `json:"title"`
	ChannelID string    `json:"channel_id,omitempty"`
	Position  int64     `json:"position_ms"`
	Duration  int64     `json:"duration_ms"`
	Completed bool      `json:"completed"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PlaylistExport é uma playlist local e seus itens no formato de exportação.
type PlaylistExport struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Color       string    `json:"color,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	VideoIDs    []string  `json:"video_ids,omitempty"`
}

// FolderExport é uma pasta local de canais e seus membros no formato de
// exportação.
type FolderExport struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	ChannelIDs []string  `json:"channel_ids,omitempty"`
}
