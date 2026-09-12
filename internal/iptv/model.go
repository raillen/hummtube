package iptv

import "time"

// ContentKind representa a categoria semântica normalizada de uma entrada da
// fonte. Unknown é deliberadamente permitido: uma lista M3U mal classificada
// não deve ser transformada em uma categoria incorreta por heurística.
type ContentKind string

const (
	ContentKindUnknown ContentKind = "unknown"
	ContentKindTV      ContentKind = "tv"
	ContentKindMovie   ContentKind = "movie"
	ContentKindSeries  ContentKind = "series"
)

// ClassificationSource informa por que a categoria foi escolhida.
type ClassificationSource string

const (
	ClassificationUnknown   ClassificationSource = "unknown"
	ClassificationAttribute ClassificationSource = "attribute"
	ClassificationGroup     ClassificationSource = "group"
	ClassificationTitle     ClassificationSource = "title"
)

// SourceFormat identifica o formato de catálogo usado por uma fonte.
type SourceFormat string

const (
	SourceFormatM3U SourceFormat = "m3u"
)

// SourceConfig contém apenas configuração pública e referências a segredos.
// Credenciais não fazem parte deste contrato e devem ser resolvidas por um
// SecretStore no adapter de rede.
type SourceConfig struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Format        SourceFormat `json:"format"`
	PlaylistURL   string       `json:"playlist_url"`
	GuideURL      string       `json:"guide_url"`
	CredentialRef string       `json:"credential_ref"`
	Enabled       bool         `json:"enabled"`
}

// SourceState combines public source configuration with synchronization
// status. Credentials remain outside this model and are referenced only by
// CredentialRef.
type SourceState struct {
	Config     SourceConfig `json:"config"`
	LastSyncAt time.Time    `json:"last_sync_at"`
	LastError  string       `json:"last_error"`
}

// EpisodeRef preserva a informação de temporada/episódio quando ela aparece
// no nome da entrada. Os booleanos distinguem ausência de informação de zero.
type EpisodeRef struct {
	Season     int  `json:"season"`
	Episode    int  `json:"episode"`
	HasSeason  bool `json:"has_season"`
	HasEpisode bool `json:"has_episode"`
}

// Item é o modelo comum consumido posteriormente pelas telas, busca e
// MediaCore. O parser não tenta enriquecer a obra com TMDB nem resolve o
// stream; ele apenas normaliza a entrada fornecida pelo provider.
type Item struct {
	ID             string               `json:"id"`
	SourceID       string               `json:"source_id"`
	SourceName     string               `json:"source_name"`
	Kind           ContentKind          `json:"kind"`
	Classification ClassificationSource `json:"classification"`
	Title          string               `json:"title"`
	RawTitle       string               `json:"raw_title"`
	Group          string               `json:"group"`
	LogoURL        string               `json:"logo_url"`
	StreamURL      string               `json:"stream_url"`
	EPGID          string               `json:"epg_id"`
	ChannelNumber  string               `json:"channel_number"`
	Language       string               `json:"language"`
	Country        string               `json:"country"`
	Episode        EpisodeRef           `json:"episode"`
}

// ParseWarning permite importar uma lista parcialmente válida sem esconder
// linhas problemáticas do diagnóstico.
type ParseWarning struct {
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Playlist é o resultado de uma importação M3U/M3U+.
type Playlist struct {
	Items    []Item         `json:"items"`
	Warnings []ParseWarning `json:"warnings"`
}

type GuideChannel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	LogoURL string `json:"logo_url"`
}

type GuideProgram struct {
	ChannelID   string    `json:"channel_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
}

// GuideEntry joins a program with the channel metadata needed by the UI.
type GuideEntry struct {
	SourceID    string       `json:"source_id"`
	ChannelID   string       `json:"channel_id"`
	ChannelName string       `json:"channel_name"`
	LogoURL     string       `json:"logo_url"`
	Program     GuideProgram `json:"program"`
}

// ParseOptions controla limites de memória e a identidade da fonte. O parser
// nunca abre URLs; o caller deve fornecer um reader já obtido pelo adapter.
type ParseOptions struct {
	SourceID     string `json:"source_id"`
	SourceName   string `json:"source_name"`
	MaxItems     int    `json:"max_items"`
	MaxLineBytes int    `json:"max_line_bytes"`
	// BatchSize controla quantos itens o parser acumula antes de entregar
	// ao handler no modo streaming. Valores <= 0 usam o padrão.
	BatchSize int `json:"batch_size"`
}

// ItemFilter define parâmetros de busca, grupo e paginação para o catálogo.
type ItemFilter struct {
	SourceID  string      `json:"source_id"`
	Kind      ContentKind `json:"kind"`
	Group     string      `json:"group"`
	Query     string      `json:"query"`
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
	SavedOnly bool        `json:"saved_only"`
}

// PageResult encapsula uma página de itens do catálogo com totais.
type PageResult struct {
	Items      []Item `json:"items"`
	TotalCount int    `json:"total_count"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
}

// PlaybackPosition é a posição local de um item VOD, usada para retomar a
// reprodução. Só faz sentido para conteúdo com duração; TV ao vivo nunca
// grava progresso.
type PlaybackPosition struct {
	ItemID    string        `json:"item_id"`
	Position  time.Duration `json:"position"`
	Duration  time.Duration `json:"duration"`
	Completed bool          `json:"completed"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ResumeEntry junta o progresso salvo ao item do catálogo para alimentar a
// linha "Continuar assistindo" da Home sem nova consulta por card.
type ResumeEntry struct {
	Item      Item          `json:"item"`
	Position  time.Duration `json:"position"`
	Duration  time.Duration `json:"duration"`
	UpdatedAt time.Time     `json:"updated_at"`
}
