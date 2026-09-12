// Package sync — RSS fallback for YouTube subscriptions (no OAuth).
// Usado quando o usuário quer inscrever um canal por URL/handle sem conta.
package sync

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

var rssClient = &http.Client{Timeout: 15 * time.Second}

// FetchChannelFeed baixa o Atom feed público de um canal (UC ID).
func FetchChannelFeed(ctx context.Context, channelID string) ([]domain.Video, error) {
	if !strings.HasPrefix(channelID, "UC") || len(channelID) < 20 {
		return nil, fmt.Errorf("channel id inválido: %q", channelID)
	}
	url := "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.8")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")
	resp, err := rssClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rss: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rss: status %d", resp.StatusCode)
	}
	var feed atomFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("rss xml: %w", err)
	}
	now := time.Now()
	var out []domain.Video
	for _, e := range feed.Entries {
		id := e.VideoID
		if id == "" {
			if p := strings.LastIndex(e.ID, ":"); p >= 0 {
				id = e.ID[p+1:]
			}
		}
		if id == "" {
			continue
		}
		published, _ := time.Parse(time.RFC3339, e.Published)
		thumb := ""
		if e.MediaGroup.Thumbnail.URL != "" {
			thumb = e.MediaGroup.Thumbnail.URL
		}
		out = append(out, domain.Video{
			ID:                 id,
			ChannelID:          channelID,
			ChannelTitle:       e.Author.Name,
			Title:              e.Title,
			Description:        e.MediaGroup.Description,
			DescriptionExcerpt: e.MediaGroup.Description,
			PublishedAt:        published,
			ThumbnailURL:       thumb,
			FirstSeenAt:        now,
			LastSeenAt:         now,
		})
		if len(out) >= 15 {
			break
		}
	}
	return out, nil
}

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}
type atomEntry struct {
	ID         string     `xml:"id"`
	VideoID    string     `xml:"videoId"`
	Title      string     `xml:"title"`
	Published  string     `xml:"published"`
	Author     atomAuthor `xml:"author"`
	MediaGroup mediaGroup `xml:"group"`
}
type atomAuthor struct {
	Name string `xml:"name"`
}
type mediaGroup struct {
	Title       string     `xml:"title"`
	Description string     `xml:"description"`
	Thumbnail   mediaThumb `xml:"thumbnail"`
}
type mediaThumb struct {
	URL string `xml:"url,attr"`
}
