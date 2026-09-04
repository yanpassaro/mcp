package sandbox

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dop251/goja"
)

func buildFS(vm *goja.Runtime, fs *Store) *goja.Object {
	o := vm.NewObject()
	o.Set("read", func(call goja.FunctionCall) goja.Value {
		content, err := fs.Read(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(content)
	})
	o.Set("lines", func(call goja.FunctionCall) goja.Value {
		lines, err := fs.ReadLines(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(lines)
	})
	o.Set("json", func(call goja.FunctionCall) goja.Value {
		content, err := fs.Read(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		var v any
		if err := json.Unmarshal([]byte(content), &v); err != nil {
			panic(vm.NewGoError(fmt.Errorf("JSON inválido: %w", err)))
		}
		return vm.ToValue(v)
	})
	o.Set("write", func(call goja.FunctionCall) goja.Value {
		n, err := fs.Write(call.Argument(0).String(), call.Argument(1).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(n)
	})
	o.Set("append", func(call goja.FunctionCall) goja.Value {
		n, err := fs.Append(call.Argument(0).String(), call.Argument(1).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(n)
	})
	o.Set("del", func(call goja.FunctionCall) goja.Value {
		if err := fs.Delete(call.Argument(0).String()); err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(true)
	})
	o.Set("exists", func(call goja.FunctionCall) goja.Value {
		st, err := fs.Stat(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(st.Exists)
	})
	o.Set("stat", func(call goja.FunctionCall) goja.Value {
		st, err := fs.Stat(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(map[string]any{
			"name": st.Name, "exists": st.Exists, "isDir": st.IsDir,
			"size": st.Size, "lines": st.Lines,
		})
	})
	o.Set("dir", func(call goja.FunctionCall) goja.Value {
		entries, err := fs.ListDir(strings.TrimSpace(call.Argument(0).String()))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		return vm.ToValue(names)
	})
	return o
}
