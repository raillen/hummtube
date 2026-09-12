package iptv

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultMaxItems     = 100_000
	defaultMaxLineBytes = 1 << 20
	defaultBatchSize    = 2_000
	maxParseWarnings    = 100
)

var seasonEpisodePattern = regexp.MustCompile(`(?i)\b(?:S|T)(\d{1,2})(?:\s*E(\d{1,3}))?\b|\b(\d{1,2})x(\d{1,3})\b`)

type m3uEntry struct {
	ID          string
	Name        string
	Group       string
	LogoURL     string
	Channel     string
	Language    string
	Country     string
	ContentType string
}

// ParseM3U importa Extended M3U de forma incremental. A função não faz rede,
// não executa comandos e limita tamanho de linha e quantidade de itens.
func ParseM3U(reader io.Reader, options ParseOptions) (Playlist, error) {
	var playlist Playlist
	summary, err := ParseM3UStream(reader, options, func(batch []Item) error {
		playlist.Items = append(playlist.Items, batch...)
		return nil
	})
	if err != nil {
		return Playlist{}, err
	}
	playlist.Warnings = summary.Warnings
	return playlist, nil
}

// M3UStreamSummary carries bounded diagnostics for a streaming import. Items
// are never accumulated: they are delivered batch by batch to the handler.
type M3UStreamSummary struct {
	Total    int
	Warnings []ParseWarning
}

// ParseM3UStream importa Extended M3U entregando itens em lotes de até
// BatchSize para o handler, mantendo a memória limitada independente do
// tamanho da playlist. O handler recebe fatias reaproveitadas apenas até
// retornar; quem acumula é responsável por copiar.
func ParseM3UStream(reader io.Reader, options ParseOptions, handle func(batch []Item) error) (M3UStreamSummary, error) {
	if reader == nil {
		return M3UStreamSummary{}, fmt.Errorf("ler M3U: reader nil")
	}
	if handle == nil {
		return M3UStreamSummary{}, fmt.Errorf("ler M3U: handler nil")
	}

	options = normalizeOptions(options)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), options.MaxLineBytes)

	var (
		summary      M3UStreamSummary
		batch        = make([]Item, 0, options.BatchSize)
		pending      *m3uEntry
		currentGroup string
		lineNumber   int
	)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := handle(batch); err != nil {
			return fmt.Errorf("ler M3U: lote: %w", err)
		}
		batch = batch[:0]
		return nil
	}

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.EqualFold(line, "#EXTM3U") {
			continue
		}

		if strings.HasPrefix(strings.ToUpper(line), "#EXTINF:") {
			if pending != nil {
				addParseWarning(&summary, ParseWarning{
					Line:    lineNumber,
					Code:    "missing-stream",
					Message: "nova entrada encontrada antes do URL da entrada anterior",
				})
			}

			entry, err := parseEXTINF(line)
			if err != nil {
				addParseWarning(&summary, ParseWarning{
					Line:    lineNumber,
					Code:    "invalid-extinf",
					Message: err.Error(),
				})
				pending = nil
				continue
			}
			if entry.Group == "" {
				entry.Group = currentGroup
			}
			pending = &entry
			continue
		}

		if strings.HasPrefix(strings.ToUpper(line), "#EXTGRP:") {
			currentGroup = strings.TrimSpace(line[len("#EXTGRP:"):])
			if pending != nil && pending.Group == "" {
				pending.Group = currentGroup
			}
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}
		if pending == nil {
			addParseWarning(&summary, ParseWarning{
				Line:    lineNumber,
				Code:    "orphan-stream",
				Message: "URL ignorada sem uma entrada EXTINF anterior",
			})
			continue
		}

		if summary.Total >= options.MaxItems {
			return summary, fmt.Errorf("importar M3U: limite de %d itens excedido", options.MaxItems)
		}

		batch = append(batch, buildItem(*pending, line, options))
		pending = nil
		summary.Total++
		if len(batch) >= options.BatchSize {
			if err := flush(); err != nil {
				return summary, err
			}
		}
	}

	if pending != nil {
		addParseWarning(&summary, ParseWarning{
			Line:    lineNumber,
			Code:    "missing-stream",
			Message: "entrada EXTINF sem URL de stream",
		})
	}
	if err := scanner.Err(); err != nil {
		return summary, fmt.Errorf("ler M3U: %w", err)
	}
	if err := flush(); err != nil {
		return summary, err
	}

	return summary, nil
}

func addParseWarning(summary *M3UStreamSummary, warning ParseWarning) {
	if len(summary.Warnings) >= maxParseWarnings {
		return
	}
	summary.Warnings = append(summary.Warnings, warning)
}

func normalizeOptions(options ParseOptions) ParseOptions {
	if strings.TrimSpace(options.SourceID) == "" {
		options.SourceID = "source-local"
	}
	if strings.TrimSpace(options.SourceName) == "" {
		options.SourceName = options.SourceID
	}
	if options.MaxItems <= 0 {
		options.MaxItems = defaultMaxItems
	}
	if options.MaxLineBytes <= 0 {
		options.MaxLineBytes = defaultMaxLineBytes
	}
	if options.BatchSize <= 0 {
		options.BatchSize = defaultBatchSize
	}
	return options
}

func parseEXTINF(line string) (m3uEntry, error) {
	value := strings.TrimSpace(line[len("#EXTINF:"):])
	metadata, title, hasTitle := splitEXTINF(value)
	attributes := parseAttributes(metadata)
	if strings.TrimSpace(title) == "" {
		title = attributes["tvg-name"]
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return m3uEntry{}, fmt.Errorf("EXTINF sem nome de entrada")
	}
	if !hasTitle && attributes["tvg-name"] == "" {
		return m3uEntry{}, fmt.Errorf("EXTINF sem separador e sem tvg-name")
	}

	return m3uEntry{
		ID:          strings.TrimSpace(attributes["tvg-id"]),
		Name:        title,
		Group:       strings.TrimSpace(attributes["group-title"]),
		LogoURL:     strings.TrimSpace(attributes["tvg-logo"]),
		Channel:     strings.TrimSpace(attributes["tvg-chno"]),
		Language:    strings.TrimSpace(attributes["tvg-language"]),
		Country:     strings.TrimSpace(attributes["tvg-country"]),
		ContentType: strings.TrimSpace(attributes["tvg-type"]),
	}, nil
}

func splitEXTINF(value string) (metadata, title string, hasTitle bool) {
	quote := byte(0)
	escaped := false
	for index := 0; index < len(value); index++ {
		character := value[index]
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && quote != 0 {
			escaped = true
			continue
		}
		if character == '\'' || character == '"' {
			if quote == 0 {
				quote = character
			} else if quote == character {
				quote = 0
			}
			continue
		}
		if character == ',' && quote == 0 {
			return value[:index], value[index+1:], true
		}
	}
	return value, "", false
}

func parseAttributes(value string) map[string]string {
	attributes := make(map[string]string)
	for index := 0; index < len(value); {
		for index < len(value) && (value[index] == ' ' || value[index] == '\t' || value[index] == ',') {
			index++
		}
		start := index
		for index < len(value) && isAttributeKeyCharacter(value[index]) {
			index++
		}
		if start == index {
			for index < len(value) && value[index] != ' ' && value[index] != '\t' {
				index++
			}
			continue
		}

		key := strings.ToLower(value[start:index])
		for index < len(value) && (value[index] == ' ' || value[index] == '\t') {
			index++
		}
		if index >= len(value) || value[index] != '=' {
			continue
		}
		index++
		for index < len(value) && (value[index] == ' ' || value[index] == '\t') {
			index++
		}
		parsed, next := parseAttributeValue(value, index)
		attributes[key] = parsed
		index = next
	}
	return attributes
}

func isAttributeKeyCharacter(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_'
}

func parseAttributeValue(value string, index int) (string, int) {
	if index >= len(value) {
		return "", index
	}
	if value[index] != '\'' && value[index] != '"' {
		start := index
		for index < len(value) && value[index] != ' ' && value[index] != '\t' {
			index++
		}
		return value[start:index], index
	}

	quote := value[index]
	index++
	var builder strings.Builder
	for index < len(value) {
		character := value[index]
		if character == '\\' && index+1 < len(value) {
			builder.WriteByte(value[index+1])
			index += 2
			continue
		}
		if character == quote {
			return builder.String(), index + 1
		}
		builder.WriteByte(character)
		index++
	}
	return builder.String(), index
}

func buildItem(entry m3uEntry, streamURL string, options ParseOptions) Item {
	kind, classification := classify(entry)
	episode := parseEpisodeRef(entry.Name)
	identity := entry.ID
	if identity == "" {
		// Provider stream URLs commonly rotate tokens. Identity must be based on
		// stable catalog metadata so persisted favorites/progress survive refresh.
		identity = strings.Join([]string{
			entry.Name, entry.Group, entry.Channel, entry.Language, entry.Country,
			strconv.Itoa(episode.Season), strconv.Itoa(episode.Episode),
		}, "\x00")
	}

	return Item{
		ID:             stableItemID(options.SourceID, kind, identity),
		SourceID:       options.SourceID,
		SourceName:     options.SourceName,
		Kind:           kind,
		Classification: classification,
		Title:          entry.Name,
		RawTitle:       entry.Name,
		Group:          entry.Group,
		LogoURL:        entry.LogoURL,
		StreamURL:      streamURL,
		EPGID:          entry.ID,
		ChannelNumber:  entry.Channel,
		Language:       entry.Language,
		Country:        entry.Country,
		Episode:        episode,
	}
}

func classify(entry m3uEntry) (ContentKind, ClassificationSource) {
	switch strings.ToLower(entry.ContentType) {
	case "tv", "live", "channel", "canais":
		return ContentKindTV, ClassificationAttribute
	case "movie", "film", "filme", "vod-movie":
		return ContentKindMovie, ClassificationAttribute
	case "series", "serie", "série", "show", "tvshow":
		return ContentKindSeries, ClassificationAttribute
	}

	if hasEpisodeMarker(entry.Name) {
		return ContentKindSeries, ClassificationTitle
	}

	group := strings.ToLower(entry.Group)
	if containsAny(group, "série", "series", "serie", "temporada", "season", "tv show",
		"anime", "desenho", "cartoon", "animation") {
		return ContentKindSeries, ClassificationGroup
	}
	if containsAny(group, "filme", "filmes", "movie", "movies", "cinema", "vod") {
		return ContentKindMovie, ClassificationGroup
	}
	if containsAny(group, "ao vivo", "live", "canais", "canal", "news", "notícias", "sports", "esportes") || group == "tv" {
		return ContentKindTV, ClassificationGroup
	}
	return ContentKindUnknown, ClassificationUnknown
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func hasEpisodeMarker(value string) bool {
	return seasonEpisodePattern.MatchString(value)
}

func parseEpisodeRef(value string) EpisodeRef {
	match := seasonEpisodePattern.FindStringSubmatch(value)
	if len(match) == 0 {
		return EpisodeRef{}
	}

	var result EpisodeRef
	if match[1] != "" {
		result.Season, _ = strconv.Atoi(match[1])
		result.HasSeason = true
		if match[2] != "" {
			result.Episode, _ = strconv.Atoi(match[2])
			result.HasEpisode = true
		}
		return result
	}
	result.Season, _ = strconv.Atoi(match[3])
	result.Episode, _ = strconv.Atoi(match[4])
	result.HasSeason = true
	result.HasEpisode = true
	return result
}

func stableItemID(sourceID string, kind ContentKind, identity string) string {
	digest := sha256.Sum256([]byte(sourceID + "\x00" + string(kind) + "\x00" + identity))
	return fmt.Sprintf("%s-%x", kind, digest[:12])
}
