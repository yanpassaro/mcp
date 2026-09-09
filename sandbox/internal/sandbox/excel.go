package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"

	lua "github.com/Shopify/go-lua"
)

func buildExcel(L *lua.State, mnt, tmp *Store) int {
	t := newTable(L)

	setGoFunc(L, t, "sheets", func(l *lua.State) int {
		path, err := excelPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		names, err := excelSheetNames(path)
		if err != nil {
			panic(err)
		}
		pushAny(l, names)
		return 1
	})

	setGoFunc(L, t, "read", func(l *lua.State) int {
		path, err := excelPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		rows, err := excelReadRows(path, argString(l, 2))
		if err != nil {
			panic(err)
		}
		pushAny(l, excelRowsToAny(rows))
		return 1
	})

	setGoFunc(L, t, "write", func(l *lua.State) int {
		path, err := excelPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		rows := rowsAnyToExcel(luaArrayAny(l, 2))
		if err := excelWriteRows(path, argString(l, 3), rows, toAnyMap(l, 4)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})

	return t
}

func excelPath(mnt, tmp *Store, p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("caminho da planilha vazio")
	}
	if after, ok :=strings.CutPrefix(p, "tmp:"); ok  {
		return tmp.resolve(after)
	}
	return mnt.resolve(p)
}

func excelSheetNames(path string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("excel: %w", err)
	}
	defer f.Close()
	return f.GetSheetList(), nil
}

func excelReadRows(path, sheet string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("excel: %w", err)
	}
	defer f.Close()
	if sheet == "" {
		sheet = f.GetSheetName(0)
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("excel: %w", err)
	}
	return rows, nil
}

func excelWriteRows(path, sheet string, rows [][]string, opts map[string]any) error {
	f := excelize.NewFile()
	defer f.Close()
	sheetName := "Sheet1"
	if sheet != "" {
		f.SetSheetName("Sheet1", sheet)
		sheetName = sheet
	}
	for i, row := range rows {
		vals := make([]string, len(row))
		copy(vals, row)
		if err := f.SetSheetRow(sheetName, fmt.Sprintf("A%d", i+1), &vals); err != nil {
			return fmt.Errorf("excel: %w", err)
		}
	}
	if err := styleExcel(f, sheetName, rows, opts); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("excel: %w", err)
	}
	if err := f.SaveAs(path); err != nil {
		return fmt.Errorf("excel: %w", err)
	}
	return nil
}

func styleExcel(f *excelize.File, sheetName string, rows [][]string, opts map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	headerColor := optColor(opts, "header", "headerColor")
	rowColor := optColor(opts, "row", "rowColor")
	altColor := optColor(opts, "alt", "altRowColor")
	zebra := truthyOpt(opts, "zebra")
	lastCol := colLetters(len(rows[0]))
	if headerColor != "" {
		st, err := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true},
			Fill: excelize.Fill{Type: "pattern", Color: []string{stripHashColor(headerColor)}, Pattern: 1},
		})
		if err != nil {
			return fmt.Errorf("excel: %w", err)
		}
		if err := f.SetCellStyle(sheetName, "A1", lastCol+"1", st); err != nil {
			return fmt.Errorf("excel: %w", err)
		}
	}
	if rowColor != "" {
		st, err := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{stripHashColor(rowColor)}, Pattern: 1},
		})
		if err != nil {
			return fmt.Errorf("excel: %w", err)
		}
		for r := 1; r < len(rows); r++ {
			if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", r+1), lastCol+fmt.Sprint(r+1), st); err != nil {
				return fmt.Errorf("excel: %w", err)
			}
		}
	}
	if zebra {
		if altColor == "" {
			altColor = "#f5f5f5"
		}
		st, err := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{stripHashColor(altColor)}, Pattern: 1},
		})
		if err != nil {
			return fmt.Errorf("excel: %w", err)
		}
		for r := 1; r < len(rows); r++ {
			if r%2 == 1 {
				if err := f.SetCellStyle(sheetName, fmt.Sprintf("A%d", r+1), lastCol+fmt.Sprint(r+1), st); err != nil {
					return fmt.Errorf("excel: %w", err)
				}
			}
		}
	}
	return nil
}

func colLetters(n int) string {
	if n < 1 {
		return "A"
	}
	var b strings.Builder
	for n > 0 {
		n--
		b.WriteByte(byte('A' + n%26))
		n /= 26
	}
	s := b.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func stripHashColor(c string) string {
	return strings.TrimPrefix(c, "#")
}

func excelRowsToAny(rows [][]string) []any {
	out := make([]any, len(rows))
	for i, row := range rows {
		cells := make([]string, len(row))
		copy(cells, row)
		out[i] = cells
	}
	return out
}

func rowsAnyToExcel(rows []any) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		cells, ok := row.([]any)
		if !ok {
			cells = []any{row}
		}
		line := make([]string, len(cells))
		for j, c := range cells {
			line[j] = fmt.Sprint(c)
		}
		out[i] = line
	}
	return out
}
