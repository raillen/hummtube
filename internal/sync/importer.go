// Package sync provê importação e sincronização de feeds e inscrições.
package sync

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nanotube/nanotube-web/internal/storage"
)

// ImportResult reporta quantos canais foram importados.
type ImportResult struct {
	TotalChannels int      `json:"total_channels"`
	Imported      int      `json:"imported"`
	Failed        int      `json:"failed"`
	ChannelIDs    []string `json:"channel_ids"`
}

type opmlHead struct {
	XMLName xml.Name `xml:"opml"`
	Body    opmlBody `xml:"body"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Title    string        `xml:"title,attr"`
	Text     string        `xml:"text,attr"`
	XMLURL   string        `xml:"xmlUrl,attr"`
	HTMLURL  string        `xml:"htmlUrl,attr"`
	Outlines []opmlOutline `xml:"outline"`
}

type newPipeBackup struct {
	Subscriptions []struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	} `json:"subscriptions"`
}

// ImportSubscriptions auto-detecta e importa canais a partir de OPML, CSV (Takeout) ou JSON (NewPipe).
func ImportSubscriptions(ctx context.Context, repo *storage.Repository, data []byte) (ImportResult, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ImportResult{}, errors.New("conteúdo vazio")
	}

	if bytes.HasPrefix(trimmed, []byte("<?xml")) || bytes.HasPrefix(trimmed, []byte("<opml")) {
		return importOPML(ctx, repo, trimmed)
	}

	if bytes.HasPrefix(trimmed, []byte("{")) {
		return importJSON(ctx, repo, trimmed)
	}

	return importCSV(ctx, repo, trimmed)
}

func importOPML(ctx context.Context, repo *storage.Repository, data []byte) (ImportResult, error) {
	var opml opmlHead
	if err := xml.Unmarshal(data, &opml); err != nil {
		return ImportResult{}, fmt.Errorf("xml opml inválido: %w", err)
	}

	var allOutlines []opmlOutline
	var collect func(list []opmlOutline)
	collect = func(list []opmlOutline) {
		for _, o := range list {
			allOutlines = append(allOutlines, o)
			if len(o.Outlines) > 0 {
				collect(o.Outlines)
			}
		}
	}
	collect(opml.Body.Outlines)

	res := ImportResult{}
	for _, o := range allOutlines {
		url := o.XMLURL
		if url == "" {
			url = o.HTMLURL
		}
		chID := extractChannelID(url)
		if chID == "" {
			continue
		}
		res.TotalChannels++
		title := o.Title
		if title == "" {
			title = o.Text
		}
		if err := repo.SubscribeChannel(ctx, chID, title); err == nil {
			res.Imported++
			res.ChannelIDs = append(res.ChannelIDs, chID)
		} else {
			res.Failed++
		}
	}

	return res, nil
}

func importCSV(ctx context.Context, repo *storage.Repository, data []byte) (ImportResult, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1

	res := ImportResult{}
	isFirst := true

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if isFirst {
			isFirst = false
			if len(rec) > 0 && strings.Contains(strings.ToLower(rec[0]), "channel id") {
				continue
			}
		}

		if len(rec) == 0 {
			continue
		}

		chID := strings.TrimSpace(rec[0])
		title := ""
		if len(rec) >= 3 {
			title = strings.TrimSpace(rec[2])
		}

		if strings.HasPrefix(chID, "http") {
			chID = extractChannelID(chID)
		}

		if chID == "" {
			continue
		}

		res.TotalChannels++
		if err := repo.SubscribeChannel(ctx, chID, title); err == nil {
			res.Imported++
			res.ChannelIDs = append(res.ChannelIDs, chID)
		} else {
			res.Failed++
		}
	}

	return res, nil
}

func importJSON(ctx context.Context, repo *storage.Repository, data []byte) (ImportResult, error) {
	var np newPipeBackup
	if err := json.Unmarshal(data, &np); err != nil {
		return ImportResult{}, fmt.Errorf("json de inscrições inválido: %w", err)
	}

	res := ImportResult{}
	for _, sub := range np.Subscriptions {
		chID := extractChannelID(sub.URL)
		if chID == "" {
			continue
		}
		res.TotalChannels++
		if err := repo.SubscribeChannel(ctx, chID, sub.Name); err == nil {
			res.Imported++
			res.ChannelIDs = append(res.ChannelIDs, chID)
		} else {
			res.Failed++
		}
	}

	return res, nil
}

func extractChannelID(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	// 1. RSS XML URL: ?channel_id=UC...
	if idx := strings.Index(rawURL, "channel_id="); idx != -1 {
		rest := rawURL[idx+len("channel_id="):]
		if amp := strings.Index(rest, "&"); amp != -1 {
			return rest[:amp]
		}
		return rest
	}

	// 2. Direct Channel ID
	if strings.HasPrefix(rawURL, "UC") && !strings.Contains(rawURL, "/") {
		return rawURL
	}

	// 3. /channel/UC...
	if idx := strings.Index(rawURL, "/channel/"); idx != -1 {
		rest := rawURL[idx+len("/channel/"):]
		if slash := strings.Index(rest, "/"); slash != -1 {
			return rest[:slash]
		}
		return rest
	}

	// 4. @handle or /c/ or /user/
	for _, prefix := range []string{"/c/", "/user/", "/@"} {
		if idx := strings.Index(rawURL, prefix); idx != -1 {
			rest := rawURL[idx+len(prefix):]
			if slash := strings.Index(rest, "/"); slash != -1 {
				return rest[:slash]
			}
			return rest
		}
	}

	return rawURL
}
