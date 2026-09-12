package sandbox

import (
	"encoding/base64"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	lua "github.com/Shopify/go-lua"
)

var (
	phonOns      = []string{"b", "c", "d", "f", "g", "j", "l", "m", "n", "p", "r", "s", "t", "v", "z", "br", "cr", "dr", "fl", "gr", "pl", "tr", "ch", "qu"}
	phonVow      = []string{"a", "e", "i", "o", "u", "ã", "é", "ó", "á"}
	phonVowAscii = []string{"a", "e", "i", "o", "u"}
)

func buildRandom(L *lua.State) int {
	var rng *rand.Rand
	newRNG := func(seed int64) {
		rng = rand.New(rand.NewPCG(uint64(seed), uint64(seed)))
	}
	newRNG(time.Now().UnixNano())
	t := newTable(L)

	setGoFunc(L, t, "seed", func(l *lua.State) int {
		newRNG(int64(argNum(l, 1)))
		return 0
	})
	setGoFunc(L, t, "int", func(l *lua.State) int {
		min, max := int(argNum(l, 1)), int(argNum(l, 2))
		if max < min {
			min, max = max, min
		}
		if min == max {
			l.PushInteger(min)
			return 1
		}
		l.PushInteger(min + rng.IntN(max-min+1))
		return 1
	})
	setGoFunc(L, t, "pick", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		if len(arr) == 0 {
			l.PushNil()
			return 1
		}
		pushAny(l, arr[rng.IntN(len(arr))])
		return 1
	})
	setGoFunc(L, t, "shuffle", func(l *lua.State) int {
		arr := luaArrayAny(l, 1)
		cp := append([]any(nil), arr...)
		rng.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
		pushAny(l, cp)
		return 1
	})
	setGoFunc(L, t, "name", func(l *lua.State) int {
		l.PushString(cap1(fakeTokenAscii(rng)) + " " + cap1(fakeTokenAscii(rng)))
		return 1
	})
	setGoFunc(L, t, "email", func(l *lua.State) int {
		f := strings.ToLower(fakeTokenAscii(rng))
		m := strings.ToLower(fakeTokenAscii(rng))
		d := strings.ToLower(fakeTokenAscii(rng))
		l.PushString(f + "." + m + "@" + d + ".com")
		return 1
	})
	setGoFunc(L, t, "username", func(l *lua.State) int {
		f := strings.ToLower(fakeTokenAscii(rng))
		m := strings.ToLower(fakeTokenAscii(rng))
		l.PushString(f + m + fmt.Sprint(10+rng.IntN(90)))
		return 1
	})
	setGoFunc(L, t, "phone", func(l *lua.State) int {
		l.PushString(fmt.Sprintf("+55 (%d%d) %04d-%04d", 1+rng.IntN(9), rng.IntN(10), rng.IntN(10000), rng.IntN(10000)))
		return 1
	})
	setGoFunc(L, t, "float", func(l *lua.State) int {
		min, max := argNum(l, 1), argNum(l, 2)
		dec := int(argNum(l, 3))
		if max < min {
			min, max = max, min
		}
		v := min + rng.Float64()*(max-min)
		l.PushNumber(round(v, dec))
		return 1
	})
	setGoFunc(L, t, "bool", func(l *lua.State) int {
		l.PushBoolean(rng.IntN(2) == 1)
		return 1
	})
	setGoFunc(L, t, "date", func(l *lua.State) int {
		from := time.Now().AddDate(0, 0, -365)
		to := time.Now()
		if s := strings.TrimSpace(argString(l, 1)); s != "" {
			if ts, err := parseTimeStr(s, time.UTC); err == nil {
				from = ts
			}
		}
		if s := strings.TrimSpace(argString(l, 2)); s != "" {
			if ts, err := parseTimeStr(s, time.UTC); err == nil {
				to = ts
			}
		}
		if to.Before(from) {
			from, to = to, from
		}
		span := to.UnixMilli() - from.UnixMilli()
		if span <= 0 {
			l.PushString(to.Format(time.RFC3339))
			return 1
		}
		ts := from.UnixMilli() + rng.Int64N(span)
		l.PushString(time.UnixMilli(ts).Format(time.RFC3339))
		return 1
	})
	setGoFunc(L, t, "sentence", func(l *lua.State) int {
		n := max(int(argNum(l, 1)), 1)
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fakeSentence(rng)
		}
		l.PushString(strings.Join(parts, " "))
		return 1
	})
	setGoFunc(L, t, "paragraph", func(l *lua.State) int {
		n := int(argNum(l, 1))
		if n < 1 {
			n = 3
		}
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fakeSentence(rng)
		}
		l.PushString(strings.Join(parts, " "))
		return 1
	})
	setGoFunc(L, t, "password", func(l *lua.State) int {
		length := int(argNum(l, 1))
		opts := toAnyMap(l, 2)
		if n := optUint(opts, "length", 0); n > 0 {
			length = int(n)
		}
		l.PushString(randomPassword(rng, length, opts))
		return 1
	})
	setGoFunc(L, t, "token", func(l *lua.State) int {
		n := int(argNum(l, 1))
		if n <= 0 {
			n = 32
		}
		opts := toAnyMap(l, 2)
		if b := optUint(opts, "bytes", uint32(n)); b > 0 {
			n = int(b)
		}
		if alphabet := optString(opts, "alphabet"); alphabet != "" {
			l.PushString(secureToken(n, alphabet))
		} else {
			l.PushString(base64.RawURLEncoding.EncodeToString(randomBytes(n)))
		}
		return 1
	})
	setGoFunc(L, t, "cpf", func(l *lua.State) int {
		l.PushString(randomCPF(rng))
		return 1
	})
	setGoFunc(L, t, "cnpj", func(l *lua.State) int {
		l.PushString(randomCNPJ(rng))
		return 1
	})
	setGoFunc(L, t, "cep", func(l *lua.State) int {
		l.PushString(randomCEP(rng))
		return 1
	})
	setGoFunc(L, t, "rg", func(l *lua.State) int {
		l.PushString(randomRG(rng))
		return 1
	})
	return t
}

func fakeToken(rng *rand.Rand) string {
	return fakeTokenWith(phonVow, rng)
}

func fakeTokenAscii(rng *rand.Rand) string {
	return fakeTokenWith(phonVowAscii, rng)
}

func fakeTokenWith(vow []string, rng *rand.Rand) string {
	n := 2 + rng.IntN(2)
	var b strings.Builder
	for range n {
		b.WriteString(phonOns[rng.IntN(len(phonOns))])
		b.WriteString(vow[rng.IntN(len(vow))])
	}
	return b.String()
}

func fakeSentence(rng *rand.Rand) string {
	n := 3 + rng.IntN(3)
	words := make([]string, n)
	for i := range words {
		words[i] = fakeToken(rng)
	}
	words[0] = cap1(words[0])
	return strings.Join(words, " ") + "."
}

func cap1(s string) string {
	return strings.ToUpper(s[:1]) + s[1:]
}

const (
	pwLower   = "abcdefghijklmnopqrstuvwxyz"
	pwUpper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	pwDigits  = "0123456789"
	pwSymbols = "!@#$%^&*()-_=+[]{};:,.<>?/|~"
)

func secureToken(n int, alphabet string) string {
	if len(alphabet) > 255 {
		panic(fmt.Errorf("token: alphabet com mais de 255 caracteres"))
	}
	out := make([]byte, n)
	limit := 256 - (256 % len(alphabet))
	for i := range out {
		for {
			b := randomBytes(1)[0]
			if int(b) < limit {
				out[i] = alphabet[int(b)%len(alphabet)]
				break
			}
		}
	}
	return string(out)
}

func randomPassword(rng *rand.Rand, length int, opts map[string]any) string {
	if length <= 0 {
		length = 16
	}
	if length < 4 {
		length = 4
	}

	var classes []string
	pool := make([]byte, 0, 128)
	addClass := func(s string) {
		if s == "" {
			return
		}
		classes = append(classes, s)
		pool = append(pool, s...)
	}
	if asBool(opts["lower"], true) {
		addClass(pwLower)
	}
	if asBool(opts["upper"], true) {
		addClass(pwUpper)
	}
	if asBool(opts["digits"], true) {
		addClass(pwDigits)
	}
	if asBool(opts["symbols"], true) {
		addClass(pwSymbols)
	}
	if len(classes) == 0 {
		addClass(pwLower)
	}

	exclude := optString(opts, "exclude")
	if allow := optString(opts, "allow"); allow != "" {
		classes = []string{allow}
		pool = []byte(allow)
	}

	filter := func(s string) string {
		if exclude == "" {
			return s
		}
		var b strings.Builder
		for i := 0; i < len(s); i++ {
			if strings.IndexByte(exclude, s[i]) < 0 {
				b.WriteByte(s[i])
			}
		}
		return b.String()
	}
	pool = []byte(filter(string(pool)))

	out := make([]byte, length)
	used := make([]bool, length)
	idx := 0
	for _, cls := range classes {
		if idx >= length {
			break
		}
		cc := filter(cls)
		if cc == "" {
			continue
		}
		out[idx] = cc[rng.IntN(len(cc))]
		used[idx] = true
		idx++
	}
	for i := 0; i < length; i++ {
		if used[i] {
			continue
		}
		if len(pool) == 0 {
			out[i] = 'x'
			continue
		}
		out[i] = pool[rng.IntN(len(pool))]
	}
	rng.Shuffle(length, func(i, j int) { out[i], out[j] = out[j], out[i] })
	return string(out)
}

func randDigits(rng *rand.Rand, n int) []int {
	d := make([]int, n)
	for i := range d {
		d[i] = rng.IntN(10)
	}
	return d
}

func randomCPF(rng *rand.Rand) string {
	d := randDigits(rng, 9)
	d[0] = 1 + rng.IntN(9)
	s1 := 0
	for i, x := range d {
		s1 += x * (10 - i)
	}
	d1 := (s1 * 10) % 11
	if d1 == 10 {
		d1 = 0
	}
	d = append(d, d1)
	s2 := 0
	for i, x := range d {
		s2 += x * (11 - i)
	}
	d2 := (s2 * 10) % 11
	if d2 == 10 {
		d2 = 0
	}
	d = append(d, d2)
	return fmt.Sprintf("%d%d%d.%d%d%d.%d%d%d-%d%d", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7], d[8], d[9], d[10])
}

func randomCNPJ(rng *rand.Rand) string {
	d := randDigits(rng, 12)
	d[0] = 1 + rng.IntN(9)
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	s1 := 0
	for i, x := range d {
		s1 += x * w1[i]
	}
	r1 := s1 % 11
	d1 := 0
	if r1 >= 2 {
		d1 = 11 - r1
	}
	d = append(d, d1)
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	s2 := 0
	for i, x := range d {
		s2 += x * w2[i]
	}
	r2 := s2 % 11
	d2 := 0
	if r2 >= 2 {
		d2 = 11 - r2
	}
	d = append(d, d2)
	return fmt.Sprintf("%d%d.%d%d%d.%d%d%d/%d%d%d%d-%d%d", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7], d[8], d[9], d[10], d[11], d[12], d[13])
}

func randomCEP(rng *rand.Rand) string {
	d := randDigits(rng, 8)
	d[0] = 1 + rng.IntN(9)
	return fmt.Sprintf("%d%d%d%d%d-%d%d%d", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7])
}

func randomRG(rng *rand.Rand) string {
	d := randDigits(rng, 8)
	d[0] = 1 + rng.IntN(9)
	s := 0
	for i, x := range d {
		s += x * (2 + i)
	}
	dv := s % 11
	if dv == 10 {
		dv = 0
	}
	return fmt.Sprintf("%d%d%d%d%d%d%d%d-%d", d[0], d[1], d[2], d[3], d[4], d[5], d[6], d[7], dv)
}


