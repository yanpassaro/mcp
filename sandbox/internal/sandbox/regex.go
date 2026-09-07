package sandbox

import (
	"fmt"
	"regexp"

	lua "github.com/Shopify/go-lua"
)

func buildRegex(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "match", func(l *lua.State) int {
		re := compileRegex(argString(l, 2))
		l.PushBoolean(re.MatchString(argString(l, 1)))
		return 1
	})

	setGoFunc(L, t, "find", func(l *lua.State) int {
		re := compileRegex(argString(l, 2))
		if m := re.FindString(argString(l, 1)); m != "" {
			l.PushString(m)
		} else {
			l.PushNil()
		}
		return 1
	})

	setGoFunc(L, t, "findAll", func(l *lua.State) int {
		re := compileRegex(argString(l, 2))
		n := regexLimit(l, 3)
		var res []string
		if n > 0 {
			res = re.FindAllString(argString(l, 1), n)
		} else {
			res = re.FindAllString(argString(l, 1), -1)
		}
		pushAny(l, res)
		return 1
	})

	setGoFunc(L, t, "replace", func(l *lua.State) int {
		re := compileRegex(argString(l, 2))
		l.PushString(re.ReplaceAllString(argString(l, 1), argString(l, 3)))
		return 1
	})

	setGoFunc(L, t, "split", func(l *lua.State) int {
		re := compileRegex(argString(l, 2))
		n := regexLimit(l, 3)
		var res []string
		if n > 0 {
			res = re.Split(argString(l, 1), n)
		} else {
			res = re.Split(argString(l, 1), -1)
		}
		pushAny(l, res)
		return 1
	})

	setGoFunc(L, t, "groups", func(l *lua.State) int {
		re := compileRegex(argString(l, 2))
		sub := re.FindStringSubmatch(argString(l, 1))
		if sub == nil {
			l.PushNil()
			return 1
		}
		pushAny(l, subToTable(sub))
		return 1
	})

	setGoFunc(L, t, "findAllGroups", func(l *lua.State) int {
		re := compileRegex(argString(l, 2))
		n := regexLimit(l, 3)
		subs := re.FindAllStringSubmatch(argString(l, 1), n)
		out := make([]any, len(subs))
		for i, sub := range subs {
			out[i] = subToTable(sub)
		}
		pushAny(l, out)
		return 1
	})

	return t
}

func compileRegex(p string) *regexp.Regexp {
	re, err := regexp.Compile(p)
	if err != nil {
		panic(fmt.Errorf("regex inválida: %w", err))
	}
	return re
}

func regexLimit(l *lua.State, i int) int {
	n := int(argNum(l, i))
	if n < 0 {
		return 0
	}
	return n
}

func subToTable(sub []string) map[string]any {
	groups := make([]any, len(sub)-1)
	for i := 1; i < len(sub); i++ {
		groups[i-1] = sub[i]
	}
	return map[string]any{
		"match":  sub[0],
		"groups": groups,
	}
}
