package sandbox

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	lua "github.com/Shopify/go-lua"
)

func buildUUID(L *lua.State) int {
	t := newTable(L)
	setGoFunc(L, t, "v4", func(l *lua.State) int {
		l.PushString(uuidV4())
		return 1
	})
	setGoFunc(L, t, "v7", func(l *lua.State) int {
		l.PushString(uuidV7(time.Now()))
		return 1
	})
	setGoFunc(L, t, "valid", func(l *lua.State) int {
		l.PushBoolean(uuidValid(argString(l, 1), int(argNum(l, 2))))
		return 1
	})
	return t
}

func uuidValid(s string, version int) bool {
	s = strings.TrimSpace(s)
	var h string
	if len(s) == 36 {
		if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
			return false
		}
		h = strings.ReplaceAll(s, "-", "")
	} else if len(s) == 32 {
		h = s
	} else {
		return false
	}
	if len(h) != 32 || !isHexStr(h) {
		return false
	}
	if version != 0 && h[12] != byte('0'+version) {
		return false
	}
	switch h[16] {
	case '8', '9', 'a', 'b', 'A', 'B':
		return true
	default:
		return false
	}
}

func isHexStr(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func uuidV4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func uuidV7(now time.Time) string {
	var b [16]byte
	ms := uint64(now.UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	_, _ = rand.Read(b[6:])
	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
