package iptv

import (
	"errors"
	"strings"
	"testing"
)

func TestParseXMLTVStreamsChannelsAndPrograms(t *testing.T) {
	var channels []GuideChannel
	var programs []GuideProgram
	err := ParseXMLTV(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="news.br"><display-name>Notícias</display-name><icon src="https://example.invalid/news.png"/></channel>
  <programme channel="news.br" start="20260821120000 -0300" stop="20260821130000 -0300">
    <title>Jornal fixture</title><desc>Descrição fixture</desc>
  </programme>
</tv>`), func(channel GuideChannel) error {
		channels = append(channels, channel)
		return nil
	}, func(program GuideProgram) error {
		programs = append(programs, program)
		return nil
	})
	if err != nil {
		t.Fatalf("ParseXMLTV() error = %v", err)
	}
	if len(channels) != 1 || channels[0].ID != "news.br" || channels[0].LogoURL == "" {
		t.Fatalf("channels = %+v", channels)
	}
	if len(programs) != 1 || programs[0].Title != "Jornal fixture" || !programs[0].End.After(programs[0].Start) {
		t.Fatalf("programs = %+v", programs)
	}
}

func TestParseXMLTVRejectsInvalidProgramWindow(t *testing.T) {
	err := ParseXMLTV(strings.NewReader(`<tv><programme channel="news.br" start="20260821130000 +0000" stop="20260821120000 +0000"><title>Inválido</title></programme></tv>`), nil, func(program GuideProgram) error {
		t.Fatalf("programa inválido recebido: %+v", program)
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "fim deve ser posterior") {
		t.Fatalf("erro = %v, want invalid window", err)
	}
}

func TestParseXMLTVAllowsCallbackCancellation(t *testing.T) {
	err := ParseXMLTV(strings.NewReader(`<tv><channel id="one"><display-name>Um</display-name></channel><channel id="two"><display-name>Dois</display-name></channel></tv>`), func(channel GuideChannel) error {
		return errors.New("callback stop")
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "callback stop") {
		t.Fatalf("erro = %v, want callback error", err)
	}
}
