package sandbox

import (
	"fmt"
	"reflect"

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
	return t
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
