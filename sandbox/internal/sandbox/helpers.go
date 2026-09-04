package sandbox

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/dop251/goja"
)

func toStringMap(v goja.Value) map[string]any {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return map[string]any{}
	}
	if m, ok := v.Export().(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func optString(call goja.FunctionCall, idx int) string {
	if idx < len(call.Arguments) {
		if v := call.Arguments[idx]; !isNilValue(v) {
			return v.String()
		}
	}
	return ""
}

func msgOr(call goja.FunctionCall, idx int, def string) string {
	if idx < len(call.Arguments) {
		if s := call.Arguments[idx].String(); s != "" {
			return s
		}
	}
	return def
}

func isNilValue(v goja.Value) bool {
	return v == nil || goja.IsUndefined(v) || goja.IsNull(v)
}

func isStringValue(v goja.Value) bool {
	if isNilValue(v) {
		return false
	}
	_, ok := v.Export().(string)
	return ok
}

func numOpt(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

func toNum(v goja.Value) float64 {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return 0
	}
	switch n := v.Export().(type) {
	case int64:
		return float64(n)
	case float64:
		return n
	case int:
		return float64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	}
	return 0
}

func loadArgs(vm *goja.Runtime, argStr string) goja.Value {
	s := strings.TrimSpace(argStr)
	if s == "" {
		return goja.Undefined()
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return vm.ToValue(v)
	}
	return vm.ToValue(argStr)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func renderData(v goja.Value) (string, bool) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return "", false
	}
	exp := v.Export()
	switch exp := exp.(type) {
	case string:
		return exp, false
	case map[string]any, []any:
		return renderMarkdown(exp), true
	default:
		return fmt.Sprint(exp), false
	}
}

func renderMarkdown(val any) string {
	return renderMD(val, "")
}

func renderMD(val any, indent string) string {
	switch x := val.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return x
	case map[string]any:
		if len(x) == 0 {
			return "{}"
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for i, k := range keys {
			if i > 0 {
				b.WriteString("\n")
				b.WriteString(indent)
			}
			fmt.Fprintf(&b, "%s**%s:** %s", indent, k, renderMD(x[k], indent))
		}
		return b.String()
	case []any:
		if len(x) == 0 {
			return "[]"
		}
		if allScalars(x) {
			parts := make([]string, len(x))
			for i, e := range x {
				parts[i] = renderScalarMD(e)
			}
			return strings.Join(parts, ", ")
		}
		var b strings.Builder
		for i, e := range x {
			if i > 0 {
				b.WriteString("\n")
				b.WriteString(indent)
			}
			fmt.Fprintf(&b, "%s- %s", indent, renderMD(e, indent+"  "))
		}
		return b.String()
	default:
		return fmt.Sprint(x)
	}
}

func allScalars(arr []any) bool {
	for _, e := range arr {
		switch e.(type) {
		case map[string]any, []any:
			return false
		}
	}
	return true
}

func renderScalarMD(v any) string {
	return renderMD(v, "")
}
