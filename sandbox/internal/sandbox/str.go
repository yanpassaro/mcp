package sandbox

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	lua "github.com/Shopify/go-lua"
	"golang.org/x/text/unicode/norm"
)

func buildStr(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "normalize", func(l *lua.State) int {
		l.PushString(normalizeStr(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "slug", func(l *lua.State) int {
		l.PushString(slugify(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "title", func(l *lua.State) int {
		l.PushString(titleCase(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "camel", func(l *lua.State) int {
		l.PushString(camelCase(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "pascal", func(l *lua.State) int {
		l.PushString(pascalCase(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "snake", func(l *lua.State) int {
		l.PushString(joinWords("_", argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "kebab", func(l *lua.State) int {
		l.PushString(joinWords("-", argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "wrap", func(l *lua.State) int {
		pushAny(l, wrapText(argString(l, 1), int(argNum(l, 2))))
		return 1
	})
	setGoFunc(L, t, "summarize", func(l *lua.State) int {
		l.PushString(summarize(argString(l, 1), int(argNum(l, 2))))
		return 1
	})
	setGoFunc(L, t, "format", func(l *lua.State) int {
		f := argString(l, 1)
		args := make([]any, 0, l.Top()-1)
		for i := 2; i <= l.Top(); i++ {
			args = append(args, luaToAny(l, i))
		}
		l.PushString(fmt.Sprintf(f, luafmtArgs(f, args)...))
		return 1
	})
	setGoFunc(L, t, "count", func(l *lua.State) int {
		l.PushInteger(strings.Count(argString(l, 1), argString(l, 2)))
		return 1
	})
	setGoFunc(L, t, "split", func(l *lua.State) int {
		s := argString(l, 1)
		sep := argString(l, 2)
		if limit := int(argNum(l, 3)); limit > 0 {
			pushAny(l, strings.SplitN(s, sep, limit))
			return 1
		}
		pushAny(l, strings.Split(s, sep))
		return 1
	})
	setGoFunc(L, t, "extract", func(l *lua.State) int {
		s := argString(l, 1)
		pattern := argString(l, 2)
		opts := toAnyMap(l, 3)
		re, err := regexp.Compile(pattern)
		if err != nil {
			panic(fmt.Errorf("str.extract: %v", err))
		}
		group := int(optUint(opts, "group", 0))
		out := []any{}
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			if group == 0 {
				out = append(out, m[0])
				continue
			}
			if group < len(m) {
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
		}
		if n > 1 {
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

func normalizeStr(s string) string {
	s = strings.TrimSpace(s)
	t := norm.NFKD.String(s)
	var b strings.Builder
	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String())
}

func slugify(s string) string {
	return joinWords("-", strings.ToLower(s))
}

func titleCase(s string) string {
	fields := strings.Fields(normalizeStr(strings.ToLower(s)))
	for i, w := range fields {
		fields[i] = upperFirst(w)
	}
	return strings.Join(fields, " ")
}

func camelCase(s string) string {
	ws := splitWords(s)
	for i := range ws {
		if i == 0 {
			ws[i] = strings.ToLower(ws[i])
			continue
		}
		ws[i] = upperFirst(strings.ToLower(ws[i]))
	}
	return strings.Join(ws, "")
}

func pascalCase(s string) string {
	ws := splitWords(s)
	for i := range ws {
		ws[i] = upperFirst(strings.ToLower(ws[i]))
	}
	return strings.Join(ws, "")
}

func joinWords(sep, s string) string {
	ws := splitWords(s)
	for i := range ws {
		ws[i] = strings.ToLower(ws[i])
	}
	return strings.Join(ws, sep)
}

func luafmtArgs(f string, args []any) []any {
	out := make([]any, 0, len(args))
	idx := 0
	for i := 0; i < len(f); i++ {
		if f[i] != '%' {
			continue
		}
		j := i + 1
		for j < len(f) {
			if !strings.ContainsRune("+-# 0.123456789", rune(f[j])) {
				break
			}
			j++
		}
		if j >= len(f) {
			continue
		}
		switch verb := f[j]; verb {
		case '%':
		case 'd', 'i', 'x', 'X', 'o', 'b', 'c':
			if idx < len(args) {
				if fl, ok := args[idx].(float64); ok {
					out = append(out, int64(fl))
					continue
				}
				out = append(out, args[idx])
			}
			idx++
		case 's':
			if idx < len(args) {
				out = append(out, fmtStrArg(args[idx]))
			}
			idx++
		default:
			if idx < len(args) {
				out = append(out, args[idx])
			}
			idx++
		}
		i = j
	}
	return out
}

func fmtStrArg(v any) any {
	switch x := v.(type) {
	case float64:
		return fmt.Sprint(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return "nil"
	default:
		return x
	}
}

func upperFirst(w string) string {
	r := []rune(w)
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
	}
	return string(r)
}

func splitWords(s string) []string {
	runes := []rune(normalizeStr(s))
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = nil
		}
	}
	for i, r := range runes {
		if !unicode.IsLetter(r) {
			if !unicode.IsDigit(r) {
				flush()
				continue
			}
		}
		if len(cur) > 0 {
			prev := cur[len(cur)-1]
			nextLower := false
			if i+1 < len(runes) {
				nextLower = unicode.IsLower(runes[i+1])
			}
			if unicode.IsUpper(r) {
				if unicode.IsLower(prev) {
					flush()
				}
				if unicode.IsUpper(prev) {
					if nextLower {
						flush()
					}
				}
			}
		}
		cur = append(cur, r)
	}
	flush()
	return words
}

const DEFAULT_WRAP_WIDTH = 80

func wrapText(s string, width int) []string {
	if width <= 0 {
		width = DEFAULT_WRAP_WIDTH
	}
	words := strings.Fields(normalizeStr(s))
	if len(words) == 0 {
		return []string{}
	}
	lines := []string{}
	cur := strings.Builder{}
	curLen := 0
	for _, w := range words {
		wl := utf8.RuneCountInString(w)
		if curLen > 0 {
			if curLen+1+wl > width {
				lines = append(lines, cur.String())
				cur.Reset()
				curLen = 0
			}
		}
		if curLen > 0 {
			cur.WriteByte(' ')
			curLen++
		}
		cur.WriteString(w)
		curLen += wl
	}
	if curLen > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

func summarize(s string, max int) string {
	r := []rune(s)
	if max <= 0 {
		return s
	}
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return fmt.Sprintf("%s...", string(r[:max-3]))
}


