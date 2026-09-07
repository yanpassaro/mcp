package sandbox

import (
	lua "github.com/Shopify/go-lua"
)

func buildTmp(L *lua.State, store *Store) int {
	t := buildIO(L, store)
	setGoFunc(L, t, "clear", func(l *lua.State) int {
		n, err := store.Clear()
		if err != nil {
			panic(err)
		}
		l.PushInteger(n)
		return 1
	})
	return t
}
