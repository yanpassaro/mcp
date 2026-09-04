package sandbox

import (
	"math"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

func buildNum(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("round", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(round(toNum(call.Argument(0)), int(toNum(call.Argument(1)))))
	})
	o.Set("clamp", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(clamp(toNum(call.Argument(0)), toNum(call.Argument(1)), toNum(call.Argument(2))))
	})
	o.Set("percent", func(call goja.FunctionCall) goja.Value {
		a, b := toNum(call.Argument(0)), toNum(call.Argument(1))
		if b == 0 {
			return vm.ToValue(0)
		}
		return vm.ToValue(a / b * 100)
	})
	o.Set("sum", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(sumNums(vm, call.Argument(0)))
	})
	o.Set("avg", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		if len(arr) == 0 {
			return vm.ToValue(0)
		}
		return vm.ToValue(sumNums(vm, call.Argument(0)) / float64(len(arr)))
	})
	o.Set("parse", func(call goja.FunctionCall) goja.Value {
		s := strings.TrimSpace(call.Argument(0).String())
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return vm.ToValue(v)
		}
		if iv, err := strconv.ParseInt(s, 10, 64); err == nil {
			return vm.ToValue(iv)
		}
		return vm.ToValue(math.NaN())
	})
	o.Set("fmt", func(call goja.FunctionCall) goja.Value {
		n := toNum(call.Argument(0))
		dec, loc := 2, ""
		if len(call.Arguments) > 1 && !isNilValue(call.Argument(1)) {
			if isStringValue(call.Argument(1)) {
				loc = call.Argument(1).String()
				if len(call.Arguments) > 2 {
					dec = int(toNum(call.Argument(2)))
				}
			} else {
				dec = int(toNum(call.Argument(1)))
				if len(call.Arguments) > 2 {
					loc = call.Argument(2).String()
				}
			}
		}
		return vm.ToValue(formatNum(n, dec, loc))
	})
	return o
}

func sumNums(vm *goja.Runtime, v goja.Value) float64 {
	arr, _ := v.Export().([]any)
	var s float64
	for _, e := range arr {
		s += toNum(vm.ToValue(e))
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
