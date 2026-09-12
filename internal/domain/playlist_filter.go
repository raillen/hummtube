// Filtros e ordenação do detalhe de uma playlist (M7/PLY-04).
// Os filtros são aplicados no SQLite (antes do LIMIT) quando o usuário os
// combina; o filtro de termo continua em memória no padrão da grade. Nada aqui
// é enviado ao YouTube: tudo deriva do catálogo e do estado local.
package domain

import "time"

// PlaylistSortKey identifica a ordem de exibição do detalhe da playlist.
// A chave "manual" é a ordem de posição persistida; as demais são apenas
// visuais e não alteram playlist_items.position.
type PlaylistSortKey string

const (
	// PlaylistSortManual é a ordem de posição (default).
	PlaylistSortManual PlaylistSortKey = "manual"
	// PlaylistSortTitle ordena pelo título, sem diferenciar maiúsculas.
	PlaylistSortTitle PlaylistSortKey = "title"
	// PlaylistSortChannel ordena pelo nome do canal, sem diferenciar maiúsculas.
	PlaylistSortChannel PlaylistSortKey = "channel"
	// PlaylistSortPublished ordena pela data de publicação, mais recente primeiro.
	PlaylistSortPublished PlaylistSortKey = "published"
	// PlaylistSortDuration ordena pela duração, crescente.
	PlaylistSortDuration PlaylistSortKey = "duration"
	// PlaylistSortAdded ordena pela data de inclusão na playlist.
	PlaylistSortAdded PlaylistSortKey = "added"
)

// PlaylistFilter é o snapshot combinável de filtros do detalhe de uma playlist.
// Valores vazios equivalem a "qualquer"/"todos"; combinações usam AND. Reaproveita
// os vocabulários de busca (SearchDuration, SearchWatchState) e de feed (FeedAge,
// FeedContent) para a UI não manter enums paralelos.
type PlaylistFilter struct {
	// Channel restringe por nome do canal (parcial, sem diferenciar caixa).
	Channel string
	// Duration é o recorte de duração; quando SearchDurationCustom, os campos
	// MinDuration/MaxDuration valem.
	Duration SearchDuration
	// MinDuration/MaxDuration delimitam o recorte customizado de duração.
	MinDuration time.Duration
	MaxDuration time.Duration
	// Age restringe pela janela de publicação (FeedAgeAny = sem restrição).
	Age FeedAge
	// Category restringe pela categoria exata, sem diferenciar caixa.
	Category string
	// Watched restringe pelo estado de reprodução (SearchWatchAny = sem
	// restrição; Unwatched/Watched/Continue espelham a busca local).
	Watched SearchWatchState
	// Favorite restringe a vídeos marcados como favoritos.
	Favorite bool
	// Content restringe pelo tipo de transmissão (FeedAll = sem restrição).
	Content FeedContent
	// ShortsOnly restringe a vídeos curtos (duração <= 3 minutos, mesma
	// aproximação da busca local).
	ShortsOnly bool
}

// IsZero informa se nenhum filtro está ativo. Duration customizada sem
// limites equivale a "qualquer"; os valores "any"/"all" explícitos (recém
// lidos dos combos) também.
func (f PlaylistFilter) IsZero() bool {
	duration := f.Duration
	if duration == SearchDurationCustom && f.MinDuration <= 0 && f.MaxDuration <= 0 {
		duration = ""
	}
	duration = trimAny(duration, SearchDurationAny)
	age := trimAny(f.Age, FeedAgeAny)
	watched := trimAny(f.Watched, SearchWatchAny)
	content := trimAny(f.Content, FeedAll)
	return f.Channel == "" &&
		duration == "" && f.MinDuration <= 0 && f.MaxDuration <= 0 &&
		age == "" && f.Category == "" &&
		watched == "" && !f.Favorite && content == "" && !f.ShortsOnly
}

// trimAny devolve "" quando s é vazio ou igual a vazio-equivalente (ex.: o
// "any" de um combo).
func trimAny[T ~string](s, emptyValue T) T {
	if s == "" || s == emptyValue {
		return ""
	}
	return s
}

// PlaylistFilterActiveCount devolve quantas dimensões de filtro estão ativas,
// para o indicador "N filtros ativos" da barra.
func (f PlaylistFilter) ActiveCount() int {
	n := 0
	if f.Channel != "" {
		n++
	}
	if d := trimAny(f.Duration, SearchDurationAny); d != "" {
		n++
	}
	if a := trimAny(f.Age, FeedAgeAny); a != "" {
		n++
	}
	if f.Category != "" {
		n++
	}
	if w := trimAny(f.Watched, SearchWatchAny); w != "" {
		n++
	}
	if f.Favorite {
		n++
	}
	if c := trimAny(f.Content, FeedAll); c != "" {
		n++
	}
	if f.ShortsOnly {
		n++
	}
	return n
}

// PlaylistFilterLimit é o teto de itens devolvidos por PlaylistVideosFiltered.
// Filtros e ordenação são aplicados antes deste LIMIT (aceite do PLY-04); a
// página renderiza a lista inteira que recebe, então o teto só protege a
// consulta de explodir em playlists enormes.
const PlaylistFilterLimit = 200
