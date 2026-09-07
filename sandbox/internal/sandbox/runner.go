package sandbox

import (
	"fmt"
	"strings"
	"time"

	lua "github.com/Shopify/go-lua"
)

const (
	maxTimeout          = 30 * time.Second
	maxOutputBytes      = 256 * 1024
	maxScriptConcurrent = 4
)

var scriptSlots = make(chan struct{}, maxScriptConcurrent)

type RunRequest struct {
	Code    string
	Args    string
	Timeout time.Duration
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

type luaResult struct {
	ok   bool
	msg  string
	data any
}

type runOutcome struct {
	res RunResult
	err error
}

func Run(store, tmp *Store, r RunRequest) (RunResult, error) {
	secrets := LoadSecrets()

	select {
	case scriptSlots <- struct{}{}:
	default:
		return RunResult{}, fmt.Errorf("limite de %d scripts simultâneos atingido; aguarde ou encerre os que travarem", maxScriptConcurrent)
	}

	ch := make(chan runOutcome, 1)
	go func() {
		defer func() { <-scriptSlots }()
		res, err := execScript(store, tmp, r, secrets)
		ch <- runOutcome{res, err}
	}()

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = maxTimeout
	}
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	select {
	case o := <-ch:
		o.res.Output = secrets.Redact(o.res.Output)
		o.res.Data = secrets.Redact(o.res.Data)
		o.res.Error = secrets.Redact(o.res.Error)
		o.res.Description = secrets.Redact(o.res.Description)
		if o.err != nil {
			return o.res, fmt.Errorf("%s", secrets.Redact(o.err.Error()))
		}
		return o.res, nil
	case <-time.After(timeout):
		return RunResult{
			Error:    fmt.Sprintf("tempo de execução excedido (%s)", timeout),
			Duration: timeout,
			Ok:       false,
		}, nil
	}
}

func execScript(store, tmp *Store, r RunRequest, secrets *Secrets) (RunResult, error) {
	code := strings.TrimSpace(r.Code)
	if code == "" {
		return RunResult{}, fmt.Errorf("código vazio: informe 'code' ou um 'name' de script salvo")
	}

	name, desc := parseMeta(code)

	reg := newSQLRegistry()
	defer reg.close()

	L := lua.NewState()
	lua.Require(L, "base", lua.BaseOpen, true)
	lua.Require(L, "table", lua.TableOpen, true)
	lua.Require(L, "string", lua.StringOpen, true)
	lua.Require(L, "math", lua.MathOpen, true)
	hardenLua(L)

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

	var res *luaResult
	buildStd(L, store, tmp, reg, r.Args, writeOut, &res, secrets)
	L.SetGlobal("std")

	L.PushGoFunction(func(l *lua.State) int {
		parts := make([]string, 0, l.Top())
		for i := 1; i <= l.Top(); i++ {
			parts = append(parts, argString(l, i))
		}
		writeOut(strings.Join(parts, " ") + "\n")
		return 0
	})
	L.SetGlobal("print")

	start := time.Now()
	if err := L.Load(strings.NewReader(code), "@script", "t"); err != nil {
		return RunResult{Name: name, Description: desc, Output: outBuf.String(), Duration: time.Since(start), Ok: false, Error: callError(L, err)}, nil
	}
	if err := L.ProtectedCall(0, 0, 0); err != nil {
		return RunResult{Name: name, Description: desc, Output: outBuf.String(), Duration: time.Since(start), Ok: false, Error: callError(L, err)}, nil
	}

	result := RunResult{Name: name, Description: desc, Output: outBuf.String(), Ok: true, Truncated: truncated}

	L.Global("main")
	if L.IsNil(L.Top()) {
		result.Ok = false
		result.Error = "o script precisa definir `function main(std)`"
		result.Duration = time.Since(start)
		return result, nil
	}
	L.Global("std")
	if err := L.ProtectedCall(1, 1, 0); err != nil {
		result.Ok = false
		result.Error = callError(L, err)
		result.Duration = time.Since(start)
		return result, nil
	}
	ret := L.Top()
	result.Duration = time.Since(start)

	if res != nil {
		result.Ok = res.ok
		if res.ok {
			result.Data, result.DataJSON = renderData(secrets.RedactValue(res.data))
		} else {
			result.Error = res.msg
		}
	} else if !L.IsNil(ret) {
		result.Data, result.DataJSON = renderData(secrets.RedactValue(luaToAny(L, ret)))
	}
	return result, nil
}

func hardenLua(l *lua.State) {
	for _, name := range []string{
		"dofile", "loadfile", "load", "loadstring", "require", "module",
		"collectgarbage", "gcinfo", "getfenv", "setfenv", "newproxy",
		"os", "io", "debug", "package", "coroutine", "cjson",
	} {
		l.PushNil()
		l.SetGlobal(name)
	}
}

func callError(l *lua.State, err error) string {
	msg := ""
	if l.Top() > 0 {
		if s, ok := l.ToString(l.Top()); ok && s != "" {
			msg = s
		}
	}
	if msg == "" {
		msg = errorText(err)
	}
	msg = strings.TrimPrefix(msg, "runtime error: ")
	msg = strings.TrimPrefix(msg, "error: ")
	return msg
}

func parseMeta(code string) (string, string) {
	var name, desc string
	for _, line := range strings.Split(code, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "--"))
			if strings.HasPrefix(line, "name=") {
				name = strings.Trim(strings.TrimPrefix(line, "name="), `"' `)
			} else if strings.HasPrefix(line, "desc=") {
				desc = strings.Trim(strings.TrimPrefix(line, "desc="), `"' `)
			}
		}
	}
	return name, desc
}

func WrapScript(name, desc, body string) string {
	return fmt.Sprintf("-- name=%q\n-- desc=%q\n\nfunction main(std)\n%s\nend\n", name, desc, strings.TrimSpace(body))
}

func buildStd(L *lua.State, store, tmp *Store, reg *sqlRegistry, args string, writeOut func(string), res **luaResult, secrets *Secrets) {
	L.NewTable()
	std := L.Top()

	buildLog(L, writeOut)
	L.SetField(std, "log")

	buildResult(L, res)
	L.SetField(std, "result")

	pushAny(L, parseArgs(args))
	L.SetField(std, "args")

	setModule := func(name string, build func() int) {
		build()
		L.SetField(std, name)
	}
	setModule("io", func() int { return buildIO(L, store) })
	setModule("tmp", func() int { return buildTmp(L, tmp) })
	setModule("date", func() int { return buildDate(L) })
	setModule("random", func() int { return buildRandom(L) })
	setModule("str", func() int { return buildStr(L) })
	setModule("list", func() int { return buildList(L) })
	setModule("num", func() int { return buildNum(L) })
	setModule("encode", func() int { return buildEncode(L) })
	setModule("json", func() int { return buildJson(L) })
	setModule("assert", func() int { return buildAssert(L) })
	setModule("fetch", func() int { return buildFetch(L) })
	setModule("secrets", func() int { return buildSecrets(L, secrets) })
	setModule("sql", func() int { return buildSQL(L, reg, store, tmp) })
	setModule("uuid", func() int { return buildUUID(L) })
	setModule("csv", func() int { return buildCSV(L) })
	setModule("xml", func() int { return buildXML(L) })
	setModule("excel", func() int { return buildExcel(L, store, tmp) })
	setModule("data", func() int { return buildData(L, reg, store, tmp) })
	setModule("regex", func() int { return buildRegex(L) })
	setModule("fake", func() int { return buildFake(L) })

	buildLog(L, writeOut)
	L.SetGlobal("console")
}

func buildLog(L *lua.State, writeOut func(string)) int {
	t := newTable(L)
	print := func(l *lua.State) int {
		parts := make([]string, 0, l.Top())
		for i := 1; i <= l.Top(); i++ {
			parts = append(parts, argString(l, i))
		}
		writeOut(strings.Join(parts, " ") + "\n")
		return 0
	}
	setGoFunc(L, t, "ok", print)
	setGoFunc(L, t, "err", print)
	setGoFunc(L, t, "log", print)
	setGoFunc(L, t, "error", print)
	setGoFunc(L, t, "info", print)
	setGoFunc(L, t, "warn", print)
	return t
}

func buildResult(L *lua.State, res **luaResult) int {
	t := newTable(L)
	setGoFunc(L, t, "ok", func(l *lua.State) int {
		*res = &luaResult{ok: true, data: luaToAny(l, 1)}
		return 0
	})
	setGoFunc(L, t, "err", func(l *lua.State) int {
		*res = &luaResult{ok: false, msg: argString(l, 1)}
		return 0
	})
	return t
}
