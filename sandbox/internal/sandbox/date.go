package sandbox

import (
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

func buildDate(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("now", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(time.Now().UnixMilli())
	})
	o.Set("iso", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(toTime(call.Argument(0), time.Now()).Format(time.RFC3339))
	})
	o.Set("format", func(call goja.FunctionCall) goja.Value {
		layout := call.Argument(0).String()
		ref := time.Now()
		if len(call.Arguments) > 1 {
			ref = toTime(call.Argument(1), ref)
		}
		return vm.ToValue(formatDate(ref, layout))
	})
	o.Set("parse", func(call goja.FunctionCall) goja.Value {
		t, err := parseTimeStr(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(t.UnixMilli())
	})
	o.Set("add", func(call goja.FunctionCall) goja.Value {
		ref := toTime(call.Argument(0), time.Now())
		amount := int64(toNum(call.Argument(1)))
		unit := call.Argument(2).String()
		return vm.ToValue(addDate(ref, amount, unit).UnixMilli())
	})
	o.Set("unix", func(call goja.FunctionCall) goja.Value {
		ref := toTime(call.Argument(0), time.Now())
		return vm.ToValue(ref.Unix())
	})
	o.Set("diff", func(call goja.FunctionCall) goja.Value {
		a := toTime(call.Argument(0), time.Now())
		b := toTime(call.Argument(1), time.Now())
		return vm.ToValue(diffIn(a, b, call.Argument(2).String()))
	})
	return o
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

func toTime(v goja.Value, def time.Time) time.Time {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return def
	}
	switch t := v.Export().(type) {
	case time.Time:
		return t
	case int64:
		return time.UnixMilli(t)
	case float64:
		return time.UnixMilli(int64(t))
	case string:
		if p, err := parseTimeStr(t); err == nil {
			return p
		}
	}
	return def
}
