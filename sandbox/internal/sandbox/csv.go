package sandbox

import (
	"encoding/csv"
	"fmt"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func buildCSV(L *lua.State) int {
	t := newTable(L)

	setGoFunc(L, t, "parse", func(l *lua.State) int {
		s := argString(l, 1)
		sep := csvSep(l, 2)
		r := csv.NewReader(strings.NewReader(s))
		r.Comma = sep
		recs, err := r.ReadAll()
		if err != nil {
			panic(fmt.Errorf("CSV inválido: %w", err))
		}
		rows := make([]any, len(recs))
		for i, rec := range recs {
			cells := make([]string, len(rec))
			copy(cells, rec)
			rows[i] = cells
		}
		pushAny(l, rows)
		return 1
	})

	setGoFunc(L, t, "stringify", func(l *lua.State) int {
		rows := luaArrayAny(l, 1)
		sep := csvSep(l, 2)
		var b strings.Builder
		w := csv.NewWriter(&b)
		w.Comma = sep
		for _, row := range rows {
			cells, ok := row.([]any)
			if !ok {
				cells = []any{row}
			}
			rec := make([]string, len(cells))
			for i, c := range cells {
				rec[i] = fmt.Sprint(c)
			}
			if err := w.Write(rec); err != nil {
				panic(fmt.Errorf("CSV inválido: %w", err))
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			panic(fmt.Errorf("CSV inválido: %w", err))
		}
		l.PushString(b.String())
		return 1
	})

	return t
}

func csvSep(l *lua.State, i int) rune {
	s := argString(l, i)
	if s == "" {
		return ','
	}
	runes := []rune(s)
	if len(runes) != 1 {
		panic(fmt.Errorf("separador do CSV deve ser um único caractere"))
	}
	return runes[0]
}
