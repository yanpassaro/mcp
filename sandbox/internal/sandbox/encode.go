package sandbox

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func buildEncode(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "crc32", func(l *lua.State) int {
		l.PushString(fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(argString(l, 1)))))
		return 1
	})
	setGoFunc(L, t, "md5", func(l *lua.State) int {
		sum := md5.Sum([]byte(argString(l, 1)))
		l.PushString(hex.EncodeToString(sum[:]))
		return 1
	})
	setGoFunc(L, t, "sha256", func(l *lua.State) int {
		sum := sha256.Sum256([]byte(argString(l, 1)))
		l.PushString(hex.EncodeToString(sum[:]))
		return 1
	})
	setGoFunc(L, t, "base64", func(l *lua.State) int {
		s := argString(l, 1)
		switch strings.ToLower(strings.TrimSpace(argString(l, 2))) {
		case "", "encode", "std", "standard":
			l.PushString(base64.StdEncoding.EncodeToString([]byte(s)))
		case "decode", "dec":
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				panic(fmt.Errorf("modo base64 inválido: %v", err))
			}
			l.PushString(string(b))
		case "url", "encodeurl", "urlencode", "urlsafe", "url-safe":
			l.PushString(base64.URLEncoding.EncodeToString([]byte(s)))
		default:
			panic(fmt.Errorf("modo base64 inválido: %s", argString(l, 2)))
		}
		return 1
	})
	setGoFunc(L, t, "hex", func(l *lua.State) int {
		s := argString(l, 1)
		switch strings.ToLower(strings.TrimSpace(argString(l, 2))) {
		case "", "encode", "enc":
			l.PushString(hex.EncodeToString([]byte(s)))
		case "decode", "dec":
			b, err := hex.DecodeString(s)
			if err != nil {
				panic(fmt.Errorf("modo hex inválido: %v", err))
			}
			l.PushString(string(b))
		default:
			panic(fmt.Errorf("modo hex inválido: %s", argString(l, 2)))
		}
		return 1
	})
	return t
}
