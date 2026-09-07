package sandbox

import (
	"crypto/rand"
	"fmt"
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
	return t
}

// uuidV4 returns a random RFC 4122 UUID (version 4).
func uuidV4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// uuidV7 returns a time-ordered RFC 9562 UUID (version 7): 48-bit unix ms
// timestamp followed by random bits.
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
	b[6] = (b[6] & 0x0f) | 0x70 // version 7
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
