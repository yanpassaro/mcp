package sandbox

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
)

const (
	maxTimeout     = 30 * time.Second
	maxOutputBytes = 256 * 1024
)

type RunRequest struct {
	Code       string
	Args       string
	Timeout    time.Duration
	FullStdlib bool
}

type RunResult struct {
	Name        string
	Description string
	Data        string
	DataJSON    bool
	Output      string
	Ok          bool
	Error       string
	Duration    time.Duration
	Truncated   bool
}

type sdkResult struct {
	Ok      bool
	Message string
	Data    goja.Value
}

var (
	exportStrip = regexp.MustCompile(`(?m)(^[ \t]*)export[ \t]+(function|const|let|var|class)`)
	metaRe      = regexp.MustCompile(`(?m)^\s*(?:export\s+)?const\s+(name|desc)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

func Run(fs *Store, r RunRequest) (RunResult, error) {
	code := strings.TrimSpace(r.Code)
	if code == "" {
		return RunResult{}, fmt.Errorf("código vazio: informe 'code' ou um 'name' de script salvo")
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = maxTimeout
	}
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	name, desc := parseMeta(r.Code)
	program := exportStrip.ReplaceAllString(r.Code, "$1$2")

	vm := goja.New()

	var outBuf strings.Builder
	truncated := false
	writeOut := func(s string) {
		if outBuf.Len() >= maxOutputBytes {
			truncated = true
			return
		}
		remaining := maxOutputBytes - outBuf.Len()
		if len(s) > remaining {
			s = s[:remaining]
			truncated = true
		}
		outBuf.WriteString(s)
	}

	std := vm.NewObject()

	printFn := func(call goja.FunctionCall) goja.Value {
		parts := make([]string, 0, len(call.Arguments))
		for _, a := range call.Arguments {
			parts = append(parts, a.String())
		}
		writeOut(strings.Join(parts, " ") + "\n")
		return goja.Undefined()
	}
	logObj := vm.NewObject()
	logObj.Set("ok", printFn)
	logObj.Set("err", printFn)
	std.Set("log", logObj)

	console := vm.NewObject()
	console.Set("log", printFn)
	console.Set("error", printFn)
	vm.Set("console", console)

	retObj := vm.NewObject()
	retObj.Set("ok", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(&sdkResult{Ok: true, Data: call.Argument(0)})
	})
	retObj.Set("err", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(&sdkResult{Ok: false, Message: call.Argument(0).String()})
	})
	std.Set("return", retObj)

	fsObj := vm.NewObject()
	fsObj.Set("read", func(call goja.FunctionCall) goja.Value {
		content, err := fs.Read(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(content)
	})
	fsObj.Set("lines", func(call goja.FunctionCall) goja.Value {
		lines, err := fs.ReadLines(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(lines)
	})
	fsObj.Set("json", func(call goja.FunctionCall) goja.Value {
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
	fsObj.Set("write", func(call goja.FunctionCall) goja.Value {
		n, err := fs.Write(call.Argument(0).String(), call.Argument(1).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(n)
	})
	fsObj.Set("append", func(call goja.FunctionCall) goja.Value {
		n, err := fs.Append(call.Argument(0).String(), call.Argument(1).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(n)
	})
	fsObj.Set("del", func(call goja.FunctionCall) goja.Value {
		if err := fs.Delete(call.Argument(0).String()); err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(true)
	})
	fsObj.Set("exists", func(call goja.FunctionCall) goja.Value {
		st, err := fs.Stat(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(st.Exists)
	})
	fsObj.Set("stat", func(call goja.FunctionCall) goja.Value {
		st, err := fs.Stat(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(map[string]any{
			"name": st.Name, "exists": st.Exists, "isDir": st.IsDir,
			"size": st.Size, "lines": st.Lines,
		})
	})
	fsObj.Set("dir", func(call goja.FunctionCall) goja.Value {
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
	std.Set("fs", fsObj)
	std.Set("args", loadArgs(vm, r.Args))

	dateObj := vm.NewObject()
	dateObj.Set("now", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(time.Now().UnixMilli())
	})
	dateObj.Set("iso", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(toTime(call.Argument(0), time.Now()).Format(time.RFC3339))
	})
	dateObj.Set("format", func(call goja.FunctionCall) goja.Value {
		layout := call.Argument(0).String()
		ref := time.Now()
		if len(call.Arguments) > 1 {
			ref = toTime(call.Argument(1), ref)
		}
		return vm.ToValue(formatDate(ref, layout))
	})
	dateObj.Set("parse", func(call goja.FunctionCall) goja.Value {
		t, err := parseTimeStr(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(t.UnixMilli())
	})
	dateObj.Set("add", func(call goja.FunctionCall) goja.Value {
		ref := toTime(call.Argument(0), time.Now())
		amount := int64(toNum(call.Argument(1)))
		unit := call.Argument(2).String()
		return vm.ToValue(addDate(ref, amount, unit).UnixMilli())
	})
	dateObj.Set("unix", func(call goja.FunctionCall) goja.Value {
		ref := toTime(call.Argument(0), time.Now())
		return vm.ToValue(ref.Unix())
	})
	dateObj.Set("diff", func(call goja.FunctionCall) goja.Value {
		a := toTime(call.Argument(0), time.Now())
		b := toTime(call.Argument(1), time.Now())
		return vm.ToValue(diffIn(a, b, call.Argument(2).String()))
	})
	std.Set("date", dateObj)

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
	std.Set("random", rndObj)

	std.Set("str", buildStr(vm))
	std.Set("list", buildList(vm))
	std.Set("num", buildNum(vm))
	std.Set("encode", buildEncode(vm))
	std.Set("json", buildJson(vm))
	std.Set("assert", buildAssert(vm))
	std.Set("fetch", buildFetch(vm))

	vm.Set("std", std)

	if !r.FullStdlib && stdlibDisabledByEnv() {
		disableStdlib(vm, "std", "console")
	}

	stop := make(chan struct{})
	go func() {
		select {
		case <-time.After(timeout):
			vm.Interrupt(fmt.Errorf("timeout excedido (%v)", timeout))
		case <-stop:
		}
	}()

	start := time.Now()
	_, runErr := vm.RunString(program)
	close(stop)
	dur := time.Since(start)

	res := RunResult{
		Name:        name,
		Description: desc,
		Output:      outBuf.String(),
		Duration:    dur,
		Truncated:   truncated,
		Ok:          true,
	}

	if runErr != nil {
		res.Ok = false
		res.Error = errorText(runErr)
		return res, nil
	}

	mainFn, ok := goja.AssertFunction(vm.Get("main"))
	if !ok {
		res.Ok = false
		res.Error = "o script precisa definir `function main(std)` (ou `export function main(std)`)"
		return res, nil
	}

	ret, callErr := mainFn(goja.Undefined(), std)
	if callErr != nil {
		res.Ok = false
		res.Error = "exceção em main: " + errorText(callErr)
		return res, nil
	}

	if sr, isRes := ret.Export().(*sdkResult); isRes {
		res.Ok = sr.Ok
		if sr.Ok {
			res.Data, res.DataJSON = renderData(sr.Data)
		} else {
			res.Error = sr.Message
		}
	} else if !goja.IsUndefined(ret) && !goja.IsNull(ret) {
		res.Data, res.DataJSON = renderData(ret)
	}
	return res, nil
}

func parseMeta(src string) (string, string) {
	var name, desc string
	for _, m := range metaRe.FindAllStringSubmatch(src, -1) {
		if len(m) != 4 {
			continue
		}
		val := m[2]
		if val == "" {
			val = m[3]
		}
		switch m[1] {
		case "name":
			name = val
		case "desc":
			desc = val
		}
	}
	return name, desc
}

func WrapScript(name, desc, body string) string {
	return fmt.Sprintf("const name = %s;\nconst desc = %s;\n\nexport function main(std) {\n%s\n}\n",
		strconv.Quote(name), strconv.Quote(desc), strings.TrimSpace(body))
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

func formatDate(t time.Time, layout string) string {
	l := layout
	l = strings.ReplaceAll(l, "YYYY", "2006")
	l = strings.ReplaceAll(l, "MM", "01")
	l = strings.ReplaceAll(l, "DD", "02")
	l = strings.ReplaceAll(l, "HH", "15")
	l = strings.ReplaceAll(l, "mm", "04")
	l = strings.ReplaceAll(l, "ss", "05")
	return t.Format(l)
}

func diffIn(a, b time.Time, unit string) float64 {
	d := a.Sub(b)
	switch strings.ToLower(unit) {
	case "week", "weeks":
		return d.Hours() / (24 * 7)
	case "day", "days":
		return d.Hours() / 24
	case "hour", "hours":
		return d.Hours()
	case "minute", "minutes":
		return d.Minutes()
	case "month", "months":
		return d.Hours() / (24 * 30.44)
	case "year", "years":
		return d.Hours() / (24 * 365.25)
	default:
		return d.Seconds()
	}
}

func addDate(t time.Time, amount int64, unit string) time.Time {
	switch strings.ToLower(unit) {
	case "day", "days":
		return t.AddDate(0, 0, int(amount))
	case "month", "months":
		return t.AddDate(0, int(amount), 0)
	case "year", "years":
		return t.AddDate(int(amount), 0, 0)
	case "hour", "hours":
		return t.Add(time.Duration(amount) * time.Hour)
	case "minute", "minutes":
		return t.Add(time.Duration(amount) * time.Minute)
	case "second", "seconds":
		return t.Add(time.Duration(amount) * time.Second)
	default:
		return t
	}
}

func parseTimeStr(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("data inválida: %s", s)
}

func toTime(v goja.Value, def time.Time) time.Time {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return def
	}
	switch t := v.Export().(type) {
	case time.Time:
		return t
	case int64:
		return time.UnixMilli(t)
	case float64:
		return time.UnixMilli(int64(t))
	case string:
		if p, err := parseTimeStr(t); err == nil {
			return p
		}
	}
	return def
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

func stdlibDisabledByEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SANDBOX_DISABLE_JS_STDLIB"))) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func disableStdlib(vm *goja.Runtime, keep ...string) {
	keepSet := make(map[string]bool, len(keep))
	for _, k := range keep {
		keepSet[k] = true
	}
	g := vm.GlobalObject()
	vm.Set("__sandbox_g", g)
	names, err := vm.RunString("Object.getOwnPropertyNames(__sandbox_g)")
	if err != nil {
		return
	}
	if arr, ok := names.Export().([]any); ok {
		for _, n := range arr {
			name, _ := n.(string)
			if name == "" || keepSet[name] {
				continue
			}
			g.Delete(name)
		}
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
