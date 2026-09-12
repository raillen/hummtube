package iptv

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	ErrStreamNotFound  = errors.New("stream IPTV não encontrado")
	ErrStreamAmbiguous = errors.New("stream IPTV ambíguo")
	ErrInvalidStream   = errors.New("stream IPTV inválido")
)

// errResolved is a sentinel used internally to abort streaming resolution.
var errResolved = errors.New("resolvido")

// M3UStreamResolver resolves a catalog item against a fresh provider
// snapshot. It intentionally ignores Item.StreamURL so a rotating provider
// URL is never persisted or reused after it expires.
type M3UStreamResolver struct {
	Source HTTPM3USource
}

// Resolve obtains the current stream URL without writing it to storage.
func (resolver M3UStreamResolver) Resolve(ctx context.Context, source SourceConfig, item Item) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("resolver IPTV: contexto nil")
	}
	if strings.TrimSpace(source.ID) == "" || source.ID != item.SourceID || strings.TrimSpace(item.ID) == "" {
		return "", fmt.Errorf("resolver IPTV: item e fonte incompatíveis: %w", ErrStreamNotFound)
	}

	playlist, err := resolver.Source.Fetch(ctx, source)
	if err != nil {
		return "", fmt.Errorf("resolver IPTV: buscar catálogo: %w", err)
	}

	exact := make([]Item, 0, 1)
	for _, candidate := range playlist.Items {
		if candidate.ID == item.ID {
			exact = append(exact, candidate)
		}
	}
	if len(exact) > 0 {
		return resolver.chooseStream(ctx, exact)
	}

	fallback := make([]Item, 0, 1)
	for _, candidate := range playlist.Items {
		if SameCatalogIdentity(candidate, item) {
			fallback = append(fallback, candidate)
		}
	}
	if len(fallback) == 0 {
		return "", fmt.Errorf("resolver IPTV: %w", ErrStreamNotFound)
	}
	if len(fallback) > 1 {
		return "", fmt.Errorf("resolver IPTV: %w", ErrStreamAmbiguous)
	}
	return resolver.chooseStream(ctx, fallback)
}

func (resolver M3UStreamResolver) chooseStream(ctx context.Context, items []Item) (string, error) {
	for _, item := range items {
		if err := resolver.validateStreamURL(ctx, item.StreamURL); err == nil {
			return item.StreamURL, nil
		}
	}
	return "", fmt.Errorf("resolver IPTV: %w", ErrInvalidStream)
}

// SameCatalogIdentity compares stable provider metadata while deliberately
// ignoring ephemeral stream URLs.
func SameCatalogIdentity(left, right Item) bool {
	return left.SourceID == right.SourceID &&
		left.Kind == right.Kind &&
		strings.EqualFold(strings.TrimSpace(left.Title), strings.TrimSpace(right.Title)) &&
		strings.EqualFold(strings.TrimSpace(left.Group), strings.TrimSpace(right.Group)) &&
		left.Episode == right.Episode
}

// ResolveFirst resolves the stream URL by streaming the playlist source
// without buffering the entire list in memory. It stops as soon as the
// first valid URL matching by exact ID is found, or collects unique
// metadata matches as fallback without exceeding bounded memory.
// ParseOptions.BatchSize controls batch granularity; source streaming
// stops early when an exact match is found.
func (resolver M3UStreamResolver) ResolveFirst(ctx context.Context, source SourceConfig, options ParseOptions, match func(Item) bool) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("resolver IPTV: contexto nil")
	}
	if match == nil {
		return "", fmt.Errorf("resolver IPTV: predicate nil")
	}
	options = normalizeResolverOptions(options, source)

	var (
		exactURL      string
		identityURLs  []string
		identityCount int
	)

	_, err := resolver.Source.FetchStream(ctx, source, options, func(batch []Item) error {
		for _, item := range batch {
			if !match(item) {
				continue
			}
			if exactURL == "" {
				if resolver.validateStreamURL(ctx, item.StreamURL) == nil {
					exactURL = item.StreamURL
					return errResolved
				}
			}
			if resolver.validateStreamURL(ctx, item.StreamURL) == nil {
				identityURLs = append(identityURLs, item.StreamURL)
				identityCount++
				if identityCount > 1 && identityURLs[len(identityURLs)-1] != identityURLs[0] {
					// ambiguous — stop
					return errResolved
				}
			}
		}
		return nil
	})

	if exactURL != "" {
		return exactURL, nil
	}
	if err != nil && !errors.Is(err, errResolved) {
		return "", fmt.Errorf("resolver IPTV: buscar catálogo: %w", err)
	}
	if identityCount == 1 {
		return identityURLs[0], nil
	}
	if identityCount > 1 {
		return "", fmt.Errorf("resolver IPTV: %w", ErrStreamAmbiguous)
	}
	return "", fmt.Errorf("resolver IPTV: %w", ErrStreamNotFound)
}

func normalizeResolverOptions(options ParseOptions, source SourceConfig) ParseOptions {
	if options.SourceID == "" {
		options.SourceID = source.ID
	}
	if options.SourceName == "" {
		options.SourceName = source.Name
	}
	if options.BatchSize <= 0 {
		options.BatchSize = defaultBatchSize
	}
	if options.MaxItems <= 0 {
		options.MaxItems = defaultMaxItems
	}
	return options
}

func validateResolvedStreamURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return ErrInvalidStream
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "rtmp", "rtmps":
		return nil
	default:
		return ErrInvalidStream
	}
}

func (resolver M3UStreamResolver) validateStreamURL(ctx context.Context, value string) error {
	if err := validateResolvedStreamURL(value); err != nil {
		return err
	}
	// Clientes injetados pertencem a testes/adapters confiáveis. O caminho de
	// produção usa o resolver padrão e sempre valida o DNS antes de devolver a
	// URL hostil ao WebView.
	if resolver.Source.Client != nil {
		return nil
	}
	parsed, _ := url.Parse(strings.TrimSpace(value))
	if _, err := resolvePublicEndpoint(ctx, parsed, net.DefaultResolver); err != nil {
		return ErrInvalidStream
	}
	return nil
}
