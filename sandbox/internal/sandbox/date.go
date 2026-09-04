package sandbox

import (
	"fmt"
	"strings"
	"time"

	lua "github.com/Shopify/go-lua"
)

func buildDate(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "now", func(l *lua.State) int {
		l.PushNumber(float64(time.Now().UnixMilli()))
		return 1
	})
	setGoFunc(L, t, "iso", func(l *lua.State) int {
		l.PushString(toTime(l, 1, time.Now()).Format(time.RFC3339))
		return 1
	})
	setGoFunc(L, t, "format", func(l *lua.State) int {
		layout := argString(l, 1)
		ref := time.Now()
		if l.Top() >= 2 {
			ref = toTime(l, 2, ref)
		}
		l.PushString(formatDate(ref, layout))
		return 1
	})
	setGoFunc(L, t, "parse", func(l *lua.State) int {
		ts, err := parseTimeStr(argString(l, 1))
		if err != nil {
			panic(err)
		}
		l.PushNumber(float64(ts.UnixMilli()))
		return 1
	})
	setGoFunc(L, t, "add", func(l *lua.State) int {
		ref := toTime(l, 1, time.Now())
		amount := int64(argNum(l, 2))
		l.PushNumber(float64(addDate(ref, amount, argString(l, 3)).UnixMilli()))
		return 1
	})
	setGoFunc(L, t, "unix", func(l *lua.State) int {
		l.PushNumber(float64(toTime(l, 1, time.Now()).Unix()))
		return 1
	})
	setGoFunc(L, t, "diff", func(l *lua.State) int {
		a := toTime(l, 1, time.Now())
		b := toTime(l, 2, time.Now())
		l.PushNumber(diffIn(a, b, argString(l, 3)))
		return 1
	})
	return t
}

func formatDate(t time.Time, layout string) string {
	l := layout
	l = strings.ReplaceAll(l, "YYYY", "2006")
	l = strings.ReplaceAll(l, "MM", "01")
	l = strings.ReplaceAll(l, "DD", "02")
	l = strings.ReplaceAll(l, "HH", "15")
	l = strings.ReplaceAll(l, "mm", "04")
	l = strings.ReplaceAll(l, "ss", "05")
	return t.Format(l)
}

func diffIn(a, b time.Time, unit string) float64 {
	d := a.Sub(b)
	switch strings.ToLower(unit) {
	case "week", "weeks":
		return d.Hours() / (24 * 7)
	case "day", "days":
		return d.Hours() / 24
	case "hour", "hours":
		return d.Hours()
	case "minute", "minutes":
		return d.Minutes()
	case "month", "months":
		return d.Hours() / (24 * 30.44)
	case "year", "years":
		return d.Hours() / (24 * 365.25)
	default:
		return d.Seconds()
	}
}

func addDate(t time.Time, amount int64, unit string) time.Time {
	switch strings.ToLower(unit) {
	case "day", "days":
		return t.AddDate(0, 0, int(amount))
	case "month", "months":
		return t.AddDate(0, int(amount), 0)
	case "year", "years":
		return t.AddDate(int(amount), 0, 0)
	case "hour", "hours":
		return t.Add(time.Duration(amount) * time.Hour)
	case "minute", "minutes":
		return t.Add(time.Duration(amount) * time.Minute)
	case "second", "seconds":
		return t.Add(time.Duration(amount) * time.Second)
	default:
		return t
	}
}

func parseTimeStr(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("data inválida: %s", s)
}

func toTime(l *lua.State, index int, def time.Time) time.Time {
	if l.IsNil(index) {
		return def
	}
	switch v := l.ToValue(index).(type) {
	case float64:
		return time.UnixMilli(int64(v))
	case string:
		if p, err := parseTimeStr(v); err == nil {
			return p
		}
	}
	return def
}
