package iptv

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestParseM3UMixedContent(t *testing.T) {
	playlist, err := ParseM3U(strings.NewReader(`#EXTM3U
#EXTINF:-1 tvg-id="news.br" tvg-logo="https://example.invalid/news.png" group-title="TV Notícias",Jornal
https://example.invalid/live/news.m3u8
#EXTINF:-1 group-title="Filmes",O Filme de Teste (2025)
https://example.invalid/movie/teste.mp4
#EXTINF:-1 group-title="Séries",Minha Série S02E03
https://example.invalid/series/teste-s02e03.mp4
`), ParseOptions{SourceID: "fixture", SourceName: "Fixture"})
	if err != nil {
		t.Fatalf("ParseM3U() error = %v", err)
	}
	if len(playlist.Warnings) != 0 {
		t.Fatalf("warnings inesperados: %+v", playlist.Warnings)
	}
	if len(playlist.Items) != 3 {
		t.Fatalf("itens = %d, want 3", len(playlist.Items))
	}

	if got := playlist.Items[0]; got.Kind != ContentKindTV || got.Classification != ClassificationGroup || got.EPGID != "news.br" {
		t.Fatalf("canal normalizado incorretamente: %+v", got)
	}
	if got := playlist.Items[1]; got.Kind != ContentKindMovie {
		t.Fatalf("filme classificado como %q", got.Kind)
	}
	if got := playlist.Items[2]; got.Kind != ContentKindSeries || !got.Episode.HasSeason || got.Episode.Season != 2 || !got.Episode.HasEpisode || got.Episode.Episode != 3 {
		t.Fatalf("série/episódio normalizados incorretamente: %+v", got)
	}
}

func TestParseM3UUsesExtgrpAndFallbackName(t *testing.T) {
	playlist, err := ParseM3U(strings.NewReader(`#EXTM3U
#EXTGRP: Ao Vivo
#EXTINF:-1 tvg-name="Canal sem título", 
https://example.invalid/live/channel.m3u8
orphan-line
`), ParseOptions{SourceID: "fixture"})
	if err != nil {
		t.Fatalf("ParseM3U() error = %v", err)
	}
	if len(playlist.Items) != 1 {
		t.Fatalf("itens = %d, want 1", len(playlist.Items))
	}
	item := playlist.Items[0]
	if item.Title != "Canal sem título" || item.Group != "Ao Vivo" || item.Kind != ContentKindTV {
		t.Fatalf("fallback EXTGRP incorreto: %+v", item)
	}
	if len(playlist.Warnings) != 1 || playlist.Warnings[0].Code != "orphan-stream" {
		t.Fatalf("warnings = %+v, want orphan-stream", playlist.Warnings)
	}
}

func TestParseM3UClassifiesAnimeAndCartoonGroupsAsSeries(t *testing.T) {
	playlist, err := ParseM3U(strings.NewReader(`#EXTM3U
#EXTINF:-1 group-title="ANIMES",Naruto Shippuden
https://example.invalid/anime/naruto.mp4
#EXTINF:-1 group-title="Desenhos",Pica-Pau
https://example.invalid/cartoon/pica-pau.mp4
`), ParseOptions{SourceID: "fixture"})
	if err != nil {
		t.Fatalf("ParseM3U() error = %v", err)
	}
	if len(playlist.Items) != 2 {
		t.Fatalf("itens = %d, want 2", len(playlist.Items))
	}
	for _, item := range playlist.Items {
		if item.Kind != ContentKindSeries || item.Classification != ClassificationGroup {
			t.Fatalf("item %q classificado como %q/%q, want series/group",
				item.Title, item.Kind, item.Classification)
		}
		if item.Group == "" {
			t.Fatalf("item %q perdeu a categoria original do grupo", item.Title)
		}
	}
}

func TestParseM3UReportsMalformedEntriesWithoutDiscardingValidOnes(t *testing.T) {
	playlist, err := ParseM3U(strings.NewReader(`#EXTM3U
#EXTINF:-1 group-title="Filmes",Filme válido
https://example.invalid/movie/valid.mp4
#EXTINF:-1
https://example.invalid/movie/invalid.mp4
#EXTINF:-1 group-title="Filmes",Outro filme
https://example.invalid/movie/valid-2.mp4
`), ParseOptions{SourceID: "fixture"})
	if err != nil {
		t.Fatalf("ParseM3U() error = %v", err)
	}
	if len(playlist.Items) != 2 {
		t.Fatalf("itens = %d, want 2", len(playlist.Items))
	}
	if len(playlist.Warnings) != 2 || playlist.Warnings[0].Code != "invalid-extinf" || playlist.Warnings[1].Code != "orphan-stream" {
		t.Fatalf("warnings = %+v, want invalid-extinf + orphan-stream", playlist.Warnings)
	}
}

func TestParseM3UEnforcesItemLimit(t *testing.T) {
	_, err := ParseM3U(strings.NewReader(`#EXTM3U
#EXTINF:-1 group-title="TV",Canal 1
https://example.invalid/live/1.m3u8
#EXTINF:-1 group-title="TV",Canal 2
https://example.invalid/live/2.m3u8
`), ParseOptions{SourceID: "fixture", MaxItems: 1})
	if err == nil || !strings.Contains(err.Error(), "limite de 1 itens") {
		t.Fatalf("erro = %v, want item limit", err)
	}
}

func TestParseM3UDoesNotExposeSourceURLInWarnings(t *testing.T) {
	streamURL := "https://example.invalid/stream.m3u8"
	playlist, err := ParseM3U(strings.NewReader("#EXTM3U\n#EXTINF:-1\n"+streamURL+"\n"), ParseOptions{SourceID: "fixture"})
	if err != nil {
		t.Fatalf("ParseM3U() error = %v", err)
	}
	for _, warning := range playlist.Warnings {
		if strings.Contains(warning.Message, streamURL) {
			t.Fatalf("warning expôs URL de origem: %+v", warning)
		}
	}
}

func TestParseM3UStreamDeliversBatchesAndMatchesParseM3U(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")
	for index := 0; index < 7; index++ {
		builder.WriteString("#EXTINF:-1 group-title=\"TV\",Fixture ")
		builder.WriteString(strconv.Itoa(index))
		builder.WriteString("\nhttps://example.invalid/live/")
		builder.WriteString(strconv.Itoa(index))
		builder.WriteString(".m3u8\n")
	}

	buffered, err := ParseM3U(strings.NewReader(builder.String()), ParseOptions{SourceID: "fixture"})
	if err != nil {
		t.Fatalf("ParseM3U() error = %v", err)
	}

	var batches [][]Item
	summary, err := ParseM3UStream(strings.NewReader(builder.String()),
		ParseOptions{SourceID: "fixture", BatchSize: 3}, func(batch []Item) error {
			copyBatch := make([]Item, len(batch))
			_ = copy(copyBatch, batch)
			batches = append(batches, copyBatch)
			return nil
		})
	if err != nil {
		t.Fatalf("ParseM3UStream() error = %v", err)
	}
	if summary.Total != 7 || len(summary.Warnings) != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	sizes := make([]int, 0, len(batches))
	for _, batch := range batches {
		sizes = append(sizes, len(batch))
	}
	if len(batches) != 3 || sizes[0] != 3 || sizes[1] != 3 || sizes[2] != 1 {
		t.Fatalf("lotes = %v", sizes)
	}
	flattened := make([]Item, 0, summary.Total)
	for _, batch := range batches {
		flattened = append(flattened, batch...)
	}
	if len(flattened) != len(buffered.Items) {
		t.Fatalf("total streaming=%d buffered=%d", len(flattened), len(buffered.Items))
	}
	for index := range flattened {
		if flattened[index].ID != buffered.Items[index].ID || flattened[index].Title != buffered.Items[index].Title {
			t.Fatalf("item %d divergente: %+v vs %+v", index, flattened[index], buffered.Items[index])
		}
	}
}

func TestParseM3UStreamCapsWarningsAndPropagatesHandlerError(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n#EXTINF:-1\nsem-url\n")
	for index := 0; index < maxParseWarnings+10; index++ {
		builder.WriteString("orfanado-sem-extinf.m3u8\n")
	}
	summary, err := ParseM3UStream(strings.NewReader(builder.String()), ParseOptions{SourceID: "fixture"}, func([]Item) error {
		return nil
	})
	if err != nil {
		t.Fatalf("ParseM3UStream() error = %v", err)
	}
	if len(summary.Warnings) != maxParseWarnings {
		t.Fatalf("warnings = %d, want %d", len(summary.Warnings), maxParseWarnings)
	}

	if _, err := ParseM3UStream(strings.NewReader("#EXTM3U\n"), ParseOptions{SourceID: "fixture"}, nil); err == nil {
		t.Fatalf("handler nil deveria falhar")
	}
	handlerErr := errors.New("falha de destino")
	if _, err := ParseM3UStream(strings.NewReader("#EXTINF:-1,Canal\nhttps://example.invalid/a.m3u8\n"),
		ParseOptions{SourceID: "fixture"}, func([]Item) error { return handlerErr }); !errors.Is(err, handlerErr) {
		t.Fatalf("erro do handler = %v", err)
	}
}

func TestItemIdentitySurvivesRotatingStreamURL(t *testing.T) {
	first, err := ParseM3U(strings.NewReader("#EXTM3U\n#EXTINF:-1 group-title=\"Filmes\",Filme Estável\nhttps://cdn.example.invalid/stream.m3u8?token=one\n"), ParseOptions{SourceID: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseM3U(strings.NewReader("#EXTM3U\n#EXTINF:-1 group-title=\"Filmes\",Filme Estável\nhttps://cdn.example.invalid/stream.m3u8?token=two\n"), ParseOptions{SourceID: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Items[0].ID != second.Items[0].ID {
		t.Fatalf("ID mudou com token efêmero: %q != %q", first.Items[0].ID, second.Items[0].ID)
	}
}
