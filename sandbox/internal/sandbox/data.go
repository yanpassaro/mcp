package sandbox

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	setGoFunc(L, t, "toJSONL", func(l *lua.State) int {
		s, err := dataRowsToJSONL(luaArrayAny(l, 1))
		if err != nil {
			panic(err)
		}
		l.PushString(s)
		return 1
	})
	setGoFunc(L, t, "fromJSONL", func(l *lua.State) int {
		rows, err := dataRowsFromJSONL(argString(l, 1))
		if err != nil {
			panic(err)
		}
		pushAny(l, rows)
		return 1
	})
	setGoFunc(L, t, "toXML", func(l *lua.State) int {
		l.PushString(dataRowsToXML(luaArrayAny(l, 1), argString(l, 2), argString(l, 3)))
		return 1
	})
	setGoFunc(L, t, "fromXML", func(l *lua.State) int {
		rows, err := dataRowsFromXML(argString(l, 1), argString(l, 2))
		if err != nil {
			panic(err)
		}
		pushAny(l, rows)
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
		rows := luaArrayAny(l, 1)
		path, err := excelPath(mnt, tmp, argString(l, 2))
		if err != nil {
			panic(err)
		}
		if err := dataRowsToExcel(rows, path, argString(l, 3)); err != nil {
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
			panic(fmt.Errorf("convert: %w", err))
		}
		l.PushBoolean(true)
		return 1
	})
	setGoFunc(L, t, "toSql", func(l *lua.State) int {
		l.PushString(dataRowsToSQL(luaArrayAny(l, 1), argString(l, 2)))
		return 1
	})
	setGoFunc(L, t, "fromSql", func(l *lua.State) int {
		rows, err := dataRowsFromSQL(argString(l, 1))
		if err != nil {
			panic(err)
		}
		pushAny(l, rows)
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

func dataRowsFromJSONL(s string) ([]any, error) {
	var out []any
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			return nil, fmt.Errorf("linha JSONL inválida: %w", err)
		}
		out = append(out, v)
	}
	if out == nil {
		out = []any{}
	}
	return out, nil
}

func dataRowsToJSONL(rows []any) (string, error) {
	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		line, err := json.Marshal(r)
		if err != nil {
			return "", err
		}
		b.Write(line)
	}
	return b.String(), nil
}

func dataRowsFromXML(s, row string) ([]any, error) {
	root, err := parseXML(s)
	if err != nil {
		return nil, err
	}
	rowName := strings.TrimSpace(row)
	if rowName == "" && len(root.children) > 0 {
		rowName = root.children[0].name
	}
	if rowName == "" {
		return []any{}, nil
	}
	var out []any
	for _, child := range root.children {
		if child.name != rowName {
			continue
		}
		m := map[string]any{}
		for _, leaf := range child.children {
			if leaf.name != "" {
				m[leaf.name] = strings.TrimSpace(leaf.chars.String())
			}
		}
		if len(child.children) == 0 {
			if txt := strings.TrimSpace(child.chars.String()); txt != "" {
				m[rowName] = txt
			}
		}
		out = append(out, m)
	}
	if out == nil {
		out = []any{}
	}
	return out, nil
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

	fullSrc, err := dataPath(mnt, tmp, src)
	if err != nil {
		return err
	}
	fullDst, err := dataPath(mnt, tmp, dst)
	if err != nil {
		return err
	}

	var value any
	switch extSrc {
	case "csv":
		content, err := os.ReadFile(fullSrc)
		if err != nil {
			return err
		}
		value, err = dataRowsFromCSV(string(content), "")
		if err != nil {
			return err
		}
	case "json":
		content, err := os.ReadFile(fullSrc)
		if err != nil {
			return err
		}
		value, err = dataFromJSON(string(content))
		if err != nil {
			return err
		}
	case "jsonl", "ndjson":
		content, err := os.ReadFile(fullSrc)
		if err != nil {
			return err
		}
		value, err = dataRowsFromJSONL(string(content))
		if err != nil {
			return err
		}
	case "sql":
		content, err := os.ReadFile(fullSrc)
		if err != nil {
			return err
		}
		value, err = dataRowsFromSQL(string(content))
		if err != nil {
			return err
		}
	case "xml":
		content, err := os.ReadFile(fullSrc)
		if err != nil {
			return err
		}
		value, err = dataRowsFromXML(string(content), "")
		if err != nil {
			return err
		}
	case "xlsx", "xls":
		value, err = dataRowsFromExcel(fullSrc, "")
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
		return osWriteFile(fullDst, content)
	case "json":
		content, err := dataToJSON(value)
		if err != nil {
			return err
		}
		return osWriteFile(fullDst, content)
	case "jsonl", "ndjson":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("origem não é um conjunto de linhas")
		}
		content, err := dataRowsToJSONL(rows)
		if err != nil {
			return err
		}
		return osWriteFile(fullDst, content)
	case "xml":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("origem não é um conjunto de linhas")
		}
		return osWriteFile(fullDst, dataRowsToXML(rows, "", ""))
	case "sql":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("origem não é um conjunto de linhas")
		}
		return osWriteFile(fullDst, dataRowsToSQL(rows, ""))
	case "xlsx":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("origem não é um conjunto de linhas")
		}
		return dataRowsToExcel(rows, fullDst, "")
	default:
		return fmt.Errorf("formato de destino não suportado: %s", extDst)
	}
}

func osWriteFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func dataPath(mnt, tmp *Store, p string) (string, error) {
	if strings.HasPrefix(p, "tmp:") {
		return tmp.resolve(strings.TrimPrefix(p, "tmp:"))
	}
	return mnt.resolve(p)
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

func dataRowsToSQL(rows []any, table string) string {
	table = strings.TrimSpace(table)
	if table == "" {
		table = "dados"
	}
	keys := dataRowKeys(rows)
	if len(keys) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("CREATE TABLE IF NOT EXISTS ")
	b.WriteString(sqlQuote(table))
	b.WriteString(" (")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(sqlQuote(k))
		b.WriteString(" TEXT")
	}
	b.WriteString(");\n")
	for _, r := range rows {
		m, _ := r.(map[string]any)
		b.WriteString("INSERT INTO ")
		b.WriteString(sqlQuote(table))
		b.WriteString(" (")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(sqlQuote(k))
		}
		b.WriteString(") VALUES (")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(sqlValLiteral(m[k]))
		}
		b.WriteString(");\n")
	}
	return b.String()
}

func sqlValLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	s := dataCellText(v)
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

var reSQLCol = regexp.MustCompile(`"([^"]+)"`)

func dataRowsFromSQL(s string) ([]any, error) {
	var cols []string
	var out []any
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, ";"))
		if line == "" {
			continue
		}
		up := strings.ToUpper(line)
		if strings.HasPrefix(up, "CREATE TABLE") {
			cols = sqlTableColumns(line)
		} else if strings.HasPrefix(up, "INSERT INTO") {
			if len(cols) == 0 {
				return nil, errors.New("INSERT sem CREATE TABLE anterior")
			}
			values, err := parseSQLValues(line)
			if err != nil {
				return nil, err
			}
			m := map[string]any{}
			for i, c := range cols {
				if i < len(values) {
					m[c] = values[i]
				} else {
					m[c] = nil
				}
			}
			out = append(out, m)
		}
	}
	if out == nil {
		out = []any{}
	}
	return out, nil
}

func sqlTableColumns(line string) []string {
	i := strings.Index(line, "(")
	j := strings.LastIndex(line, ")")
	if i < 0 || j < 0 || j <= i {
		return nil
	}
	var cols []string
	for _, part := range strings.Split(line[i+1:j], ",") {
		if m := reSQLCol.FindStringSubmatch(part); len(m) == 2 {
			cols = append(cols, m[1])
		}
	}
	return cols
}

func parseSQLValues(line string) ([]any, error) {
	v := strings.Index(strings.ToUpper(line), "VALUES")
	if v < 0 {
		return nil, nil
	}
	rest := line[v+len("VALUES"):]
	open := strings.Index(rest, "(")
	close := strings.LastIndex(rest, ")")
	if open < 0 || close < 0 || close < open {
		return nil, errors.New("sem tupla VALUES")
	}
	return parseSQLTuple(rest[open+1 : close])
}

func parseSQLTuple(s string) ([]any, error) {
	var out []any
	i := 0
	for {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}
		var val any
		if s[i] == '\'' {
			i++
			var b strings.Builder
			closed := false
			for i < len(s) {
				if s[i] == '\'' {
					if i+1 < len(s) && s[i+1] == '\'' {
						b.WriteByte('\'')
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				b.WriteByte(s[i])
				i++
			}
			if !closed {
				return nil, errors.New("literal não fechado nas VALUES")
			}
			val = b.String()
		} else {
			j := i
			for j < len(s) && s[j] != ',' && s[j] != ')' {
				j++
			}
			token := strings.TrimSpace(s[i:j])
			if strings.EqualFold(token, "NULL") {
				val = nil
			} else {
				val = token
			}
			i = j
		}
		out = append(out, val)
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i < len(s) && s[i] == ',' {
			i++
			continue
		}
		break
	}
	return out, nil
}
