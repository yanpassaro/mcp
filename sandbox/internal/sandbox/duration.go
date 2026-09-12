package sandbox

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	lua "github.com/Shopify/go-lua"
)

const (
	msPerSecond int64 = 1000
	msPerMinute       = 60 * msPerSecond
	msPerHour         = 60 * msPerMinute
	msPerDay          = 24 * msPerHour
	msPerWeek         = 7 * msPerDay
)

func buildDuration(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "parse", func(l *lua.State) int {
		if ms, ok := durationParse(argString(l, 1)); ok {
			l.PushNumber(float64(ms))
		} else {
			l.PushNil()
		}
		return 1
	})
	setGoFunc(L, t, "format", func(l *lua.State) int {
		l.PushString(durationFormat(int64(argNum(l, 1))))
		return 1
	})
	setGoFunc(L, t, "parts", func(l *lua.State) int {
		pushAny(l, durationParts(int64(argNum(l, 1))))
		return 1
	})
	setGoFunc(L, t, "total", func(l *lua.State) int {
		unit := strings.ToLower(strings.TrimSpace(argString(l, 2)))
		l.PushNumber(cleanFloat(durationTotal(int64(argNum(l, 1)), unit)))
		return 1
	})
	setGoFunc(L, t, "compare", func(l *lua.State) int {
		a, b := durationMS(l, 1), durationMS(l, 2)
		switch {
		case a < b:
			l.PushInteger(-1)
		case a > b:
			l.PushInteger(1)
		default:
			l.PushInteger(0)
		}
		return 1
	})
	setGoFunc(L, t, "add", func(l *lua.State) int {
		l.PushNumber(float64(int64(argNum(l, 1)) + durationMS(l, 2)))
		return 1
	})
	setGoFunc(L, t, "sub", func(l *lua.State) int {
		l.PushNumber(float64(int64(argNum(l, 1)) - durationMS(l, 2)))
		return 1
	})

	return t
}

func durationMS(l *lua.State, index int) int64 {
	if v, ok := l.ToNumber(index); ok {
		return int64(v)
	}
	if s, ok := l.ToString(index); ok {
		if ms, ok := durationParse(s); ok {
			return ms
		}
	}
	panic(fmt.Errorf("duração inválida: %s", argString(l, index)))
}

func durationParse(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	s = expandDaysWeeks(s)
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, false
	}
	return d.Milliseconds(), true
}

func expandDaysWeeks(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		j := i
		for j < len(s) && (s[j] == '.' || s[j] == '-' || s[j] == '+' || (s[j] >= '0' && s[j] <= '9')) {
			j++
		}
		if j >= len(s) {
			b.WriteString(s[i:])
			break
		}
		c := s[j]
		if (c == 'd' || c == 'D' || c == 'w' || c == 'W') && j > i {
			if f, err := strconv.ParseFloat(s[i:j], 64); err == nil {
				if c == 'd' || c == 'D' {
					b.WriteString(strconv.FormatFloat(f*24, 'f', -1, 64))
				} else {
					b.WriteString(strconv.FormatFloat(f*24*7, 'f', -1, 64))
				}
				b.WriteString("h")
				i = j + 1
				continue
			}
		}
		b.WriteString(s[i:j])
		b.WriteByte(c)
		i = j + 1
	}
	return b.String()
}

func durationFormat(ms int64) string {
	neg := ms < 0
	if neg {
		ms = -ms
	}
	w := ms / msPerWeek
	ms %= msPerWeek
	d := ms / msPerDay
	ms %= msPerDay
	h := ms / msPerHour
	ms %= msPerHour
	m := ms / msPerMinute
	ms %= msPerMinute
	s := ms / msPerSecond
	ms %= msPerSecond

	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	if w > 0 {
		fmt.Fprintf(&b, "%dw", w)
	}
	if d > 0 {
		fmt.Fprintf(&b, "%dd", d)
	}
	if h > 0 {
		fmt.Fprintf(&b, "%dh", h)
	}
	if m > 0 {
		fmt.Fprintf(&b, "%dm", m)
	}
	if s > 0 {
		fmt.Fprintf(&b, "%ds", s)
	}
	if ms > 0 {
		fmt.Fprintf(&b, "%dms", ms)
	}
	if b.Len() == 0 || (b.Len() == 1 && neg) {
		b.WriteString("0s")
	}
	return b.String()
}

func durationParts(ms int64) map[string]any {
	neg := ms < 0
	if neg {
		ms = -ms
	}
	w := ms / msPerWeek
	ms %= msPerWeek
	d := ms / msPerDay
	ms %= msPerDay
	h := ms / msPerHour
	ms %= msPerHour
	m := ms / msPerMinute
	ms %= msPerMinute
	s := ms / msPerSecond
	ms %= msPerSecond
	return map[string]any{
		"w": float64(w), "d": float64(d), "h": float64(h),
		"m": float64(m), "s": float64(s), "ms": float64(ms),
	}
}

func durationTotal(ms int64, unit string) float64 {
	switch unit {
	case "ms", "millisecond", "milliseconds":
		return float64(ms)
	case "second", "seconds", "s":
		return float64(ms) / float64(msPerSecond)
	case "minute", "minutes", "min", "m":
		return float64(ms) / float64(msPerMinute)
	case "hour", "hours", "h":
		return float64(ms) / float64(msPerHour)
	case "day", "days", "d":
		return float64(ms) / float64(msPerDay)
	case "week", "weeks", "w":
		return float64(ms) / float64(msPerWeek)
	}
	return math.NaN()
}
