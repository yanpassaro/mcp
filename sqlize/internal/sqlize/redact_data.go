package sqlize

import (
	"encoding/json"
	"os"
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
	Name      piiName             `json:"name"`
	Address   piiAddress          `json:"address"`
	OrgSuffix []string            `json:"org_suffix"`
}

type piiName struct {
	FirstNames []string `json:"first_names"`
	Stopwords  []string `json:"stopwords"`
	Deny       []string `json:"deny"`
	Geo        []string `json:"geo"`
	Context    []string `json:"context"`
}

type piiAddress struct {
	Starts []string `json:"starts"`
	Preps  []string `json:"preps"`
}

var (
	columnEntityMap map[string]string
	contextRe       map[string]*regexp.Regexp
	labels          map[string]string
	brFirstNames    map[string]bool
	wordDeny        map[string]bool
	geoDeny         map[string]bool
	nameStopwords   map[string]bool
	addressPreps    map[string]bool
	orgSuffix       map[string]bool
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

	brFirstNames = toSet(cfg.Name.FirstNames)
	wordDeny = toSet(cfg.Name.Deny)
	geoDeny = toSet(cfg.Name.Geo)
	nameStopwords = toSet(cfg.Name.Stopwords)
	addressPreps = toSet(cfg.Address.Preps)
	orgSuffix = toSet(cfg.OrgSuffix)

	loadWordList(os.Getenv("SQLIZE_PII_NAMES"), brFirstNames)
	loadWordList(os.Getenv("SQLIZE_PII_WORDS"), wordDeny)
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
	return "[" + entity + "]"
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
