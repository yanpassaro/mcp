package sandbox

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dop251/goja"
	"golang.org/x/text/unicode/norm"
)

func buildStr(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("normalize", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(normalizeStr(call.Argument(0).String()))
	})
	o.Set("slug", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(slugify(call.Argument(0).String()))
	})
	o.Set("title", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(titleCase(call.Argument(0).String()))
	})
	o.Set("camel", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(camelCase(call.Argument(0).String()))
	})
	o.Set("pascal", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(pascalCase(call.Argument(0).String()))
	})
	o.Set("snake", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(joinWords("_", call.Argument(0).String()))
	})
	o.Set("kebab", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(joinWords("-", call.Argument(0).String()))
	})
	o.Set("wrap", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(wrapText(call.Argument(0).String(), int(toNum(call.Argument(1)))))
	})
	o.Set("summarize", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(summarize(call.Argument(0).String(), int(toNum(call.Argument(1)))))
	})
	o.Set("format", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(renderTemplate(call.Argument(0).String(), toStringMap(call.Argument(1))))
	})
	o.Set("count", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(strings.Count(call.Argument(0).String(), optString(call, 1)))
	})
	o.Set("split", func(call goja.FunctionCall) goja.Value {
		s := call.Argument(0).String()
		sep := optString(call, 1)
		if limit := int(toNum(call.Argument(2))); limit > 0 {
			return vm.ToValue(strings.SplitN(s, sep, limit))
		}
		return vm.ToValue(strings.Split(s, sep))
	})
	return o
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
		} else {
			ws[i] = upperFirst(strings.ToLower(ws[i]))
		}
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
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if len(cur) > 0 {
			prev := cur[len(cur)-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsUpper(r) && (unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextLower)) {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return words
}

func wrapText(s string, width int) []string {
	if width <= 0 {
		width = 80
	}
	words := strings.Fields(normalizeStr(s))
	if len(words) == 0 {
		return []string{}
	}
	var lines []string
	var cur strings.Builder
	curLen := 0
	for _, w := range words {
		wl := utf8.RuneCountInString(w)
		if curLen > 0 && curLen+1+wl > width {
			lines = append(lines, cur.String())
			cur.Reset()
			curLen = 0
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
	if max <= 0 || len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

var tplRe = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

func renderTemplate(tpl string, ctx map[string]any) string {
	return tplRe.ReplaceAllStringFunc(tpl, func(m string) string {
		if sub := tplRe.FindStringSubmatch(m); len(sub) == 2 {
			if v, ok := ctx[strings.TrimSpace(sub[1])]; ok {
				return fmt.Sprint(v)
			}
		}
		return m
	})
}
