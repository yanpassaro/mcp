package sandbox

import (
	"math/rand/v2"
	"time"

	"github.com/dop251/goja"
)

func buildRandom(vm *goja.Runtime) *goja.Object {
	var rng *rand.Rand
	newRNG := func(seed int64) {
		rng = rand.New(rand.NewPCG(uint64(seed), uint64(seed)))
	}
	newRNG(time.Now().UnixNano())
	if m := vm.Get("Math"); m != nil {
		if mo, ok := m.(*goja.Object); ok && !goja.IsUndefined(mo) {
			mo.Set("random", func(call goja.FunctionCall) goja.Value {
				return vm.ToValue(rng.Float64())
			})
		}
	}
	rndObj := vm.NewObject()
	rndObj.Set("seed", func(call goja.FunctionCall) goja.Value {
		newRNG(int64(toNum(call.Argument(0))))
		return goja.Undefined()
	})
	rndObj.Set("int", func(call goja.FunctionCall) goja.Value {
		min, max := int(toNum(call.Argument(0))), int(toNum(call.Argument(1)))
		if max < min {
			min, max = max, min
		}
		if min == max {
			return vm.ToValue(min)
		}
		return vm.ToValue(min + rng.IntN(max-min+1))
	})
	rndObj.Set("pick", func(call goja.FunctionCall) goja.Value {
		arr, ok := call.Argument(0).Export().([]any)
		if !ok || len(arr) == 0 {
			return goja.Undefined()
		}
		return vm.ToValue(arr[rng.IntN(len(arr))])
	})
	rndObj.Set("shuffle", func(call goja.FunctionCall) goja.Value {
		arr, ok := call.Argument(0).Export().([]any)
		if !ok {
			return vm.ToValue([]any{})
		}
		cp := append([]any(nil), arr...)
		rng.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
		return vm.ToValue(cp)
	})
	return rndObj
}
