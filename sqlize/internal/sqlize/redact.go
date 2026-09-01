package sqlize

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maskThreshold = 0.5

type match struct {
	start  int
	end    int
	entity string
	score  float64
}

type patternRule struct {
	entity           string
	re               *regexp.Regexp
	score            float64
	skipIfAlphaParen bool
}

var patternRules = []patternRule{
	{entity: "CPF", re: regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`), score: 0.9},
	{entity: "CNPJ", re: regexp.MustCompile(`\b\d{2}\.?\d{3}\.?\d{3}\/?\d{4}-?\d{2}\b`), score: 0.9},
	{entity: "EMAIL", re: regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`), score: 1.0},
	{entity: "PHONE", re: regexp.MustCompile(`\b(?:\+?55[\s.\-]?)?\(?\d{2}\)?[\s.\-]?9?\d{4}[\s.\-]?\d{4}\b`), score: 0.8},
	{entity: "PHONE", re: regexp.MustCompile(`\(\d{2}\)[\s.\-]?9?\d{4}[\s.\-]?\d{4}\b`), score: 0.8},
	{entity: "CEP", re: regexp.MustCompile(`\b\d{5}-?\d{3}\b`), score: 0.8},
	{entity: "RG", re: regexp.MustCompile(`\b\d{1,2}\.?\d{3}\.?\d{3}-?[\dxX]\b`), score: 0.45},
	{entity: "CARD", re: regexp.MustCompile(`\b(?:\d{4}[ \-]?){3}\d{4}\b`), score: 0.9},
	{entity: "IP", re: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), score: 0.7},
	{entity: "MAC", re: regexp.MustCompile(`(?i)\b(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}\b`), score: 1.0},
	{entity: "JWT", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b`), score: 1.0},
	{entity: "HASH", re: regexp.MustCompile(`\b0x[a-fA-F0-9]{40}\b`), score: 1.0},
	{entity: "BTC", re: regexp.MustCompile(`\b(?:bc1|[13])[a-zA-HJ-NP-Z0-9]{25,39}\b`), score: 1.0},
	{entity: "URL", re: regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.\-]*:\/\/[^\s"'<>]+`), score: 1.0},
	{entity: "URL", re: regexp.MustCompile(`(?i)\bwww\.[^\s"'<>]+`), score: 1.0},
	{entity: "URL", re: regexp.MustCompile(`\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(?::\d{1,5})?(?:/[^\s"'<>]*)?`), score: 0.95, skipIfAlphaParen: true},
}

func validCPF(doc string) bool {
	d := digitsOnly(doc)
	if len(d) != 11 {
		return false
	}
	allSame := true
	for i := range d {
		if d[i] != d[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}
	sum := 0
	for i := range 9 {
		sum += int(d[i]-'0') * (10 - i)
	}
	r := 11 - sum%11
	if r >= 10 {
		r = 0
	}
	if r != int(d[9]-'0') {
		return false
	}
	sum = 0
	for i := range 10 {
		sum += int(d[i]-'0') * (11 - i)
	}
	r = 11 - sum%11
	if r >= 10 {
		r = 0
	}
	return r == int(d[10]-'0')
}

func validCNPJ(doc string) bool {
	d := digitsOnly(doc)
	if len(d) != 14 {
		return false
	}
	allSame := true
	for i := range d {
		if d[i] != d[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i := range 12 {
		sum += int(d[i]-'0') * w1[i]
	}
	r := sum % 11
	if r < 2 {
		r = 0
	} else {
		r = 11 - r
	}
	if r != int(d[12]-'0') {
		return false
	}
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum = 0
	for i := range 13 {
		sum += int(d[i]-'0') * w2[i]
	}
	r = sum % 11
	if r < 2 {
		r = 0
	} else {
		r = 11 - r
	}
	return r == int(d[13]-'0')
}

func luhn(s string) bool {
	d := digitsOnly(s)
	if len(d) < 12 {
		return false
	}
	sum, alt := 0, false
	for i := len(d) - 1; i >= 0; i-- {
		n := int(d[i] - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

func columnEntity(name string) (string, bool) {
	ent, ok := columnEntityMap[normalizeWord(name)]
	return ent, ok
}

var reNameSpan = regexp.MustCompile(`\p{Lu}\p{Ll}*(?:(?:\s+(?:de|da|do|dos|das|e)\s+|\s+)\p{Lu}\p{Ll}*)+`)

var reNameContext = regexp.MustCompile(`(?i)\b(?:com|para|por|sr\.?|sra\.?|srta\.?|dr\.?|dra\.?|senhor|senhora|dona|dom|prof\.?|eng\.?|adv\.?)\s+$`)

var reSingleName = regexp.MustCompile(`(?i:\b(?:sr\.?|sra\.?|srta\.?|dr\.?|dra\.?|senhor|senhora|dona|dom|prof\.?|eng\.?|adv\.?|com|para|por)\s+)(\p{Lu}\p{Ll}+)`)

func nameContext(prev string) bool {
	if len(prev) > 24 {
		prev = prev[len(prev)-24:]
	}
	return reNameContext.MatchString(prev)
}

func findNameMatches(colName, cell string) []match {
	if cell == "" {
		return nil
	}
	var out []match

	for _, idx := range reNameSpan.FindAllStringIndex(cell, -1) {
		span := cell[idx[0]:idx[1]]
		if s := nameScore(span, cell[:idx[0]]); s > 0 {
			out = append(out, match{start: idx[0], end: idx[1], entity: "PERSON", score: s})
		}
	}

	for _, idx := range reSingleName.FindAllStringSubmatchIndex(cell, -1) {
		start, end := idx[2], idx[3]
		span := cell[start:end]
		if s := nameScore(span, cell[:start]); s > 0 {
			out = append(out, match{start: start, end: end, entity: "PERSON", score: s})
		}
	}

	if len(out) == 0 {
		if m, ok := wholeCellName(colName, cell); ok {
			out = append(out, m)
		}
	}
	return out
}

func deniedToken(t string) bool {
	if brFirstNames[t] {
		return false
	}
	return wordDeny[t] || geoDeny[t]
}

func wholeCellName(_ string, cell string) (match, bool) {
	raw := strings.TrimSpace(cell)
	if raw == "" {
		return match{}, false
	}
	r0, _ := utf8.DecodeRuneInString(raw)
	if !unicode.IsUpper(r0) {
		return match{}, false
	}
	norm := normalizeWord(raw)
	tokens := strings.Fields(norm)
	if len(tokens) == 0 || len(tokens) > 4 {
		return match{}, false
	}
	known := 0
	first := ""
	for _, t := range tokens {
		if nameStopwords[t] {
			continue
		}
		if first == "" {
			first = t
		}
		if brFirstNames[t] {
			known++
		}
	}
	if first == "" || deniedToken(first) || geoDeny[norm] || orgSuffix[tokens[len(tokens)-1]] {
		return match{}, false
	}
	for _, t := range tokens {
		if t != first && wordDeny[t] {
			return match{}, false
		}
	}
	if known < 1 {
		return match{}, false
	}
	return match{start: 0, end: len(cell), entity: "PERSON", score: 0.9}, true
}

func nameScore(span, prev string) float64 {
	norm := normalizeWord(span)
	tokens := strings.Fields(norm)
	if len(tokens) == 0 {
		return 0
	}
	known := 0
	first := ""
	for _, t := range tokens {
		if nameStopwords[t] {
			continue
		}
		if first == "" {
			first = t
		}
		if brFirstNames[t] {
			known++
		}
	}
	if first == "" || deniedToken(first) || geoDeny[norm] || orgSuffix[tokens[len(tokens)-1]] {
		return 0
	}
	switch {
	case known >= 1:
		return 0.9
	case nameContext(prev):
		return 0.85
	default:
		return 0
	}
}

func analyzeCell(colName, cell string) []match {
	var ms []match
	for _, r := range patternRules {
		for _, idx := range r.re.FindAllStringIndex(cell, -1) {
			if r.skipIfAlphaParen && afterAlphaParen(cell, idx[1]) {
				continue
			}
			text := cell[idx[0]:idx[1]]
			score := r.score
			switch r.entity {
			case "CPF":
				if validCPF(text) {
					score = 1.0
				} else {
					score = 0.15
				}
			case "CNPJ":
				if validCNPJ(text) {
					score = 1.0
				} else {
					score = 0.15
				}
			case "CARD":
				if luhn(text) {
					score = 1.0
				} else {
					score = 0.15
				}
			}
			if score < 1.0 {
				if ent, ok := columnEntity(colName); ok && ent == r.entity {
					score = 1.0
				}
			}
			if score < 0.85 && contextBoost(r.entity, cell, idx[0], idx[1]) {
				score = 0.85
			}
			ms = append(ms, match{start: idx[0], end: idx[1], entity: r.entity, score: score})
		}
	}
	ms = append(ms, findNameMatches(colName, cell)...)
	ms = append(ms, findAddressMatches(cell)...)
	return ms
}

var reAddressSpan = regexp.MustCompile(`\b(?i:rua|r\.|av\.|avenida|travessa|alameda|estrada|rodovia|praca|praça|p[çc]a\.|beco|largo|viela|condominio|condomínio|conjunto|residencial|loteamento|chacara|chácara|sitio|sítio|fazenda)\s+[A-Za-z0-9á-úÁ-Ú.\-']+(?:\s+[A-Za-z0-9á-úÁ-Ú.\-']+){0,4}(?:[,\s]+\d{1,6}(?:[-\s/]\d{1,5})?)?(?:\s*,\s*[A-ZÀ-Ú]\p{Ll}+(?:\s+[A-ZÀ-Ú]\p{Ll}+)*)?`)

func findAddressMatches(cell string) []match {
	var out []match
	for _, idx := range reAddressSpan.FindAllStringIndex(cell, -1) {
		span := cell[idx[0]:idx[1]]
		if addrStartsWithPrep(span) {
			continue
		}
		out = append(out, match{start: idx[0], end: idx[1], entity: "ADDRESS", score: 0.85})
	}
	return out
}

func addrStartsWithPrep(span string) bool {
	words := strings.Fields(normalizeWord(span))
	return len(words) >= 2 && addressPreps[words[1]]
}

func resolveSpans(spans []match) []match {
	if len(spans) <= 1 {
		return spans
	}
	ss := append([]match(nil), spans...)
	sort.Slice(ss, func(i, j int) bool {
		if ss[i].start != ss[j].start {
			return ss[i].start < ss[j].start
		}
		if ss[i].score != ss[j].score {
			return ss[i].score > ss[j].score
		}
		return ss[i].end > ss[j].end
	})
	out := []match{}
	lastEnd := -1
	for _, m := range ss {
		if m.start < lastEnd {
			continue
		}
		out = append(out, m)
		lastEnd = m.end
	}
	return out
}

func maskSpan(entity, _ string) string {
	return labelFor(entity)
}

func applyColumnMask(colName, cell string, spans []match) string {
	if cell == "" {
		return cell
	}
	var usable []match
	for _, m := range spans {
		if m.score >= maskThreshold {
			usable = append(usable, m)
		}
	}
	if len(usable) == 0 {
		if ent, ok := columnEntity(colName); ok {
			return labelFor(ent)
		}
		return cell
	}
	usable = resolveSpans(usable)
	var b strings.Builder
	last := 0
	for _, m := range usable {
		b.WriteString(cell[last:m.start])
		b.WriteString(maskSpan(m.entity, cell[m.start:m.end]))
		last = m.end
	}
	b.WriteString(cell[last:])
	return b.String()
}

func RedactValue(s string) string {
	return applyColumnMask("", s, analyzeCell("", s))
}

func RedactRows(cols []string, rows [][]string) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = make([]string, len(row))
		for j, cell := range row {
			col := ""
			if j < len(cols) {
				col = cols[j]
			}
			out[i][j] = redactCell(col, cell)
		}
	}
	return out
}

func redactCell(colName, cell string) string {
	return applyColumnMask(colName, cell, analyzeCell(colName, cell))
}

func afterAlphaParen(s string, end int) bool {
	if end < 0 || end >= len(s) {
		return false
	}
	c := s[end]
	if c == '(' {
		return true
	}
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
