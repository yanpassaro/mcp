package sandbox

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"

	lua "github.com/Shopify/go-lua"
)

func buildTmp(L *lua.State, store *Store) int {
	t := buildIO(L, store)
	setGoFunc(L, t, "clear", func(l *lua.State) int {
		n, err := store.Clear()
		if err != nil {
			panic(err)
		}
		l.PushInteger(n)
		return 1
	})
	return t
}

func CleanupTTL(root string, ttl time.Duration) (int, error) {
	if ttl <= 0 || root == "" {
		return 0, nil
	}
	now := time.Now()
	removed := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, e := d.Info(); e == nil && now.Sub(info.ModTime()) > ttl {
			if os.Remove(path) == nil {
				removed++
			}
		}
		return nil
	})
	return removed, err
}
