package sandbox

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"strings"

	lua "github.com/Shopify/go-lua"
	"golang.org/x/crypto/sha3"
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
	setGoFunc(L, t, "sha1", func(l *lua.State) int {
		sum := sha1.Sum([]byte(argString(l, 1)))
		l.PushString(hex.EncodeToString(sum[:]))
		return 1
	})
	setGoFunc(L, t, "sha224", func(l *lua.State) int {
		sum := sha256.Sum224([]byte(argString(l, 1)))
		l.PushString(hex.EncodeToString(sum[:]))
		return 1
	})
	setGoFunc(L, t, "sha384", func(l *lua.State) int {
		sum := sha512.Sum384([]byte(argString(l, 1)))
		l.PushString(hex.EncodeToString(sum[:]))
		return 1
	})
	setGoFunc(L, t, "sha512", func(l *lua.State) int {
		sum := sha512.Sum512([]byte(argString(l, 1)))
		l.PushString(hex.EncodeToString(sum[:]))
		return 1
	})
	setGoFunc(L, t, "sha3_256", func(l *lua.State) int {
		sum := sha3.Sum256([]byte(argString(l, 1)))
		l.PushString(hex.EncodeToString(sum[:]))
		return 1
	})
	setGoFunc(L, t, "sha3_512", func(l *lua.State) int {
		sum := sha3.Sum512([]byte(argString(l, 1)))
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
	setGoFunc(L, t, "base32", func(l *lua.State) int {
		s := argString(l, 1)
		switch strings.ToLower(strings.TrimSpace(argString(l, 2))) {
		case "", "encode", "std", "standard":
			l.PushString(base32.StdEncoding.EncodeToString([]byte(s)))
		case "decode", "dec":
			b, err := base32.StdEncoding.DecodeString(s)
			if err != nil {
				panic(fmt.Errorf("modo base32 inválido: %v", err))
			}
			l.PushString(string(b))
		case "hex", "hexencode", "base32hex":
			l.PushString(base32.HexEncoding.EncodeToString([]byte(s)))
		case "nopad", "nopadding", "raw":
			l.PushString(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(s)))
		default:
			panic(fmt.Errorf("modo base32 inválido: %s", argString(l, 2)))
		}
		return 1
	})
	setGoFunc(L, t, "argon2", argon2HashFn)
	setGoFunc(L, t, "argon2_verify", argon2VerifyFn)
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
