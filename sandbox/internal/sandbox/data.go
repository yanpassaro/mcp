package sandbox

import (
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
	"strconv"
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

func dataConvert(mnt, tmp *Store, src, dst string, opts map[string]any) error {
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
			key := "col" + strconv.Itoa(i)
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
		thStyle += ";background:" + headerColor
	}
	tdStyle := "padding:4px 8px"
	if rowColor != "" {
		tdStyle += ";background:" + rowColor
	}
	if zebra && altColor == "" {
		altColor = "#f5f5f5"
	}
	var b strings.Builder
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
		if zebra && ri%2 == 1 {
			cellStyle += ";background:" + altColor
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
		if s, ok := opts[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func truthyOpt(opts map[string]any, key string) bool {
	switch v := opts[key].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	}
	return false
}

func findFirstTable(n *xhtml.Node) *xhtml.Node {
	if n.Type == xhtml.ElementNode && n.DataAtom == atom.Table {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := findFirstTable(c); t != nil {
			return t
		}
	}
	return nil
}

func tableRows(table *xhtml.Node) [][]string {
	var rows [][]string
	for c := table.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != xhtml.ElementNode {
			continue
		}
		if c.DataAtom == atom.Tr {
			rows = append(rows, rowCells(c))
		}
		if c.DataAtom == atom.Tbody || c.DataAtom == atom.Thead || c.DataAtom == atom.Tfoot {
			for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
				if cc.Type == xhtml.ElementNode && cc.DataAtom == atom.Tr {
					rows = append(rows, rowCells(cc))
				}
			}
		}
	}
	return rows
}

func rowCells(tr *xhtml.Node) []string {
	var cells []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == xhtml.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
			cells = append(cells, strings.TrimSpace(nodeText(c)))
		}
	}
	return cells
}

func nodeText(root *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return b.String()
}
