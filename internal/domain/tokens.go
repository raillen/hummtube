// Tokenização de tópicos compartilhada pelo engine de recomendação (que casa
// tópicos com vídeos) e pela derivação de temas no storage (que cria os temas
// base a partir do histórico). Uma única regra evita temas que o engine nunca
// encontraria.
package domain

import (
	"strings"
	"unicode"
)

// topicStopwords são as palavras ignoradas na tokenização (pt-BR + en).
var topicStopwords = map[string]bool{
	"de": true, "do": true, "da": true, "dos": true, "das": true,
	"em": true, "no": true, "na": true, "nos": true, "nas": true, "o": true,
	"e": true, "que": true, "um": true, "uma": true, "com": true, "para": true,
	"como": true, "por": true, "the": true, "of": true, "and": true, "to": true,
	"in": true, "on": true, "is": true, "for": true,
}

// TopicTokens normaliza texto livre nos tokens usados para casar tópicos com
// vídeos: minúsculas, só letras/dígitos, mínimo 2 caracteres, sem stopwords.
func TopicTokens(s string) []string {
	out := make([]string, 0, 8)
	for _, field := range strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		tok := strings.ToLower(field)
		if len(tok) < 2 || topicStopwords[tok] {
			continue
		}
		out = append(out, tok)
	}
	return out
}
