package sandbox

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func buildInfer(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "type", func(l *lua.State) int {
		l.PushString(inferType(luaArrayAny(l, 1)))
		return 1
	})
	setGoFunc(L, t, "detect", func(l *lua.State) int {
		l.PushString(inferType([]any{luaToAny(l, 1)}))
		return 1
	})
	setGoFunc(L, t, "types", func(l *lua.State) int {
		rows := luaArrayAny(l, 1)
		opts := toAnyMap(l, 2)
		fields := stringSlice(opts["fields"])
		if len(fields) == 0 {
			fields = rowFields(rows)
		}
		out := map[string]any{}
		for _, f := range fields {
			vals := make([]any, 0, len(rows))
			for _, r := range rows {
				if m, ok := r.(map[string]any); ok {
					vals = append(vals, m[f])
				}
			}
			out[f] = inferType(vals)
		}
		pushAny(l, out)
		return 1
	})
	setGoFunc(L, t, "coerce", func(l *lua.State) int {
		vals := luaArrayAny(l, 1)
		typ := argString(l, 2)
		if typ == "" {
			typ = inferType(vals)
		}
		out := make([]any, len(vals))
		for i, v := range vals {
			out[i] = coerceValue(v, typ)
		}
		pushAny(l, out)
		return 1
	})

	return t
}

func inferType(vals []any) string {
	var clean []any
	for _, v := range vals {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		clean = append(clean, v)
	}
	if len(clean) == 0 {
		return "string"
	}
	allNum, allInt := true, true
	for _, v := range clean {
		n, ok := numOrNil(v)
		if !ok {
			allNum, allInt = false, false
			break
		}
		if n != math.Trunc(n) {
			allInt = false
		}
	}
	if allNum {
		if allInt {
			return "integer"
		}
		return "number"
	}
	allBool := true
	for _, v := range clean {
		s := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		if s != "true" && s != "false" && s != "1" && s != "0" && s != "yes" && s != "no" {
			allBool = false
			break
		}
	}
	if allBool {
		return "boolean"
	}
	allDate := true
	for _, v := range clean {
		s, ok := v.(string)
		if !ok || !isDateStr(s) {
			allDate = false
			break
		}
	}
	if allDate {
		return "date"
	}
	return "string"
}

func numOrNil(v any) (float64, bool) {
	if n, ok := numOpt(v); ok {
		return n, true
	}
	if s, ok := v.(string); ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func stringSlice(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, len(x))
		for i, e := range x {
			out[i] = fmt.Sprint(e)
		}
		return out
	case []string:
		return x
	case string:
		parts := strings.Split(x, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

func rowFields(rows []any) []string {
	keys := map[string]bool{}
	for _, r := range rows {
		if m, ok := r.(map[string]any); ok {
			for k := range m {
				keys[k] = true
			}
		}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cmpAny(a, b any) int {
	if an, ok := numOrNil(a); ok {
		if bn, ok2 := numOrNil(b); ok2 {
			if an < bn {
				return -1
			}
			if an > bn {
				return 1
			}
			return 0
		}
	}
	s1, s2 := fmt.Sprint(a), fmt.Sprint(b)
	switch {
	case s1 < s2:
		return -1
	case s1 > s2:
		return 1
	}
	return 0
}
