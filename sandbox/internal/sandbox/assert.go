package sandbox

import (
	"fmt"

	"github.com/dop251/goja"
)

func buildAssert(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("ok", func(call goja.FunctionCall) goja.Value {
		if !call.Argument(0).ToBoolean() {
			panic(vm.NewGoError(fmt.Errorf("assert.ok falhou: %s", msgOr(call, 1, "esperado um valor truthy"))))
		}
		return goja.Undefined()
	})
	o.Set("equal", func(call goja.FunctionCall) goja.Value {
		a, b := call.Argument(0), call.Argument(1)
		if !a.StrictEquals(b) {
			panic(vm.NewGoError(fmt.Errorf("assert.equal falhou: %s != %s", a.String(), b.String())))
		}
		return goja.Undefined()
	})
	o.Set("throws", func(call goja.FunctionCall) goja.Value {
		fn, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			panic(vm.NewGoError(fmt.Errorf("assert.throws espera uma função")))
		}
		if _, err := fn(goja.Undefined()); err == nil {
			panic(vm.NewGoError(fmt.Errorf("assert.throws falhou: nada foi lançado")))
		}
		return goja.Undefined()
	})
	return o
}
