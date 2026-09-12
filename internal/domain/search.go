package domain

import "time"

// SearchResourceType identifies the kind of item returned by a remote search.
// Empty in SearchOptions means every resource type; an empty ResourceType on
// a persisted Video still means an ordinary video for catalog compatibility.
type SearchResourceType string

const (
	SearchResourceVideo    SearchResourceType = "video"
	SearchResourceChannel  SearchResourceType = "channel"
	SearchResourcePlaylist SearchResourceType = "playlist"
)

type SearchOrder string

const (
	SearchOrderRelevance  SearchOrder = "relevance"
	SearchOrderDate       SearchOrder = "date"
	SearchOrderViewCount  SearchOrder = "viewCount"
	SearchOrderRating     SearchOrder = "rating"
	SearchOrderTitle      SearchOrder = "title"
	SearchOrderVideoCount SearchOrder = "videoCount"
)

type SearchDuration string

const (
	SearchDurationAny    SearchDuration = "any"
	SearchDurationShort  SearchDuration = "short"
	SearchDurationMedium SearchDuration = "medium"
	SearchDurationLong   SearchDuration = "long"
	SearchDurationCustom SearchDuration = "custom"
)

type SearchEventType string

const (
	SearchEventAny       SearchEventType = ""
	SearchEventLive      SearchEventType = "live"
	SearchEventCompleted SearchEventType = "completed"
	SearchEventUpcoming  SearchEventType = "upcoming"
)

type SearchCaption string

const (
	SearchCaptionAny    SearchCaption = "any"
	SearchCaptionClosed SearchCaption = "closedCaption"
	SearchCaptionNone   SearchCaption = "none"
)

type SearchDefinition string

const (
	SearchDefinitionAny      SearchDefinition = "any"
	SearchDefinitionHigh     SearchDefinition = "high"
	SearchDefinitionStandard SearchDefinition = "standard"
)

type SearchDimension string

const (
	SearchDimensionAny SearchDimension = "any"
	SearchDimension2D  SearchDimension = "2d"
	SearchDimension3D  SearchDimension = "3d"
)

type SearchLicense string

const (
	SearchLicenseAny            SearchLicense = "any"
	SearchLicenseYouTube        SearchLicense = "youtube"
	SearchLicenseCreativeCommon SearchLicense = "creativeCommon"
)

type SearchVideoType string

const (
	SearchVideoAny     SearchVideoType = "any"
	SearchVideoMovie   SearchVideoType = "movie"
	SearchVideoEpisode SearchVideoType = "episode"
)

type SearchSafe string

const (
	SearchSafeAny      SearchSafe = "any"
	SearchSafeModerate SearchSafe = "moderate"
	SearchSafeNone     SearchSafe = "none"
	SearchSafeStrict   SearchSafe = "strict"
)

// SearchTriState represents a server filter that can be omitted, required or
// explicitly rejected. Not every provider supports all three states; adapters
// must return a capability error instead of silently changing the meaning.
type SearchTriState string

const (
	SearchAny SearchTriState = "any"
	SearchYes SearchTriState = "yes"
	SearchNo  SearchTriState = "no"
)

type SearchWatchState string

const (
	SearchWatchAny       SearchWatchState = "any"
	SearchWatchUnwatched SearchWatchState = "unwatched"
	SearchWatchWatched   SearchWatchState = "watched"
	SearchWatchContinue  SearchWatchState = "continue"
)

type SearchSavedState string

const (
	SearchSavedAny       SearchSavedState = "any"
	SearchSavedFavorites SearchSavedState = "favorites"
	SearchSavedQueue     SearchSavedState = "queue"
	SearchSavedNotSaved  SearchSavedState = "not-saved"
)

// SearchOptions is the toolkit- and provider-independent search contract.
// Provider filters are applied by the remote adapter. Personal filters are
// applied locally after the response and never send viewing state to YouTube.
type SearchOptions struct {
	Query         string               `json:"query"`
	ExactPhrase   string               `json:"exact_phrase,omitempty"`
	IncludeTerms  string               `json:"include_terms,omitempty"`
	ExcludeTerms  string               `json:"exclude_terms,omitempty"`
	MaxResults    int                  `json:"max_results,omitempty"`
	PageToken     string               `json:"page_token,omitempty"`
	ResourceTypes []SearchResourceType `json:"resource_types,omitempty"`
	Order         SearchOrder          `json:"order,omitempty"`

	PublishedAfter  time.Time       `json:"published_after,omitempty"`
	PublishedBefore time.Time       `json:"published_before,omitempty"`
	Duration        SearchDuration  `json:"duration,omitempty"`
	MinDuration     time.Duration   `json:"min_duration,omitempty"`
	MaxDuration     time.Duration   `json:"max_duration,omitempty"`
	ShortsOnly      bool            `json:"shorts_only,omitempty"`
	RegularOnly     bool            `json:"regular_only,omitempty"`
	EventType       SearchEventType `json:"event_type,omitempty"`

	ChannelID      string           `json:"channel_id,omitempty"`
	OnlySubscribed bool             `json:"only_subscribed,omitempty"`
	Caption        SearchCaption    `json:"caption,omitempty"`
	WatchState     SearchWatchState `json:"watch_state,omitempty"`
	SavedState     SearchSavedState `json:"saved_state,omitempty"`
	HideRejected   bool             `json:"hide_rejected,omitempty"`

	Definition        SearchDefinition `json:"definition,omitempty"`
	Dimension         SearchDimension  `json:"dimension,omitempty"`
	License           SearchLicense    `json:"license,omitempty"`
	Embeddable        SearchTriState   `json:"embeddable,omitempty"`
	Syndicated        SearchTriState   `json:"syndicated,omitempty"`
	PaidPromotion     SearchTriState   `json:"paid_promotion,omitempty"`
	VideoType         SearchVideoType  `json:"video_type,omitempty"`
	CategoryID        string           `json:"category_id,omitempty"`
	TopicID           string           `json:"topic_id,omitempty"`
	RelevanceLanguage string           `json:"relevance_language,omitempty"`
	RegionCode        string           `json:"region_code,omitempty"`
	SafeSearch        SearchSafe       `json:"safe_search,omitempty"`
	Location          string           `json:"location,omitempty"`
	LocationRadius    string           `json:"location_radius,omitempty"`
}

type SearchSource string

const (
	SearchSourcePublic   SearchSource = "yt-dlp"
	SearchSourceOfficial SearchSource = "youtube-api"
)

type SearchPage struct {
	Items         []Video      `json:"items"`
	NextPageToken string       `json:"next_page_token,omitempty"`
	Source        SearchSource `json:"source"`
	Notices       []string     `json:"notices,omitempty"`
}
