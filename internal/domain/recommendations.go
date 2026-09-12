// Recommendation domain types
// (docs/03-implementation/RECOMMENDATIONS.md).
package domain

import "time"

// TopicScore is a learned interest in a topic, normalized to 0..1.
type TopicScore struct {
	Topic string
	Score float64
}

// ChannelAffinity expresses how relevant a channel is to the user, 0..1.
type ChannelAffinity struct {
	ChannelID      string
	ChannelName    string
	Score          float64
	Subscribed     bool
	Favorite       bool
	CompletedCount int
	WatchVolume    float64
	LastWatched    time.Time
}

// ProgressInfo is the resume state used to build "Continue watching".
type ProgressInfo struct {
	Position  time.Duration
	Duration  time.Duration
	UpdatedAt time.Time
	Completed bool
}

// InterestProfile is the local, explainable user profile. Everything in it is
// derived from local SQLite state; nothing leaves the machine.
type InterestProfile struct {
	Topics             []TopicScore
	Channels           map[string]ChannelAffinity
	WatchState         map[string]ProgressInfo
	ExcludedVideoIDs   map[string]bool
	ExcludedChannelIDs map[string]bool
}

// HomeVideo is a recommended video with its deterministic score and the
// top reasons it was chosen.
type HomeVideo struct {
	Video   Video
	Score   float64
	Reasons []string
}

// HomeSection is a labelled row/section of the Home.
type HomeSection struct {
	ID     string
	Title  string
	Videos []HomeVideo
}

// HomeModel is the assembled local Home
// (docs/03-implementation/RECOMMENDATIONS.md).
type HomeModel struct {
	ContinueWatching    []HomeVideo
	ForYou              []HomeVideo
	TopicSections       []HomeSection
	RecentSubscriptions []HomeVideo
	Rediscovery         []HomeVideo
}

// FeedbackAction é uma opinião explícita do usuário sobre uma recomendação.
//
// Só existem aqui as ações que o perfil realmente consome
// (`Repository.BuildProfile`): exclusões de vídeo/canal e os ajustes de tema,
// que alteram `interest_topics` — a mesma tabela que o ranking lê. Gravar um
// sinal que nenhum ranking lê seria um controle falso na interface.
type FeedbackAction string

const (
	// FeedbackHideVideo remove um vídeo específico das recomendações.
	FeedbackHideVideo FeedbackAction = "hide_video"
	// FeedbackDontRecommend é equivalente a HideVideo, vindo de um "não
	// recomendar" explícito.
	FeedbackDontRecommend FeedbackAction = "dont_recommend"
	// FeedbackIgnoreChannel remove um canal inteiro das recomendações.
	FeedbackIgnoreChannel FeedbackAction = "ignore_channel"
	// FeedbackMoreTopic reforça um tema (+1 em interest_topics por ocorrência).
	FeedbackMoreTopic FeedbackAction = "more_topic"
	// FeedbackLessTopic reduz um tema (−1 em interest_topics por ocorrência;
	// em 0 o tema sai do perfil).
	FeedbackLessTopic FeedbackAction = "less_topic"
)

// RecommendationFeedback é o registro local dessa opinião. Nada disso sai da
// máquina (docs/03-implementation/RECOMMENDATIONS.md).
type RecommendationFeedback struct {
	VideoID   string
	ChannelID string
	Topic     string
	Action    FeedbackAction
}

// RecommendationReason is a deterministic, human-readable factor.
type RecommendationReason struct {
	Label string
	Kind  string
}

// RecommendationExplanation lists why a video was recommended.
type RecommendationExplanation struct {
	VideoID string
	Score   float64
	Reasons []RecommendationReason
}
