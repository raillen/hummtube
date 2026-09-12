package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/nanotube/nanotube-web/internal/domain"
)

func TestFrontendJSONContractsUseStableLowercaseKeys(t *testing.T) {
	values := []struct {
		name      string
		value     any
		required  []string
		forbidden []string
	}{
		{
			name: "account", value: domain.AccountInfo{Email: "user@example.invalid"},
			required: []string{"email", "connected_at"}, forbidden: []string{"Email", "ConnectedAt"},
		},
		{
			name: "video", value: domain.Video{ID: "video", Title: "Vídeo"},
			required: []string{"id", "title", "thumbnail_url"}, forbidden: []string{"ID", "Title"},
		},
		{
			name: "playback", value: domain.PlaybackPlan{
				Mode:    domain.PlaybackModeResolvedMedia,
				Primary: domain.ResolvedStream{URL: "https://media.example.invalid/video.mp4"},
				Variants: []domain.PlaybackVariant{{
					ID: "360", Label: "360p", Height: 360,
					Stream: domain.ResolvedStream{URL: "https://media.example.invalid/video-360.mp4"},
				}},
				AudioOnly: &domain.ResolvedStream{URL: "https://media.example.invalid/audio.m4a"},
			},
			required: []string{"mode", "primary", "metadata", "variants", "audio_only"}, forbidden: []string{"Mode", "Primary", "Variants", "AudioOnly"},
		},
	}

	for _, testCase := range values {
		t.Run(testCase.name, func(t *testing.T) {
			encoded, err := json.Marshal(testCase.value)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]any
			if err := json.Unmarshal(encoded, &object); err != nil {
				t.Fatal(err)
			}
			for _, key := range testCase.required {
				if _, ok := object[key]; !ok {
					t.Errorf("chave obrigatória %q ausente em %s", key, encoded)
				}
			}
			for _, key := range testCase.forbidden {
				if _, ok := object[key]; ok {
					t.Errorf("chave incompatível %q presente em %s", key, encoded)
				}
			}
		})
	}
}
