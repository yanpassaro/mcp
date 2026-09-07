package sandbox

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	lua "github.com/Shopify/go-lua"
)

// buildExcel exposes read/write access to .xlsx spreadsheets confined to the
// sandbox filesystem. Paths are relative to mnt/ (or prefixed with tmp:).
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
		if err := excelWriteRows(path, argString(l, 3), rows); err != nil {
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
	if strings.HasPrefix(p, "tmp:") {
		return tmp.resolve(strings.TrimPrefix(p, "tmp:"))
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

func excelWriteRows(path, sheet string, rows [][]string) error {
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
	if err := f.SaveAs(path); err != nil {
		return fmt.Errorf("excel: %w", err)
	}
	return nil
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
