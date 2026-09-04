package sqlize

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"golang.org/x/net/html"
)

func (s *store) importFile(ctx context.Context, path, table, sheet string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("caminho do arquivo é obrigatório")
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("arquivo não encontrado: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		cols, rows, err := parseJSON(path)
		if err != nil {
			return "", err
		}
		name := deriveTableName(table, path, "json")
		if err := s.loadTable(ctx, name, cols, rows); err != nil {
			return "", err
		}
		return fmt.Sprintf("Tabela %q criada a partir de %s: %d colunas, %d linhas.", name, filepath.Base(path), len(cols), len(rows)), nil
	case ".jsonl", ".ndjson":
		cols, rows, err := parseJSONL(path)
		if err != nil {
			return "", err
		}
		name := deriveTableName(table, path, "json")
		if err := s.loadTable(ctx, name, cols, rows); err != nil {
			return "", err
		}
		return fmt.Sprintf("Tabela %q criada a partir de %s: %d colunas, %d linhas.", name, filepath.Base(path), len(cols), len(rows)), nil
	case ".csv":
		cols, rows, err := parseDelimited(path, ',')
		if err != nil {
			return "", err
		}
		name := deriveTableName(table, path, "csv")
		if err := s.loadTable(ctx, name, cols, rows); err != nil {
			return "", err
		}
		return fmt.Sprintf("Tabela %q criada a partir de %s: %d colunas, %d linhas.", name, filepath.Base(path), len(cols), len(rows)), nil
	case ".tsv":
		cols, rows, err := parseDelimited(path, '\t')
		if err != nil {
			return "", err
		}
		name := deriveTableName(table, path, "tsv")
		if err := s.loadTable(ctx, name, cols, rows); err != nil {
			return "", err
		}
		return fmt.Sprintf("Tabela %q criada a partir de %s: %d colunas, %d linhas.", name, filepath.Base(path), len(cols), len(rows)), nil
	case ".xlsx", ".xlsm", ".xls":
		sheets, err := parseWorkbook(path, sheet)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(sheet) != "" && strings.TrimSpace(table) != "" {
			if len(sheets) != 1 {
				return "", fmt.Errorf("sheet + table só valem para uma aba; encontradas %d", len(sheets))
			}
			for name, d := range sheets {
				if err := s.loadTable(ctx, strings.TrimSpace(table), d.cols, d.rows); err != nil {
					return "", err
				}
				return fmt.Sprintf("Aba %q importada como tabela %q: %d colunas, %d linhas.", name, strings.TrimSpace(table), len(d.cols), len(d.rows)), nil
			}
		}
		created := make([]string, 0, len(sheets))
		for name, d := range sheets {
			if err := s.loadTable(ctx, name, d.cols, d.rows); err != nil {
				return "", err
			}
			created = append(created, fmt.Sprintf("%q (%d linhas)", name, len(d.rows)))
		}
		return fmt.Sprintf("Planilha %s importada. Tabelas: %s.", filepath.Base(path), strings.Join(created, ", ")), nil
	case ".sql":
		n, err := s.runSQLScript(ctx, path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d instrução(ções) executada(s) a partir de %s.", n, filepath.Base(path)), nil
	case ".sqlite", ".sqlite3", ".db":
		alias, err := s.attachDB(ctx, path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Banco %s anexado como esquema %q. Use sqlize_structure para listar as tabelas.", filepath.Base(path), alias), nil
	case ".xml":
		cols, rows, err := parseXML(path)
		if err != nil {
			return "", err
		}
		name := deriveTableName(table, path, "xml")
		if err := s.loadTable(ctx, name, cols, rows); err != nil {
			return "", err
		}
		return fmt.Sprintf("Tabela %q criada a partir de %s: %d colunas, %d linhas.", name, filepath.Base(path), len(cols), len(rows)), nil
	default:
		return "", fmt.Errorf("formato não suportado: %s (use .json, .jsonl, .ndjson, .csv, .tsv, .xlsx, .xlsm, .xls, .sql, .sqlite, .db, .xml)", ext)
	}
}

func deriveTableName(given, path, fallback string) string {
	if strings.TrimSpace(given) != "" {
		return strings.TrimSpace(given)
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.Join(strings.Fields(base), "_")
	if base == "" {
		base = fallback
	}
	return base
}

func parseDelimited(path string, comma rune) ([]string, [][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = comma
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	recs, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("ler %s: %w", filepath.Base(path), err)
	}
	if len(recs) == 0 {
		return nil, nil, fmt.Errorf("arquivo vazio: %s", filepath.Base(path))
	}
	cols := recs[0]
	for i := range cols {
		if strings.TrimSpace(cols[i]) == "" {
			cols[i] = fmt.Sprintf("col_%d", i+1)
		}
	}
	rows := make([][]string, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		rows = append(rows, rec)
	}
	return cols, rows, nil
}

func parseJSON(path string) ([]string, [][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, nil, fmt.Errorf("decodificar JSON: %w", err)
	}
	switch t := v.(type) {
	case []any:
		return rowsFromArray(t)
	case map[string]any:
		cols := make([]string, 0, len(t))
		for k := range t {
			cols = append(cols, k)
		}
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = cellToString(t[c])
		}
		return cols, [][]string{row}, nil
	default:
		return nil, nil, fmt.Errorf("JSON: formato não suportado (esperado array ou objeto)")
	}
}

func parseJSONL(path string) ([]string, [][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(string(data), "\n")
	items := make([]any, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, nil, fmt.Errorf("JSONL: linha %d inválida: %w", i+1, err)
		}
		items = append(items, v)
	}
	if len(items) == 0 {
		return nil, nil, fmt.Errorf("JSONL: arquivo vazio")
	}
	return rowsFromArray(items)
}

func rowsFromArray(t []any) ([]string, [][]string, error) {
	if len(t) == 0 {
		return nil, nil, fmt.Errorf("JSON: array vazio")
	}
	switch t[0].(type) {
	case map[string]any:
		cols := make([]string, 0)
		seen := map[string]bool{}
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				for k := range m {
					if !seen[k] {
						seen[k] = true
						cols = append(cols, k)
					}
				}
			}
		}
		rows := make([][]string, 0, len(t))
		for _, item := range t {
			row := make([]string, len(cols))
			if m, ok := item.(map[string]any); ok {
				for i, c := range cols {
					row[i] = cellToString(m[c])
				}
			}
			rows = append(rows, row)
		}
		return cols, rows, nil
	case []any:
		maxW := 0
		for _, item := range t {
			if arr, ok := item.([]any); ok && len(arr) > maxW {
				maxW = len(arr)
			}
		}
		cols := make([]string, maxW)
		for i := range cols {
			cols[i] = fmt.Sprintf("col_%d", i+1)
		}
		rows := make([][]string, 0, len(t))
		for _, item := range t {
			row := make([]string, maxW)
			if arr, ok := item.([]any); ok {
				for i, e := range arr {
					if i < maxW {
						row[i] = cellToString(e)
					}
				}
			}
			rows = append(rows, row)
		}
		return cols, rows, nil
	default:
		rows := make([][]string, 0, len(t))
		for _, item := range t {
			rows = append(rows, []string{cellToString(item)})
		}
		return []string{"value"}, rows, nil
	}
}

func cellToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		return t.String()
	case float64:
		if math.IsInf(t, 0) || math.IsNaN(t) {
			return ""
		}
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

type sheetData struct {
	cols []string
	rows [][]string
}

func parseExcel(path, only string) (map[string]sheetData, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("abrir Excel: %w", err)
	}
	defer f.Close()
	out := map[string]sheetData{}
	used := map[string]int{}
	names := f.GetSheetList()
	if strings.TrimSpace(only) != "" {
		found := ""
		for _, sh := range names {
			if strings.EqualFold(sh, only) {
				found = sh
				break
			}
		}
		if found == "" {
			return nil, fmt.Errorf("aba %q não encontrada no Excel; abas disponíveis: %s", only, strings.Join(names, ", "))
		}
		names = []string{found}
	}
	for _, sheet := range names {
		recs, err := f.GetRows(sheet)
		if err != nil || len(recs) == 0 {
			continue
		}
		name := sheetToTable(sheet, used)
		cols := recs[0]
		for i := range cols {
			if strings.TrimSpace(cols[i]) == "" {
				cols[i] = fmt.Sprintf("col_%d", i+1)
			}
		}
		rows := make([][]string, 0, len(recs)-1)
		for _, rec := range recs[1:] {
			rows = append(rows, rec)
		}
		out[name] = sheetData{cols: cols, rows: rows}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("planilha sem dados: %s", filepath.Base(path))
	}
	return out, nil
}

func parseWorkbook(path, only string) (map[string]sheetData, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".xlsx" || ext == ".xlsm" {
		return parseExcel(path, only)
	}
	head, err := readFileHead(path, 64*1024)
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(bytes.TrimSpace(head), []byte("PK")) {
		return parseExcel(path, only)
	}
	if looksLikeHTML(head) {
		return parseHTMLWorkbook(path)
	}
	return nil, fmt.Errorf("formato .xls não reconhecido (o binário legado BIFF não é suportado). Converta o arquivo para .xlsx antes de importar.")
}

func readFileHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	read, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf[:read], nil
}

func looksLikeHTML(head []byte) bool {
	lower := strings.ToLower(string(head))
	for _, m := range []string{"<html", "<table", "<!doctype html"} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func parseHTMLWorkbook(path string) (map[string]sheetData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("analisar HTML: %w", err)
	}
	var tables []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			tables = append(tables, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(tables) == 0 {
		return nil, fmt.Errorf("arquivo .xls (HTML) sem tabela <table>: %s", filepath.Base(path))
	}
	base := deriveTableName("", path, "xls")
	used := map[string]int{}
	out := map[string]sheetData{}
	for _, t := range tables {
		rows := extractHTMLRows(t)
		if len(rows) == 0 {
			continue
		}
		cols := rows[0]
		for i := range cols {
			if strings.TrimSpace(cols[i]) == "" {
				cols[i] = fmt.Sprintf("col_%d", i+1)
			}
		}
		name := sheetToTable(base, used)
		out[name] = sheetData{cols: cols, rows: rows[1:]}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("planilha .xls (HTML) sem dados: %s", filepath.Base(path))
	}
	return out, nil
}

func extractHTMLRows(table *html.Node) [][]string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cells = append(cells, strings.TrimSpace(htmlCellText(c)))
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}

func htmlCellText(n *html.Node) string {
	var b strings.Builder
	var collect func(*html.Node)
	collect = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return b.String()
}

func sheetToTable(sheet string, used map[string]int) string {
	base := strings.Join(strings.Fields(sheet), "_")
	base = strings.ReplaceAll(base, "/", "_")
	if base == "" {
		base = "sheet"
	}
	n := used[base]
	used[base] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s_%d", base, n)
}

func (s *store) runSQLScript(ctx context.Context, path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	stmts := splitSQL(string(data))
	count := 0
	for _, st := range stmts {
		st = strings.TrimSpace(st)
		if st == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, st); err != nil {
			return count, fmt.Errorf("executar SQL (instrução %d): %w", count+1, err)
		}
		count++
	}
	return count, nil
}

func splitSQL(s string) []string {
	var out []string
	var b strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
			b.WriteByte(c)
		case c == '"' && !inSingle:
			inDouble = !inDouble
			b.WriteByte(c)
		case inSingle || inDouble:
			b.WriteByte(c)
		case c == '-' && i+1 < len(s) && s[i+1] == '-':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			if i < len(s) {
				b.WriteByte('\n')
			}
		case c == ';':
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	if strings.TrimSpace(b.String()) != "" {
		out = append(out, b.String())
	}
	return out
}
