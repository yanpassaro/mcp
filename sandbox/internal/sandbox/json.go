package sandbox

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

func buildJson(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("parse", func(call goja.FunctionCall) goja.Value {
		var v any
		if err := json.Unmarshal([]byte(call.Argument(0).String()), &v); err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(v)
	})
	o.Set("stringify", func(call goja.FunctionCall) goja.Value {
		exp := call.Argument(0).Export()
		if indent := int(toNum(call.Argument(1))); indent > 0 {
			b, err := json.MarshalIndent(exp, "", strings.Repeat(" ", indent))
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(string(b))
		}
		b, err := json.Marshal(exp)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(string(b))
	})
	o.Set("format", func(call goja.FunctionCall) goja.Value {
		b, err := json.MarshalIndent(call.Argument(0).Export(), "", "  ")
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(string(b))
	})
	o.Set("minify", func(call goja.FunctionCall) goja.Value {
		var v any
		if err := json.Unmarshal([]byte(call.Argument(0).String()), &v); err != nil {
			panic(vm.NewGoError(err))
		}
		b, err := json.Marshal(v)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(string(b))
	})
	o.Set("path", func(call goja.FunctionCall) goja.Value {
		v, ok := jsonPath(call.Argument(0).Export(), call.Argument(1).String())
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(v)
	})
	return o
}

func jsonPath(v any, path string) (any, bool) {
	if strings.TrimSpace(path) == "" {
		return v, true
	}
	for _, part := range strings.Split(path, ".") {
		part = strings.TrimSpace(part)
		switch cur := v.(type) {
		case map[string]any:
			var ok bool
			v, ok = cur[part]
			if !ok {
				return nil, false
			}
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(cur) {
				return nil, false
			}
			v = cur[idx]
		default:
			return nil, false
		}
	}
	return v, true
}
