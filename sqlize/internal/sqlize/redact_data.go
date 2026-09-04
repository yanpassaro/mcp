package sqlize

import (
	"encoding/json"
	"regexp"
	"strings"

	"ntdsk.com/mcp/sqlize/internal/data"
)

var piiJSON = data.PII

type piiConfig struct {
	Version   int                 `json:"version"`
	Labels    map[string]string   `json:"labels"`
	Columns   map[string][]string `json:"columns"`
	Contexts  map[string][]string `json:"contexts"`
	Address   piiAddress          `json:"address"`
}

type piiAddress struct {
	Starts []string `json:"starts"`
	Preps  []string `json:"preps"`
}

	var (
		columnEntityMap map[string]string
		contextRe       map[string]*regexp.Regexp
		labels          map[string]string
		addressPreps    map[string]bool
	)

func init() { loadPII() }

func loadPII() {
	var cfg piiConfig
	if err := json.Unmarshal(piiJSON, &cfg); err != nil {
		panic("pii.json inválido: " + err.Error())
	}

	columnEntityMap = make(map[string]string)
	contextRe = make(map[string]*regexp.Regexp)
	labels = cfg.Labels
	for ent, names := range cfg.Columns {
		for _, n := range names {
			columnEntityMap[normalizeWord(n)] = ent
		}
	}
	for ent, words := range cfg.Contexts {
		if len(words) == 0 {
			continue
		}
		parts := make([]string, 0, len(words))
		for _, w := range words {
			if w = normalizeWord(strings.TrimSpace(w)); w != "" {
				parts = append(parts, regexp.QuoteMeta(w))
			}
		}
		contextRe[ent] = regexp.MustCompile(`(?i)\b(?:` + strings.Join(parts, "|") + `)\b`)
	}

	addressPreps = toSet(cfg.Address.Preps)
}

func toSet(words []string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		if w = normalizeWord(strings.TrimSpace(w)); w != "" {
			m[w] = true
		}
	}
	return m
}

func labelFor(entity string) string {
	if l, ok := labels[entity]; ok {
		return "[" + l + "]"
	}
	return "[VALOR]"
}

func contextBoost(entity, cell string, start, end int) bool {
	re := contextRe[entity]
	if re == nil || start < 0 || end > len(cell) {
		return false
	}
	lo := max(start - 24, 0)
	hi := min(end + 24, len(cell))
	return re.MatchString(normalizeWord(cell[lo:hi]))
}
