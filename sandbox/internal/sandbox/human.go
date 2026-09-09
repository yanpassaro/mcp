package sandbox

import (
	"strconv"
	"strings"

	lua "github.com/Shopify/go-lua"
)

var humanUnits = []string{"KB", "MB", "GB", "TB", "PB"}

func buildHuman(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "bytes", func(l *lua.State) int {
		l.PushString(humanBytes(argNum(l, 1)))
		return 1
	})
	setGoFunc(L, t, "duration", func(l *lua.State) int {
		l.PushString(humanDuration(argNum(l, 1)))
		return 1
	})
	setGoFunc(L, t, "compact", func(l *lua.State) int {
		l.PushString(humanCompact(argNum(l, 1)))
		return 1
	})
	setGoFunc(L, t, "money", func(l *lua.State) int {
		cur := ""
		if l.Top() >= 2 {
			cur = argString(l, 2)
		}
		l.PushString(humanMoney(argNum(l, 1), cur))
		return 1
	})
	setGoFunc(L, t, "ordinal", func(l *lua.State) int {
		l.PushString(humanOrdinal(int64(argNum(l, 1))))
		return 1
	})
	setGoFunc(L, t, "plural", func(l *lua.State) int {
		l.PushString(humanPlural(argNum(l, 1), argString(l, 2), argString(l, 3)))
		return 1
	})
	setGoFunc(L, t, "list", func(l *lua.State) int {
		l.PushString(humanList(luaArrayAny(l, 1)))
		return 1
	})
	return t
}

func humanBytes(n float64) string {
	if n < 0 {
		n = 0
	}
	if n < 1024 {
		return strconv.FormatInt(int64(n), 10) + " B"
	}
	val := n
	i := -1
	for val >= 1024 && i < len(humanUnits)-1 {
		val /= 1024
		i++
	}
	return trimZeroDec(formatNum(val, 1, "pt-br")) + " " + humanUnits[i]
}

func humanDuration(ms float64) string {
	if ms < 0 {
		ms = 0
	}
	if ms < 1000 {
		return strconv.FormatInt(int64(ms), 10) + " ms"
	}
	if ms < 60000 {
		return trimZeroDec(formatNum(ms/1000, 1, "pt-br")) + " s"
	}
	totalMin := int64(ms / 60000)
	if totalMin < 60 {
		return strconv.FormatInt(totalMin, 10) + " min"
	}
	h := totalMin / 60
	m := totalMin % 60
	if h >= 24 {
		d := h / 24
		h = h % 24
		if h == 0 && m == 0 {
			return strconv.FormatInt(d, 10) + " d"
		}
		if m == 0 {
			return strconv.FormatInt(d, 10) + " d " + strconv.FormatInt(h, 10) + " h"
		}
		return strconv.FormatInt(d, 10) + " d " + strconv.FormatInt(h, 10) + " h " + strconv.FormatInt(m, 10) + " min"
	}
	if m == 0 {
		return strconv.FormatInt(h, 10) + " h"
	}
	return strconv.FormatInt(h, 10) + " h " + strconv.FormatInt(m, 10) + " min"
}

func humanCompact(n float64) string {
	if n < 0 {
		n = 0
	}
	if n < 1000 {
		return trimZeroDec(formatNum(n, 0, "pt-br"))
	}
	units := []string{"mil", "mi", "bi", "tri"}
	val := n
	i := -1
	for val >= 1000 && i < len(units)-1 {
		val /= 1000
		i++
	}
	return trimZeroDec(formatNum(val, 1, "pt-br")) + " " + units[i]
}

func humanMoney(n float64, cur string) string {
	if cur == "" {
		cur = "R$"
	}
	return cur + " " + formatNum(n, 2, "pt-br")
}

func humanOrdinal(n int64) string {
	return strconv.FormatInt(n, 10) + "º"
}

func humanPlural(n float64, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func humanList(items []any) string {
	n := len(items)
	if n == 0 {
		return ""
	}
	parts := make([]string, n)
	for i, it := range items {
		parts[i] = luaValueString(it)
	}
	if n == 1 {
		return parts[0]
	}
	if n == 2 {
		return parts[0] + " e " + parts[1]
	}
	return strings.Join(parts[:n-1], ", ") + " e " + parts[n-1]
}

func trimZeroDec(s string) string {
	if strings.HasSuffix(s, ",0") {
		return s[:len(s)-2]
	}
	return s
}
