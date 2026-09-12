package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/search"
	"github.com/nanotube/nanotube-web/internal/server"
	"github.com/nanotube/nanotube-web/internal/services"
	"github.com/nanotube/nanotube-web/internal/storage"
)

type mockSecretStore struct {
	data map[domain.SecretKey][]byte
}

func TestSearchRPCSupportsTypedAndLegacyRequests(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	binary := filepath.Join(t.TempDir(), "fake-yt-dlp")
	payload := `{"entries":[{"id":"one","title":"Linux One"},{"id":"two","title":"Linux Two"}]}`
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s' '"+payload+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	appServices := services.NewAppServices(storage.NewRepository(db), &mockSecretStore{})
	appServices.SearchService.Public = search.YtDlp{Binary: binary}
	httpServer := httptest.NewServer(server.NewServer(appServices, server.Config{}).Handler())
	defer httpServer.Close()

	callSearch := func(t *testing.T, args []interface{}) domain.SearchPage {
		t.Helper()
		body, err := json.Marshal(map[string]interface{}{"service": "Search", "method": "Search", "args": args})
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.Post(httpServer.URL+"/api/rpc", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var rpcResponse struct {
			Result domain.SearchPage `json:"result"`
			Error  string            `json:"error"`
		}
		if err := json.NewDecoder(response.Body).Decode(&rpcResponse); err != nil {
			t.Fatal(err)
		}
		if rpcResponse.Error != "" {
			t.Fatalf("RPC: %s", rpcResponse.Error)
		}
		return rpcResponse.Result
	}

	typed := callSearch(t, []interface{}{map[string]interface{}{
		"query": "linux", "max_results": 1, "resource_types": []string{"video"}, "order": "relevance",
	}})
	if len(typed.Items) != 1 || typed.Items[0].ID != "one" || typed.NextPageToken != "1" {
		t.Fatalf("request tipado = %+v", typed)
	}
	legacy := callSearch(t, []interface{}{"linux", 1, 1})
	if len(legacy.Items) != 1 || legacy.Items[0].ID != "two" {
		t.Fatalf("request legado = %+v", legacy)
	}
}

func TestAccountProfileRPCContract(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}
	appServices := services.NewAppServices(storage.NewRepository(db), &mockSecretStore{})
	httpServer := httptest.NewServer(server.NewServer(appServices, server.Config{}).Handler())
	defer httpServer.Close()

	call := func(method string, args []interface{}, result interface{}) {
		t.Helper()
		body, err := json.Marshal(map[string]interface{}{"service": "Account", "method": method, "args": args})
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.Post(httpServer.URL+"/api/rpc", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		payload := struct {
			Result json.RawMessage `json:"result"`
			Error  string          `json:"error"`
		}{}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Error != "" {
			t.Fatalf("RPC Account.%s: %s", method, payload.Error)
		}
		if result != nil {
			if err := json.Unmarshal(payload.Result, result); err != nil {
				t.Fatalf("decode Account.%s: %v", method, err)
			}
		}
	}

	var created domain.Profile
	call("CreateProfile", []interface{}{"Perfil RPC"}, &created)
	if created.ID == "" || created.Name != "Perfil RPC" || created.Kind != domain.ProfileKindPersistent {
		t.Fatalf("perfil criado = %+v", created)
	}
	var active domain.Profile
	call("GetActiveProfile", nil, &active)
	if active.ID != created.ID {
		t.Fatalf("perfil ativo = %+v, esperado %q", active, created.ID)
	}
	var profiles []domain.Profile
	call("ListProfiles", nil, &profiles)
	if len(profiles) != 2 || profiles[0].ID != storage.DefaultProfileID {
		t.Fatalf("perfis RPC = %+v", profiles)
	}
}

func TestDeviceCodePublicContractDoesNotExposePollingSecret(t *testing.T) {
	payload, err := json.Marshal(services.DeviceCodeInfo{
		UserCode: "ABCD-EFGH", VerificationURL: "https://example.invalid/device", ExpiresIn: 600,
	})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exposed := fields["device_code"]; exposed {
		t.Fatalf("device_code atravessou contrato público: %s", payload)
	}
	if _, exposed := fields["interval"]; exposed {
		t.Fatalf("interval interno atravessou contrato público: %s", payload)
	}
	if fields["user_code"] != "ABCD-EFGH" {
		t.Fatalf("user_code ausente: %s", payload)
	}
}

func TestRPCPolicyRejectsCrossProductAndServiceSpoofing(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(db); err != nil {
		t.Fatal(err)
	}

	appServices := services.NewAppServices(storage.NewRepository(db), &mockSecretStore{})
	handler := server.NewServer(appServices, server.Config{AllowedRPCMethods: map[string][]string{
		"IPTV": {"ListIPTVSources"},
	}}).Handler()

	call := func(service, method string) string {
		t.Helper()
		body, _ := json.Marshal(map[string]interface{}{"service": service, "method": method, "args": []interface{}{}})
		request := httptest.NewRequest(http.MethodPost, "/api/rpc", bytes.NewReader(body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		var payload struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		return payload.Error
	}

	if got := call("IPTV", "ListIPTVSources"); got != "" {
		t.Fatalf("RPC IPTV permitida foi recusada: %s", got)
	}
	if got := call("Search", "Search"); got == "" {
		t.Fatal("política do NanoIPTV expôs busca do NanoTube")
	}
	if got := call("IPTV", "DeleteProfile"); got == "" {
		t.Fatal("serviço forjado conseguiu chamar método de outra fronteira")
	}
}

func (m *mockSecretStore) Get(ctx context.Context, key domain.SecretKey) ([]byte, error) {
	return m.data[key], nil
}

func (m *mockSecretStore) Set(ctx context.Context, key domain.SecretKey, value []byte) error {
	if m.data == nil {
		m.data = make(map[domain.SecretKey][]byte)
	}
	m.data[key] = value
	return nil
}

func (m *mockSecretStore) Delete(ctx context.Context, key domain.SecretKey) error {
	delete(m.data, key)
	return nil
}

func TestServer_LifecycleAndRPC(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	if err := storage.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	defer db.Close()

	repo := storage.NewRepository(db)
	secretStore := &mockSecretStore{}
	appServices := services.NewAppServices(repo, secretStore)

	srv := server.NewServer(appServices, server.Config{
		Addr: "127.0.0.1:0",
	})

	url, err := srv.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = srv.Shutdown(ctx) }()

	// 1. Healthcheck
	resp, err := http.Get(url + "/api/health")
	if err != nil {
		t.Fatalf("Health check: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 2. RPC - GetDiagnostics
	body, _ := json.Marshal(map[string]interface{}{
		"service": "Settings",
		"method":  "GetDiagnostics",
		"args":    []interface{}{},
	})
	resp, err = http.Post(url+"/api/rpc", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("RPC GetDiagnostics: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
	var rpcRes struct {
		Result map[string]interface{} `json:"result"`
		Error  string                 `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&rpcRes)
	_ = resp.Body.Close()

	if rpcRes.Error != "" {
		t.Errorf("unexpected RPC error: %s", rpcRes.Error)
	}
	if rpcRes.Result["version"] == "" {
		t.Errorf("expected version in diagnostics")
	}

	// 3. RPC - SaveSetting & GetSettings
	saveBody, _ := json.Marshal(map[string]interface{}{
		"service": "Settings",
		"method":  "SaveSetting",
		"args":    []interface{}{"test_key", "test_value"},
	})
	resp, err = http.Post(url+"/api/rpc", "application/json", bytes.NewReader(saveBody))
	if err != nil {
		t.Fatalf("RPC SaveSetting failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("RPC SaveSetting status: %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 4. Unknown RPC method
	unknownBody, _ := json.Marshal(map[string]interface{}{
		"service": "Unknown",
		"method":  "InvalidMethod",
		"args":    []interface{}{},
	})
	resp, err = http.Post(url+"/api/rpc", "application/json", bytes.NewReader(unknownBody))
	if err != nil {
		t.Fatalf("RPC Unknown failed: %v", err)
	}
	var rpcErrResp struct {
		Result interface{} `json:"result"`
		Error  string      `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&rpcErrResp)
	if rpcErrResp.Error == "" {
		t.Errorf("expected error field in JSON-RPC response for invalid method")
	}
	_ = resp.Body.Close()

	// 5. O contrato JSON precisa usar as mesmas chaves consumidas pelo Svelte.
	subscribeBody, _ := json.Marshal(map[string]interface{}{
		"service": "Catalog", "method": "SubscribeChannel", "args": []interface{}{"contract-channel", "Canal Contrato"},
	})
	resp, err = http.Post(url+"/api/rpc", "application/json", bytes.NewReader(subscribeBody))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	channelsBody, _ := json.Marshal(map[string]interface{}{
		"service": "Catalog", "method": "GetChannels", "args": []interface{}{},
	})
	resp, err = http.Post(url+"/api/rpc", "application/json", bytes.NewReader(channelsBody))
	if err != nil {
		t.Fatal(err)
	}
	var channelsResponse struct {
		Result []map[string]interface{} `json:"result"`
		Error  string                   `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&channelsResponse); err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if len(channelsResponse.Result) != 1 || channelsResponse.Result[0]["id"] != "contract-channel" {
		t.Fatalf("contrato de canais incompatível: %+v", channelsResponse.Result)
	}
	if _, leakedPascalCase := channelsResponse.Result[0]["ID"]; leakedPascalCase {
		t.Fatal("contrato expôs chave PascalCase ID")
	}

	// 6. Uma página externa não pode mutar a API local via CORS.
	req, err := http.NewRequest(http.MethodOptions, url+"/api/rpc", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://attacker.example")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("origem externa recebeu status %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}
