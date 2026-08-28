package sqlize

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

func (s *store) exportFile(ctx context.Context, outPath, source, table, target string, redact bool) (string, error) {
	if strings.TrimSpace(outPath) == "" {
		return "", fmt.Errorf("caminho de saída é obrigatório")
	}
	format, err := formatFromPath(outPath)
	if err != nil {
		return "", err
	}
	var q string
	if strings.TrimSpace(source) != "" {
		q = source
	} else if strings.TrimSpace(table) != "" {
		q = "SELECT * FROM " + quoteIdent(strings.TrimSpace(table))
	} else {
		return "", fmt.Errorf("informe 'query' (SQL) ou 'table' (nome da tabela)")
	}
	cols, rows, err := s.query(ctx, q)
	if err != nil {
		return "", err
	}
	if redact {
		rows = RedactRows(cols, rows)
	}
	switch format {
	case "json":
		err = writeJSON(outPath, cols, rows)
	case "csv":
		err = writeDelimited(outPath, cols, rows, ',')
	case "tsv":
		err = writeDelimited(outPath, cols, rows, '\t')
	case "xlsx":
		err = writeExcel(outPath, cols, rows)
	case "sql":
		err = writeSQL(outPath, cols, rows, target)
	case "html":
		err = writeHTML(outPath, cols, rows)
	case "xml":
		err = writeXML(outPath, cols, rows)
	default:
		return "", fmt.Errorf("formato de exportação não suportado: %s", format)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Exportado %d linha(s) para %s (%s).", len(rows), outPath, format), nil
}

func formatFromPath(p string) (string, error) {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".json":
		return "json", nil
	case ".csv":
		return "csv", nil
	case ".tsv":
		return "tsv", nil
	case ".xlsx", ".xlsm":
		return "xlsx", nil
	case ".sql":
		return "sql", nil
	case ".html", ".htm":
		return "html", nil
	case ".xml":
		return "xml", nil
	default:
		return "", fmt.Errorf("extensão não suportada para exportação: %s (use .json, .csv, .tsv, .xlsx, .sql, .html, .xml)", ext)
	}
}

func valueFromStr(s string) any {
	if s == "" {
		return ""
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

func writeJSON(path string, cols []string, rows [][]string) error {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			v := ""
			if i < len(r) {
				v = r[i]
			}
			m[c] = valueFromStr(v)
		}
		out = append(out, m)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func writeDelimited(path string, cols []string, rows [][]string, comma rune) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Comma = comma
	if err := w.Write(cols); err != nil {
		return err
	}
	for _, r := range rows {
		rec := make([]string, len(cols))
		for i := range cols {
			if i < len(r) {
				rec[i] = r[i]
			}
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeExcel(path string, cols []string, rows [][]string) error {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	for i, c := range cols {
		cell, _ := cellName(i, 1)
		if err := f.SetCellValue(sheet, cell, c); err != nil {
			return err
		}
	}
	for r, row := range rows {
		for i, v := range row {
			cell, _ := cellName(i, r+2)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
	}
	return f.SaveAs(path)
}

func writeSQL(path string, cols []string, rows [][]string, tableName string) error {
	if strings.TrimSpace(tableName) == "" {
		tableName = "exported"
	}
	var b strings.Builder
	b.WriteString("BEGIN;\n")
	fmt.Fprintf(&b, "DROP TABLE IF EXISTS %s;\n", quoteIdent(tableName))
	b.WriteString("CREATE TABLE ")
	b.WriteString(quoteIdent(tableName))
	b.WriteString(" (\n")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString("  ")
		b.WriteString(quoteIdent(c))
		b.WriteString(" TEXT")
	}
	b.WriteString("\n);\n")
	for _, r := range rows {
		b.WriteString("INSERT INTO ")
		b.WriteString(quoteIdent(tableName))
		b.WriteString(" (")
		for i, c := range cols {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(quoteIdent(c))
		}
		b.WriteString(") VALUES (")
		for i := range cols {
			v := ""
			if i < len(r) {
				v = r[i]
			}
			b.WriteString(sqlLit(v))
		}
		b.WriteString(");\n")
	}
	b.WriteString("COMMIT;\n")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func writeHTML(path string, cols []string, rows [][]string) error {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<title>sqlize export</title>\n")
	b.WriteString("<style>body{font-family:system-ui,sans-serif;margin:2rem}table{border-collapse:collapse}th,td{border:1px solid #ccc;padding:.35rem .6rem;text-align:left}th{background:#f2f2f2}tr:nth-child(even){background:#fafafa}</style>\n")
	b.WriteString("</head>\n<body>\n<table>\n<thead><tr>")
	for _, c := range cols {
		fmt.Fprintf(&b, "<th>%s</th>", html.EscapeString(c))
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	for _, r := range rows {
		b.WriteString("<tr>")
		for i := range cols {
			v := ""
			if i < len(r) {
				v = r[i]
			}
			fmt.Fprintf(&b, "<td>%s</td>", html.EscapeString(v))
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table>\n</body>\n</html>\n")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func writeXML(path string, cols []string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "result"}}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "row"}}); err != nil {
			return err
		}
		for i, c := range cols {
			name := xmlName(c)
			text := ""
			if i < len(r) {
				text = r[i]
			}
			if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: name}}); err != nil {
				return err
			}
			if err := enc.EncodeToken(xml.CharData(text)); err != nil {
				return err
			}
			if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: name}}); err != nil {
				return err
			}
		}
		if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "row"}}); err != nil {
			return err
		}
	}
	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "result"}}); err != nil {
		return err
	}
	return enc.Flush()
}

func exportLiveFile(path string, cols []string, rows [][]string, target string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("'export_to' é obrigatório")
	}
	format, err := formatFromPath(path)
	if err != nil {
		return "", err
	}
	rows = RedactRows(cols, rows)
	switch format {
	case "json":
		err = writeJSON(path, cols, rows)
	case "csv":
		err = writeDelimited(path, cols, rows, ',')
	case "tsv":
		err = writeDelimited(path, cols, rows, '\t')
	case "xlsx":
		err = writeExcel(path, cols, rows)
	case "html":
		err = writeHTML(path, cols, rows)
	case "xml":
		err = writeXML(path, cols, rows)
	case "sql":
		err = writeSQL(path, cols, rows, target)
	default:
		return "", fmt.Errorf("formato de exportação não suportado: %s (use .csv, .html, .xlsx, .tsv, .json, .xml, .sql)", format)
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Exportado %d linha(s) para %s (%s).", len(rows), path, format), nil
}

func sqlLit(s string) string {
	if s == "" {
		return "NULL"
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func xmlName(s string) string {
	fields := strings.Fields(s)
	joined := strings.Join(fields, "_")
	if joined == "" {
		joined = "col"
	}
	var b strings.Builder
	for i, r := range joined {
		switch {
		case r == '_' || r == '-' || r == '.':
			b.WriteRune(r)
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			if i == 0 {
				b.WriteRune('_')
			}
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func cellName(col, row int) (string, error) {
	var b strings.Builder
	n := col + 1
	for n > 0 {
		n--
		b.WriteByte(byte('A' + n%26))
		n /= 26
	}
	runes := []byte(b.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes) + strconv.Itoa(row), nil
}
