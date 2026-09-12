package iptv_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/nanotube/nanotube-web/internal/iptv"
	"github.com/nanotube/nanotube-web/internal/storage"
)

// TestHTTPM3USourceRealSmoke is opt-in and never runs in the default gate.
// Use only a public endpoint or a reference already provisioned in the
// nanotube-iptv keyring service. The test never accepts credentials inline.
func TestHTTPM3USourceRealSmoke(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("NANOIPTV_SMOKE_URL"))
	if endpoint == "" {
		t.Skip("NANOIPTV_SMOKE_URL não definido")
	}
	credentialRef := strings.TrimSpace(os.Getenv("NANOIPTV_SMOKE_CREDENTIAL_REF"))

	playlist, err := (iptv.HTTPM3USource{
		Credentials: storage.NewKeyringSecretStore("nanotube-iptv"),
	}).Fetch(context.Background(), iptv.SourceConfig{
		ID: "real-smoke", Name: "Real smoke", Format: iptv.SourceFormatM3U,
		PlaylistURL: endpoint, CredentialRef: credentialRef,
	})
	if err != nil {
		t.Fatalf("smoke M3U falhou: %v", err)
	}
	if len(playlist.Items) == 0 {
		t.Fatal("smoke M3U retornou uma playlist sem itens")
	}
	t.Logf("itens importados: %d; avisos: %d", len(playlist.Items), len(playlist.Warnings))
}
