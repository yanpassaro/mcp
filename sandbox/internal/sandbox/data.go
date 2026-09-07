package sandbox

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	lua "github.com/Shopify/go-lua"
)

func buildData(L *lua.State, reg *sqlRegistry, mnt, tmp *Store) int {
	t := newTable(L)

	setGoFunc(L, t, "fromCSV", func(l *lua.State) int {
		rows, err := dataRowsFromCSV(argString(l, 1), argString(l, 2))
		if err != nil {
			panic(err)
		}
		pushAny(l, rows)
		return 1
	})
	setGoFunc(L, t, "toCSV", func(l *lua.State) int {
		s, err := dataRowsToCSV(luaArrayAny(l, 1), argString(l, 2))
		if err != nil {
			panic(err)
		}
		l.PushString(s)
		return 1
	})
	setGoFunc(L, t, "fromJSON", func(l *lua.State) int {
		v, err := dataFromJSON(argString(l, 1))
		if err != nil {
			panic(err)
		}
		pushAny(l, v)
		return 1
	})
	setGoFunc(L, t, "toJSON", func(l *lua.State) int {
		s, err := dataToJSON(luaToAny(l, 1))
		if err != nil {
			panic(err)
		}
		l.PushString(s)
		return 1
	})
	setGoFunc(L, t, "toXML", func(l *lua.State) int {
		l.PushString(dataRowsToXML(luaArrayAny(l, 1), argString(l, 2), argString(l, 3)))
		return 1
	})
	setGoFunc(L, t, "fromExcel", func(l *lua.State) int {
		path, err := excelPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		rows, err := dataRowsFromExcel(path, argString(l, 2))
		if err != nil {
			panic(err)
		}
		pushAny(l, rows)
		return 1
	})
	setGoFunc(L, t, "toExcel", func(l *lua.State) int {
		path, err := excelPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		if err := dataRowsToExcel(luaArrayAny(l, 2), path, argString(l, 3)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})
	setGoFunc(L, t, "sqlImport", func(l *lua.State) int {
		dbPath, err := sqlPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		db, err := reg.get(dbPath)
		if err != nil {
			panic(err)
		}
		n, err := dataSQLImport(db, argString(l, 2), luaArrayAny(l, 3), toAnyMap(l, 4))
		if err != nil {
			panic(err)
		}
		pushAny(l, map[string]any{"imported": n})
		return 1
	})
	setGoFunc(L, t, "sqlExport", func(l *lua.State) int {
		dbPath, err := sqlPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		conn, err := reg.conn(dbPath)
		if err != nil {
			panic(err)
		}
		rows, err := dataSQLExport(conn, argString(l, 2))
		if err != nil {
			panic(err)
		}
		pushAny(l, rows)
		return 1
	})
	setGoFunc(L, t, "convert", func(l *lua.State) int {
		if err := dataConvert(mnt, tmp, argString(l, 1), argString(l, 2)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})

	return t
}

func dataRowsFromCSV(s, sep string) ([]any, error) {
	r := csv.NewReader(strings.NewReader(s))
	if sep != "" {
		runes := []rune(sep)
		if len(runes) != 1 {
			return nil, fmt.Errorf("separador do CSV deve ser um único caractere")
		}
		r.Comma = runes[0]
	}
	recs, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV inválido: %w", err)
	}
	if len(recs) == 0 {
		return []any{}, nil
	}
	header := recs[0]
	out := make([]any, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		m := map[string]any{}
		for i, col := range header {
			if i < len(rec) {
				m[col] = rec[i]
			} else {
				m[col] = ""
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func dataRowsToCSV(rows []any, sep string) (string, error) {
	keys := dataRowKeys(rows)
	var b strings.Builder
	w := csv.NewWriter(&b)
	if sep != "" {
		runes := []rune(sep)
		if len(runes) != 1 {
			return "", fmt.Errorf("separador do CSV deve ser um único caractere")
		}
		w.Comma = runes[0]
	}
	if len(keys) > 0 {
		if err := w.Write(keys); err != nil {
			return "", err
		}
	}
	for _, r := range rows {
		m, _ := r.(map[string]any)
		line := make([]string, len(keys))
		for i, k := range keys {
			line[i] = dataCellText(m[k])
		}
		if err := w.Write(line); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func dataFromJSON(s string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("JSON inválido: %w", err)
	}
	return v, nil
}

func dataToJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("JSON inválido: %w", err)
	}
	return string(b), nil
}

func dataRowsToXML(rows []any, root, row string) string {
	if root == "" {
		root = "root"
	}
	if row == "" {
		row = "item"
	}
	var b strings.Builder
	b.WriteByte('<'); b.WriteString(root); b.WriteByte('>')
	for _, r := range rows {
		m, _ := r.(map[string]any)
		b.WriteByte('<'); b.WriteString(row); b.WriteByte('>')
		for _, k := range dataRowKeys([]any{r}) {
			b.WriteByte('<'); b.WriteString(k); b.WriteByte('>')
			if v := m[k]; v != nil && v != "" {
				xmlEscape(&b, dataCellText(v))
			}
			b.WriteString("</"); b.WriteString(k); b.WriteByte('>')
		}
		b.WriteString("</"); b.WriteString(row); b.WriteByte('>')
	}
	b.WriteString("</"); b.WriteString(root); b.WriteByte('>')
	return b.String()
}

func dataRowsFromExcel(path, sheet string) ([]any, error) {
	rows, err := excelReadRows(path, sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []any{}, nil
	}
	header := rows[0]
	out := make([]any, 0, len(rows)-1)
	for _, rec := range rows[1:] {
		m := map[string]any{}
		for i, col := range header {
			if i < len(rec) {
				m[col] = rec[i]
			} else {
				m[col] = ""
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func dataRowsToExcel(rows []any, path, sheet string) error {
	keys := dataRowKeys(rows)
	grid := make([][]string, 0, len(rows)+1)
	if len(keys) > 0 {
		grid = append(grid, keys)
	}
	for _, r := range rows {
		m, _ := r.(map[string]any)
		line := make([]string, len(keys))
		for i, k := range keys {
			line[i] = dataCellText(m[k])
		}
		grid = append(grid, line)
	}
	return excelWriteRows(path, sheet, grid)
}

func dataSQLImport(db *sql.DB, table string, rows []any, opts map[string]any) (int, error) {
	keys := dataRowKeys(rows)
	if len(keys) == 0 {
		return 0, fmt.Errorf("sem colunas para importar")
	}
	create := opts["create"] == true || opts["create"] == "true"
	if create || !sqlTableExists(db, table) {
		cols := make([]string, len(keys))
		for i, k := range keys {
			cols[i] = sqlQuote(k) + " TEXT"
		}
		if _, err := db.Exec("CREATE TABLE IF NOT EXISTS " + sqlQuote(table) + " (" + strings.Join(cols, ", ") + ")"); err != nil {
			return 0, sqlErr(err)
		}
	}
	quoted := make([]string, len(keys))
	for i, k := range keys {
		quoted[i] = sqlQuote(k)
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(keys)), ",")
	ins := "INSERT INTO " + sqlQuote(table) + " (" + strings.Join(quoted, ", ") + ") VALUES (" + placeholders + ")"
	tx, err := db.Begin()
	if err != nil {
		return 0, sqlErr(err)
	}
	n := 0
	for _, r := range rows {
		m, _ := r.(map[string]any)
		args := make([]any, len(keys))
		for i, k := range keys {
			args[i] = dataCellSQL(m[k])
		}
		if _, err := tx.Exec(ins, args...); err != nil {
			tx.Rollback()
			return n, sqlErr(err)
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return n, sqlErr(err)
	}
	return n, nil
}

func sqlTableExists(db *sql.DB, table string) bool {
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
	return err == nil
}

func dataSQLExport(conn sqlConn, query string) ([]any, error) {
	return sqlQueryRows(conn, query, nil)
}

func dataConvert(mnt, tmp *Store, src, dst string) error {
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return fmt.Errorf("informe origem e destino")
	}
	extSrc := strings.ToLower(strings.TrimPrefix(filepath.Ext(src), "."))
	extDst := strings.ToLower(strings.TrimPrefix(filepath.Ext(dst), "."))

	srcStore, srcName := dataStore(mnt, tmp, src)
	dstStore, dstName := dataStore(mnt, tmp, dst)

	var value any
	switch extSrc {
	case "csv":
		content, err := srcStore.Read(srcName)
		if err != nil {
			return err
		}
		value, err = dataRowsFromCSV(content, "")
		if err != nil {
			return err
		}
	case "json":
		content, err := srcStore.Read(srcName)
		if err != nil {
			return err
		}
		value, err = dataFromJSON(content)
		if err != nil {
			return err
		}
	case "xlsx", "xls":
		full, err := srcStore.resolve(srcName)
		if err != nil {
			return err
		}
		value, err = dataRowsFromExcel(full, "")
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("formato de origem não suportado: %s", extSrc)
	}

	switch extDst {
	case "csv":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("origem não é um conjunto de linhas")
		}
		content, err := dataRowsToCSV(rows, "")
		if err != nil {
			return err
		}
		_, err = dstStore.Write(dstName, content)
		return err
	case "json":
		content, err := dataToJSON(value)
		if err != nil {
			return err
		}
		_, err = dstStore.Write(dstName, content)
		return err
	case "xml":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("origem não é um conjunto de linhas")
		}
		_, err := dstStore.Write(dstName, dataRowsToXML(rows, "", ""))
		return err
	case "xlsx":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("origem não é um conjunto de linhas")
		}
		full, err := dstStore.resolve(dstName)
		if err != nil {
			return err
		}
		return dataRowsToExcel(rows, full, "")
	default:
		return fmt.Errorf("formato de destino não suportado: %s", extDst)
	}
}

func dataStore(mnt, tmp *Store, p string) (*Store, string) {
	if strings.HasPrefix(p, "tmp:") {
		return tmp, strings.TrimPrefix(p, "tmp:")
	}
	return mnt, p
}

func dataRowKeys(rows []any) []string {
	set := map[string]bool{}
	for _, r := range rows {
		if m, ok := r.(map[string]any); ok {
			for k := range m {
				set[k] = true
			}
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func dataCellText(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func dataCellSQL(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case map[string]any, []any:
		if b, err := json.Marshal(x); err == nil {
			return string(b)
		}
		return nil
	default:
		return x
	}
}
