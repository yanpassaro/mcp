package sqlize

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const dbFileName = "sqlize.db"

type tableInfo struct {
	Schema string
	Name   string
}

type columnInfo struct {
	Name string
	Type string
}

type fkInfo struct {
	Column    string
	RefTable  string
	RefColumn string
}

type indexInfo struct {
	Name    string
	Unique  bool
	Columns []string
}

type store struct {
	db        *sql.DB
	path      string
	attachSeq int
	attached  []string
}

func newStore(stateDir string) (*store, error) {
	dir, err := resolveStateDir(stateDir)
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, dbFileName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("abrir banco em %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys=OFF"); err != nil {
		db.Close()
		return nil, err
	}
	return &store{db: db, path: dbPath}, nil
}

func resolveStateDir(stateDir string) (string, error) {
	if strings.TrimSpace(stateDir) != "" {
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return "", fmt.Errorf("criar diretório de estado %s: %w", stateDir, err)
		}
		return stateDir, nil
	}
	home := os.Getenv("USERPROFILE")
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		return "", fmt.Errorf("não foi possível determinar o diretório do usuário; defina SQLIZE_STATE_DIR")
	}
	dir := filepath.Join(home, ".local", "state", "sqlize")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("criar diretório de estado %s: %w", dir, err)
	}
	return dir, nil
}

func (s *store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteLit(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func sanitizeReadQuery(q string) (string, error) {
	t := strings.TrimSpace(q)
	if t == "" {
		return "", fmt.Errorf("consulta vazia")
	}
	lower := strings.ToLower(t)
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return "", fmt.Errorf("apenas consultas SELECT ou WITH são permitidas")
	}
	if strings.Contains(t, ";") {
		return "", fmt.Errorf("consultas não podem conter ';' (apenas uma instrução por vez)")
	}
	return t, nil
}

func (s *store) query(ctx context.Context, q string, args []string) (columns []string, rows [][]string, err error) {
	clean, err := sanitizeReadQuery(q)
	if err != nil {
		return nil, nil, err
	}
	if err := enforceQueryRules(clean, args, "?"); err != nil {
		return nil, nil, err
	}
	if err := checkFuncAllowlist(clean); err != nil {
		return nil, nil, err
	}
	params := make([]any, len(args))
	for i, a := range args {
		params[i] = a
	}
	rs, err := s.db.QueryContext(ctx, clean, params...)
	if err != nil {
		return nil, nil, fmt.Errorf("executar consulta: %w", err)
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, nil, err
	}
	out := make([][]string, 0)
	for rs.Next() {
		raw := make([]sql.RawBytes, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make([]string, len(cols))
		for i, rb := range raw {
			if rb != nil {
				row[i] = string(rb)
			}
		}
		out = append(out, row)
	}
	if err := rs.Err(); err != nil {
		return nil, nil, err
	}
	return cols, out, nil
}

func sanitizeStoreQuery(q string, args []string) (string, error) {
	t := strings.TrimSpace(q)
	if t == "" {
		return "", fmt.Errorf("consulta vazia")
	}
	if strings.Contains(t, ";") {
		return "", fmt.Errorf("consultas não podem conter ';' (apenas uma instrução por vez)")
	}
	lower := strings.ToLower(t)
	allowed := []string{"select", "with", "insert", "update", "delete", "replace"}
	ok := false
	for _, p := range allowed {
		if strings.HasPrefix(lower, p) {
			ok = true
			break
		}
	}
	if !ok {
		return "", fmt.Errorf("apenas SELECT/WITH/INSERT/UPDATE/DELETE/REPLACE são permitidos no SQLite local")
	}
	if reStringInWhere.MatchString(t) {
		return "", fmt.Errorf("consulta rejeitada: não inclua literais de string na cláusula WHERE; passe valores via 'args' (use ? no SQLite)")
	}
	if reWhereLiteral.MatchString(t) && !strings.Contains(t, "?") {
		return "", fmt.Errorf("consulta rejeitada: cláusula WHERE compara valores; use ? no lugar do valor e passe-o em 'args' (ex.: WHERE id = ?)")
	}
	if len(args) > 0 && !strings.Contains(t, "?") {
		return "", fmt.Errorf("'args' informado mas o SQL não contém placeholders; use ? e passe os valores em 'args'")
	}
	if err := checkFuncAllowlist(t); err != nil {
		return "", err
	}
	return t, nil
}

func (s *store) runQuery(ctx context.Context, q string, args []string) (string, error) {
	clean, err := sanitizeStoreQuery(q, args)
	if err != nil {
		return "", err
	}
	params := make([]any, len(args))
	for i, a := range args {
		params[i] = a
	}
	lower := strings.ToLower(strings.TrimSpace(clean))
	isRow := strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with") ||
		strings.HasPrefix(lower, "values") || strings.HasPrefix(lower, "pragma")
	if isRow {
		rs, err := s.db.QueryContext(ctx, clean, params...)
		if err != nil {
			return "", fmt.Errorf("executar consulta: %w", err)
		}
		defer rs.Close()
		cols, err := rs.Columns()
		if err != nil {
			return "", err
		}
		out := make([][]string, 0)
		for rs.Next() {
			raw := make([]sql.RawBytes, len(cols))
			ptrs := make([]any, len(cols))
			for i := range raw {
				ptrs[i] = &raw[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				return "", err
			}
			row := make([]string, len(cols))
			for i, rb := range raw {
				if rb != nil {
					row[i] = string(rb)
				}
			}
			out = append(out, row)
		}
		if err := rs.Err(); err != nil {
			return "", err
		}
		const max = 200
		shown := out
		if len(out) > max {
			shown = out[:max]
		}
		shown = RedactRows(cols, shown)
		label := " (mascaradas)"
		var b strings.Builder
		b.WriteString(markdownTable(cols, shown))
		if len(out) > max {
			fmt.Fprintf(&b, "\n... %d linhas no total (mostrando %d%s).\n", len(out), max, label)
		} else {
			fmt.Fprintf(&b, "\n%d linha(s)%s.\n", len(out), label)
		}
		return b.String(), nil
	}
	res, err := s.db.ExecContext(ctx, clean, params...)
	if err != nil {
		return "", fmt.Errorf("executar comando: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("OK. %d linha(s) afetada(s).", n), nil
}

func (s *store) dropTables(ctx context.Context, name string) (string, error) {
	if strings.TrimSpace(name) != "" {
		return s.dropOneMainTable(ctx, strings.TrimSpace(name))
	}
	return s.dropAllMainTables(ctx)
}

func (s *store) dropAllMainTables(ctx context.Context) (string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return "", err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return "", err
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	rows.Close()
	if len(names) == 0 {
		return "Banco de trabalho já está vazio (nenhuma tabela).", nil
	}
	if err := s.dropTablesInTx(ctx, names); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d tabela(s) removida(s) do banco de trabalho.", len(names)), nil
}

func (s *store) dropOneMainTable(ctx context.Context, name string) (string, error) {
	var exists string
	err := s.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name = ? AND name NOT LIKE 'sqlite_%'", name).Scan(&exists)
	if err == sql.ErrNoRows {
		tables, lerr := s.listTables(ctx)
		if lerr != nil {
			return "", lerr
		}
		names := make([]string, 0, len(tables))
		for _, t := range tables {
			names = append(names, t.Name)
		}
		return "", fmt.Errorf("tabela %q não encontrada no banco de trabalho; disponíveis: %s", name, strings.Join(names, ", "))
	}
	if err != nil {
		return "", err
	}
	if err := s.dropTablesInTx(ctx, []string{name}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Tabela %q removida.", name), nil
}

func (s *store) dropTablesInTx(ctx context.Context, names []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, n := range names {
		if _, err := tx.ExecContext(ctx, "DROP TABLE "+quoteIdent(n)); err != nil {
			tx.Rollback()
			return fmt.Errorf("dropar tabela %q: %w", n, err)
		}
	}
	return tx.Commit()
}

func (s *store) listTables(ctx context.Context) ([]tableInfo, error) {
	var out []tableInfo
	schemas := append([]string{"main"}, s.attached...)
	for _, sch := range schemas {
		var q string
		if sch == "main" {
			q = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
		} else {
			q = "SELECT name FROM " + quoteIdent(sch) + ".sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
		}
		rows, err := s.db.QueryContext(ctx, q)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, tableInfo{Schema: sch, Name: name})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

func (s *store) tableColumns(ctx context.Context, schema, name string) ([]columnInfo, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, type FROM pragma_table_info(?, ?)`, name, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []columnInfo
	for rows.Next() {
		var c columnInfo
		if err := rows.Scan(&c.Name, &c.Type); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *store) tableForeignKeys(ctx context.Context, schema, name string) ([]fkInfo, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT "from", "table" AS ref_table, "to" AS ref_column FROM pragma_foreign_key_list(?, ?) ORDER BY id, seq`, name, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []fkInfo
	for rows.Next() {
		var f fkInfo
		if err := rows.Scan(&f.Column, &f.RefTable, &f.RefColumn); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *store) tableIndexes(ctx context.Context, schema, name string) ([]indexInfo, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, "unique" FROM pragma_index_list(?, ?) ORDER BY name`, name, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var idx []indexInfo
	for rows.Next() {
		var ix indexInfo
		if err := rows.Scan(&ix.Name, &ix.Unique); err != nil {
			return nil, err
		}
		idx = append(idx, ix)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range idx {
		crows, err := s.db.QueryContext(ctx, `SELECT name FROM pragma_index_info(?) ORDER BY seqno`, idx[i].Name)
		if err != nil {
			return nil, err
		}
		for crows.Next() {
			var c string
			if err := crows.Scan(&c); err != nil {
				crows.Close()
				return nil, err
			}
			idx[i].Columns = append(idx[i].Columns, c)
		}
		if err := crows.Err(); err != nil {
			crows.Close()
			return nil, err
		}
		crows.Close()
	}
	return idx, nil
}

func (s *store) attachDB(ctx context.Context, path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("arquivo não encontrado: %w", err)
	}
	alias := "db" + strconv.Itoa(s.attachSeq)
	s.attachSeq++
	s.attached = append(s.attached, alias)
	q := fmt.Sprintf("ATTACH DATABASE %s AS %s", quoteLit(path), quoteIdent(alias))
	if _, err := s.db.ExecContext(ctx, q); err != nil {
		return "", fmt.Errorf("anexar banco: %w", err)
	}
	return alias, nil
}

func inferColumnType(values []string) string {
	has := false
	allInt := true
	allFloat := true
	allDate := true
	for _, v := range values {
		if v == "" {
			continue
		}
		has = true
		if allInt {
			if _, err := strconv.ParseInt(v, 10, 64); err != nil {
				allInt = false
			}
		}
		if allFloat {
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				allFloat = false
			}
		}
		if allDate {
			if !isDateLike(v) {
				allDate = false
			}
		}
		if !allInt && !allFloat && !allDate {
			break
		}
	}
	if !has {
		return "TEXT"
	}
	if allInt {
		return "INTEGER"
	}
	if allFloat {
		return "REAL"
	}
	if allDate {
		return "DATE"
	}
	return "TEXT"
}

var dateLayouts = []string{"2006-01-02", "2006/01/02", "02/01/2006", "2006-1-2", "2/1/2006", "02-01-2006"}

func isDateLike(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return false
	}
	for _, layout := range dateLayouts {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}

func (s *store) loadTable(ctx context.Context, name string, columns []string, rows [][]string) error {
	if len(columns) == 0 {
		return fmt.Errorf("nenhuma coluna detectada")
	}
	colData := make([][]string, len(columns))
	for i := range columns {
		if strings.TrimSpace(columns[i]) == "" {
			columns[i] = fmt.Sprintf("col_%d", i+1)
		}
		colData[i] = make([]string, 0, len(rows))
	}
	for _, r := range rows {
		for i := range columns {
			if i < len(r) {
				colData[i] = append(colData[i], r[i])
			} else {
				colData[i] = append(colData[i], "")
			}
		}
	}
	types := make([]string, len(columns))
	for i := range columns {
		types[i] = inferColumnType(colData[i])
	}
	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	b.WriteString(quoteIdent(name))
	b.WriteString(" (")
	for i, c := range columns {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoteIdent(c))
		b.WriteString(" ")
		b.WriteString(types[i])
	}
	b.WriteString(")")

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS "+quoteIdent(name)); err != nil {
		tx.Rollback()
		return fmt.Errorf("recriar tabela: %w", err)
	}
	if _, err := tx.ExecContext(ctx, b.String()); err != nil {
		tx.Rollback()
		return fmt.Errorf("criar tabela: %w", err)
	}
	placeholder := "(" + strings.TrimRight(strings.Repeat("?,", len(columns)), ",") + ")"
	insertSQL := "INSERT INTO " + quoteIdent(name) + " VALUES " + placeholder
	for _, r := range rows {
		args := make([]any, len(columns))
		for i := range columns {
			if i < len(r) && r[i] != "" {
				args[i] = r[i]
			} else {
				args[i] = nil
			}
		}
		if _, err := tx.ExecContext(ctx, insertSQL, args...); err != nil {
			tx.Rollback()
			return fmt.Errorf("inserir linha em %q: %w", name, err)
		}
	}
	return tx.Commit()
}
