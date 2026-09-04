package sandbox

import (
	"math"
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
		if b == 0 {
			l.PushNumber(0)
		} else {
			l.PushNumber(a / b * 100)
		}
		return 1
	})
	setGoFunc(L, t, "sum", func(l *lua.State) int {
		l.PushNumber(sumNums(l, 1))
		return 1
	})
	setGoFunc(L, t, "avg", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		if len(arr) == 0 {
			l.PushNumber(0)
		} else {
			l.PushNumber(sumNums(l, 1) / float64(len(arr)))
		}
		return 1
	})
	setGoFunc(L, t, "parse", func(l *lua.State) int {
		s := strings.TrimSpace(argString(l, 1))
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			l.PushNumber(v)
		} else if iv, err := strconv.ParseInt(s, 10, 64); err == nil {
			l.PushNumber(float64(iv))
		} else {
			l.PushNumber(math.NaN())
		}
		return 1
	})
	setGoFunc(L, t, "fmt", func(l *lua.State) int {
		dec, loc := 2, ""
		if l.Top() >= 2 {
			if _, isStr := l.ToValue(2).(string); isStr {
				loc = argString(l, 2)
				if l.Top() >= 3 {
					dec = int(argNum(l, 3))
				}
			} else {
				dec = int(argNum(l, 2))
				if l.Top() >= 3 {
					loc = argString(l, 3)
				}
			}
		}
		l.PushString(formatNum(argNum(l, 1), dec, loc))
		return 1
	})
	return t
}

func sumNums(l *lua.State, index int) float64 {
	var s float64
	for _, e := range luaArrayAny(l, index) {
		if n, ok := numOpt(e); ok {
			s += n
		}
	}
	return s
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
	neg := n < 0 || (n == 0 && math.Signbit(n))
	s := strconv.FormatFloat(math.Abs(n), 'f', dec, 64)
	var intPart, decPart string
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, decPart = s[:i], s[i+1:]
	} else {
		intPart = s
	}
	thousandSep, decimalSep := ",", "."
	l := strings.ToLower(loc)
	if l == "pt-br" || l == "pt_br" || l == "pt" {
		thousandSep, decimalSep = ".", ","
	}
	grouped := groupThousands(intPart, thousandSep, 3)
	if neg {
		grouped = "-" + grouped
	}
	if decPart != "" {
		return grouped + decimalSep + decPart
	}
	return grouped
}

func groupThousands(s, sep string, width int) string {
	if len(s) <= width {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%width == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(c)
	}
	return b.String()
}
