package sandbox

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"strings"

	"github.com/dop251/goja"
)

func buildEncode(vm *goja.Runtime) *goja.Object {
	o := vm.NewObject()
	o.Set("crc32", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(call.Argument(0).String()))))
	})
	o.Set("md5", func(call goja.FunctionCall) goja.Value {
		sum := md5.Sum([]byte(call.Argument(0).String()))
		return vm.ToValue(hex.EncodeToString(sum[:]))
	})
	o.Set("sha256", func(call goja.FunctionCall) goja.Value {
		sum := sha256.Sum256([]byte(call.Argument(0).String()))
		return vm.ToValue(hex.EncodeToString(sum[:]))
	})
	o.Set("base64", func(call goja.FunctionCall) goja.Value {
		s := call.Argument(0).String()
		switch strings.ToLower(strings.TrimSpace(optString(call, 1))) {
		case "", "encode", "std", "standard":
			return vm.ToValue(base64.StdEncoding.EncodeToString([]byte(s)))
		case "decode", "dec":
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(string(b))
		case "url", "encodeurl", "urlencode", "urlsafe", "url-safe":
			return vm.ToValue(base64.URLEncoding.EncodeToString([]byte(s)))
		case "urldecode", "decodeurl", "url_decode":
			b, err := base64.URLEncoding.DecodeString(s)
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(string(b))
		default:
			panic(vm.NewGoError(fmt.Errorf("modo base64 inválido: %s", optString(call, 1))))
		}
	})
	o.Set("hex", func(call goja.FunctionCall) goja.Value {
		s := call.Argument(0).String()
		switch strings.ToLower(strings.TrimSpace(optString(call, 1))) {
		case "", "encode", "enc":
			return vm.ToValue(hex.EncodeToString([]byte(s)))
		case "decode", "dec":
			b, err := hex.DecodeString(s)
			if err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(string(b))
		default:
			panic(vm.NewGoError(fmt.Errorf("modo hex inválido: %s", optString(call, 1))))
		}
	})
	return o
}
