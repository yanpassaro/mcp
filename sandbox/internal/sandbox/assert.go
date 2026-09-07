package sandbox

import (
	"fmt"
	"reflect"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func buildAssert(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "ok", func(l *lua.State) int {
		v := l.ToValue(1)
		if v == nil || v == false {
			panic(fmt.Errorf("assert.ok falhou: %s", argStr(l, 2, "esperado um valor truthy")))
		}
		return 0
	})
	setGoFunc(L, t, "equal", func(l *lua.State) int {
		a, b := l.ToValue(1), l.ToValue(2)
		if !reflect.DeepEqual(a, b) {
			panic(fmt.Errorf("assert.equal falhou: %v != %v", a, b))
		}
		return 0
	})
	setGoFunc(L, t, "throws", func(l *lua.State) int {
		f := l.ToGoFunction(1)
		if f == nil {
			panic(fmt.Errorf("assert.throws espera uma função"))
		}
		l.PushGoFunction(f)
		if err := callRecover(l, 0); err == nil {
			panic(fmt.Errorf("assert.throws falhou: nada foi lançado"))
		}
		return 0
	})
	setGoFunc(L, t, "type", func(l *lua.State) int {
		if kind := luaKind(l, 1); kind != argString(l, 2) {
			panic(fmt.Errorf("assert.type falhou: esperado %s, obteve %s", argString(l, 2), kind))
		}
		return 0
	})
	setGoFunc(L, t, "notNil", func(l *lua.State) int {
		if l.IsNil(1) {
			panic(fmt.Errorf("assert.notNil falhou: %s", argStr(l, 2, "valor nil")))
		}
		return 0
	})
	setGoFunc(L, t, "number", func(l *lua.State) int {
		if !l.IsNumber(1) {
			panic(fmt.Errorf("assert.number falhou: %s", argStr(l, 2, "esperado number")))
		}
		return 0
	})
	setGoFunc(L, t, "string", func(l *lua.State) int {
		if !l.IsString(1) {
			panic(fmt.Errorf("assert.string falhou: %s", argStr(l, 2, "esperado string")))
		}
		return 0
	})
	setGoFunc(L, t, "boolean", func(l *lua.State) int {
		if !l.IsBoolean(1) {
			panic(fmt.Errorf("assert.boolean falhou: %s", argStr(l, 2, "esperado boolean")))
		}
		return 0
	})
	setGoFunc(L, t, "table", func(l *lua.State) int {
		if !l.IsTable(1) {
			panic(fmt.Errorf("assert.table falhou: %s", argStr(l, 2, "esperado table")))
		}
		return 0
	})
	setGoFunc(L, t, "contains", func(l *lua.State) int {
		h := argString(l, 1)
		sub := argString(l, 2)
		if !strings.Contains(h, sub) {
			panic(fmt.Errorf("assert.contains falhou: %q não contém %q", h, sub))
		}
		return 0
	})
	setGoFunc(L, t, "matches", func(l *lua.State) int {
		re := compileRegex(argString(l, 2))
		if !re.MatchString(argString(l, 1)) {
			panic(fmt.Errorf("assert.matches falhou: %q não corresponde a %s", argString(l, 1), argString(l, 2)))
		}
		return 0
	})
	setGoFunc(L, t, "between", func(l *lua.State) int {
		v, lo, hi := argNum(l, 1), argNum(l, 2), argNum(l, 3)
		if v < lo || v > hi {
			panic(fmt.Errorf("assert.between falhou: %v fora de [%v, %v]", v, lo, hi))
		}
		return 0
	})
	setGoFunc(L, t, "length", func(l *lua.State) int {
		n := argNum(l, 2)
		switch x := luaToAny(l, 1).(type) {
		case string:
			if int64(len(x)) != int64(n) {
				panic(fmt.Errorf("assert.length falhou: esperado %d, obteve %d", int(n), len(x)))
			}
		case []any:
			if int64(len(x)) != int64(n) {
				panic(fmt.Errorf("assert.length falhou: esperado %d, obteve %d", int(n), len(x)))
			}
		case map[string]any:
			if int64(len(x)) != int64(n) {
				panic(fmt.Errorf("assert.length falhou: esperado %d, obteve %d", int(n), len(x)))
			}
		default:
			panic(fmt.Errorf("assert.length: valor sem comprimento"))
		}
		return 0
	})
	return t
}

func luaKind(l *lua.State, i int) string {
	switch {
	case l.IsNil(i):
		return "nil"
	case l.IsBoolean(i):
		return "boolean"
	case l.IsNumber(i):
		return "number"
	case l.IsString(i):
		return "string"
	case l.IsTable(i):
		return "table"
	case l.ToGoFunction(i) != nil:
		return "function"
	default:
		return "unknown"
	}
}

func callRecover(l *lua.State, nResults int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	l.Call(0, nResults)
	return nil
}

func argStr(l *lua.State, i int, def string) string {
	if l.Top() >= i {
		if s := argString(l, i); s != "" {
			return s
		}
	}
	return def
}
