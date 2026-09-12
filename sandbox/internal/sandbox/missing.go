package sandbox

import (
	"fmt"
	"strings"

	lua "github.com/Shopify/go-lua"
)

var defaultMissing = map[string]bool{
	"NA": true, "N/A": true, "NULL": true, "NIL": true, "NONE": true,
	"-": true, "(NULL)": true, "#N/A": true, "TBD": true, "UNKNOWN": true,
}

func buildMissing(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "is_null", func(l *lua.State) int {
		l.PushBoolean(isNullVal(luaToAny(l, 1), toAnyMap(l, 2)))
		return 1
	})

	setGoFunc(L, t, "count", func(l *lua.State) int {
		vals := missingVals(luaArrayAny(l, 1), optString(toAnyMap(l, 2), "field"))
		c := 0
		for _, v := range vals {
			if isNullVal(v, toAnyMap(l, 2)) {
				c++
			}
		}
		l.PushNumber(float64(c))
		return 1
	})

	setGoFunc(L, t, "which", func(l *lua.State) int {
		vals := missingVals(luaArrayAny(l, 1), optString(toAnyMap(l, 2), "field"))
		opts := toAnyMap(l, 2)
		out := []any{}
		for i, v := range vals {
			if isNullVal(v, opts) {
				out = append(out, float64(i+1))
			}
		}
		pushAny(l, out)
		return 1
	})

	setGoFunc(L, t, "fill", func(l *lua.State) int {
		opts := toAnyMap(l, 2)
		vals := missingVals(luaArrayAny(l, 1), optString(opts, "field"))
		pushAny(l, missingFill(vals, opts))
		return 1
	})

	setGoFunc(L, t, "drop", func(l *lua.State) int {
		pushAny(l, missingDrop(luaArrayAny(l, 1), toAnyMap(l, 2)))
		return 1
	})

	return t
}

func isNullVal(v any, opts map[string]any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	if !ok {
		return false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	up := strings.ToUpper(s)
	if defaultMissing[up] {
		return true
	}
	if arr, ok := opts["sentinels"].([]any); ok {
		for _, e := range arr {
			if strings.ToUpper(strings.TrimSpace(fmt.Sprint(e))) == up {
				return true
			}
		}
	}
	return false
}

func missingVals(values []any, field string) []any {
	if field == "" {
		return values
	}
	out := make([]any, 0, len(values))
	for _, r := range values {
		if m, ok := r.(map[string]any); ok {
			out = append(out, m[field])
		}
	}
	return out
}

func missingFill(vals []any, opts map[string]any) []any {
	out := append([]any(nil), vals...)

	if v, present := opts["value"]; present {
		for i, e := range out {
			if isNullVal(e, opts) {
				out[i] = v
			}
		}
		return out
	}

	method := optString(opts, "method")
	switch method {
	case "zero":
		for i, e := range out {
			if isNullVal(e, opts) {
				out[i] = float64(0)
			}
		}
	case "mean", "median":
		var nums []float64
		for _, e := range out {
			if n, ok := numOrNil(e); ok {
				nums = append(nums, n)
			}
		}
		fill := 0.0
		if len(nums) > 0 {
			if method == "mean" {
				fill = mean(nums)
			} else {
				fill = quantile(nums, 0.5)
			}
		}
		for i, e := range out {
			if isNullVal(e, opts) {
				out[i] = cleanFloat(fill)
			}
		}
	case "prev", "previous":
		last := any(nil)
		for i, e := range out {
			if isNullVal(e, opts) {
				if last != nil {
					out[i] = last
				}
			} else {
				last = e
			}
		}
	case "next":
		nxt := any(nil)
		for i := len(out) - 1; i >= 0; i-- {
			e := out[i]
			if isNullVal(e, opts) {
				if nxt != nil {
					out[i] = nxt
				}
			} else {
				nxt = e
			}
		}
	case "first":
		fill := any(nil)
		for _, e := range out {
			if !isNullVal(e, opts) {
				fill = e
				break
			}
		}
		for i, e := range out {
			if isNullVal(e, opts) && fill != nil {
				out[i] = fill
			}
		}
	case "last":
		fill := any(nil)
		for _, e := range out {
			if !isNullVal(e, opts) {
				fill = e
			}
		}
		for i, e := range out {
			if isNullVal(e, opts) && fill != nil {
				out[i] = fill
			}
		}
	}
	return out
}

func missingDrop(rows []any, opts map[string]any) []any {
	field := optString(opts, "field")
	out := []any{}
	for _, r := range rows {
		m, ok := r.(map[string]any)
		if !ok {
			out = append(out, r)
			continue
		}
		var fields []string
		if field != "" {
			fields = []string{field}
		} else {
			fields = make([]string, 0, len(m))
			for k := range m {
				fields = append(fields, k)
			}
		}
		has := false
		for _, f := range fields {
			if isNullVal(m[f], opts) {
				has = true
				break
			}
		}
		if !has {
			out = append(out, r)
		}
	}
	return out
}
