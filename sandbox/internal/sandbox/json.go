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
		b, err := json.MarshalIndent(luaToAny(l, 1), "", "  ")
		if err != nil {
			panic(err)
		}
		l.PushString(string(b))
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
