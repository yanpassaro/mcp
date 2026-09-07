package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func ioErr(op, name string, err error) error {
	if os.IsNotExist(err) {
		return fmt.Errorf("%s %q: arquivo não encontrado", op, name)
	}
	if os.IsPermission(err) {
		return fmt.Errorf("%s %q: sem permissão", op, name)
	}
	return fmt.Errorf("%s %q: %w", op, name, err)
}

func buildIO(L *lua.State, store *Store) int {
	t := newTable(L)
	setGoFunc(L, t, "read", func(l *lua.State) int {
		name := argString(l, 1)
		content, err := store.Read(name)
		if err != nil {
			panic(ioErr("ler", name, err))
		}
		l.PushString(content)
		return 1
	})
	setGoFunc(L, t, "lines", func(l *lua.State) int {
		name := argString(l, 1)
		lines, err := store.ReadLines(name)
		if err != nil {
			panic(ioErr("ler", name, err))
		}
		pushAny(l, lines)
		return 1
	})
	setGoFunc(L, t, "json", func(l *lua.State) int {
		name := argString(l, 1)
		content, err := store.Read(name)
		if err != nil {
			panic(ioErr("ler", name, err))
		}
		var v any
		if err := json.Unmarshal([]byte(content), &v); err != nil {
			panic(fmt.Errorf("JSON inválido em %q: %w", name, err))
		}
		pushAny(l, v)
		return 1
	})
	setGoFunc(L, t, "write", func(l *lua.State) int {
		n, err := store.Write(argString(l, 1), argString(l, 2))
		if err != nil {
			panic(err)
		}
		l.PushInteger(n)
		return 1
	})
	setGoFunc(L, t, "append", func(l *lua.State) int {
		n, err := store.Append(argString(l, 1), argString(l, 2))
		if err != nil {
			panic(err)
		}
		l.PushInteger(n)
		return 1
	})
	setGoFunc(L, t, "del", func(l *lua.State) int {
		if err := store.Delete(argString(l, 1)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})
	setGoFunc(L, t, "exists", func(l *lua.State) int {
		st, err := store.Stat(argString(l, 1))
		if err != nil {
			panic(err)
		}
		l.PushBoolean(st.Exists)
		return 1
	})
	setGoFunc(L, t, "stat", func(l *lua.State) int {
		st, err := store.Stat(argString(l, 1))
		if err != nil {
			panic(err)
		}
		pushAny(l, map[string]any{
			"name": st.Name, "exists": st.Exists, "isDir": st.IsDir,
			"size": st.Size, "lines": st.Lines,
		})
		return 1
	})
	setGoFunc(L, t, "dir", func(l *lua.State) int {
		entries, err := store.ListDir(strings.TrimSpace(argString(l, 1)))
		if err != nil {
			panic(err)
		}
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name
		}
		pushAny(l, names)
		return 1
	})
	setGoFunc(L, t, "copy", func(l *lua.State) int {
		if _, err := store.CopyWithin(argString(l, 1), argString(l, 2)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})
	setGoFunc(L, t, "move", func(l *lua.State) int {
		if _, err := store.Move(argString(l, 1), argString(l, 2)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})
	setGoFunc(L, t, "mkdir", func(l *lua.State) int {
		if _, err := store.Mkdir(argString(l, 1)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})
	setGoFunc(L, t, "glob", func(l *lua.State) int {
		names, err := store.Glob(argString(l, 1))
		if err != nil {
			panic(err)
		}
		pushAny(l, names)
		return 1
	})
	setGoFunc(L, t, "walk", func(l *lua.State) int {
		node, err := store.Tree(strings.TrimSpace(argString(l, 1)))
		if err != nil {
			panic(err)
		}
		pushAny(l, treeNodeToAny(node))
		return 1
	})
	return t
}

func treeNodeToAny(n TreeNode) map[string]any {
	out := map[string]any{
		"name": n.Name, "isDir": n.IsDir, "size": n.Size, "lines": n.Lines,
	}
	if len(n.Children) > 0 {
		children := make([]any, len(n.Children))
		for i, c := range n.Children {
			children[i] = treeNodeToAny(c)
		}
		out["children"] = children
	}
	return out
}
