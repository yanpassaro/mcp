package sandbox

import (
	"math/rand/v2"
	"time"

	lua "github.com/Shopify/go-lua"
)

func buildRandom(L *lua.State) int {
	var rng *rand.Rand
	newRNG := func(seed int64) {
		rng = rand.New(rand.NewPCG(uint64(seed), uint64(seed)))
	}
	newRNG(time.Now().UnixNano())
	t := newTable(L)
	setGoFunc(L, t, "seed", func(l *lua.State) int {
		newRNG(int64(argNum(l, 1)))
		return 0
	})
	setGoFunc(L, t, "int", func(l *lua.State) int {
		min, max := int(argNum(l, 1)), int(argNum(l, 2))
		if max < min {
			min, max = max, min
		}
		if min == max {
			l.PushInteger(min)
		} else {
			l.PushInteger(min + rng.IntN(max-min+1))
		}
		return 1
	})
	setGoFunc(L, t, "pick", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		if len(arr) == 0 {
			l.PushNil()
		} else {
			pushAny(l, arr[rng.IntN(len(arr))])
		}
		return 1
	})
	setGoFunc(L, t, "shuffle", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		cp := append([]any(nil), arr...)
		rng.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
		pushAny(l, cp)
		return 1
	})
	return t
}
