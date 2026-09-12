package sandbox

import (
	"fmt"
	"strings"

	lua "github.com/Shopify/go-lua"
)

var unitFactors = map[string]map[string]float64{
	"length": {"m": 1, "km": 1000, "cm": 0.01, "mm": 0.001, "um": 1e-6, "nm": 1e-9, "mi": 1609.344, "yd": 0.9144, "ft": 0.3048, "in": 0.0254},
	"mass":   {"kg": 1, "g": 0.001, "mg": 1e-6, "ug": 1e-9, "t": 1000, "lb": 0.45359237, "oz": 0.028349523125},
	"data":   {"byte": 1, "b": 1, "kb": 1024, "mb": 1048576, "gb": 1073741824, "tb": 1099511627776, "bit": 0.125},
	"time":   {"s": 1, "ms": 0.001, "us": 1e-6, "ns": 1e-9, "min": 60, "h": 3600, "d": 86400, "w": 604800},
	"volume": {"l": 1, "ml": 0.001, "cl": 0.01, "m3": 1000, "cm3": 0.001, "gal": 3.785411784, "qt": 0.946352946, "floz": 0.0295735295625},
}

var unitAlias = map[string]string{
	"m": "m", "meter": "m", "meters": "m", "metro": "m", "metros": "m", "metre": "m", "metres": "m",
	"km": "km", "kilometer": "km", "kilometers": "km", "quilometro": "km", "quilometros": "km",
	"cm": "cm", "centimeter": "cm", "centimeters": "cm", "mm": "mm", "millimeter": "mm", "um": "um",
	"mi": "mi", "mile": "mi", "miles": "mi", "yd": "yd", "yard": "yd", "yards": "yd",
	"ft": "ft", "foot": "ft", "feet": "ft", "in": "in", "inch": "in", "inches": "in",
	"kg": "kg", "kilogram": "kg", "kilograms": "kg", "quilo": "kg", "quilos": "kg",
	"g": "g", "gram": "g", "grams": "g", "grama": "g", "gramas": "g", "mg": "mg", "ug": "ug",
	"t": "t", "ton": "t", "tonne": "t", "tonelada": "t", "lb": "lb", "pound": "lb", "pounds": "lb", "libra": "lb",
	"oz": "oz", "ounce": "oz", "ounces": "oz",
	"byte": "byte", "bytes": "byte", "b": "byte", "kb": "kb", "kilobyte": "kb", "kib": "kb",
	"mb": "mb", "megabyte": "mb", "mib": "mb", "gb": "gb", "gigabyte": "gb", "gib": "gb",
	"tb": "tb", "terabyte": "tb", "tib": "tb", "bit": "bit", "bits": "bit",
	"s": "s", "sec": "s", "second": "s", "seconds": "s", "seg": "s", "segundo": "s", "segundos": "s",
	"ms": "ms", "millisecond": "ms", "milliseconds": "ms", "us": "us", "microsecond": "us", "ns": "ns", "nanosecond": "ns",
	"min": "min", "minute": "min", "minutes": "min", "minuto": "min", "minutos": "min",
	"h": "h", "hour": "h", "hours": "h", "hora": "h", "horas": "h", "hr": "h",
	"d": "d", "day": "d", "days": "d", "dia": "d", "dias": "d", "w": "w", "week": "w", "weeks": "w", "semana": "w", "semanas": "w",
	"l": "l", "liter": "l", "liters": "l", "litre": "l", "litros": "l", "litro": "l",
	"ml": "ml", "milliliter": "ml", "cl": "cl", "m3": "m3", "cm3": "cm3", "gal": "gal", "gallon": "gal", "qt": "qt", "floz": "floz",
	"c": "c", "celsius": "c", "°c": "c",
	"f": "f", "fahrenheit": "f", "°f": "f",
	"k": "k", "kelvin": "k",
}

func buildUnit(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "convert", func(l *lua.State) int {
		v := argNum(l, 1)
		res, err := unitConvert(v, argString(l, 2), argString(l, 3))
		if err != nil {
			panic(err)
		}
		l.PushNumber(cleanFloat(res))
		return 1
	})
	setGoFunc(L, t, "factor", func(l *lua.State) int {
		u := unitAlias[strings.ToLower(strings.TrimSpace(argString(l, 1)))]
		for _, m := range unitFactors {
			if f, ok := m[u]; ok {
				l.PushNumber(cleanFloat(f))
				return 1
			}
		}
		panic(fmt.Errorf("unidade não linear: %s", argString(l, 1)))
	})
	setGoFunc(L, t, "list", func(l *lua.State) int {
		out := map[string]any{}
		for cat, m := range unitFactors {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sortStrings(keys)
			arr := make([]any, len(keys))
			for i, k := range keys {
				arr[i] = k
			}
			out[cat] = arr
		}
		pushAny(l, out)
		return 1
	})

	return t
}

func unitConvert(v float64, from, to string) (float64, error) {
	from = unitAlias[strings.ToLower(strings.TrimSpace(from))]
	to = unitAlias[strings.ToLower(strings.TrimSpace(to))]
	if from == "" || to == "" {
		return 0, fmt.Errorf("unidade desconhecida")
	}
	if from == to {
		return v, nil
	}
	fcat := unitCategoryOf(from)
	tcat := unitCategoryOf(to)
	if fcat == "" || fcat != tcat {
		return 0, fmt.Errorf("categorias diferentes (%s ≠ %s)", fcat, tcat)
	}
	if fcat == "temp" {
		var k float64
		switch from {
		case "c":
			k = v + 273.15
		case "f":
			k = (v - 32) * 5 / 9
			k += 273.15
		case "k":
			k = v
		}
		switch to {
		case "c":
			return k - 273.15, nil
		case "f":
			return (k-273.15)*9/5 + 32, nil
		case "k":
			return k, nil
		}
	}
	return v * unitFactors[fcat][from] / unitFactors[fcat][to], nil
}

func unitCategoryOf(unit string) string {
	for cat, m := range unitFactors {
		if _, ok := m[unit]; ok {
			return cat
		}
	}
	if unit == "c" || unit == "f" || unit == "k" {
		return "temp"
	}
	return ""
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
