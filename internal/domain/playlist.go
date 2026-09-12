// Playlists locais (M7/PLY-01). Coleções independentes do YouTube que
// referenciam só metadata e estado local; nada é sincronizado com a conta
// (docs/03-implementation/STORAGE.md, docs/00-product/SCOPE_AND_ROADMAP.md M7).
package domain

import (
	"context"
	"time"
)

// Playlist is a local, ordered collection of videos.
type Playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Color is an optional CSS color (e.g. "#e91e63") used as a visual tag.
	Color     string    `json:"color,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	IsSmart   bool      `json:"is_smart"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// ItemCount é o número de itens (usado pela listagem sem carregar vídeos).
	ItemCount int `json:"item_count"`
}

// PlaylistItem is a video slot inside a playlist. Videos missing from the
// local catalog keep their slot (the item is not deleted), like favorites.
type PlaylistItem struct {
	PlaylistID string
	VideoID    string
	Position   int
	AddedAt    time.Time
}

// PlaylistStore persists playlists and their items
// (docs/03-implementation/STORAGE.md).
type PlaylistStore interface {
	// CreatePlaylist cria uma playlist vazia com os metadados dados e devolve
	// a playlist persistida (com o id gerado).
	CreatePlaylist(ctx context.Context, name, description, color string) (Playlist, error)
	// Playlists lista todas as playlists, ordenadas por data de criação, com
	// ItemCount preenchido.
	Playlists(ctx context.Context) ([]Playlist, error)
	// UpdatePlaylist renomeia/altera a descrição e a cor de uma playlist.
	// Campos vazios são mantidos como estão; só os não vazios mudam.
	UpdatePlaylist(ctx context.Context, id, name, description, color string) error
	// DeletePlaylist apaga a playlist e todos os seus itens.
	DeletePlaylist(ctx context.Context, id string) error
	// DuplicatePlaylist copia uma playlist (nome, descrição, cor e itens, na
	// mesma ordem) para uma nova, devolvendo a nova playlist.
	DuplicatePlaylist(ctx context.Context, id, newName string) (Playlist, error)

	// AddPlaylistItem acrescenta um vídeo ao fim da playlist. Adicionar um
	// vídeo já presente não duplica nem reordena.
	AddPlaylistItem(ctx context.Context, playlistID, videoID string) error
	// RemovePlaylistItem tira um vídeo da playlist. Remover o que não está lá
	// não é erro.
	RemovePlaylistItem(ctx context.Context, playlistID, videoID string) error
	// CopyPlaylistItems copia vários vídeos para outra playlist, sem remover
	// da origem (duplicados no destino são ignorados; operação atômica).
	CopyPlaylistItems(ctx context.Context, fromID, toID string, videoIDs []string) error
	// MovePlaylistItems move vários vídeos para outra playlist (copiar no
	// destino e remover da origem; operação atômica).
	MovePlaylistItems(ctx context.Context, fromID, toID string, videoIDs []string) error
	// RemovePlaylistItems tira vários vídeos da playlist fechando os buracos
	// de posição (operação atômica).
	RemovePlaylistItems(ctx context.Context, playlistID string, videoIDs []string) error
	// PlaylistVideos lista os vídeos da playlist na ordem, com a metadata do
	// catálogo. Vídeos que saíram do catálogo são ignorados (o item persiste).
	PlaylistVideos(ctx context.Context, playlistID string) ([]Video, error)
	// PlaylistItemCount devolve quantos itens tem a playlist.
	PlaylistItemCount(ctx context.Context, playlistID string) (int, error)
	// PlaylistsContaining devolve o conjunto de playlists que já têm o vídeo,
	// para a UI marcar no submenu "Adicionar à playlist".
	PlaylistsContaining(ctx context.Context, videoID string) (map[string]bool, error)
	// MovePlaylistItem move um item para uma nova posição (0-based), empurrando
	// os demais. Delta positivo desce, negativo sobe.
	MovePlaylistItem(ctx context.Context, playlistID, videoID string, delta int) error
	// MovePlaylistItemToIndex reposiciona um item em um índice absoluto da
	// ordem contígua (0..n-1), como o drag-and-drop precisa.
	MovePlaylistItemToIndex(ctx context.Context, playlistID, videoID string, index int) error
	PlaylistTags(ctx context.Context, playlistID string) ([]string, error)
	SetPlaylistTags(ctx context.Context, playlistID string, tags []string) error
	DeduplicatePlaylist(ctx context.Context, playlistID string) (int, error)
}
