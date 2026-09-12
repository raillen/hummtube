package desktop

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/nanotube/nanotube-web/internal/domain"
	"github.com/nanotube/nanotube-web/internal/playback"
)

func TestRouteDesktopRequestsKeepsAPIOutsideViteProxy(t *testing.T) {
	rpcHandler := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("X-Handler", "rpc")
	})
	assetHandler := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("X-Handler", "assets")
	})
	handler := routeDesktopRequests(rpcHandler, assetHandler)

	tests := []struct {
		path string
		want string
	}{
		{path: "/api/health", want: "rpc"},
		{path: "/api/rpc", want: "rpc"},
		{path: "/", want: "assets"},
		{path: "/src/main.ts", want: "assets"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if got := response.Header().Get("X-Handler"); got != test.want {
				t.Fatalf("handler = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDesktopMediaServerUsesHTTPAndAllowsAnonymousMediaCORS(t *testing.T) {
	proxy := playback.NewMediaProxy()
	mediaServer, err := startDesktopMediaServer(proxy)
	if err != nil {
		t.Fatalf("startDesktopMediaServer: %v", err)
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = mediaServer.Shutdown(shutdownContext)
	}()

	plan, err := proxy.WrapPlan(context.Background(), domain.PlaybackPlan{
		Mode:    domain.PlaybackModeResolvedMedia,
		Primary: domain.ResolvedStream{URL: "https://media.example/video.mp4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodOptions, plan.Primary.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS respondeu %d", response.StatusCode)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS de mídia = %q", got)
	}

	mediaURL, err := url.Parse(plan.Primary.URL)
	if err != nil {
		t.Fatal(err)
	}
	mediaURL.Path = "/api/rpc"
	rpcResponse, err := http.Get(mediaURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer rpcResponse.Body.Close()
	if rpcResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("servidor de mídia expôs rota RPC: status=%d", rpcResponse.StatusCode)
	}
}
