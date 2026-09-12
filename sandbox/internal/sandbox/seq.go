package sandbox

import (
	lua "github.com/Shopify/go-lua"
)

func buildSeq(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "range", func(l *lua.State) int {
		n := int(argNum(l, 1))
		if n < 0 {
			n = 0
		}
		out := make([]any, 0, n)
		for i := 1; i <= n; i++ {
			out = append(out, float64(i))
		}
		pushAny(l, out)
		return 1
	})
	setGoFunc(L, t, "iota", func(l *lua.State) int {
		n := int(argNum(l, 1))
		if n < 0 {
			n = 0
		}
		out := make([]any, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, float64(i))
		}
		pushAny(l, out)
		return 1
	})
	setGoFunc(L, t, "arange", func(l *lua.State) int {
		start, stop := argNum(l, 1), argNum(l, 2)
		step := argNum(l, 3)
		if step == 0 {
			step = 1
		}
		out := []any{}
		if step > 0 {
			for v := start; v < stop; v += step {
				out = append(out, float64(v))
			}
		} else {
			for v := start; v > stop; v += step {
				out = append(out, float64(v))
			}
		}
		pushAny(l, out)
		return 1
	})
	setGoFunc(L, t, "repeat", func(l *lua.State) int {
		v := luaToAny(l, 1)
		n := int(argNum(l, 2))
		if n < 0 {
			n = 0
		}
		out := make([]any, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, v)
		}
		pushAny(l, out)
		return 1
	})
	setGoFunc(L, t, "zip", func(l *lua.State) int {
		arrays := make([][]any, 0, l.Top())
		min := -1
		for i := 1; i <= l.Top(); i++ {
			arr := luaArrayAny(l, i)
			arrays = append(arrays, arr)
			if min == -1 || len(arr) < min {
				min = len(arr)
			}
		}
		out := []any{}
		for i := 0; i < min; i++ {
			tup := make([]any, len(arrays))
			for j, a := range arrays {
				tup[j] = a[i]
			}
			out = append(out, tup)
		}
		pushAny(l, out)
		return 1
	})
	setGoFunc(L, t, "enumerate", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		start := int(argNum(l, 2))
		if start == 0 {
			start = 1
		}
		out := make([]any, len(arr))
		for i, e := range arr {
			out[i] = []any{float64(start + i), e}
		}
		pushAny(l, out)
		return 1
	})

	return t
}
