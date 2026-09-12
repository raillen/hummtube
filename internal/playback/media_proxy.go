package playback

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
)

const (
	mediaProxyPathPrefix   = "/api/media/"
	mediaProxyTokenBytes   = 24
	mediaProxyDefaultTTL   = 6 * time.Hour
	mediaProxyMaxSessions  = 128
	mediaProxyMaxRedirects = 3
	mediaProxyMaxManifest  = 4 << 20
)

// MediaProxy mantém URLs assinadas e headers de playback somente no backend.
type MediaProxy struct {
	mu                  sync.Mutex
	sessions            map[string]mediaProxySession
	cancelPlan          context.CancelFunc
	localBaseURL        string
	client              *http.Client
	resolver            mediaProxyResolver
	allowPrivateNetwork bool
	clock               func() time.Time
	maxSessions         int
}

// SetLocalBaseURL faz os tokens apontarem para um servidor HTTP de loopback.
// WebKitGTK não preserva de forma confiável os headers de mídia e Range quando
// o corpo atravessa o protocolo customizado wails://; um origin HTTP real evita
// que streams H.264/AAC válidos sejam reportados como código 4.
func (p *MediaProxy) SetLocalBaseURL(rawBaseURL string) error {
	if p == nil {
		return errors.New("proxy de mídia indisponível")
	}
	normalizedBaseURL, err := normalizeLocalMediaBaseURL(rawBaseURL)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.localBaseURL = normalizedBaseURL
	p.mu.Unlock()
	return nil
}

func normalizeLocalMediaBaseURL(rawBaseURL string) (string, error) {
	value := strings.TrimSpace(rawBaseURL)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("URL local do proxy inválida: %w", err)
	}
	if parsed.Scheme != "http" || parsed.User != nil || parsed.Host == "" || parsed.Port() == "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return "", errors.New("URL local do proxy deve usar http://IP_DE_LOOPBACK:PORTA sem caminho")
	}
	address, err := netip.ParseAddr(parsed.Hostname())
	if err != nil || !address.IsLoopback() {
		return "", errors.New("URL local do proxy deve usar um endereço IP de loopback")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

type mediaProxySession struct {
	target    *url.URL
	headers   map[string]string
	expiresAt time.Time
	isHLS     bool
	lastUsed  time.Time
	observed  bool
	lifetime  context.Context
}

type mediaProxyResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

// NewMediaProxy cria o proxy de mídia local com resolução DNS e conexão
// restritas a endereços públicos. O transporte não usa proxy de ambiente.
func NewMediaProxy() *MediaProxy {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = mediaPublicDialer{
		resolver: net.DefaultResolver,
		dialer:   &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
	}.DialContext
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 20 * time.Second
	transport.MaxIdleConns = 32
	transport.MaxIdleConnsPerHost = 8
	transport.IdleConnTimeout = 90 * time.Second
	transport.MaxConnsPerHost = 16
	transport.ForceAttemptHTTP2 = true
	proxy := &MediaProxy{
		sessions:    make(map[string]mediaProxySession),
		resolver:    net.DefaultResolver,
		clock:       time.Now,
		maxSessions: mediaProxyMaxSessions,
	}
	proxy.client = &http.Client{
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= mediaProxyMaxRedirects {
				return errors.New("stream excedeu o limite de redirecionamentos")
			}
			if _, err := validateMediaTarget(request.URL.String(), proxy.allowPrivateNetwork); err != nil {
				return fmt.Errorf("redirecionamento de mídia rejeitado: %w", err)
			}
			return nil
		},
	}
	return proxy
}

// WrapPlan troca URLs remotas de um PlaybackPlan por referências locais.
// Planos diretos, como arquivos locais, não passam pelo proxy.
func (p *MediaProxy) WrapPlan(ctx context.Context, plan domain.PlaybackPlan) (domain.PlaybackPlan, error) {
	if p == nil {
		return plan, nil
	}
	if ctx == nil {
		return domain.PlaybackPlan{}, errors.New("proxy de mídia: contexto nil")
	}
	if err := ctx.Err(); err != nil {
		return domain.PlaybackPlan{}, err
	}
	if plan.Mode != domain.PlaybackModeResolvedMedia {
		p.Revoke()
		return plan, nil
	}
	now := p.now()
	expiresAt := now.Add(mediaProxyDefaultTTL)
	urls := playbackPlanURLs(plan)
	for _, subtitle := range plan.Subtitles {
		urls = append(urls, subtitle.URL)
	}
	limit := p.maxSessions
	if limit <= 0 {
		limit = mediaProxyMaxSessions
	}
	if len(urls) > limit {
		return domain.PlaybackPlan{}, errors.New("plano excede o limite de recursos de mídia")
	}
	for _, rawURL := range urls {
		if expiry := mediaURLExpiry(rawURL); !expiry.IsZero() && expiry.Before(expiresAt) {
			expiresAt = expiry
		}
	}
	if !plan.ExpiresAt.IsZero() && plan.ExpiresAt.Before(expiresAt) {
		expiresAt = plan.ExpiresAt
	}
	if !expiresAt.After(now) {
		return domain.PlaybackPlan{}, errors.New("stream expirado")
	}
	lifetime, cancel := context.WithTimeout(context.Background(), expiresAt.Sub(now))
	p.mu.Lock()
	staged := &MediaProxy{
		sessions: make(map[string]mediaProxySession), localBaseURL: p.localBaseURL,
		allowPrivateNetwork: p.allowPrivateNetwork, clock: p.clock, maxSessions: p.maxSessions,
	}
	p.mu.Unlock()
	plan = clonePlaybackPlan(plan)
	plan.ExpiresAt = expiresAt
	wrapped, err := staged.wrapPlan(lifetime, plan)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		cancel()
		return domain.PlaybackPlan{}, err
	}
	p.mu.Lock()
	if p.cancelPlan != nil {
		p.cancelPlan()
	}
	p.sessions = staged.sessions
	p.cancelPlan = cancel
	p.mu.Unlock()
	return wrapped, nil
}

func (p *MediaProxy) Revoke() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancelPlan != nil {
		p.cancelPlan()
		p.cancelPlan = nil
	}
	clear(p.sessions)
}

func (p *MediaProxy) wrapPlan(ctx context.Context, plan domain.PlaybackPlan) (domain.PlaybackPlan, error) {
	primary, err := p.wrapStream(ctx, plan.Primary, plan.ExpiresAt)
	if err != nil {
		return domain.PlaybackPlan{}, fmt.Errorf("proxy da mídia principal: %w", err)
	}
	plan.Primary = primary
	if plan.Audio != nil {
		wrapped, wrapErr := p.wrapStream(ctx, *plan.Audio, plan.ExpiresAt)
		if wrapErr != nil {
			return domain.PlaybackPlan{}, fmt.Errorf("proxy da faixa de áudio: %w", wrapErr)
		}
		plan.Audio = &wrapped
	}
	if plan.AudioOnly != nil {
		wrapped, wrapErr := p.wrapStream(ctx, *plan.AudioOnly, plan.ExpiresAt)
		if wrapErr != nil {
			return domain.PlaybackPlan{}, fmt.Errorf("proxy da faixa somente áudio: %w", wrapErr)
		}
		plan.AudioOnly = &wrapped
	}
	for index := range plan.Variants {
		wrapped, wrapErr := p.wrapStream(ctx, plan.Variants[index].Stream, plan.ExpiresAt)
		if wrapErr != nil {
			return domain.PlaybackPlan{}, fmt.Errorf("proxy da variante %s: %w", plan.Variants[index].ID, wrapErr)
		}
		plan.Variants[index].Stream = wrapped
	}
	for index := range plan.Subtitles {
		if strings.TrimSpace(plan.Subtitles[index].URL) == "" {
			continue
		}
		wrapped, wrapErr := p.wrapStream(ctx, domain.ResolvedStream{URL: plan.Subtitles[index].URL}, plan.ExpiresAt)
		if wrapErr != nil {
			return domain.PlaybackPlan{}, fmt.Errorf("proxy da legenda: %w", wrapErr)
		}
		plan.Subtitles[index].URL = wrapped.URL
	}
	return plan, nil
}

func (p *MediaProxy) wrapStream(ctx context.Context, stream domain.ResolvedStream, planExpiry time.Time) (domain.ResolvedStream, error) {
	target, err := validateMediaTarget(stream.URL, p.allowPrivateNetwork)
	if err != nil {
		return domain.ResolvedStream{}, err
	}
	now := p.now()
	expiresAt := now.Add(mediaProxyDefaultTTL)
	if !planExpiry.IsZero() && planExpiry.Before(expiresAt) {
		expiresAt = planExpiry
	}
	if expiry := mediaURLExpiry(stream.URL); !expiry.IsZero() && expiry.Before(expiresAt) {
		expiresAt = expiry
	}
	if !expiresAt.After(now) {
		return domain.ResolvedStream{}, errors.New("stream expirado")
	}
	token, err := randomMediaToken()
	if err != nil {
		return domain.ResolvedStream{}, err
	}
	path := mediaProxyPathPrefix + token
	isHLS := strings.HasSuffix(strings.ToLower(target.Path), ".m3u8")
	if isHLS {
		path += ".m3u8"
	}
	p.mu.Lock()
	if err := ctx.Err(); err != nil {
		p.mu.Unlock()
		return domain.ResolvedStream{}, err
	}
	p.pruneLocked(now)
	p.sessions[token] = mediaProxySession{
		target:    target,
		headers:   sanitizePlaybackHeaders(stream.Headers),
		expiresAt: expiresAt,
		isHLS:     isHLS,
		lastUsed:  now,
		lifetime:  ctx,
	}
	localURL := p.localBaseURL + path
	p.mu.Unlock()
	return domain.ResolvedStream{URL: localURL}, nil
}

// ServeHTTP atende somente GET/HEAD de tokens emitidos por WrapPlan.
func (p *MediaProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p == nil {
		http.Error(w, "proxy de mídia indisponível", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}
	token, ok := parseMediaToken(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	session, ok := p.session(token)
	if !ok {
		http.NotFound(w, r)
		return
	}
	requestCtx, cancel := context.WithTimeout(r.Context(), session.expiresAt.Sub(p.now()))
	defer cancel()
	stop := context.AfterFunc(session.lifetime, cancel)
	defer stop()
	if session.lifetime.Err() != nil {
		cancel()
	}
	request, err := http.NewRequestWithContext(requestCtx, r.Method, session.target.String(), nil)
	if err != nil {
		http.Error(w, "stream inválido", http.StatusBadGateway)
		return
	}
	copyRequestHeader(request.Header, r.Header, "Range")
	copyRequestHeader(request.Header, r.Header, "If-Range")
	copyRequestHeader(request.Header, r.Header, "Accept")
	for name, value := range session.headers {
		if isProxyHeaderAllowed(name) {
			request.Header.Set(name, value)
		}
	}
	// Mantém o corpo sem compressão para que Content-Length/Range permaneçam
	// coerentes com o que o elemento <video> solicitou.
	request.Header.Set("Accept-Encoding", "identity")
	response, err := p.client.Do(request)
	if err != nil {
		log.Printf("playback: proxy de mídia falhou antes da resposta tipo=%T", err)
		http.Error(w, "não foi possível carregar o stream", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if p.markSessionObserved(token) {
		log.Printf(
			"playback: proxy iniciou stream status=%d content_type=%q range=%t hls=%t",
			response.StatusCode,
			strings.TrimSpace(response.Header.Get("Content-Type")),
			strings.TrimSpace(r.Header.Get("Range")) != "",
			session.isHLS,
		)
	}
	copyResponseHeaders(w.Header(), response.Header)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		w.WriteHeader(response.StatusCode)
		return
	}
	if r.Method == http.MethodHead {
		if session.isHLS {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			// O tamanho do manifesto muda após reescrever as referências.
			w.Header().Del("Content-Length")
		}
		w.WriteHeader(response.StatusCode)
		return
	}
	if session.isHLS {
		body, err := io.ReadAll(io.LimitReader(response.Body, mediaProxyMaxManifest+1))
		if err != nil || len(body) > mediaProxyMaxManifest {
			http.Error(w, "manifesto HLS excede o limite permitido", http.StatusBadGateway)
			return
		}
		rewritten, err := p.rewriteHLSManifest(session.lifetime, body, session.target, session.headers, session.expiresAt)
		if err != nil {
			http.Error(w, "manifesto HLS inválido", http.StatusBadGateway)
			return
		}
		w.Header().Del("Content-Range")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Content-Length", strconv.Itoa(len(rewritten)))
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(rewritten)
		return
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (p *MediaProxy) rewriteHLSManifest(ctx context.Context, body []byte, base *url.URL, headers map[string]string, expiresAt time.Time) ([]byte, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("#EXTM3U")) {
		return nil, errors.New("manifesto HLS sem cabeçalho EXT M3U")
	}
	lines := strings.SplitAfter(string(body), "\n")
	var rewritten strings.Builder
	rewritten.Grow(len(body))
	for _, line := range lines {
		updated, err := p.rewriteHLSLine(ctx, line, base, headers, expiresAt)
		if err != nil {
			return nil, err
		}
		rewritten.WriteString(updated)
	}
	return []byte(rewritten.String()), nil
}

func (p *MediaProxy) rewriteHLSLine(ctx context.Context, line string, base *url.URL, headers map[string]string, expiresAt time.Time) (string, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#EXTM3U") {
		return line, nil
	}
	if strings.HasPrefix(trimmed, "#") {
		return p.rewriteHLSAttributes(ctx, line, base, headers, expiresAt)
	}
	start := strings.Index(line, trimmed)
	if start < 0 {
		return line, nil
	}
	local, err := p.wrapHLSReference(ctx, trimmed, base, headers, expiresAt)
	if err != nil {
		return "", err
	}
	return line[:start] + local + line[start+len(trimmed):], nil
}

func (p *MediaProxy) rewriteHLSAttributes(ctx context.Context, line string, base *url.URL, headers map[string]string, expiresAt time.Time) (string, error) {
	var rewritten strings.Builder
	position := 0
	for position < len(line) {
		relative := strings.Index(strings.ToUpper(line[position:]), "URI=")
		if relative < 0 {
			rewritten.WriteString(line[position:])
			break
		}
		attribute := position + relative
		quoteStart := attribute + len("URI=")
		if quoteStart >= len(line) || (line[quoteStart] != '"' && line[quoteStart] != '\'') {
			rewritten.WriteString(line[position : attribute+len("URI=")])
			position = attribute + len("URI=")
			continue
		}
		quote := line[quoteStart]
		quoteEnd := strings.IndexByte(line[quoteStart+1:], quote)
		if quoteEnd < 0 {
			return "", errors.New("atributo URI HLS sem fechamento")
		}
		quoteEnd += quoteStart + 1
		raw := line[quoteStart+1 : quoteEnd]
		local, err := p.wrapHLSReference(ctx, raw, base, headers, expiresAt)
		if err != nil {
			return "", err
		}
		rewritten.WriteString(line[position : quoteStart+1])
		rewritten.WriteString(local)
		rewritten.WriteByte(quote)
		position = quoteEnd + 1
	}
	return rewritten.String(), nil
}

func (p *MediaProxy) wrapHLSReference(ctx context.Context, raw string, base *url.URL, headers map[string]string, expiresAt time.Time) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.HasPrefix(value, mediaProxyPathPrefix) {
		return raw, nil
	}
	reference, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if reference.IsAbs() && reference.Scheme != "http" && reference.Scheme != "https" {
		// Esquemas DRM como skd:// não são recursos HTTP; permanecem intactos
		// para que o player reporte suporte (ou ausência dele) explicitamente.
		return raw, nil
	}
	resolved := base.ResolveReference(reference)
	wrapped, err := p.wrapStream(ctx, domain.ResolvedStream{URL: resolved.String(), Headers: headers}, expiresAt)
	if err != nil {
		return "", err
	}
	return wrapped.URL, nil
}

func (p *MediaProxy) session(token string) (mediaProxySession, bool) {
	now := p.now()
	p.mu.Lock()
	defer p.mu.Unlock()
	session, ok := p.sessions[token]
	if !ok || !now.Before(session.expiresAt) || session.lifetime.Err() != nil {
		delete(p.sessions, token)
		return mediaProxySession{}, false
	}
	session.lastUsed = now
	p.sessions[token] = session
	return session, true
}

func (p *MediaProxy) markSessionObserved(token string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	session, exists := p.sessions[token]
	if !exists || session.lifetime.Err() != nil || session.observed {
		if exists && session.lifetime.Err() != nil {
			delete(p.sessions, token)
		}
		return false
	}
	session.observed = true
	p.sessions[token] = session
	return true
}

func (p *MediaProxy) pruneLocked(now time.Time) {
	for token, session := range p.sessions {
		if !now.Before(session.expiresAt) || session.lifetime == nil || session.lifetime.Err() != nil {
			delete(p.sessions, token)
		}
	}
	limit := p.maxSessions
	if limit <= 0 {
		limit = mediaProxyMaxSessions
	}
	for len(p.sessions) >= limit {
		oldestToken := ""
		var oldest time.Time
		for token, session := range p.sessions {
			if oldestToken == "" || session.lastUsed.Before(oldest) {
				oldestToken = token
				oldest = session.lastUsed
			}
		}
		if oldestToken == "" {
			return
		}
		delete(p.sessions, oldestToken)
	}
}

func (p *MediaProxy) now() time.Time {
	if p.clock != nil {
		return p.clock()
	}
	return time.Now()
}

func validateMediaTarget(raw string, allowPrivate bool) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errors.New("URL de mídia ausente")
	}
	target, err := url.Parse(value)
	if err != nil || target.Hostname() == "" {
		return nil, errors.New("URL de mídia inválida")
	}
	target.Scheme = strings.ToLower(target.Scheme)
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, errors.New("URL de mídia deve usar HTTP(S)")
	}
	if target.User != nil || target.Fragment != "" {
		return nil, errors.New("URL de mídia contém campos não permitidos")
	}
	host := strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
	if !allowPrivate && (host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local")) {
		return nil, errors.New("destino de mídia não público")
	}
	if literal, parseErr := netip.ParseAddr(host); parseErr == nil && !allowPrivate && isForbiddenMediaAddress(literal.Unmap()) {
		return nil, errors.New("destino de mídia não público")
	}
	return target, nil
}

func parseMediaToken(rawPath string) (string, bool) {
	value := strings.TrimPrefix(rawPath, mediaProxyPathPrefix)
	if value == rawPath || strings.Contains(value, "/") {
		return "", false
	}
	value = strings.TrimSuffix(value, ".m3u8")
	if len(value) != mediaProxyTokenBytes*2 {
		return "", false
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", false
	}
	return value, true
}

func randomMediaToken() (string, error) {
	var raw [mediaProxyTokenBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("gerar token de mídia: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func copyRequestHeader(destination, source http.Header, name string) {
	if value := strings.TrimSpace(source.Get(name)); value != "" && !strings.ContainsAny(value, "\r\n") {
		destination.Set(name, value)
	}
}

func isProxyHeaderAllowed(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "accept", "accept-encoding", "accept-language", "origin", "referer", "user-agent":
		return true
	default:
		return false
	}
}

func copyResponseHeaders(destination, source http.Header) {
	for _, name := range []string{
		"Accept-Ranges", "Cache-Control", "Content-Length", "Content-Range",
		"Content-Type", "ETag", "Last-Modified",
	} {
		if value := source.Values(name); len(value) > 0 {
			destination[name] = append([]string(nil), value...)
		}
	}
}

type mediaPublicDialer struct {
	resolver mediaProxyResolver
	dialer   interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}
}

func (dialer mediaPublicDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("destino de mídia inválido")
	}
	addresses, err := resolvePublicMediaAddresses(ctx, host, dialer.resolver)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, resolved := range addresses {
		connection, dialErr := dialer.dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("destino de mídia indisponível")
}

func resolvePublicMediaAddresses(ctx context.Context, host string, resolver mediaProxyResolver) ([]netip.Addr, error) {
	if literal, err := netip.ParseAddr(host); err == nil {
		if isForbiddenMediaAddress(literal.Unmap()) {
			return nil, errors.New("destino de mídia não público")
		}
		return []netip.Addr{literal.Unmap()}, nil
	}
	if resolver == nil {
		return nil, errors.New("DNS do stream indisponível")
	}
	resolved, err := resolver.LookupNetIP(ctx, "ip", strings.TrimSuffix(strings.ToLower(host), "."))
	if err != nil || len(resolved) == 0 {
		return nil, errors.New("DNS do stream indisponível")
	}
	addresses := make([]netip.Addr, 0, len(resolved))
	seen := make(map[netip.Addr]struct{}, len(resolved))
	for _, address := range resolved {
		address = address.Unmap()
		if isForbiddenMediaAddress(address) {
			return nil, errors.New("destino de mídia não público")
		}
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		addresses = append(addresses, address)
	}
	if len(addresses) == 0 {
		return nil, errors.New("DNS do stream sem endereço público")
	}
	return addresses, nil
}

func isForbiddenMediaAddress(address netip.Addr) bool {
	return !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() ||
		address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsMulticast() || address.IsUnspecified()
}
