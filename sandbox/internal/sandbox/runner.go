package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
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

	std.Set("fs", buildFS(vm, fs))
	std.Set("args", loadArgs(vm, r.Args))

	std.Set("date", buildDate(vm))
	std.Set("random", buildRandom(vm))

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
