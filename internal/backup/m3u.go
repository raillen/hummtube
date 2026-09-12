package backup

import (
	"fmt"
	"strings"
)

// ToM3U serializa videoIDs em M3U8 com URLs do YouTube. Campos não preserváveis:
// ordem vs published, tags, cor, descrição, notas.
func ToM3U(videoIDs []string) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, id := range videoIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("#EXTINF:-1,%s\nhttps://www.youtube.com/watch?v=%s\n", id, id))
	}
	return b.String()
}

// FromM3U extrai video IDs de um M3U (linhas com watch?v= ou youtu.be/).
func FromM3U(data string) []string {
	var out []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		id := extractVideoID(line)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func extractVideoID(url string) string {
	if idx := strings.Index(url, "watch?v="); idx >= 0 {
		rest := url[idx+8:]
		if amp := strings.Index(rest, "&"); amp >= 0 {
			rest = rest[:amp]
		}
		if hash := strings.Index(rest, "#"); hash >= 0 {
			rest = rest[:hash]
		}
		return strings.TrimSpace(rest)
	}
	if idx := strings.Index(url, "youtu.be/"); idx >= 0 {
		rest := url[idx+9:]
		if q := strings.Index(rest, "?"); q >= 0 {
			rest = rest[:q]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}
