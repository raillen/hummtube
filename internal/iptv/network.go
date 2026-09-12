package iptv

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const endpointDiagnosticTimeout = 4 * time.Second

var (
	ErrInvalidEndpoint     = errors.New("endpoint HTTP(S) inválido")
	ErrEndpointDNS         = errors.New("DNS do endpoint indisponível")
	ErrUnsafeEndpoint      = errors.New("endpoint aponta para rede não pública")
	ErrEndpointUnreachable = errors.New("endpoint não aceitou conexão TCP")
)

// EndpointDiagnostic descreve somente dados públicos e redigidos do destino.
// Paths, query strings e credenciais nunca retornam ao frontend.
type EndpointDiagnostic struct {
	BaseURL           string   `json:"base_url"`
	Scheme            string   `json:"scheme"`
	Host              string   `json:"host"`
	Port              string   `json:"port"`
	ResolvedAddresses []string `json:"resolved_addresses"`
	URLValid          bool     `json:"url_valid"`
	DNSResolved       bool     `json:"dns_resolved"`
	PublicTarget      bool     `json:"public_target"`
	TCPReachable      bool     `json:"tcp_reachable"`
	CredentialHint    bool     `json:"credential_hint"`
	Error             string   `json:"error,omitempty"`
}

type endpointResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type endpointDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type publicEndpoint struct {
	Addresses []netip.Addr
}

var forbiddenEndpointPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// DiagnoseEndpoint valida a URL, resolve DNS e testa somente a conexão TCP.
// Uma conexão bem-sucedida não garante autenticação nem conteúdo M3U válido.
func DiagnoseEndpoint(ctx context.Context, rawURL string) EndpointDiagnostic {
	resolver := net.DefaultResolver
	dialer := &net.Dialer{Timeout: endpointDiagnosticTimeout}
	return diagnoseEndpoint(ctx, rawURL, resolver, dialer)
}

func diagnoseEndpoint(ctx context.Context, rawURL string, resolver endpointResolver, dialer endpointDialer) EndpointDiagnostic {
	diagnostic := EndpointDiagnostic{}
	parsed, err := parseHTTPEndpoint(rawURL)
	if err != nil {
		diagnostic.Error = err.Error()
		return diagnostic
	}
	diagnostic.URLValid = true
	diagnostic.Scheme = parsed.Scheme
	diagnostic.Host = parsed.Hostname()
	diagnostic.Port = endpointPort(parsed)
	diagnostic.BaseURL = (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()
	diagnostic.CredentialHint = parsed.User != nil || hasSensitiveQueryKey(parsed)

	endpoint, err := resolvePublicEndpoint(ctx, parsed, resolver)
	if err != nil {
		diagnostic.Error = err.Error()
		return diagnostic
	}
	diagnostic.DNSResolved = true
	diagnostic.PublicTarget = true
	for _, address := range endpoint.Addresses {
		diagnostic.ResolvedAddresses = append(diagnostic.ResolvedAddresses, address.String())
	}

	probeCtx, cancel := context.WithTimeout(ctx, endpointDiagnosticTimeout)
	defer cancel()
	for _, address := range endpoint.Addresses {
		connection, dialErr := dialer.DialContext(probeCtx, "tcp", net.JoinHostPort(address.String(), diagnostic.Port))
		if dialErr != nil {
			continue
		}
		_ = connection.Close()
		diagnostic.TCPReachable = true
		return diagnostic
	}
	diagnostic.Error = ErrEndpointUnreachable.Error()
	return diagnostic
}

func parseHTTPEndpoint(rawURL string) (*url.URL, error) {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return nil, fmt.Errorf("%w: URL ausente", ErrInvalidEndpoint)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return nil, ErrInvalidEndpoint
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w: inclua http:// ou https://", ErrInvalidEndpoint)
	}
	if parsed.Fragment != "" {
		return nil, fmt.Errorf("%w: fragmento não é permitido", ErrInvalidEndpoint)
	}
	if parsed.Port() != "" {
		port, err := strconv.Atoi(parsed.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("%w: porta inválida", ErrInvalidEndpoint)
		}
	}
	return parsed, nil
}

func resolvePublicEndpoint(ctx context.Context, parsed *url.URL, resolver endpointResolver) (publicEndpoint, error) {
	if ctx == nil {
		return publicEndpoint{}, fmt.Errorf("resolver endpoint: contexto nil")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return publicEndpoint{}, ErrUnsafeEndpoint
	}

	addresses := make([]netip.Addr, 0, 4)
	if literal, err := netip.ParseAddr(host); err == nil {
		addresses = append(addresses, literal.Unmap())
	} else {
		resolved, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(resolved) == 0 {
			return publicEndpoint{}, ErrEndpointDNS
		}
		addresses = append(addresses, resolved...)
	}

	unique := make(map[netip.Addr]struct{}, len(addresses))
	publicAddresses := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		if isForbiddenEndpointAddress(address) {
			return publicEndpoint{}, ErrUnsafeEndpoint
		}
		if _, exists := unique[address]; exists {
			continue
		}
		unique[address] = struct{}{}
		publicAddresses = append(publicAddresses, address)
	}
	if len(publicAddresses) == 0 {
		return publicEndpoint{}, ErrEndpointDNS
	}
	sort.Slice(publicAddresses, func(left, right int) bool {
		return publicAddresses[left].Compare(publicAddresses[right]) < 0
	})
	return publicEndpoint{Addresses: publicAddresses}, nil
}

func isForbiddenEndpointAddress(address netip.Addr) bool {
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() ||
		address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsMulticast() || address.IsUnspecified() {
		return true
	}
	for _, prefix := range forbiddenEndpointPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func endpointPort(parsed *url.URL) string {
	if parsed.Port() != "" {
		return parsed.Port()
	}
	if parsed.Scheme == "https" {
		return "443"
	}
	return "80"
}

type publicNetworkDialer struct {
	resolver endpointResolver
	dialer   endpointDialer
}

func (dialer publicNetworkDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrInvalidEndpoint
	}
	parsed := &url.URL{Scheme: "http", Host: net.JoinHostPort(host, port)}
	endpoint, err := resolvePublicEndpoint(ctx, parsed, dialer.resolver)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, resolvedAddress := range endpoint.Addresses {
		connection, dialErr := dialer.dialer.DialContext(ctx, network, net.JoinHostPort(resolvedAddress.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, ErrEndpointUnreachable
	}
	return nil, ErrEndpointDNS
}

func newPublicHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = publicNetworkDialer{
		resolver: net.DefaultResolver,
		dialer:   &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
	}.DialContext
	return &http.Client{Transport: transport, Timeout: timeout}
}
