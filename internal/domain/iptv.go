package domain

import "time"

// IPTVSource representa uma fonte de lista de reprodução M3U ou Xtream.
type IPTVSource struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	URL           string    `json:"url"`
	EPGURL        string    `json:"epg_url,omitempty"`
	CredentialRef string    `json:"credential_ref,omitempty"`
	LastSyncAt    time.Time `json:"last_sync_at"`
	ChannelCount  int       `json:"channel_count"`
}

// IPTVChannel representa um canal de TV ao vivo ou VOD.
type IPTVChannel struct {
	ID        string `json:"id"`
	SourceID  string `json:"source_id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	LogoURL   string `json:"logo_url,omitempty"`
	StreamURL string `json:"stream_url"`
	TvgID     string `json:"tvg_id,omitempty"`
	TvgName   string `json:"tvg_name,omitempty"`
}

// EPGProgram representa uma entrada no guia de programação eletrônico.
type EPGProgram struct {
	ChannelTvgID string    `json:"channel_tvg_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
}
