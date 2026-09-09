package sandbox

import (
	"encoding/json"
	"strconv"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func buildJson(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "parse", func(l *lua.State) int {
		var v any
		if err := json.Unmarshal([]byte(argString(l, 1)), &v); err != nil {
			panic(err)
		}
		pushAny(l, v)
		return 1
	})
	setGoFunc(L, t, "stringify", func(l *lua.State) int {
		exp := luaToAny(l, 1)
		if indent := int(argNum(l, 2)); indent > 0 {
			b, err := json.MarshalIndent(exp, "", strings.Repeat(" ", indent))
			if err != nil {
				panic(err)
			}
			l.PushString(string(b))
		} else {
			b, err := json.Marshal(exp)
			if err != nil {
				panic(err)
			}
			l.PushString(string(b))
		}
		return 1
	})
	setGoFunc(L, t, "format", func(l *lua.State) int {
		val := luaToAny(l, 1)
		if str, ok := val.(string); ok {
			var parsed any
			if err := json.Unmarshal([]byte(strings.TrimSpace(str)), &parsed); err == nil {
				val = parsed
			}
		}
		opts := toAnyMap(l, 2)
		indent := 2
		if d, ok := numOpt(opts["indent"]); ok && d > 0 {
			indent = int(d)
		}
		if truthyOpt(opts, "nested") {
			val = parseNestedJSON(val)
		}
		b, err := json.MarshalIndent(val, "", strings.Repeat(" ", indent))
		if err != nil {
			panic(err)
		}
		out := string(b)
		if mx, ok := numOpt(opts["max"]); ok && mx > 0 && len(out) > int(mx) {
			out = out[:int(mx)] + "\n... (truncado)"
		}
		l.PushString(out)
		return 1
	})
	setGoFunc(L, t, "minify", func(l *lua.State) int {
		var v any
		if err := json.Unmarshal([]byte(argString(l, 1)), &v); err != nil {
			panic(err)
		}
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		l.PushString(string(b))
		return 1
	})
	setGoFunc(L, t, "path", func(l *lua.State) int {
		v, ok := jsonPath(luaToAny(l, 1), argString(l, 2))
		if !ok {
			l.PushNil()
		} else {
			pushAny(l, v)
		}
		return 1
	})
	return t
}

func parseNestedJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			out[k] = parseNestedJSON(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = parseNestedJSON(val)
		}
		return out
	case string:
		s := strings.TrimSpace(x)
		if len(s) > 0 && (s[0] == '{' || s[0] == '[') {
			var parsed any
			if err := json.Unmarshal([]byte(s), &parsed); err == nil {
				return parseNestedJSON(parsed)
			}
		}
		return x
	default:
		return v
	}
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
