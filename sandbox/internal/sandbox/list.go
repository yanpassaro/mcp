package sandbox

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/dop251/goja"
)

func buildList(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("chunk", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		return vm.ToValue(chunk(arr, int(toNum(call.Argument(1)))))
	})
	o.Set("groupBy", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		keyArg := call.Argument(1)
		fn, isFn := goja.AssertFunction(keyArg)
		prop := keyProp(keyArg, isFn)
		groups := map[string][]any{}
		for _, item := range arr {
			k := keyOf(vm, item, keyArg, fn, isFn, prop).String()
			groups[k] = append(groups[k], item)
		}
		res := make(map[string]any, len(groups))
		for k, v := range groups {
			res[k] = v
		}
		return vm.ToValue(res)
	})
	o.Set("unique", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		return vm.ToValue(uniqueItems(arr))
	})
	o.Set("flatten", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		return vm.ToValue(flatten(arr))
	})
	o.Set("sortBy", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		keyArg := call.Argument(1)
		fn, isFn := goja.AssertFunction(keyArg)
		prop := keyProp(keyArg, isFn)
		cp := append([]any(nil), arr...)
		sort.SliceStable(cp, func(i, j int) bool {
			return lessValue(keyOf(vm, cp[i], keyArg, fn, isFn, prop), keyOf(vm, cp[j], keyArg, fn, isFn, prop))
		})
		return vm.ToValue(cp)
	})
	o.Set("countBy", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		keyArg := call.Argument(1)
		fn, isFn := goja.AssertFunction(keyArg)
		prop := keyProp(keyArg, isFn)
		counts := map[string]int{}
		for _, item := range arr {
			counts[keyOf(vm, item, keyArg, fn, isFn, prop).String()]++
		}
		return vm.ToValue(counts)
	})
	o.Set("first", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		n := int(toNum(call.Argument(1)))
		if n <= 0 {
			n = 1
		}
		if n >= len(arr) {
			return vm.ToValue(arr)
		}
		return vm.ToValue(arr[:n])
	})
	o.Set("last", func(call goja.FunctionCall) goja.Value {
		arr, _ := call.Argument(0).Export().([]any)
		n := int(toNum(call.Argument(1)))
		if n <= 0 {
			n = 1
		}
		if n >= len(arr) {
			return vm.ToValue(arr)
		}
		return vm.ToValue(arr[len(arr)-n:])
	})
	return o
}

func keyProp(keyArg goja.Value, isFn bool) string {
	if isFn {
		return ""
	}
	return keyArg.String()
}

func keyOf(vm *goja.Runtime, item any, keyArg goja.Value, fn goja.Callable, isFn bool, prop string) goja.Value {
	if isFn {
		v, err := fn(goja.Undefined(), vm.ToValue(item))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return v
	}
	if prop == "" {
		return goja.Undefined()
	}
	return vm.ToValue(propValue(item, prop))
}

func propValue(item any, prop string) any {
	if m, ok := item.(map[string]any); ok {
		return m[prop]
	}
	return nil
}

func chunk(arr []any, n int) [][]any {
	if n <= 0 {
		return [][]any{}
	}
	var out [][]any
	for i := 0; i < len(arr); i += n {
		end := i + n
		if end > len(arr) {
			end = len(arr)
		}
		out = append(out, arr[i:end])
	}
	return out
}

func uniqueItems(arr []any) []any {
	seen := map[string]bool{}
	var out []any
	for _, v := range arr {
		k := identityKey(v)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}

func identityKey(v any) string {
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return fmt.Sprint(v)
}

func flatten(arr []any) []any {
	var out []any
	for _, v := range arr {
		if sub, ok := v.([]any); ok {
			out = append(out, flatten(sub)...)
		} else {
			out = append(out, v)
		}
	}
	return out
}

func lessValue(a, b goja.Value) bool {
	if isNil(a) {
		return !isNil(b)
	}
	if isNil(b) {
		return false
	}
	if an, ok := toFloat(a); ok {
		if bn, okB := toFloat(b); okB {
			return an < bn
		}
	}
	return a.String() < b.String()
}

func isNil(v goja.Value) bool {
	return v == nil || goja.IsUndefined(v) || goja.IsNull(v)
}

func toFloat(v goja.Value) (float64, bool) {
	switch n := v.Export().(type) {
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}
