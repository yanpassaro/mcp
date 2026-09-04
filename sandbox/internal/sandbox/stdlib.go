package sandbox

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math"
	"regexp"
	"sort"
	"strconv"
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

func toStringMap(v goja.Value) map[string]any {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return map[string]any{}
	}
	if m, ok := v.Export().(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func buildList(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("chunk", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		return vm.ToValue(chunk(arr, int(toNum(call.Argument(1)))))
	})
	o.Set("groupBy", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		keyArg := call.Argument(1)
		fn, isFn := goja.AssertFunction(keyArg)
		prop := keyProp(keyArg, isFn)
		groups := map[string][]any{}
		for _, item := range arr {
			k := keyOf(vm, item, keyArg, fn, isFn, prop).String()
			groups[k] = append(groups[k], item)
		}
		res := make(map[string]any, len(groups))
		for k, v := range groups {
			res[k] = v
		}
		return vm.ToValue(res)
	})
	o.Set("unique", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		return vm.ToValue(uniqueItems(arr))
	})
	o.Set("flatten", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		return vm.ToValue(flatten(arr))
	})
	o.Set("sortBy", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		keyArg := call.Argument(1)
		fn, isFn := goja.AssertFunction(keyArg)
		prop := keyProp(keyArg, isFn)
		cp := append([]any(nil), arr...)
		sort.SliceStable(cp, func(i, j int) bool {
			return lessValue(keyOf(vm, cp[i], keyArg, fn, isFn, prop), keyOf(vm, cp[j], keyArg, fn, isFn, prop))
		})
		return vm.ToValue(cp)
	})
	o.Set("countBy", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		keyArg := call.Argument(1)
		fn, isFn := goja.AssertFunction(keyArg)
		prop := keyProp(keyArg, isFn)
		counts := map[string]int{}
		for _, item := range arr {
			counts[keyOf(vm, item, keyArg, fn, isFn, prop).String()]++
		}
		return vm.ToValue(counts)
	})
	o.Set("first", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		n := int(toNum(call.Argument(1)))
		if n <= 0 {
			n = 1
		}
		if n >= len(arr) {
			return vm.ToValue(arr)
		}
		return vm.ToValue(arr[:n])
	})
	o.Set("last", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		n := int(toNum(call.Argument(1)))
		if n <= 0 {
			n = 1
		}
		if n >= len(arr) {
			return vm.ToValue(arr)
		}
		return vm.ToValue(arr[len(arr)-n:])
	})
	return o
}

func keyProp(keyArg goja.Value, isFn bool) string {
	if isFn {
		return ""
	}
	return keyArg.String()
}

func keyOf(vm *goja.Runtime, item any, keyArg goja.Value, fn goja.Callable, isFn bool, prop string) goja.Value {
	if isFn {
		v, err := fn(goja.Undefined(), vm.ToValue(item))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return v
	}
	if prop == "" {
		return goja.Undefined()
	}
	return vm.ToValue(propValue(item, prop))
}

func propValue(item any, prop string) any {
	if m, ok := item.(map[string]any); ok {
		return m[prop]
	}
	return nil
}

func chunk(arr []any, n int) [][]any {
	if n <= 0 {
		return [][]any{}
	}
	var out [][]any
	for i := 0; i < len(arr); i += n {
		end := i + n
		if end > len(arr) {
			end = len(arr)
		}
		out = append(out, arr[i:end])
	}
	return out
}

func uniqueItems(arr []any) []any {
	seen := map[string]bool{}
	var out []any
	for _, v := range arr {
		k := identityKey(v)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}

func identityKey(v any) string {
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return fmt.Sprint(v)
}

func flatten(arr []any) []any {
	var out []any
	for _, v := range arr {
		if sub, ok := v.([]any); ok {
			out = append(out, flatten(sub)...)
		} else {
			out = append(out, v)
		}
	}
	return out
}

func lessValue(a, b goja.Value) bool {
	if isNil(a) {
		return !isNil(b)
	}
	if isNil(b) {
		return false
	}
	if an, ok := toFloat(a); ok {
		if bn, okB := toFloat(b); okB {
			return an < bn
		}
	}
	return a.String() < b.String()
}

func isNil(v goja.Value) bool {
	return v == nil || goja.IsUndefined(v) || goja.IsNull(v)
}

func toFloat(v goja.Value) (float64, bool) {
	switch n := v.Export().(type) {
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

func buildNum(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("round", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(round(toNum(call.Argument(0)), int(toNum(call.Argument(1)))))
	})
	o.Set("clamp", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(clamp(toNum(call.Argument(0)), toNum(call.Argument(1)), toNum(call.Argument(2))))
	})
	o.Set("percent", func(call goja.FunctionCall) goja.Value {
		a, b := toNum(call.Argument(0)), toNum(call.Argument(1))
		if b == 0 {
			return vm.ToValue(0)
		}
		return vm.ToValue(a / b * 100)
	})
	o.Set("sum", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sumNums(vm, call.Argument(0)))
	})
	o.Set("avg", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		if len(arr) == 0 {
			return vm.ToValue(0)
		}
		return vm.ToValue(sumNums(vm, call.Argument(0)) / float64(len(arr)))
	})
	o.Set("parse", func(call goja.FunctionCall) goja.Value {
		s := strings.TrimSpace(call.Argument(0).String())
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return vm.ToValue(v)
		}
		if iv, err := strconv.ParseInt(s, 10, 64); err == nil {
			return vm.ToValue(iv)
		}
		return vm.ToValue(math.NaN())
	})
	o.Set("fmt", func(call goja.FunctionCall) goja.Value {
		n := toNum(call.Argument(0))
		dec, loc := 2, ""
		if len(call.Arguments) > 1 && !isNilValue(call.Argument(1)) {
			if isStringValue(call.Argument(1)) {
				loc = call.Argument(1).String()
				if len(call.Arguments) > 2 {
					dec = int(toNum(call.Argument(2)))
				}
			} else {
				dec = int(toNum(call.Argument(1)))
				if len(call.Arguments) > 2 {
					loc = call.Argument(2).String()
				}
			}
		}
		return vm.ToValue(formatNum(n, dec, loc))
	})
	return o
}

func sumNums(vm *goja.Runtime, v goja.Value) float64 {
	arr, _ := v.Export().([]any)
	var s float64
	for _, e := range arr {
		s += toNum(vm.ToValue(e))
	}
	return s
}

func round(v float64, digits int) float64 {
	if digits < 0 {
		digits = 0
	}
	p := math.Pow10(digits)
	return math.Round(v*p) / p
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func buildEncode(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("crc32", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(call.Argument(0).String()))))
	})
	o.Set("md5", func(call goja.FunctionCall) goja.Value {
		sum := md5.Sum([]byte(call.Argument(0).String()))
		return vm.ToValue(hex.EncodeToString(sum[:]))
	})
	o.Set("sha256", func(call goja.FunctionCall) goja.Value {
		sum := sha256.Sum256([]byte(call.Argument(0).String()))
		return vm.ToValue(hex.EncodeToString(sum[:]))
	})
	o.Set("base64", func(call goja.FunctionCall) goja.Value {
		s := call.Argument(0).String()
		switch strings.ToLower(strings.TrimSpace(optString(call, 1))) {
		case "", "encode", "std", "standard":
			return vm.ToValue(base64.StdEncoding.EncodeToString([]byte(s)))
		case "decode", "dec":
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(string(b))
		case "url", "encodeurl", "urlencode", "urlsafe", "url-safe":
			return vm.ToValue(base64.URLEncoding.EncodeToString([]byte(s)))
		case "urldecode", "decodeurl", "url_decode":
			b, err := base64.URLEncoding.DecodeString(s)
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(string(b))
		default:
			panic(vm.NewGoError(fmt.Errorf("modo base64 inválido: %s", optString(call, 1))))
		}
	})
	o.Set("hex", func(call goja.FunctionCall) goja.Value {
		s := call.Argument(0).String()
		switch strings.ToLower(strings.TrimSpace(optString(call, 1))) {
		case "", "encode", "enc":
			return vm.ToValue(hex.EncodeToString([]byte(s)))
		case "decode", "dec":
			b, err := hex.DecodeString(s)
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(string(b))
		default:
			panic(vm.NewGoError(fmt.Errorf("modo hex inválido: %s", optString(call, 1))))
		}
	})
	return o
}

func formatNum(n float64, dec int, loc string) string {
	if dec < 0 {
		dec = 2
	}
	if dec > 20 {
		dec = 20
	}
	neg := n < 0 || (n == 0 && math.Signbit(n))
	s := strconv.FormatFloat(math.Abs(n), 'f', dec, 64)
	var intPart, decPart string
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, decPart = s[:i], s[i+1:]
	} else {
		intPart = s
	}
	thousandSep, decimalSep := ",", "."
	l := strings.ToLower(loc)
	if l == "pt-br" || l == "pt_br" || l == "pt" {
		thousandSep, decimalSep = ".", ","
	}
	grouped := groupThousands(intPart, thousandSep, 3)
	if neg {
		grouped = "-" + grouped
	}
	if decPart != "" {
		return grouped + decimalSep + decPart
	}
	return grouped
}

func groupThousands(s, sep string, width int) string {
	if len(s) <= width {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%width == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(c)
	}
	return b.String()
}

func buildJson(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("parse", func(call goja.FunctionCall) goja.Value {
		var v any
		if err := json.Unmarshal([]byte(call.Argument(0).String()), &v); err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(v)
	})
	o.Set("stringify", func(call goja.FunctionCall) goja.Value {
		exp := call.Argument(0).Export()
		if indent := int(toNum(call.Argument(1))); indent > 0 {
			b, err := json.MarshalIndent(exp, "", strings.Repeat(" ", indent))
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(string(b))
		}
		b, err := json.Marshal(exp)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(string(b))
	})
	o.Set("format", func(call goja.FunctionCall) goja.Value {
		b, err := json.MarshalIndent(call.Argument(0).Export(), "", "  ")
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(string(b))
	})
	o.Set("minify", func(call goja.FunctionCall) goja.Value {
		var v any
		if err := json.Unmarshal([]byte(call.Argument(0).String()), &v); err != nil {
			panic(vm.NewGoError(err))
		}
		b, err := json.Marshal(v)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(string(b))
	})
	o.Set("path", func(call goja.FunctionCall) goja.Value {
		v, ok := jsonPath(call.Argument(0).Export(), call.Argument(1).String())
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(v)
	})
	return o
}

func jsonPath(v any, path string) (any, bool) {
	if strings.TrimSpace(path) == "" {
		return v, true
	}
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		switch cur := v.(type) {
		case map[string]any:
			var ok bool
			v, ok = cur[part]
			if !ok {
				return nil, false
			}
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(cur) {
				return nil, false
			}
			v = cur[idx]
		default:
			return nil, false
		}
	}
	return v, true
}

func buildAssert(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("ok", func(call goja.FunctionCall) goja.Value {
		if !call.Argument(0).ToBoolean() {
			panic(vm.NewGoError(fmt.Errorf("assert.ok falhou: %s", msgOr(call, 1, "esperado um valor truthy"))))
		}
		return goja.Undefined()
	})
	o.Set("equal", func(call goja.FunctionCall) goja.Value {
		a, b := call.Argument(0), call.Argument(1)
		if !a.StrictEquals(b) {
			panic(vm.NewGoError(fmt.Errorf("assert.equal falhou: %s != %s", a.String(), b.String())))
		}
		return goja.Undefined()
	})
	o.Set("throws", func(call goja.FunctionCall) goja.Value {
		fn, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("assert.throws espera uma função")))
		}
		if _, err := fn(goja.Undefined()); err == nil {
			panic(vm.NewGoError(fmt.Errorf("assert.throws falhou: nada foi lançado")))
		}
		return goja.Undefined()
	})
	return o
}

func optString(call goja.FunctionCall, idx int) string {
	if idx < len(call.Arguments) {
		if v := call.Arguments[idx]; !isNilValue(v) {
			return v.String()
		}
	}
	return ""
}

func msgOr(call goja.FunctionCall, idx int, def string) string {
	if idx < len(call.Arguments) {
		if s := call.Arguments[idx].String(); s != "" {
			return s
		}
	}
	return def
}
