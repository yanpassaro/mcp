package sqlize

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"ntdsk.com/mcp/sqlize/internal/data"
)

type piiConfig struct {
	Version  int                 `json:"version"`
	Labels   map[string]string   `json:"labels"`
	Columns  map[string][]string `json:"columns"`
	Contexts map[string][]string `json:"contexts"`
	Address  piiAddress          `json:"address"`
}

type piiAddress struct {
	Starts []string `json:"starts"`
	Preps  []string `json:"preps"`
}

type piiState struct {
	columnEntityMap map[string]string
	contextRe       map[string]*regexp.Regexp
	labels          map[string]string
	addressPreps    map[string]bool
}

var (
	pii    = piiState{}
	piiErr = loadPII()
)

func loadPII() error {
	cfg := piiConfig{}
	if err := json.Unmarshal(data.PII, &cfg); err != nil {
		return fmt.Errorf("pii.json inválido: %w", err)
	}
	pii.columnEntityMap = make(map[string]string)
	pii.contextRe = make(map[string]*regexp.Regexp)
	pii.labels = cfg.Labels
	for ent, names := range cfg.Columns {
		for _, n := range names {
			pii.columnEntityMap[colKey(n)] = ent
		}
	}
	for ent, words := range cfg.Contexts {
		if len(words) == 0 {
			continue
		}
		parts := make([]string, 0, len(words))
		for _, w := range words {
			w = normalizeWord(strings.TrimSpace(w))
			if w != "" {
				parts = append(parts, regexp.QuoteMeta(w))
			}
		}
		pii.contextRe[ent] = regexp.MustCompile(fmt.Sprintf(`(?i)\b(?:%s)\b`, strings.Join(parts, "|")))
	}
	pii.addressPreps = toSet(cfg.Address.Preps)
	return nil
}

func toSet(words []string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		w = normalizeWord(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		m[w] = true
	}
	return m
}

func labelFor(entity string) string {
	l, ok := pii.labels[entity]
	if !ok {
		return "[VALOR]"
	}
	return fmt.Sprintf("[%s]", l)
}

func contextBoost(entity, cell string, start, end int) bool {
	re := pii.contextRe[entity]
	if re == nil {
		return false
	}
	if start < 0 {
		return false
	}
	if end > len(cell) {
		return false
	}
	lo := max(start-24, 0)
	hi := min(end+24, len(cell))
	return re.MatchString(normalizeWord(cell[lo:hi]))
}
