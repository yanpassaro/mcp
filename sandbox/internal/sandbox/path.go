package sandbox

import (
	"fmt"
	"path"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func buildPath(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "join", func(l *lua.State) int {
		parts := make([]string, 0, l.Top())
		for i := 1; i <= l.Top(); i++ {
			parts = append(parts, argString(l, i))
		}
		l.PushString(path.Join(parts...))
		return 1
	})
	setGoFunc(L, t, "basename", func(l *lua.State) int {
		l.PushString(path.Base(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "dirname", func(l *lua.State) int {
		l.PushString(path.Dir(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "ext", func(l *lua.State) int {
		l.PushString(path.Ext(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "stem", func(l *lua.State) int {
		base := path.Base(argString(l, 1))
		l.PushString(strings.TrimSuffix(base, path.Ext(base)))
		return 1
	})
	setGoFunc(L, t, "split", func(l *lua.State) int {
		dir, file := path.Split(argString(l, 1))
		pushAny(l, map[string]any{"dir": dir, "file": file})
		return 1
	})
	setGoFunc(L, t, "normalize", func(l *lua.State) int {
		l.PushString(path.Clean(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "is_abs", func(l *lua.State) int {
		l.PushBoolean(path.IsAbs(argString(l, 1)))
		return 1
	})
	setGoFunc(L, t, "rel", func(l *lua.State) int {
		r, err := pathRel(argString(l, 1), argString(l, 2))
		if err != nil {
			panic(err)
		}
		l.PushString(r)
		return 1
	})
	setGoFunc(L, t, "within", func(l *lua.State) int {
		base := path.Clean(argString(l, 1))
		p := path.Clean(argString(l, 2))
		l.PushBoolean(p == base || strings.HasPrefix(p, base+"/"))
		return 1
	})

	return t
}

func pathRel(base, target string) (string, error) {
	b := path.Clean(base)
	t := path.Clean(target)
	if b == t {
		return ".", nil
	}
	bAbs, tAbs := path.IsAbs(b), path.IsAbs(t)
	if bAbs != tAbs {
		return "", fmt.Errorf("caminhos com raízes diferentes: %q vs %q", base, target)
	}
	bSegs := pathSegments(b)
	tSegs := pathSegments(t)
	i := 0
	for i < len(bSegs) && i < len(tSegs) && bSegs[i] == tSegs[i] {
		i++
	}
	parts := make([]string, 0, len(bSegs)-i+len(tSegs)-i)
	for j := i; j < len(bSegs); j++ {
		parts = append(parts, "..")
	}
	parts = append(parts, tSegs[i:]...)
	if len(parts) == 0 {
		return ".", nil
	}
	return strings.Join(parts, "/"), nil
}

func pathSegments(p string) []string {
	if p == "/" {
		return []string{}
	}
	segs := strings.Split(p, "/")
	if len(segs) > 0 && segs[0] == "" {
		segs = segs[1:]
	}
	return segs
}
