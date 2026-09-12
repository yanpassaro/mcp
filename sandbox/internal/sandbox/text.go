package sandbox

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	lua "github.com/Shopify/go-lua"
)

var reToken = regexp.MustCompile(`[\pL\pN]+`)

func buildText(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "extract", func(l *lua.State) int {
		s := argString(l, 1)
		pattern := argString(l, 2)
		opts := toAnyMap(l, 3)
		re, err := regexp.Compile(pattern)
		if err != nil {
			panic(fmt.Errorf("text.extract: %v", err))
		}
		group := int(optUint(opts, "group", 0))
		out := []any{}
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			if group == 0 {
				out = append(out, m[0])
			} else if group < len(m) {
				out = append(out, m[group])
			}
		}
		pushAny(l, out)
		return 1
	})

	setGoFunc(L, t, "tokens", func(l *lua.State) int {
		pushAny(l, textToAny(textTokens(argString(l, 1), toAnyMap(l, 2))))
		return 1
	})

	setGoFunc(L, t, "ngrams", func(l *lua.State) int {
		toks := textTokens(argString(l, 1), toAnyMap(l, 3))
		n := int(argNum(l, 2))
		if n <= 0 {
			n = 2
		}
		out := []any{}
		if n <= 1 {
			for _, tk := range toks {
				out = append(out, tk)
			}
		} else {
			for i := 0; i+n <= len(toks); i++ {
				out = append(out, strings.Join(toks[i:i+n], " "))
			}
		}
		pushAny(l, out)
		return 1
	})

	setGoFunc(L, t, "similarity", func(l *lua.State) int {
		a, b := argString(l, 1), argString(l, 2)
		opts := toAnyMap(l, 3)
		method := optString(opts, "method")
		if method == "" {
			method = "levenshtein"
		}
		if method == "jaccard" {
			l.PushNumber(cleanFloat(jaccard(tokenSet(textTokens(a, map[string]any{})), tokenSet(textTokens(b, map[string]any{})))))
			return 1
		}
		l.PushNumber(cleanFloat(stringRatio(a, b)))
		return 1
	})

	setGoFunc(L, t, "match", func(l *lua.State) int {
		a, b := argString(l, 1), argString(l, 2)
		threshold := argNum(l, 3)
		if threshold <= 0 {
			threshold = 0.7
		}
		l.PushBoolean(stringRatio(a, b) >= threshold)
		return 1
	})

	setGoFunc(L, t, "keywords", func(l *lua.State) int {
		s := argString(l, 1)
		n := int(argNum(l, 2))
		if n <= 0 {
			n = 10
		}
		counts := map[string]int{}
		for _, w := range textTokens(s, map[string]any{"min_len": 1}) {
			counts[w]++
		}
		type kv struct {
			w string
			c int
		}
		arr := make([]kv, 0, len(counts))
		for w, c := range counts {
			arr = append(arr, kv{w, c})
		}
		sort.Slice(arr, func(i, j int) bool {
			if arr[i].c == arr[j].c {
				return arr[i].w < arr[j].w
			}
			return arr[i].c > arr[j].c
		})
		if n > len(arr) {
			n = len(arr)
		}
		out := []any{}
		for _, e := range arr[:n] {
			out = append(out, map[string]any{"word": e.w, "count": float64(e.c)})
		}
		pushAny(l, out)
		return 1
	})

	return t
}

func textTokens(s string, opts map[string]any) []string {
	toks := reToken.FindAllString(s, -1)
	lower := asBool(opts["lower"], true)
	minLen := int(optUint(opts, "min_len", 1))
	stop := stringSet(opts["stopwords"])
	out := make([]string, 0, len(toks))
	for _, w := range toks {
		if lower {
			w = strings.ToLower(w)
		}
		if utf8.RuneCountInString(w) < minLen {
			continue
		}
		if stop[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

func textToAny(tokens []string) []any {
	out := make([]any, len(tokens))
	for i, tk := range tokens {
		out[i] = tk
	}
	return out
}

func stringSet(v any) map[string]bool {
	m := map[string]bool{}
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			m[fmt.Sprint(e)] = true
		}
	case []string:
		for _, s := range x {
			m[s] = true
		}
	}
	return m
}

func tokenSet(tokens []string) map[string]bool {
	m := map[string]bool{}
	for _, tk := range tokens {
		m[tk] = true
	}
	return m
}

func jaccard(a, b map[string]bool) float64 {
	inter, union := 0, 0
	for k := range a {
		if b[k] {
			inter++
		}
		union++
	}
	for k := range b {
		if !a[k] {
			union++
		}
	}
	if union == 0 {
		return 1
	}
	return float64(inter) / float64(union)
}
