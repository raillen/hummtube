package product

import "testing"

func TestProductsOwnIndependentRuntimeState(t *testing.T) {
	products := []Spec{NanoTube, NanoIPTV, NanoMusic}
	seenData := make(map[string]bool)
	seenDatabase := make(map[string]bool)
	seenKeyring := make(map[string]bool)
	seenFrontend := make(map[string]bool)
	for _, spec := range products {
		for label, value := range map[string]string{
			"data": spec.DataNamespace, "database": spec.DatabaseName,
			"keyring": spec.KeyringService, "frontend": spec.FrontendTarget,
		} {
			seen := map[string]map[string]bool{
				"data": seenData, "database": seenDatabase, "keyring": seenKeyring, "frontend": seenFrontend,
			}[label]
			if seen[value] {
				t.Fatalf("%s compartilhado: %q", label, value)
			}
			seen[value] = true
		}
	}
}

func TestProductRPCBoundaries(t *testing.T) {
	if _, exposed := NanoTube.AllowedRPCMethods["IPTV"]; exposed {
		t.Fatal("NanoTube expôs IPTV")
	}
	if _, exposed := NanoTube.AllowedRPCMethods["LastFM"]; exposed {
		t.Fatal("NanoTube expôs Last.fm")
	}
	if _, exposed := NanoIPTV.AllowedRPCMethods["Search"]; exposed {
		t.Fatal("NanoIPTV expôs busca do YouTube")
	}
	if methods := NanoMusic.AllowedRPCMethods["Catalog"]; len(methods) != 1 || methods[0] != "RememberVideo" {
		t.Fatalf("NanoMusic expôs catálogo além da persistência necessária: %v", methods)
	}
}
