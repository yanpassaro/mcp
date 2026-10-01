package sandbox

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func buildJson(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "parse", func(l *lua.State) int {
		v := any(nil)
		if err := json.Unmarshal([]byte(argString(l, 1)), &v); err != nil {
			panic(err)
		}
		pushAny(l, v)
		return 1
	})
	setGoFunc(L, t, "stringify", func(l *lua.State) int {
		exp := luaToAny(l, 1)
		indent := int(argNum(l, 2))
		if indent > 0 {
			b, err := json.MarshalIndent(exp, "", strings.Repeat(" ", indent))
			if err != nil {
				panic(err)
			}
			l.PushString(string(b))
			return 1
		}
		b, err := json.Marshal(exp)
		if err != nil {
			panic(err)
		}
		l.PushString(string(b))
		return 1
	})
	setGoFunc(L, t, "format", func(l *lua.State) int {
		val := luaToAny(l, 1)
		str, ok := val.(string)
		if ok {
			parsed := any(nil)
			if err := json.Unmarshal([]byte(strings.TrimSpace(str)), &parsed); err == nil {
				val = parsed
			}
		}
		opts := toAnyMap(l, 2)
		indent := 2
		d, ok := numOpt(opts["indent"])
		if ok {
			if d > 0 {
				indent = int(d)
			}
		}
		if truthyOpt(opts, "nested") {
			val = parseNestedJSON(val)
		}
		b, err := json.MarshalIndent(val, "", strings.Repeat(" ", indent))
		if err != nil {
			panic(err)
		}
		out := string(b)
		mx, ok := numOpt(opts["max"])
		if ok {
			if mx > 0 {
				if len(out) > int(mx) {
					out = fmt.Sprintf("%s\n... (truncado)", out[:int(mx)])
				}
			}
		}
		l.PushString(out)
		return 1
	})
	setGoFunc(L, t, "minify", func(l *lua.State) int {
		v := any(nil)
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
			return 1
		}
		pushAny(l, v)
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
		if len(s) > 0 {
			if isJSONStart(s) {
				parsed := any(nil)
				if err := json.Unmarshal([]byte(s), &parsed); err == nil {
					return parseNestedJSON(parsed)
				}
			}
		}
		return x
	default:
		return v
	}
}

func isJSONStart(s string) bool {
	if s[0] == '{' {
		return true
	}
	return s[0] == '['
}

func jsonPath(v any, path string) (any, bool) {
	if strings.TrimSpace(path) == "" {
		return v, true
	}
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		switch cur := v.(type) {
		case map[string]any:
			nv, ok := cur[part]
			if !ok {
				return nil, false
			}
			v = nv
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return nil, false
			}
			if idx < 0 {
				return nil, false
			}
			if idx >= len(cur) {
				return nil, false
			}
			v = cur[idx]
		default:
			return nil, false
		}
	}
	return v, true
}
