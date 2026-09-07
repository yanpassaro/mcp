package sandbox

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	lua "github.com/Shopify/go-lua"
)

var (
	fakeFirst = []string{"Ava", "Noah", "Liam", "Maya", "Elena", "Theo", "Isla", "Hugo", "Nora", "Finn", "Lena", "Omar", "Chloe", "Zoe", "Kai"}
	fakeLast  = []string{"Silva", "Santos", "Oliveira", "Souza", "Costa", "Pereira", "Almeida", "Ferreira", "Rodrigues", "Gomes", "Martins"}
	fakeWords = []string{"lua", "sandbox", "dados", "planilha", "sqlite", "script", "api", "token", "linha", "coluna", "tabela", "arquivo", "rede", "cache", "funcao", "vetor", "mapa", "chave", "valor", "tempo"}
)

func buildFake(L *lua.State) int {
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
	setGoFunc(L, t, "firstName", func(l *lua.State) int {
		l.PushString(fakeFirst[rng.IntN(len(fakeFirst))])
		return 1
	})
	setGoFunc(L, t, "lastName", func(l *lua.State) int {
		l.PushString(fakeLast[rng.IntN(len(fakeLast))])
		return 1
	})
	setGoFunc(L, t, "name", func(l *lua.State) int {
		l.PushString(fakeFirst[rng.IntN(len(fakeFirst))] + " " + fakeLast[rng.IntN(len(fakeLast))])
		return 1
	})
	setGoFunc(L, t, "email", func(l *lua.State) int {
		f := strings.ToLower(fakeFirst[rng.IntN(len(fakeFirst))])
		m := strings.ToLower(fakeLast[rng.IntN(len(fakeLast))])
		domains := []string{"example.com", "mail.test", "acme.dev"}
		l.PushString(f + "." + m + "@" + domains[rng.IntN(len(domains))])
		return 1
	})
	setGoFunc(L, t, "username", func(l *lua.State) int {
		f := strings.ToLower(fakeFirst[rng.IntN(len(fakeFirst))])
		m := strings.ToLower(fakeLast[rng.IntN(len(fakeLast))])
		l.PushString(f + m + fmt.Sprint(10+rng.IntN(90)))
		return 1
	})
	setGoFunc(L, t, "phone", func(l *lua.State) int {
		l.PushString(fmt.Sprintf("+55 (%d%d) %04d-%04d", 1+rng.IntN(9), rng.IntN(10), rng.IntN(10000), rng.IntN(10000)))
		return 1
	})
	setGoFunc(L, t, "int", func(l *lua.State) int {
		min, max := int(argNum(l, 1)), int(argNum(l, 2))
		if max < min {
			min, max = max, min
		}
		if min == max {
			l.PushInteger(min)
		} else {
			l.PushInteger(min + rng.IntN(max-min+1))
		}
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
	setGoFunc(L, t, "uuid", func(l *lua.State) int {
		l.PushString(uuidV4())
		return 1
	})
	setGoFunc(L, t, "date", func(l *lua.State) int {
		from := time.Now().AddDate(0, 0, -365)
		to := time.Now()
		if s := strings.TrimSpace(argString(l, 1)); s != "" {
			if ts, err := parseTimeStr(s); err == nil {
				from = ts
			}
		}
		if s := strings.TrimSpace(argString(l, 2)); s != "" {
			if ts, err := parseTimeStr(s); err == nil {
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
	setGoFunc(L, t, "words", func(l *lua.State) int {
		n := int(argNum(l, 2))
		l.PushString(joinFakeWords(rng, fakeWords, n))
		return 1
	})
	setGoFunc(L, t, "sentence", func(l *lua.State) int {
		n := int(argNum(l, 2))
		s := joinFakeWords(rng, fakeWords, n)
		if s != "" {
			s = strings.ToUpper(s[:1]) + s[1:] + "."
		}
		l.PushString(s)
		return 1
	})
	setGoFunc(L, t, "paragraph", func(l *lua.State) int {
		n := int(argNum(l, 2))
		parts := make([]string, n)
		for i := range parts {
			parts[i] = strings.ToUpper(joinFakeWords(rng, fakeWords, 6)[:1]) + joinFakeWords(rng, fakeWords, 6)[1:] + "."
		}
		l.PushString(strings.Join(parts, " "))
		return 1
	})

	return t
}

func joinFakeWords(rng *rand.Rand, list []string, n int) string {
	if n <= 0 {
		n = 3
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = list[rng.IntN(len(list))]
	}
	return strings.Join(parts, " ")
}
