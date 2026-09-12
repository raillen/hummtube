package services_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/lastfm"
)

func TestLastFMOptInLifecycleAndScrobble(t *testing.T) {
	var scrobbled bool
	api := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
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
			_ = json.NewEncoder(response).Encode(map[string]any{"session": map[string]string{"name": "nano-listener", "key": "session-key"}})
		case "track.scrobble":
			scrobbled = request.Form.Get("artist") == "Canal" && request.Form.Get("track") == "Faixa"
			_ = json.NewEncoder(response).Encode(map[string]any{"scrobbles": map[string]int{"accepted": 1}})
		default:
			http.Error(response, "unexpected", http.StatusBadRequest)
		}
	}))
	defer api.Close()

	service, _ := setupTestServices(t)
	service.LastFM = lastfm.NewClient(lastfm.Config{APIKey: "api", SharedSecret: "secret"})
	service.LastFM.APIURL = api.URL + "/"
	service.LastFM.AuthURL = api.URL + "/authorize"

	status, err := service.GetLastFMStatus(context.Background())
	if err != nil || status.Connected {
		t.Fatalf("initial status = %+v, %v", status, err)
	}
	authorizationURL, err := service.StartLastFMAuthorization(context.Background())
	if err != nil || authorizationURL == "" {
		t.Fatalf("start = %q, %v", authorizationURL, err)
	}
	status, err = service.CompleteLastFMAuthorization(context.Background())
	if err != nil || !status.Connected || status.Username != "nano-listener" {
		t.Fatalf("complete = %+v, %v", status, err)
	}
	if err := service.ScrobbleLastFM(context.Background(), "Canal", "Faixa", int64((3*time.Minute)/time.Millisecond), int64((5*time.Minute)/time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if !scrobbled {
		t.Fatal("scrobble não foi enviado")
	}
	if err := service.DisconnectLastFM(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, err = service.GetLastFMStatus(context.Background())
	if err != nil || status.Connected {
		t.Fatalf("disconnected status = %+v, %v", status, err)
	}
}
