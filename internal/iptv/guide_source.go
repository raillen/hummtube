package iptv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const defaultMaxGuideBytes = 32 << 20

// HTTPXMLTVSource fetches an XMLTV guide without knowing about storage or UI.
type HTTPXMLTVSource struct {
	Client       *http.Client
	MaxBodyBytes int64
}

// Fetch downloads and parses the complete guide into bounded in-memory slices.
// The parser itself remains streaming; this boundary is what enables one
// atomic guide snapshot in the repository.
func (source HTTPXMLTVSource) Fetch(ctx context.Context, config SourceConfig) ([]GuideChannel, []GuideProgram, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("buscar XMLTV: contexto nil")
	}
	guideURL, err := url.Parse(config.GuideURL)
	if err != nil || guideURL.Host == "" || (guideURL.Scheme != "http" && guideURL.Scheme != "https") {
		return nil, nil, fmt.Errorf("buscar XMLTV: %w", ErrInvalidSource)
	}
	client := source.Client
	usesTrustedClient := client != nil
	if client == nil {
		client = newPublicHTTPClient(30 * time.Second)
	}
	maxBodyBytes := source.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxGuideBytes
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, config.GuideURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("buscar XMLTV: %w", ErrInvalidSource)
	}
	request.Header.Set("Accept", "application/xml, text/xml")
	if !usesTrustedClient {
		if _, err := resolvePublicEndpoint(ctx, request.URL, net.DefaultResolver); err != nil {
			return nil, nil, fmt.Errorf("buscar XMLTV: destino bloqueado: %w", err)
		}
	}
	client = clientWithEndpointRedirectPolicy(client, request.URL, !usesTrustedClient, usesTrustedClient)
	response, err := client.Do(request)
	if err != nil {
		return nil, nil, safeGuideRequestError(err)
	}
	if response == nil || response.Body == nil {
		return nil, nil, fmt.Errorf("buscar XMLTV: resposta sem corpo")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, nil, fmt.Errorf("buscar XMLTV: resposta HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxBodyBytes {
		return nil, nil, fmt.Errorf("buscar XMLTV: %w", ErrPlaylistTooLarge)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("ler XMLTV: %w", err)
	}
	if int64(len(body)) > maxBodyBytes {
		return nil, nil, fmt.Errorf("ler XMLTV: %w", ErrPlaylistTooLarge)
	}

	channels := make([]GuideChannel, 0, 64)
	programs := make([]GuideProgram, 0, 256)
	err = ParseXMLTV(bytes.NewReader(body), func(channel GuideChannel) error {
		channels = append(channels, channel)
		return nil
	}, func(program GuideProgram) error {
		programs = append(programs, program)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return channels, programs, nil
}

func safeGuideRequestError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidSource):
		return fmt.Errorf("buscar XMLTV: %w", ErrInvalidSource)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("buscar XMLTV: cancelada: %w", context.Canceled)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("buscar XMLTV: timeout: %w", context.DeadlineExceeded)
	default:
		return fmt.Errorf("buscar XMLTV: requisição HTTP falhou")
	}
}
