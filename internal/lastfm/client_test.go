package lastfm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestAuthorizationSessionAndScrobble(t *testing.T) {
	var scrobble url.Values
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
		}
		method := request.URL.Query().Get("method")
		if request.Method == http.MethodPost {
			method = request.Form.Get("method")
		}
		switch method {
		case "auth.getToken":
			_ = json.NewEncoder(response).Encode(map[string]string{"token": "temporary"})
		case "auth.getSession":
			_ = json.NewEncoder(response).Encode(map[string]any{"session": map[string]string{"name": "listener", "key": "session-secret"}})
		case "track.scrobble":
			scrobble = request.Form
			_ = json.NewEncoder(response).Encode(map[string]any{"scrobbles": map[string]any{"accepted": 1}})
		default:
			http.Error(response, "unexpected method", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := NewClient(Config{APIKey: "api", SharedSecret: "secret"})
	client.APIURL = server.URL + "/"
	client.AuthURL = server.URL + "/authorize"
	token, authURL, err := client.StartAuthorization(context.Background())
	if err != nil || token != "temporary" {
		t.Fatalf("StartAuthorization = %q, %q, %v", token, authURL, err)
	}
	parsed, _ := url.Parse(authURL)
	if parsed.Query().Get("token") != token || parsed.Query().Get("api_key") != "api" {
		t.Fatalf("auth URL = %s", authURL)
	}
	session, err := client.ExchangeSession(context.Background(), token)
	if err != nil || session.Key != "session-secret" {
		t.Fatalf("ExchangeSession = %+v, %v", session, err)
	}
	startedAt := time.Now().Add(-2 * time.Minute).Truncate(time.Second)
	if err := client.Scrobble(context.Background(), session.Key, "Artist", "Track", startedAt); err != nil {
		t.Fatal(err)
	}
	if scrobble.Get("artist") != "Artist" || scrobble.Get("track") != "Track" || scrobble.Get("api_sig") == "" {
		t.Fatalf("scrobble = %v", scrobble)
	}
}

func TestEligibleForScrobble(t *testing.T) {
	tests := []struct {
		played, duration time.Duration
		want             bool
	}{
		{29 * time.Second, 5 * time.Minute, false},
		{2 * time.Minute, 5 * time.Minute, false},
		{150 * time.Second, 5 * time.Minute, true},
		{4 * time.Minute, 20 * time.Minute, true},
		{20 * time.Second, 20 * time.Second, false},
	}
	for _, test := range tests {
		if got := EligibleForScrobble(test.played, test.duration); got != test.want {
			t.Fatalf("EligibleForScrobble(%s, %s) = %v, want %v", test.played, test.duration, got, test.want)
		}
	}
}
