package domain

import "time"

// QueueItemState diferencia itens aguardando reprodução dos que já tocaram.
type QueueItemState string

const (
	QueueItemPending QueueItemState = "pending"
	QueueItemPlayed  QueueItemState = "played"
)

// QueueItem é a projeção persistente da fila consumida pelo player.
type QueueItem struct {
	ID       string         `json:"id"`
	Video    Video          `json:"video"`
	Position int            `json:"position"`
	State    QueueItemState `json:"state"`
	AddedAt  time.Time      `json:"added_at"`
	PlayedAt time.Time      `json:"played_at,omitempty"`
}

// QueuePreferences controla o avanço automático sem esconder itens tocados.
type QueuePreferences struct {
	Autoplay     bool `json:"autoplay"`
	RemovePlayed bool `json:"remove_played"`
}

// QueueSnapshot mantém itens e preferências consistentes na mesma carga.
type QueueSnapshot struct {
	Items       []QueueItem      `json:"items"`
	Preferences QueuePreferences `json:"preferences"`
}
