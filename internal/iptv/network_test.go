package iptv

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
)

type stubEndpointResolver struct {
	addresses []netip.Addr
	err       error
}

func (resolver stubEndpointResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return resolver.addresses, resolver.err
}

type stubEndpointDialer struct {
	addresses []string
	err       error
}

func (dialer *stubEndpointDialer) DialContext(_ context.Context, _, address string) (net.Conn, error) {
	dialer.addresses = append(dialer.addresses, address)
	if dialer.err != nil {
		return nil, dialer.err
	}
	client, server := net.Pipe()
	_ = server.Close()
	return client, nil
}

func TestDiagnoseEndpointRedactsCredentialsAndPinsPublicAddress(t *testing.T) {
	dialer := &stubEndpointDialer{}
	diagnostic := diagnoseEndpoint(
		context.Background(),
		"https://user:secret@provider.example/private/list.m3u?username=user&password=secret",
		stubEndpointResolver{addresses: []netip.Addr{netip.MustParseAddr("1.1.1.1")}},
		dialer,
	)

	if !diagnostic.URLValid || !diagnostic.DNSResolved || !diagnostic.PublicTarget || !diagnostic.TCPReachable {
		t.Fatalf("diagnóstico inesperado: %+v", diagnostic)
	}
	if !diagnostic.CredentialHint || diagnostic.BaseURL != "https://provider.example" {
		t.Fatalf("redaction/hint inesperado: %+v", diagnostic)
	}
	if strings.Contains(diagnostic.BaseURL, "secret") || strings.Contains(diagnostic.BaseURL, "list.m3u") {
		t.Fatalf("diagnóstico expôs dados sensíveis: %q", diagnostic.BaseURL)
	}
	if len(dialer.addresses) != 1 || dialer.addresses[0] != "1.1.1.1:443" {
		t.Fatalf("destino discado = %v", dialer.addresses)
	}
}

func TestDiagnoseEndpointRejectsAnyPrivateDNSAnswerBeforeDial(t *testing.T) {
	dialer := &stubEndpointDialer{}
	diagnostic := diagnoseEndpoint(
		context.Background(),
		"https://provider.example/list.m3u",
		stubEndpointResolver{addresses: []netip.Addr{
			netip.MustParseAddr("1.1.1.1"),
			netip.MustParseAddr("127.0.0.1"),
		}},
		dialer,
	)

	if !diagnostic.URLValid || diagnostic.PublicTarget || diagnostic.TCPReachable {
		t.Fatalf("diagnóstico inseguro aceito: %+v", diagnostic)
	}
	if diagnostic.Error != ErrUnsafeEndpoint.Error() || len(dialer.addresses) != 0 {
		t.Fatalf("erro=%q dials=%v", diagnostic.Error, dialer.addresses)
	}
}

func TestDiagnoseEndpointRequiresExplicitHTTPAndValidPort(t *testing.T) {
	for _, endpoint := range []string{"provider.example/list.m3u", "ftp://provider.example/list.m3u", "https://provider.example:70000/list.m3u"} {
		diagnostic := diagnoseEndpoint(context.Background(), endpoint, stubEndpointResolver{}, &stubEndpointDialer{})
		if diagnostic.URLValid || !strings.Contains(diagnostic.Error, ErrInvalidEndpoint.Error()) {
			t.Fatalf("endpoint %q: diagnóstico=%+v", endpoint, diagnostic)
		}
	}
}

func TestPublicNetworkDialerRejectsLoopbackLiteral(t *testing.T) {
	dialer := publicNetworkDialer{
		resolver: stubEndpointResolver{},
		dialer:   &stubEndpointDialer{err: errors.New("não deve discar")},
	}
	_, err := dialer.DialContext(context.Background(), "tcp", "127.0.0.1:80")
	if !errors.Is(err, ErrUnsafeEndpoint) {
		t.Fatalf("erro = %v, want ErrUnsafeEndpoint", err)
	}
}
