package sandbox

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func buildNum(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "round", func(l *lua.State) int {
		l.PushNumber(round(argNum(l, 1), int(argNum(l, 2))))
		return 1
	})
	setGoFunc(L, t, "clamp", func(l *lua.State) int {
		l.PushNumber(clamp(argNum(l, 1), argNum(l, 2), argNum(l, 3)))
		return 1
	})
	setGoFunc(L, t, "percent", func(l *lua.State) int {
		a, b := argNum(l, 1), argNum(l, 2)
		v := 0.0
		if b != 0 {
			v = a / b * 100
		}
		l.PushNumber(cleanFloat(v))
		return 1
	})
	setGoFunc(L, t, "sum", func(l *lua.State) int {
		l.PushNumber(sumNums(l, 1))
		return 1
	})
	setGoFunc(L, t, "avg", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		v := 0.0
		if len(arr) > 0 {
			v = sumNums(l, 1) / float64(len(arr))
		}
		l.PushNumber(cleanFloat(v))
		return 1
	})
	setGoFunc(L, t, "parse", func(l *lua.State) int {
		s := strings.TrimSpace(argString(l, 1))
		v, err := strconv.ParseFloat(s, 64)
		if err == nil {
			l.PushNumber(v)
			return 1
		}
		iv, ierr := strconv.ParseInt(s, 10, 64)
		if ierr == nil {
			l.PushNumber(float64(iv))
			return 1
		}
		l.PushNumber(math.NaN())
		return 1
	})
	setGoFunc(L, t, "fmt", func(l *lua.State) int {
		dec, loc := 2, ""
		if l.Top() >= 2 {
			_, isStr := l.ToValue(2).(string)
			if isStr {
				loc = argString(l, 2)
				if l.Top() >= 3 {
					dec = int(argNum(l, 3))
				}
			}
			if !isStr {
				dec = int(argNum(l, 2))
				if l.Top() >= 3 {
					loc = argString(l, 3)
				}
			}
		}
		l.PushString(formatNum(argNum(l, 1), dec, loc))
		return 1
	})
	setGoFunc(L, t, "count", func(l *lua.State) int {
		l.PushNumber(float64(len(statsNums(l, 1, statsField(l, 2)))))
		return 1
	})
	setGoFunc(L, t, "min", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 2))
		if len(v) == 0 {
			l.PushNumber(math.NaN())
			return 1
		}
		m := v[0]
		for _, x := range v[1:] {
			if x < m {
				m = x
			}
		}
		l.PushNumber(cleanFloat(m))
		return 1
	})
	setGoFunc(L, t, "max", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 2))
		if len(v) == 0 {
			l.PushNumber(math.NaN())
			return 1
		}
		m := v[0]
		for _, x := range v[1:] {
			if x > m {
				m = x
			}
		}
		l.PushNumber(cleanFloat(m))
		return 1
	})
	setGoFunc(L, t, "median", func(l *lua.State) int {
		l.PushNumber(cleanFloat(quantile(statsNums(l, 1, statsField(l, 2)), 0.5)))
		return 1
	})
	setGoFunc(L, t, "quantile", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 3))
		l.PushNumber(cleanFloat(quantile(v, argNum(l, 2))))
		return 1
	})
	setGoFunc(L, t, "percentile", func(l *lua.State) int {
		v := statsNums(l, 1, statsField(l, 3))
		l.PushNumber(cleanFloat(quantile(v, argNum(l, 2)/100)))
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

func sumNums(l *lua.State, index int) float64 {
	s := float64(0)
	for _, e := range luaArrayAny(l, index) {
		if n, ok := numOpt(e); ok {
			s += n
		}
	}
	return s
}

func cleanFloat(v float64) float64 {
	return math.Round(v*1e10) / 1e10
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

func formatNum(n float64, dec int, loc string) string {
	if dec < 0 {
		dec = 2
	}
	if dec > 20 {
		dec = 20
	}
	neg := n < 0
	if n == 0 {
		neg = math.Signbit(n)
	}
	s := strconv.FormatFloat(math.Abs(n), 'f', dec, 64)
	intPart, decPart := "", ""
	before, after, ok := strings.Cut(s, ".")
	if ok {
		intPart, decPart = before, after
	}
	if !ok {
		intPart = s
	}
	thousandSep, decimalSep := ",", "."
	if isPTLoc(locale(loc)) {
		thousandSep, decimalSep = ".", ","
	}
	grouped := groupThousands(intPart, thousandSep, 3)
	if neg {
		grouped = fmt.Sprintf("-%s", grouped)
	}
	if decPart != "" {
		return fmt.Sprintf("%s%s%s", grouped, decimalSep, decPart)
	}
	return grouped
}

func locale(loc string) string {
	return strings.ToLower(loc)
}

func isPTLoc(l string) bool {
	if l == "pt-br" {
		return true
	}
	if l == "pt_br" {
		return true
	}
	return l == "pt"
}

func groupThousands(s, sep string, width int) string {
	if len(s) <= width {
		return s
	}
	b := strings.Builder{}
	for i, c := range s {
		if i > 0 {
			if (len(s)-i)%width == 0 {
				b.WriteString(sep)
			}
		}
		b.WriteRune(c)
	}
	return b.String()
}
