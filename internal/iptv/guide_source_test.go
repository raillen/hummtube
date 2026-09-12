package iptv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPXMLTVSourceFetchesGuide(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`<tv><channel id="news"><display-name>Notícias</display-name></channel><programme channel="news" start="20260821120000 +0000" stop="20260821130000 +0000"><title>Jornal</title></programme></tv>`))
	}))
	defer server.Close()

	channels, programs, err := (HTTPXMLTVSource{Client: server.Client()}).Fetch(context.Background(), SourceConfig{ID: "fixture", GuideURL: server.URL})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(channels) != 1 || len(programs) != 1 || programs[0].Title != "Jornal" {
		t.Fatalf("channels=%+v programs=%+v", channels, programs)
	}
}
