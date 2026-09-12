// Regras versionadas para playlists inteligentes (M7/PLY-05).
// Sem duplicar vídeos materializados: a regra é avaliada sob demanda sobre o
// catálogo local + estado pessoal.
package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

const CurrentSmartRuleVersion = 1

// SmartRule é o contrato versionado de uma playlist inteligente.
type SmartRule struct {
	Version        int             `json:"version"`
	Filter         PlaylistFilter  `json:"filter"`
	Sort           PlaylistSortKey `json:"sort"`
	Term           string          `json:"term,omitempty"`
	SubscribedOnly bool            `json:"subscribed_only,omitempty"`
}

func (r SmartRule) IsZero() bool {
	return r.Filter.IsZero() && r.Sort == "" && r.Term == "" && !r.SubscribedOnly
}

func (r SmartRule) Validate() error {
	if r.Version != CurrentSmartRuleVersion {
		return fmt.Errorf("versão de regra %d não suportada (esperado %d)", r.Version, CurrentSmartRuleVersion)
	}
	return nil
}

func MarshalSmartRule(r SmartRule) ([]byte, error) {
	r.Version = CurrentSmartRuleVersion
	return json.Marshal(r)
}

func UnmarshalSmartRule(data []byte) (SmartRule, error) {
	var r SmartRule
	if err := json.Unmarshal(data, &r); err != nil {
		return SmartRule{}, fmt.Errorf("regra inválida: %w", err)
	}
	if r.Version > CurrentSmartRuleVersion {
		return SmartRule{}, fmt.Errorf("regra versão futura %d", r.Version)
	}
	if r.Version == 0 {
		r.Version = CurrentSmartRuleVersion
	}
	if err := r.Validate(); err != nil {
		return SmartRule{}, err
	}
	return r, nil
}

// UpgradeSmartRule é o ponto único de conversão entre versões de regra
// (modelo versionado, D-064). Contrato:
//   - regras sem versão (0) assumem a versão atual — legado da 00009;
//   - a versão publicada hoje é apenas a 1, então nenhuma conversão existe;
//   - cada versão nova adiciona um case aqui e incrementa
//     CurrentSmartRuleVersion junto com migration;
//   - versão futura é erro: o app nunca inventa compatibilidade.
func UpgradeSmartRule(data []byte, fromVersion int) (SmartRule, error) {
	if fromVersion > CurrentSmartRuleVersion {
		return SmartRule{}, fmt.Errorf("regra versão futura %d (suportado até %d)", fromVersion, CurrentSmartRuleVersion)
	}
	rule, err := UnmarshalSmartRule(data)
	if err != nil {
		return SmartRule{}, err
	}
	// Coluna e JSON discordam só é aceitável no legado sem versão (coluna 0).
	if fromVersion != 0 && rule.Version != fromVersion {
		return SmartRule{}, fmt.Errorf("regra inconsistente: coluna v%d ≠ conteúdo v%d", fromVersion, rule.Version)
	}
	return rule, nil
}

func (r SmartRule) Summary() string {
	if r.IsZero() {
		return "sem filtros"
	}
	parts := []string{}
	if r.Term != "" {
		parts = append(parts, "termo: "+r.Term)
	}
	if !r.Filter.IsZero() {
		parts = append(parts, fmt.Sprintf("%d filtros", r.Filter.ActiveCount()))
	}
	if r.Sort != "" && r.Sort != PlaylistSortManual {
		parts = append(parts, "ordem: "+string(r.Sort))
	}
	if r.SubscribedOnly {
		parts = append(parts, "só inscrições")
	}
	if len(parts) == 0 {
		return "sem filtros"
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " · "
		}
		out += p
	}
	return out
}

// BuiltInPresets retorna os 4 presets do accept de PLY-05.
func BuiltInPresets() map[string]SmartRule {
	return map[string]SmartRule{
		"nao_assistidos_inscricoes": {
			Version:        CurrentSmartRuleVersion,
			Filter:         PlaylistFilter{Watched: SearchWatchUnwatched, Age: FeedAgeWeek},
			Sort:           PlaylistSortPublished,
			SubscribedOnly: true,
		},
		"continuar": {
			Version: CurrentSmartRuleVersion,
			Filter:  PlaylistFilter{Watched: SearchWatchContinue},
			Sort:    PlaylistSortAdded,
		},
		"favoritos_recentes": {
			Version: CurrentSmartRuleVersion,
			Filter:  PlaylistFilter{Favorite: true},
			Sort:    PlaylistSortAdded,
		},
		"canal_duracao": {
			Version: CurrentSmartRuleVersion,
			Filter:  PlaylistFilter{Duration: SearchDurationMedium},
			Sort:    PlaylistSortDuration,
		},
	}
}

// Explain retorna razões pelas quais o vídeo casa com a regra.
func Explain(rule SmartRule, video Video, state WatchState) []string {
	var reasons []string
	f := rule.Filter
	if f.Channel != "" {
		reasons = append(reasons, "canal: "+f.Channel)
	}
	if !f.IsZero() {
		if f.Favorite {
			reasons = append(reasons, "favorito")
		}
		if f.Watched != "" && f.Watched != SearchWatchAny {
			reasons = append(reasons, string(f.Watched))
		}
		if f.ShortsOnly {
			reasons = append(reasons, "Shorts")
		}
		if f.Content != "" && f.Content != FeedAll {
			reasons = append(reasons, string(f.Content))
		}
	}
	if rule.Term != "" {
		reasons = append(reasons, "termo: "+rule.Term)
	}
	if rule.SubscribedOnly {
		reasons = append(reasons, "inscrição")
	}
	if state.Progress != nil {
		if p, ok := state.Progress[video.ID]; ok {
			if p.Completed {
				reasons = append(reasons, "já assistido")
			} else if p.Position > 0 {
				reasons = append(reasons, "em andamento")
			}
		}
	}
	if state.Favorite != nil && state.Favorite[video.ID] && !f.Favorite {
		reasons = append(reasons, "salvo nos favoritos")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "regra: sem filtros")
	}
	return reasons
}

// WatchState é o estado pessoal mínimo para explain.
type WatchState struct {
	Progress map[string]ProgressInfo
	Favorite map[string]bool
}

// SmartRulesBackup é a seção "rules" do .ntbackup (PLY-06/D-064): regras de
// playlists inteligentes e presets, com o JSON da regra intacto (a conversão
// de versão acontece na carga, via UpgradeSmartRule).
type SmartRulesBackup struct {
	Format  string           `json:"format"` // nanotube-smart-rules
	Version int              `json:"version"`
	Rules   []SmartRuleRow   `json:"rules,omitempty"`
	Presets []SmartPresetRow `json:"presets,omitempty"`
}

// SmartRuleRow é uma regra vinculada a uma playlist local.
type SmartRuleRow struct {
	PlaylistID string    `json:"playlist_id"`
	Version    int       `json:"version"`
	Rule       string    `json:"rule"` // rule_json original
	UpdatedAt  time.Time `json:"updated_at"`
}

// SmartPresetRow é um preset salvo pelo usuário.
type SmartPresetRow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	Rule      string    `json:"rule"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
