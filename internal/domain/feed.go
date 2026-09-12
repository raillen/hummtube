// Tipos de filtro local de feed (Home "Para você" e Inscrições).
// Os filtros são aplicados só sobre o catálogo local, sem chamadas de rede
// (docs/03-implementation/RECOMMENDATIONS.md).
package domain

import (
	"strings"
	"time"
)

// LiveKind é a classificação normalizada do estado de transmissão de um
// vídeo. Os valores brutos vêm de duas fontes com vocabulários diferentes:
//   - yt-dlp:  "is_live", "was_live", "is_upcoming", "post_live", "not_live";
//   - API oficial: "live", "upcoming", "completed", "none".
//
// LiveKind unifica as duas em um vocabulário só para a UI.
type LiveKind string

const (
	// LiveUnknown não existe dado confiável sobre o estado (vazio/desconhecido).
	LiveUnknown LiveKind = "unknown"
	// LiveNow é uma transmissão acontecendo agora.
	LiveNow LiveKind = "live"
	// LiveUpcoming é uma transmissão agendada, ainda não iniciada.
	LiveUpcoming LiveKind = "upcoming"
	// LiveCompleted é uma transmissão encerrada (o vídeo é o VOD dela).
	LiveCompleted LiveKind = "completed"
	// NotLive é um vídeo comum (nunca foi transmissão).
	NotLive LiveKind = "not_live"
)

// liveStatusAliases mapeia os valores brutos das duas fontes para o
// vocabulário normalizado.
var liveStatusAliases = map[string]LiveKind{
	"":            LiveUnknown,
	"none":        NotLive,
	"not_live":    NotLive,
	"is_live":     LiveNow,
	"live":        LiveNow,
	"post_live":   LiveCompleted,
	"was_live":    LiveCompleted,
	"completed":   LiveCompleted,
	"is_upcoming": LiveUpcoming,
	"upcoming":    LiveUpcoming,
}

// LiveKindOf normaliza um valor bruto de live_status de qualquer fonte.
func LiveKindOf(raw string) LiveKind {
	if kind, ok := liveStatusAliases[strings.ToLower(strings.TrimSpace(raw))]; ok {
		return kind
	}
	return LiveUnknown
}

// IsLiveNow informa se o vídeo é uma transmissão acontecendo agora.
func (v Video) IsLiveNow() bool { return LiveKindOf(v.LiveStatus) == LiveNow }

// IsLiveUpcoming informa se o vídeo é uma transmissão agendada.
func (v Video) IsLiveUpcoming() bool { return LiveKindOf(v.LiveStatus) == LiveUpcoming }

// IsLiveCompleted informa se o vídeo é o VOD de uma transmissão encerrada.
func (v Video) IsLiveCompleted() bool { return LiveKindOf(v.LiveStatus) == LiveCompleted }

// IsLive é verdadeiro para qualquer variante de transmissão (agora, agendada
// ou encerrada).
func (v Video) IsLive() bool {
	kind := LiveKindOf(v.LiveStatus)
	return kind == LiveNow || kind == LiveUpcoming || kind == LiveCompleted
}

// FeedContent é o recorte de conteúdo de um filtro de feed.
type FeedContent string

const (
	// FeedAll não restringe o tipo de conteúdo.
	FeedAll FeedContent = "all"
	// FeedRegular mostra só vídeos comuns (nunca transmissão).
	FeedRegular FeedContent = "regular"
	// FeedLive mostra só transmissões (agora, agendadas ou encerradas).
	FeedLive FeedContent = "live"
)

// FeedAge é a janela de data de envio de um filtro de feed.
type FeedAge string

const (
	// FeedAgeAny não restringe a data de envio.
	FeedAgeAny FeedAge = "any"
	// FeedAgeToday restringe ao dia de hoje.
	FeedAgeToday FeedAge = "today"
	// FeedAgeYesterday restringe ao dia civil anterior.
	FeedAgeYesterday FeedAge = "yesterday"
	// FeedAgeWeek restringe aos últimos 7 dias.
	FeedAgeWeek FeedAge = "week"
	// FeedAgeMonth restringe aos últimos 30 dias.
	FeedAgeMonth FeedAge = "month"
	// FeedAgeYear restringe ao último ano.
	FeedAgeYear FeedAge = "year"
)

// FeedFilter é o snapshot local de filtros de um feed (Home "Para você" e
// Inscrições). Nunca é enviado à rede: filtra o que já está no catálogo.
type FeedFilter struct {
	Content  FeedContent
	Age      FeedAge
	Category string
}

// SubscriptionWatchState filtra o estado local de reprodução sem consultar o YouTube.
type SubscriptionWatchState string

const (
	SubscriptionWatchAny       SubscriptionWatchState = "any"
	SubscriptionWatchWatched   SubscriptionWatchState = "watched"
	SubscriptionWatchUnwatched SubscriptionWatchState = "unwatched"
)

// SubscriptionVideoQuery pagina o catálogo persistido de canais inscritos.
// From/To são limites RFC3339 definidos pela UI no fuso local do usuário.
type SubscriptionVideoQuery struct {
	Offset    int                    `json:"offset"`
	Limit     int                    `json:"limit"`
	Content   FeedContent            `json:"content"`
	Category  string                 `json:"category,omitempty"`
	ChannelID string                 `json:"channel_id,omitempty"`
	Watch     SubscriptionWatchState `json:"watch"`
	From      time.Time              `json:"from,omitempty"`
	To        time.Time              `json:"to,omitempty"`
}

type SubscriptionVideoPage struct {
	Videos     []Video  `json:"videos"`
	Categories []string `json:"categories"`
	Total      int      `json:"total"`
	Offset     int      `json:"offset"`
	Limit      int      `json:"limit"`
	HasMore    bool     `json:"has_more"`
}

// ManagedChannel agrega somente projeções locais para organização da conta.
type ManagedChannel struct {
	Channel       Channel   `json:"channel"`
	Category      string    `json:"category,omitempty"`
	Tags          []string  `json:"tags"`
	SubscribedAt  time.Time `json:"subscribed_at,omitempty"`
	LastWatchedAt time.Time `json:"last_watched_at,omitempty"`
}

// Matches decide se o vídeo passa pelo filtro. Category compara sem caixa
// (os nomes de categoria vêm da API e podem mudar de caixa entre sincronias).
func (f FeedFilter) Matches(v Video, now time.Time) bool {
	if !f.matchesContent(v) {
		return false
	}
	if !f.matchesAge(v, now) {
		return false
	}
	if f.Category != "" && !strings.EqualFold(v.Category, f.Category) {
		return false
	}
	return true
}

func (f FeedFilter) matchesContent(v Video) bool {
	switch f.Content {
	case FeedRegular:
		return !v.IsLive()
	case FeedLive:
		return v.IsLive()
	default:
		return true
	}
}

func (f FeedFilter) matchesAge(v Video, now time.Time) bool {
	if f.Age == FeedAgeAny || v.PublishedAt.IsZero() {
		return true
	}
	since := now.Sub(v.PublishedAt)
	if since < 0 {
		since = 0
	}
	switch f.Age {
	case FeedAgeToday:
		return since < 24*time.Hour
	case FeedAgeWeek:
		return since < 7*24*time.Hour
	case FeedAgeMonth:
		return since < 30*24*time.Hour
	case FeedAgeYear:
		return since < 365*24*time.Hour
	default:
		return true
	}
}

// IsZero informa se nenhum filtro está ativo. Valores vazios equivalem aos
// defaults ("all" e "any") para tolerar filtros recém-criados.
func (f FeedFilter) IsZero() bool {
	content := f.Content
	if content == "" {
		content = FeedAll
	}
	age := f.Age
	if age == "" {
		age = FeedAgeAny
	}
	return content == FeedAll && age == FeedAgeAny && f.Category == ""
}

// ApplyFeedFilter devolve a sublista de videos que passa pelo filtro,
// preservando a ordem original.
func ApplyFeedFilter(filter FeedFilter, videos []Video, now time.Time) []Video {
	if filter.IsZero() {
		return videos
	}
	out := make([]Video, 0, len(videos))
	for _, v := range videos {
		if filter.Matches(v, now) {
			out = append(out, v)
		}
	}
	return out
}

// FeedCategories lista as categorias distintas presentes em videos, na ordem
// de aparição e sem caixa duplicada. Vazio quando o catálogo não tem
// categoria alguma.
func FeedCategories(videos []Video) []string {
	seen := make(map[string]bool, 8)
	out := make([]string, 0, 8)
	for _, v := range videos {
		name := strings.TrimSpace(v.Category)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out
}
