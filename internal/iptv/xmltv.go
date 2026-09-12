package iptv

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// ParseXMLTV percorre o guia incrementalmente. Os callbacks controlam como o
// caller persiste os dados e podem interromper a importação com erro.
func ParseXMLTV(reader io.Reader, onChannel func(GuideChannel) error, onProgram func(GuideProgram) error) error {
	if reader == nil {
		return fmt.Errorf("ler XMLTV: reader nil")
	}

	decoder := xml.NewDecoder(reader)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("ler XMLTV: %w", err)
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		switch start.Name.Local {
		case "channel":
			var value struct {
				ID          string `xml:"id,attr"`
				DisplayName string `xml:"display-name"`
				Icon        struct {
					Source string `xml:"src,attr"`
				} `xml:"icon"`
			}
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return fmt.Errorf("decodificar canal XMLTV: %w", err)
			}
			channel := GuideChannel{
				ID:      strings.TrimSpace(value.ID),
				Name:    strings.TrimSpace(value.DisplayName),
				LogoURL: strings.TrimSpace(value.Icon.Source),
			}
			if channel.ID == "" || channel.Name == "" {
				continue
			}
			if onChannel != nil {
				if err := onChannel(channel); err != nil {
					return err
				}
			}

		case "programme":
			var value struct {
				ChannelID   string `xml:"channel,attr"`
				Start       string `xml:"start,attr"`
				Stop        string `xml:"stop,attr"`
				Title       string `xml:"title"`
				Description string `xml:"desc"`
			}
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return fmt.Errorf("decodificar programa XMLTV: %w", err)
			}
			program, err := normalizeGuideProgram(value.ChannelID, value.Start, value.Stop, value.Title, value.Description)
			if err != nil {
				return err
			}
			if onProgram != nil {
				if err := onProgram(program); err != nil {
					return err
				}
			}
		}
	}
}

func normalizeGuideProgram(channelID, startValue, stopValue, title, description string) (GuideProgram, error) {
	start, err := parseXMLTVTime(startValue)
	if err != nil {
		return GuideProgram{}, fmt.Errorf("decodificar início XMLTV: %w", err)
	}
	end, err := parseXMLTVTime(stopValue)
	if err != nil {
		return GuideProgram{}, fmt.Errorf("decodificar fim XMLTV: %w", err)
	}
	if strings.TrimSpace(channelID) == "" || strings.TrimSpace(title) == "" {
		return GuideProgram{}, fmt.Errorf("decodificar programa XMLTV: canal e título são obrigatórios")
	}
	if !end.After(start) {
		return GuideProgram{}, fmt.Errorf("decodificar programa XMLTV: fim deve ser posterior ao início")
	}
	return GuideProgram{
		ChannelID:   strings.TrimSpace(channelID),
		Title:       strings.TrimSpace(title),
		Description: strings.TrimSpace(description),
		Start:       start,
		End:         end,
	}, nil
}

func parseXMLTVTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{
		"20060102150405 -0700",
		"20060102150405Z0700",
		"20060102150405",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("horário XMLTV inválido")
}
