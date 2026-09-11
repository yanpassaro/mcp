package sandbox

import (
	"math"
	"sort"

	lua "github.com/Shopify/go-lua"
)

func buildStats(L *lua.State) int {
	t := newTable(L)

	single := func(name string, f func([]float64) float64) {
		setGoFunc(L, t, name, func(l *lua.State) int {
			l.PushNumber(cleanFloat(f(statsNums(l, 1, statsField(l, 2)))))
			return 1
		})
	}
	single("count", func(v []float64) float64 { return float64(len(v)) })
	single("min", func(v []float64) float64 {
		if len(v) == 0 {
			return math.NaN()
		}
		m := v[0]
		for _, x := range v[1:] {
			if x < m {
				m = x
			}
		}
		return m
	})
	single("max", func(v []float64) float64 {
		if len(v) == 0 {
			return math.NaN()
		}
		m := v[0]
		for _, x := range v[1:] {
			if x > m {
				m = x
			}
		}
		return m
	})
	single("avg", func(v []float64) float64 {
		if len(v) == 0 {
			return math.NaN()
		}
		return mean(v)
	})
	single("median", func(v []float64) float64 { return quantile(v, 0.5) })

	setGoFunc(L, t, "quantile", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 3))
		q := argNum(l, 2)
		l.PushNumber(cleanFloat(quantile(v, q)))
		return 1
	})
	setGoFunc(L, t, "percentile", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 3))
		p := argNum(l, 2)
		l.PushNumber(cleanFloat(quantile(v, p/100)))
		return 1
	})
	setGoFunc(L, t, "variance", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 3))
		l.PushNumber(cleanFloat(variance(v, asBool(luaToAny(l, 2), false))))
		return 1
	})
	setGoFunc(L, t, "stdev", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 3))
		l.PushNumber(cleanFloat(math.Sqrt(variance(v, asBool(luaToAny(l, 2), false)))))
		return 1
	})
	setGoFunc(L, t, "range", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 2))
		if len(v) == 0 {
			pushAny(l, map[string]any{})
			return 1
		}
		lo, hi := v[0], v[0]
		for _, x := range v[1:] {
			if x < lo {
				lo = x
			}
			if x > hi {
				hi = x
			}
		}
		pushAny(l, map[string]any{"min": cleanFloat(lo), "max": cleanFloat(hi)})
		return 1
	})
	setGoFunc(L, t, "mode", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 2))
		counts := map[float64]int{}
		for _, x := range v {
			counts[x]++
		}
		best := 0
		for _, c := range counts {
			if c > best {
				best = c
			}
		}
		vals := []float64{}
		for x, c := range counts {
			if c == best {
				vals = append(vals, x)
			}
		}
		sort.Float64s(vals)
		out := make([]any, len(vals))
		for i, x := range vals {
			out[i] = x
		}
		pushAny(l, out)
		return 1
	})
	setGoFunc(L, t, "histogram", func(l *lua.State) int {
		opts := toAnyMap(l, 2)
		v := statsNums(l, 1, optString(opts, "field"))
		bins := 0
		if b := optUint(opts, "bins", 0); b > 0 {
			bins = int(b)
		}
		pushAny(l, histogram(v, bins))
		return 1
	})

	return t
}

func statsNums(l *lua.State, index int, field string) []float64 {
	arr := luaArrayAny(l, index)
	out := make([]float64, 0, len(arr))
	for _, e := range arr {
		var v any = e
		if field != "" {
			v = itemProp(e, field)
		}
		if n, ok := numOpt(v); ok && !math.IsNaN(n) && !math.IsInf(n, 0) {
			out = append(out, n)
		}
	}
	return out
}

func statsField(l *lua.State, index int) string {
	if s, ok := luaToAny(l, index).(map[string]any); ok {
		return optString(s, "field")
	}
	return ""
}

func mean(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func quantile(v []float64, q float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s) == 1 {
		return s[0]
	}
	pos := q * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return s[lo]
	}
	return s[lo] + (pos-float64(lo))*(s[hi]-s[lo])
}

func variance(v []float64, sample bool) float64 {
	n := len(v)
	if n == 0 {
		return math.NaN()
	}
	m := mean(v)
	var ss float64
	for _, x := range v {
		d := x - m
		ss += d * d
	}
	if sample {
		if n < 2 {
			return math.NaN()
		}
		return ss / float64(n-1)
	}
	return ss / float64(n)
}

func histogram(v []float64, bins int) []any {
	if len(v) == 0 {
		return []any{}
	}
	if bins <= 0 {
		bins = int(math.Ceil(math.Log2(float64(len(v)) + 1)))
		if bins < 1 {
			bins = 1
		}
		if bins > 20 {
			bins = 20
		}
	}
	lo, hi := v[0], v[0]
	for _, x := range v[1:] {
		if x < lo {
			lo = x
		}
		if x > hi {
			hi = x
		}
	}
	if lo == hi {
		return []any{map[string]any{"lo": cleanFloat(lo), "hi": cleanFloat(hi), "count": float64(len(v))}}
	}
	width := (hi - lo) / float64(bins)
	counts := make([]int, bins)
	for _, x := range v {
		idx := int((x - lo) / width)
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		counts[idx]++
	}
	out := make([]any, bins)
	for i := 0; i < bins; i++ {
		bLo := lo + float64(i)*width
		bHi := bLo + width
		if i == bins-1 {
			bHi = hi
		}
		out[i] = map[string]any{"lo": cleanFloat(bLo), "hi": cleanFloat(bHi), "count": float64(counts[i])}
	}
	return out
}
