package sandbox

import (
	"cmp"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	lua "github.com/Shopify/go-lua"
	xhtml "golang.org/x/net/html"
	atom "golang.org/x/net/html/atom"
)

func buildData(L *lua.State, reg *sqlRegistry, mnt, tmp *Store) int {
	t := newTable(L)

	setGoFunc(L, t, "from", func(l *lua.State) int {
		f := strings.ToLower(strings.TrimSpace(argString(l, 1)))
		text := argString(l, 2)
		extra := ""
		if l.Top() >= 3 {
			extra = argString(l, 3)
		}
		switch f {
		case "csv":
			rows, err := dataRowsFromCSV(text, extra)
			if err != nil {
				panic(err)
			}
			pushAny(l, rows)
		case "json":
			v, err := dataFromJSON(text)
			if err != nil {
				panic(err)
			}
			pushAny(l, v)
		case "jsonl", "ndjson":
			rows, err := dataRowsFromJSONL(text)
			if err != nil {
				panic(err)
			}
			pushAny(l, rows)
		case "xml":
			rows, err := dataRowsFromXML(text, extra)
			if err != nil {
				panic(err)
			}
			pushAny(l, rows)
		case "sql":
			rows, err := dataRowsFromSQL(text)
			if err != nil {
				panic(err)
			}
			pushAny(l, rows)
		case "excel", "xlsx", "xls":
			path, err := excelPath(mnt, tmp, text)
			if err != nil {
				panic(err)
			}
			rows, err := dataRowsFromExcel(path, extra)
			if err != nil {
				panic(err)
			}
			pushAny(l, rows)
		case "html":
			rows, err := dataRowsFromHTML(text)
			if err != nil {
				panic(err)
			}
			pushAny(l, rows)
		default:
			panic(fmt.Errorf("formato não suportado em data.from: %s", f))
		}
		return 1
	})
	setGoFunc(L, t, "to", func(l *lua.State) int {
		f := strings.ToLower(strings.TrimSpace(argString(l, 1)))
		value := luaToAny(l, 2)
		switch f {
		case "csv":
			s, err := dataRowsToCSV(luaArrayAny(l, 2), argString(l, 3))
			if err != nil {
				panic(err)
			}
			l.PushString(s)
		case "json":
			s, err := dataToJSON(value)
			if err != nil {
				panic(err)
			}
			l.PushString(s)
		case "jsonl", "ndjson":
			s, err := dataRowsToJSONL(luaArrayAny(l, 2))
			if err != nil {
				panic(err)
			}
			l.PushString(s)
		case "xml":
			root := ""
			row := ""
			if l.Top() >= 3 {
				root = argString(l, 3)
			}
			if l.Top() >= 4 {
				row = argString(l, 4)
			}
			l.PushString(dataRowsToXML(luaArrayAny(l, 2), root, row))
		case "sql":
			l.PushString(dataRowsToSQL(luaArrayAny(l, 2), argString(l, 3)))
		case "excel", "xlsx", "xls":
			rows := luaArrayAny(l, 2)
			path, err := excelPath(mnt, tmp, argString(l, 3))
			if err != nil {
				panic(err)
			}
			if err := dataRowsToExcel(rows, path, argString(l, 4), toAnyMap(l, 5)); err != nil {
				panic(err)
			}
			l.PushBoolean(true)
		case "html":
			l.PushString(dataRowsToHTML(luaArrayAny(l, 2), toAnyMap(l, 3)))
		default:
			panic(fmt.Errorf("formato não suportado em data.to: %s", f))
		}
		return 1
	})

	setGoFunc(L, t, "convert", func(l *lua.State) int {
		if err := dataConvert(mnt, tmp, argString(l, 1), argString(l, 2), toAnyMap(l, 3)); err != nil {
			panic(fmt.Errorf("convert: %w", err))
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
			m[col] = recCell(rec, i)
		}
		out = append(out, m)
	}
	return out, nil
}

func recCell(rec []string, i int) string {
	if i >= len(rec) {
		return ""
	}
	return rec[i]
}

func dataRowsToCSV(rows []any, sep string) (string, error) {
	keys := dataRowKeys(rows)
	b := strings.Builder{}
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
	v := any(nil)
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
	out := []any{}
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		v := any(nil)
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			return nil, fmt.Errorf("linha JSONL inválida: %w", err)
		}
		out = append(out, v)
	}
	return out, nil
}

func dataRowsToJSONL(rows []any) (string, error) {
	b := strings.Builder{}
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
	if rowName == "" {
		if len(root.children) > 0 {
			rowName = root.children[0].name
		}
	}
	if rowName == "" {
		return []any{}, nil
	}
	out := []any{}
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
	return out, nil
}

func dataRowsToXML(rows []any, root, row string) string {
	root = cmp.Or(root, "root")
	row = cmp.Or(row, "item")
	b := strings.Builder{}
	xmlTag(&b, root, false)
	for _, r := range rows {
		m, _ := r.(map[string]any)
		xmlTag(&b, row, false)
		for _, k := range dataRowKeys([]any{r}) {
			xmlTag(&b, k, false)
			if v := m[k]; v != nil {
				if v != "" {
					xmlEscape(&b, dataCellText(v))
				}
			}
			xmlTag(&b, k, true)
		}
		xmlTag(&b, row, true)
	}
	xmlTag(&b, root, true)
	return b.String()
}

func xmlTag(b *strings.Builder, name string, close bool) {
	if close {
		fmt.Fprintf(b, "</%s>", name)
		return
	}
	fmt.Fprintf(b, "<%s>", name)
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
			m[col] = recCell(rec, i)
		}
		out = append(out, m)
	}
	return out, nil
}

func dataRowsToExcel(rows []any, path, sheet string, opts map[string]any) error {
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
	return excelWriteRows(path, sheet, grid, opts)
}

func dataSQLImport(db *sql.DB, table string, rows []any, opts map[string]any) (int, error) {
	keys := dataRowKeys(rows)
	if len(keys) == 0 {
		return 0, fmt.Errorf("sem colunas para importar")
	}
	if dataWantsCreate(opts) {
		return 0, sqlCreateTable(db, table, keys)
	}
	if !sqlTableExists(db, table) {
		if err := sqlCreateTable(db, table, keys); err != nil {
			return 0, err
		}
	}
	quoted := make([]string, len(keys))
	for i, k := range keys {
		quoted[i] = sqlQuote(k)
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(keys)), ",")
	ins := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", sqlQuote(table), strings.Join(quoted, ", "), placeholders)
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
			if rerr := tx.Rollback(); rerr != nil {
				return n, sqlErr(rerr)
			}
			return n, sqlErr(err)
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return n, sqlErr(err)
	}
	return n, nil
}

func dataWantsCreate(opts map[string]any) bool {
	if opts["create"] == true {
		return true
	}
	return opts["create"] == "true"
}

func sqlCreateTable(db *sql.DB, table string, keys []string) error {
	cols := make([]string, len(keys))
	for i, k := range keys {
		cols[i] = fmt.Sprintf("%s TEXT", sqlQuote(k))
	}
	q := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", sqlQuote(table), strings.Join(cols, ", "))
	_, err := db.Exec(q)
	if err != nil {
		return sqlErr(err)
	}
	return nil
}

func sqlTableExists(db *sql.DB, table string) bool {
	name := ""
	err := db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
			AND name = ?
	`, table).Scan(&name)
	return err == nil
}

func dataSQLExport(conn sqlConn, query string) ([]any, error) {
	return sqlQueryRows(conn, query, nil)
}

func dataConvert(mnt, tmp *Store, src, dst string, opts map[string]any) error {
	if strings.TrimSpace(src) == "" {
		return fmt.Errorf("informe origem e destino")
	}
	if strings.TrimSpace(dst) == "" {
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

	value := any(nil)
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
	case "html":
		content, err := os.ReadFile(fullSrc)
		if err != nil {
			return err
		}
		value, err = dataRowsFromHTML(string(content))
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
		return dataRowsToExcel(rows, fullDst, "", opts)
	case "html":
		rows, ok := value.([]any)
		if !ok {
			return fmt.Errorf("origem não é um conjunto de linhas")
		}
		return osWriteFile(fullDst, dataRowsToHTML(rows, opts))
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
			continue
		}
		if !strings.HasPrefix(up, "INSERT INTO") {
			continue
		}
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
				continue
			}
			m[c] = nil
		}
		out = append(out, m)
	}
	if out == nil {
		out = []any{}
	}
	return out, nil
}

func sqlTableColumns(line string) []string {
	i := strings.Index(line, "(")
	if i < 0 {
		return nil
	}
	j := strings.LastIndex(line, ")")
	if j < 0 {
		return nil
	}
	if j <= i {
		return nil
	}
	cols := []string{}
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
	if open < 0 {
		return nil, errors.New("sem tupla VALUES")
	}
	close := strings.LastIndex(rest, ")")
	if close < 0 {
		return nil, errors.New("sem tupla VALUES")
	}
	if close < open {
		return nil, errors.New("sem tupla VALUES")
	}
	return parseSQLTuple(rest[open+1 : close])
}

func parseSQLTuple(s string) ([]any, error) {
	out := []any{}
	i := 0
	for i < len(s) {
		for i < len(s) {
			if !isTupleSpace(s[i]) {
				break
			}
			i++
		}
		if i >= len(s) {
			break
		}
		val := any(nil)
		if s[i] == '\'' {
			i++
			b := strings.Builder{}
			closed := false
			for i < len(s) {
				if s[i] == '\'' {
					if i+1 < len(s) {
						if s[i+1] == '\'' {
							b.WriteByte('\'')
							i += 2
							continue
						}
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
		}
		if s[i] != '\'' {
			j := i
			for j < len(s) {
				if isTupleStop(s[j]) {
					break
				}
				j++
			}
			token := strings.TrimSpace(s[i:j])
			if strings.EqualFold(token, "NULL") {
				val = nil
			}
			if !strings.EqualFold(token, "NULL") {
				val = token
			}
			i = j
		}
		out = append(out, val)
		for i < len(s) {
			if !isTupleSpace(s[i]) {
				break
			}
			i++
		}
		if i < len(s) {
			if s[i] == ',' {
				i++
				continue
			}
		}
		break
	}
	return out, nil
}

func isTupleStop(c byte) bool {
	if c == ',' {
		return true
	}
	return c == ')'
}

func isTupleSpace(c byte) bool {
	if c == ' ' {
		return true
	}
	return c == '\t'
}

func dataRowsFromHTML(s string) ([]any, error) {
	doc, err := xhtml.Parse(strings.NewReader(s))
	if err != nil {
		return nil, err
	}
	table := findFirstTable(doc)
	if table == nil {
		return nil, fmt.Errorf("nenhuma tabela encontrada no HTML")
	}
	rows := tableRows(table)
	if len(rows) == 0 {
		return []any{}, nil
	}
	header := rows[0]
	out := make([]any, 0, len(rows)-1)
	for _, rec := range rows[1:] {
		m := map[string]any{}
		for i, v := range rec {
			key := fmt.Sprintf("col%d", i)
			if i < len(header) {
				if h := strings.TrimSpace(header[i]); h != "" {
					key = h
				}
			}
			m[key] = v
		}
		out = append(out, m)
	}
	return out, nil
}

func dataRowsToHTML(rows []any, opts map[string]any) string {
	keys := dataRowKeys(rows)
	if len(keys) == 0 {
		return ""
	}
	headerColor := optColor(opts, "header", "headerColor")
	rowColor := optColor(opts, "row", "rowColor")
	altColor := optColor(opts, "alt", "altRowColor")
	zebra := truthyOpt(opts, "zebra")
	thStyle := "padding:4px 8px"
	if headerColor != "" {
		thStyle = fmt.Sprintf("%s;background:%s", thStyle, headerColor)
	}
	tdStyle := "padding:4px 8px"
	if rowColor != "" {
		tdStyle = fmt.Sprintf("%s;background:%s", tdStyle, rowColor)
	}
	if altColor == "" {
		if zebra {
			altColor = "#f5f5f5"
		}
	}
	b := strings.Builder{}
	b.WriteString("<table border=\"1\" style=\"border-collapse:collapse\">\n<thead>\n<tr>")
	for _, k := range keys {
		b.WriteString("<th style=\"")
		b.WriteString(thStyle)
		b.WriteString("\">")
		b.WriteString(stdhtml.EscapeString(k))
		b.WriteString("</th>")
	}
	b.WriteString("</tr>\n</thead>\n<tbody>\n")
	for ri, r := range rows {
		m, _ := r.(map[string]any)
		cellStyle := tdStyle
		if ri%2 == 1 {
			if zebra {
				cellStyle = fmt.Sprintf("%s;background:%s", cellStyle, altColor)
			}
		}
		b.WriteString("<tr>")
		for _, k := range keys {
			b.WriteString("<td style=\"")
			b.WriteString(cellStyle)
			b.WriteString("\">")
			b.WriteString(stdhtml.EscapeString(dataCellText(m[k])))
			b.WriteString("</td>")
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table>\n")
	return b.String()
}

func optColor(opts map[string]any, keys ...string) string {
	for _, k := range keys {
		s, ok := opts[k].(string)
		if !ok {
			continue
		}
		if s == "" {
			continue
		}
		return s
	}
	return ""
}

func truthyOpt(opts map[string]any, key string) bool {
	switch v := opts[key].(type) {
	case bool:
		return v
	case string:
		if v == "true" {
			return true
		}
		return v == "1"
	}
	return false
}

func findFirstTable(n *xhtml.Node) *xhtml.Node {
	if n.Type == xhtml.ElementNode {
		if n.DataAtom == atom.Table {
			return n
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := findFirstTable(c); t != nil {
			return t
		}
	}
	return nil
}

func tableRows(table *xhtml.Node) [][]string {
	rows := [][]string{}
	for c := table.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != xhtml.ElementNode {
			continue
		}
		if c.DataAtom == atom.Tr {
			rows = append(rows, rowCells(c))
		}
		if isRowGroup(c.DataAtom) {
			for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
				if cc.Type == xhtml.ElementNode {
					if cc.DataAtom == atom.Tr {
						rows = append(rows, rowCells(cc))
					}
				}
			}
		}
	}
	return rows
}

func isRowGroup(a atom.Atom) bool {
	if a == atom.Tbody {
		return true
	}
	if a == atom.Thead {
		return true
	}
	return a == atom.Tfoot
}

func rowCells(tr *xhtml.Node) []string {
	cells := []string{}
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == xhtml.ElementNode {
			if isTableCell(c.DataAtom) {
				cells = append(cells, strings.TrimSpace(nodeText(c)))
			}
		}
	}
	return cells
}

func isTableCell(a atom.Atom) bool {
	if a == atom.Td {
		return true
	}
	return a == atom.Th
}

func nodeText(root *xhtml.Node) string {
	t := htmlText{}
	t.walk(root)
	return t.b.String()
}

type htmlText struct {
	b strings.Builder
}

func (t *htmlText) walk(n *xhtml.Node) {
	if n.Type == xhtml.TextNode {
		t.b.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		t.walk(c)
	}
}
