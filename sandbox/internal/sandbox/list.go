package sandbox

import (
	"encoding/json"
	"fmt"
	"sort"

	lua "github.com/Shopify/go-lua"
)

func buildList(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "chunk", func(l *lua.State) int {
		pushAny(l, chunk(luaArrayAny(l, 1), int(argNum(l, 2))))
		return 1
	})
	setGoFunc(L, t, "groupBy", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		prop := argString(l, 2)
		groups := map[string][]any{}
		for _, item := range arr {
			k := fmt.Sprint(itemProp(item, prop))
			groups[k] = append(groups[k], item)
		}
		res := make(map[string]any, len(groups))
		for k, v := range groups {
			res[k] = v
		}
		pushAny(l, res)
		return 1
	})
	setGoFunc(L, t, "unique", func(l *lua.State) int {
		pushAny(l, uniqueItems(luaArrayAny(l, 1)))
		return 1
	})
	setGoFunc(L, t, "flatten", func(l *lua.State) int {
		pushAny(l, flatten(luaArrayAny(l, 1)))
		return 1
	})
	setGoFunc(L, t, "sortBy", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		cp := append([]any(nil), arr...)
		prop := argString(l, 2)
		sort.SliceStable(cp, func(i, j int) bool {
			return fmt.Sprint(itemProp(cp[i], prop)) < fmt.Sprint(itemProp(cp[j], prop))
		})
		pushAny(l, cp)
		return 1
	})
	setGoFunc(L, t, "countBy", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		prop := argString(l, 2)
		counts := map[string]int{}
		for _, item := range arr {
			counts[fmt.Sprint(itemProp(item, prop))]++
		}
		pushAny(l, counts)
		return 1
	})
	setGoFunc(L, t, "first", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		n := int(argNum(l, 2))
		if n <= 0 {
			n = 1
		}
		if n > len(arr) {
			n = len(arr)
		}
		pushAny(l, arr[:n])
		return 1
	})
	setGoFunc(L, t, "last", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		n := int(argNum(l, 2))
		if n <= 0 {
			n = 1
		}
		if n > len(arr) {
			n = len(arr)
		}
		pushAny(l, arr[len(arr)-n:])
		return 1
	})
	return t
}

func luaArrayAny(l *lua.State, index int) []any {
	switch v := luaToAny(l, index).(type) {
	case []any:
		return v
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out
	case []float64:
		out := make([]any, len(v))
		for i, n := range v {
			out[i] = n
		}
		return out
	case []int:
		out := make([]any, len(v))
		for i, n := range v {
			out[i] = float64(n)
		}
		return out
	default:
		return []any{}
	}
}

func itemProp(item any, prop string) any {
	switch m := item.(type) {
	case map[string]any:
		return m[prop]
	case map[any]any:
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
