package sandbox

import (
	"fmt"
	"strconv"

	lua "github.com/Shopify/go-lua"
)

func argString(l *lua.State, i int) string {
	if s, ok := l.ToString(i); ok {
		return s
	}
	return luaValueString(l.ToValue(i))
}

func luaValueString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprint(x)
	case int:
		return fmt.Sprint(x)
	case int64:
		return fmt.Sprint(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return ""
}

func argNum(l *lua.State, i int) float64 {
	if n, ok := l.ToNumber(i); ok {
		return n
	}
	return 0
}

func argBool(l *lua.State, i int) bool {
	return l.ToBoolean(i)
}

func pushAny(l *lua.State, v any) {
	switch x := v.(type) {
	case nil:
		l.PushNil()
	case bool:
		l.PushBoolean(x)
	case string:
		l.PushString(x)
	case int:
		l.PushInteger(x)
	case int64:
		l.PushNumber(float64(x))
	case float64:
		l.PushNumber(x)
	case map[string]any:
		l.NewTable()
		t := l.Top()
		for k, e := range x {
			pushAny(l, e)
			l.SetField(t, k)
		}
	case map[string]string:
		l.NewTable()
		t := l.Top()
		for k, e := range x {
			l.PushString(e)
			l.SetField(t, k)
		}
	case []any:
		l.NewTable()
		t := l.Top()
		for i, e := range x {
			pushAny(l, e)
			l.RawSetInt(t, i+1)
		}
	case []string:
		l.NewTable()
		t := l.Top()
		for i, e := range x {
			l.PushString(e)
			l.RawSetInt(t, i+1)
		}
	case []int:
		l.NewTable()
		t := l.Top()
		for i, e := range x {
			l.PushInteger(e)
			l.RawSetInt(t, i+1)
		}
	case []int64:
		l.NewTable()
		t := l.Top()
		for i, e := range x {
			l.PushNumber(float64(e))
			l.RawSetInt(t, i+1)
		}
	case []float64:
		l.NewTable()
		t := l.Top()
		for i, e := range x {
			l.PushNumber(e)
			l.RawSetInt(t, i+1)
		}
	default:
		l.PushString(fmt.Sprint(x))
	}
}

func newTable(l *lua.State) int {
	l.NewTable()
	return l.Top()
}

func setGoFunc(l *lua.State, t int, name string, fn lua.Function) {
	l.PushGoFunction(fn)
	l.SetField(t, name)
}

func setFieldValue(l *lua.State, t int, name string) {
	l.SetField(t, name)
}

func toAnyMap(l *lua.State, index int) map[string]any {
	if m, ok := luaToAny(l, index).(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func luaToAny(l *lua.State, index int) any {
	index = l.AbsIndex(index)
	switch {
	case l.IsNil(index):
		return nil
	case l.IsBoolean(index):
		return l.ToBoolean(index)
	case l.IsNumber(index):
		if n, ok := l.ToNumber(index); ok {
			return n
		}
		return nil
	case l.IsString(index):
		if s, ok := l.ToString(index); ok {
			return s
		}
		return nil
	case l.IsTable(index):
		return tableToAny(l, index)
	default:
		return nil
	}
}

func tableToAny(l *lua.State, index int) any {
	index = l.AbsIndex(index)
	isArray := true
	maxIdx := 0
	count := 0
	items := map[string]any{}

	l.PushNil()
	for l.Next(index) {
		keyIdx := l.AbsIndex(l.Top() - 1)
		valIdx := l.AbsIndex(l.Top())

		keyNum := -1
		if l.IsNumber(keyIdx) {
			if n, ok := l.ToInteger(keyIdx); ok {
				keyNum = n
			}
		}
		val := luaToAny(l, valIdx)
		items[keyStr(l, keyIdx, keyNum)] = val
		l.Pop(1)

		if keyNum >= 1 {
			if keyNum > maxIdx {
				maxIdx = keyNum
			}
		} else {
			isArray = false
		}
		count++
	}

	if count == 0 {
		return map[string]any{}
	}
	if isArray && maxIdx == count {
		arr := make([]any, maxIdx)
		for i := 1; i <= maxIdx; i++ {
			arr[i-1] = items[strconv.Itoa(i)]
		}
		return arr
	}
	return items
}

func keyStr(l *lua.State, idx, keyNum int) string {
	if keyNum >= 1 {
		return strconv.Itoa(keyNum)
	}
	if s, ok := l.ToString(idx); ok {
		return s
	}
	return fmt.Sprint(l.ToValue(idx))
}


