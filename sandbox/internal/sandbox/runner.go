package sandbox

import (
	"fmt"
	"strings"
	"time"

	lua "github.com/Shopify/go-lua"
)

const (
	maxTimeout     = 30 * time.Second
	maxOutputBytes = 256 * 1024
)

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

	name, desc := parseMeta(code)

	L := lua.NewState()
	lua.Require(L, "base", lua.BaseOpen, true)
	lua.Require(L, "table", lua.TableOpen, true)
	lua.Require(L, "string", lua.StringOpen, true)
	lua.Require(L, "math", lua.MathOpen, true)

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
	buildStd(L, fs, r.Args, writeOut, &res)
	L.SetGlobal("std")

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
			result.Data, result.DataJSON = renderData(res.data)
		} else {
			result.Error = res.msg
		}
	} else if !L.IsNil(ret) {
		result.Data, result.DataJSON = renderData(luaToAny(L, ret))
	}
	return result, nil
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

func buildStd(L *lua.State, fs *Store, args string, writeOut func(string), res **luaResult) {
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
	setModule("fs", func() int { return buildFS(L, fs) })
	setModule("date", func() int { return buildDate(L) })
	setModule("random", func() int { return buildRandom(L) })
	setModule("str", func() int { return buildStr(L) })
	setModule("list", func() int { return buildList(L) })
	setModule("num", func() int { return buildNum(L) })
	setModule("encode", func() int { return buildEncode(L) })
	setModule("json", func() int { return buildJson(L) })
	setModule("assert", func() int { return buildAssert(L) })
	setModule("fetch", func() int { return buildFetch(L) })

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
