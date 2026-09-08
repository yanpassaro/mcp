package sandbox

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	lua "github.com/Shopify/go-lua"
)

type tmplPart struct {
	text  string
	path  string
	isVar bool
}

func buildTemplate(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "render", func(l *lua.State) int {
		parts := compileTemplate(argString(l, 1))
		l.PushString(renderTemplate(parts, toAnyMap(l, 2)))
		return 1
	})

	setGoFunc(L, t, "compile", func(l *lua.State) int {
		parts := compileTemplate(argString(l, 1))
		l.PushGoFunction(func(l *lua.State) int {
			l.PushString(renderTemplate(parts, toAnyMap(l, 1)))
			return 1
		})
		return 1
	})

	setGoFunc(L, t, "loop", func(l *lua.State) int {
		frag := argString(l, 1)
		items := luaArrayAny(l, 2)
		base := toAnyMap(l, 3)
		parts := compileTemplate(frag)
		var b strings.Builder
		for _, item := range items {
			m := mergeMap(base, asMap(item))
			b.WriteString(renderTemplate(parts, m))
		}
		l.PushString(b.String())
		return 1
	})

	setGoFunc(L, t, "cond", func(l *lua.State) int {
		truthy := l.ToValue(1) != nil && l.ToValue(1) != false
		vars := toAnyMap(l, 4)
		if truthy {
			l.PushString(renderTemplate(compileTemplate(argString(l, 2)), vars))
		} else {
			elseStr := ""
			if l.Top() >= 3 {
				elseStr = argString(l, 3)
			}
			l.PushString(renderTemplate(compileTemplate(elseStr), vars))
		}
		return 1
	})

	return t
}

func compileTemplate(s string) []tmplPart {
	var parts []tmplPart
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, tmplPart{text: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(s); {
		if s[i] == '{' {
			if i+1 < len(s) && s[i+1] == '{' {
				if end := strings.Index(s[i+2:], "}}"); end >= 0 {
					key := strings.TrimSpace(s[i+2 : i+2+end])
					if key != "" {
						flush()
						parts = append(parts, tmplPart{path: key, isVar: true})
						i += 2 + end + 2
						continue
					}
				}
			} else if end := strings.Index(s[i+1:], "}"); end >= 0 {
				key := strings.TrimSpace(s[i+1 : i+1+end])
				if key != "" {
					flush()
					parts = append(parts, tmplPart{path: key, isVar: true})
					i += 1 + end + 1
					continue
				}
			}
		}
		lit.WriteByte(s[i])
		i++
	}
	flush()
	return parts
}

func renderTemplate(parts []tmplPart, vars map[string]any) string {
	var b strings.Builder
	for _, p := range parts {
		if !p.isVar {
			b.WriteString(p.text)
			continue
		}
		if v, ok := jsonPath(vars, p.path); ok {
			b.WriteString(tmplText(v))
		}
	}
	return b.String()
}

func tmplText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case map[string]any, []any:
		if b, err := json.Marshal(x); err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(v)
}

func mergeMap(base, over map[string]any) map[string]any {
	m := make(map[string]any, len(base)+len(over))
	for k, v := range base {
		m[k] = v
	}
	for k, v := range over {
		m[k] = v
	}
	return m
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
