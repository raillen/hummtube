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
	"strings"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const defaultMaxPlaylistBytes = 32 << 20

var (
	ErrPlaylistTooLarge = errors.New("playlist excede o limite de tamanho")
	ErrInvalidSource    = errors.New("fonte IPTV inválida")
)

// HTTPM3USource busca uma playlist M3U por HTTP(S) e delega a interpretação
// para ParseM3U. A fonte não conhece GTK, player ou armazenamento.
type HTTPM3USource struct {
	Client       *http.Client
	Credentials  domain.SecretStore
	MaxBodyBytes int64
}

// Fetch obtém e normaliza uma playlist. O contexto controla timeout e
// cancelamento; URLs e cabeçalhos não são incluídos nas mensagens de erro.
func (source HTTPM3USource) Fetch(ctx context.Context, config SourceConfig) (Playlist, error) {
	body, err := source.open(ctx, config)
	if err != nil {
		return Playlist{}, err
	}
	defer body.Close()

	maxBodyBytes := source.maxBodyBytes()
	buffered, err := io.ReadAll(io.LimitReader(body, maxBodyBytes+1))
	if err != nil {
		return Playlist{}, fmt.Errorf("ler playlist: %w", err)
	}
	if int64(len(buffered)) > maxBodyBytes {
		return Playlist{}, fmt.Errorf("buscar playlist: %w", ErrPlaylistTooLarge)
	}

	return ParseM3U(bytes.NewReader(buffered), ParseOptions{
		SourceID:   config.ID,
		SourceName: config.Name,
	})
}

// FetchStream entrega a playlist ao parser sem carregar o corpo inteiro em
// memória. Itens saem em lotes pelo handler; o limite de tamanho continua
// valendo sobre o corpo lido — exceder falha com ErrPlaylistTooLarge em vez
// de truncar silenciosamente.
func (source HTTPM3USource) FetchStream(ctx context.Context, config SourceConfig, options ParseOptions, handle func(batch []Item) error) (M3UStreamSummary, error) {
	body, err := source.open(ctx, config)
	if err != nil {
		return M3UStreamSummary{}, err
	}
	defer body.Close()

	capped := &limitExceededReader{r: io.LimitReader(body, source.maxBodyBytes()+1), remaining: source.maxBodyBytes() + 1}
	summary, parseErr := ParseM3UStream(capped, options, handle)
	if capped.exceeded {
		return summary, fmt.Errorf("buscar playlist: %w", ErrPlaylistTooLarge)
	}
	return summary, parseErr
}

// limitExceededReader marca quando o fluxo passou do teto, permitindo que o
// chamador diferencie fim de arquivo real de truncamento.
type limitExceededReader struct {
	r         io.Reader
	remaining int64
	exceeded  bool
}

func (l *limitExceededReader) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		l.exceeded = true
		return 0, io.EOF
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.r.Read(p)
	l.remaining -= int64(n)
	return n, err
}

func (source HTTPM3USource) maxBodyBytes() int64 {
	if source.MaxBodyBytes > 0 {
		return source.MaxBodyBytes
	}
	return defaultMaxPlaylistBytes
}

// open valida a fonte, resolve credenciais efêmeras e retorna o corpo da
// resposta com os limites de segurança aplicados.
func (source HTTPM3USource) open(ctx context.Context, config SourceConfig) (io.ReadCloser, error) {
	if ctx == nil {
		return nil, fmt.Errorf("buscar playlist: contexto nil")
	}
	if err := validateSource(config); err != nil {
		return nil, err
	}

	client := source.Client
	usesTrustedClient := client != nil
	if client == nil {
		client = newPublicHTTPClient(30 * time.Second)
	}
	maxBodyBytes := source.maxBodyBytes()

	playlistURL, err := source.playlistURL(ctx, config)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, playlistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("buscar playlist: %w", ErrInvalidSource)
	}
	request.Header.Set("Accept", "audio/x-mpegurl, application/vnd.apple.mpegurl, text/plain")
	if !usesTrustedClient {
		if _, err := resolvePublicEndpoint(ctx, request.URL, net.DefaultResolver); err != nil {
			return nil, fmt.Errorf("buscar playlist: destino bloqueado: %w", err)
		}
	}

	client = clientWithEndpointRedirectPolicy(client, request.URL, !usesTrustedClient, usesTrustedClient || config.CredentialRef != "")
	response, err := client.Do(request)
	if err != nil {
		return nil, safeRequestError(err)
	}
	if response == nil || response.Body == nil {
		return nil, fmt.Errorf("buscar playlist: resposta sem corpo")
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_ = response.Body.Close()
		return nil, fmt.Errorf("buscar playlist: resposta HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxBodyBytes {
		_ = response.Body.Close()
		return nil, fmt.Errorf("buscar playlist: %w", ErrPlaylistTooLarge)
	}
	return response.Body, nil
}

func (source HTTPM3USource) playlistURL(ctx context.Context, config SourceConfig) (string, error) {
	credentials, err := loadPlaylistCredentials(ctx, source.Credentials, config.CredentialRef)
	if err != nil {
		return "", fmt.Errorf("buscar playlist: %w", err)
	}
	if credentials == nil {
		return config.PlaylistURL, nil
	}

	parsed, err := url.Parse(config.PlaylistURL)
	if err != nil {
		return "", fmt.Errorf("buscar playlist: %w", ErrInvalidSource)
	}
	query := parsed.Query()
	query.Set("username", credentials.Username)
	query.Set("password", credentials.Password)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func validateSource(config SourceConfig) error {
	if strings.TrimSpace(config.ID) == "" {
		return fmt.Errorf("%w: ID ausente", ErrInvalidSource)
	}
	if config.Format != "" && config.Format != SourceFormatM3U {
		return fmt.Errorf("%w: formato %q não suportado", ErrInvalidSource, config.Format)
	}
	parsed, err := url.Parse(config.PlaylistURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%w: endpoint HTTP(S) necessário", ErrInvalidSource)
	}
	if parsed.User != nil || hasSensitiveQueryKey(parsed) {
		return fmt.Errorf("%w: credenciais devem usar SecretStore", ErrInvalidSource)
	}
	return nil
}

func hasSensitiveQueryKey(parsed *url.URL) bool {
	for key := range parsed.Query() {
		switch strings.ToLower(key) {
		case "user", "username", "pass", "password", "token", "auth", "key", "api_key", "apikey", "credential":
			return true
		}
	}
	return false
}

func clientWithEndpointRedirectPolicy(client *http.Client, origin *url.URL, validatePublic, requireSameOrigin bool) *http.Client {
	clone := *client
	previousPolicy := client.CheckRedirect
	clone.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if requireSameOrigin && (request.URL.Scheme != origin.Scheme || request.URL.Host != origin.Host) {
			return fmt.Errorf("%w: redirect para outra origem", ErrInvalidSource)
		}
		if validatePublic {
			if _, err := resolvePublicEndpoint(request.Context(), request.URL, net.DefaultResolver); err != nil {
				return fmt.Errorf("%w: redirect inseguro", ErrInvalidSource)
			}
		}
		if previousPolicy != nil {
			return previousPolicy(request, via)
		}
		return nil
	}
	return &clone
}

func safeRequestError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidSource):
		return fmt.Errorf("buscar playlist: %w", ErrInvalidSource)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("buscar playlist: cancelada: %w", context.Canceled)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("buscar playlist: timeout: %w", context.DeadlineExceeded)
	default:
		return fmt.Errorf("buscar playlist: requisição HTTP falhou")
	}
}
