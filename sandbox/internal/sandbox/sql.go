package sandbox

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	lua "github.com/Shopify/go-lua"
)

const defaultMaxSQLRows = 10000

type sqlConn interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

type sqlRegistry struct {
	dbs map[string]*sql.DB
	txs map[string]*sql.Tx
}

func newSQLRegistry() *sqlRegistry {
	return &sqlRegistry{dbs: map[string]*sql.DB{}, txs: map[string]*sql.Tx{}}
}

func (r *sqlRegistry) get(path string) (*sql.DB, error) {
	if db, ok := r.dbs[path]; ok {
		return db, nil
	}
	db, err := openSQLite(path)
	if err != nil {
		return nil, err
	}
	r.dbs[path] = db
	return db, nil
}

func (r *sqlRegistry) conn(path string) (sqlConn, error) {
	if tx, ok := r.txs[path]; ok {
		return tx, nil
	}
	return r.get(path)
}

func (r *sqlRegistry) begin(path string) error {
	db, err := r.get(path)
	if err != nil {
		return err
	}
	if _, ok := r.txs[path]; ok {
		return fmt.Errorf("sqlite: transação já aberta em %q", path)
	}
	tx, err := db.Begin()
	if err != nil {
		return sqlErr(err)
	}
	r.txs[path] = tx
	return nil
}

func (r *sqlRegistry) commit(path string) error {
	tx, ok := r.txs[path]
	if !ok {
		return fmt.Errorf("sqlite: nenhuma transação aberta")
	}
	if err := tx.Commit(); err != nil {
		return sqlErr(err)
	}
	delete(r.txs, path)
	return nil
}

func (r *sqlRegistry) rollback(path string) error {
	tx, ok := r.txs[path]
	if !ok {
		return fmt.Errorf("sqlite: nenhuma transação aberta")
	}
	err := tx.Rollback()
	delete(r.txs, path)
	if err != nil {
		return sqlErr(err)
	}
	return nil
}

func (r *sqlRegistry) closePath(path string) bool {
	if tx, ok := r.txs[path]; ok {
		tx.Rollback()
		delete(r.txs, path)
	}
	if db, ok := r.dbs[path]; ok {
		db.Close()
		delete(r.dbs, path)
		return true
	}
	return false
}

func (r *sqlRegistry) close() {
	for path, tx := range r.txs {
		tx.Rollback()
		delete(r.txs, path)
	}
	for path, db := range r.dbs {
		db.Close()
		delete(r.dbs, path)
	}
}

func buildSQL(L *lua.State, reg *sqlRegistry, mnt, tmp *Store) int {
	t := newTable(L)
	setGoFunc(L, t, "connect", func(l *lua.State) int {
		path, err := sqlPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		c := newTable(L)
		addSQLConn(L, c, reg, path)
		return 1
	})
	setGoFunc(L, t, "import", func(l *lua.State) int {
		path, err := sqlPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		db, err := reg.get(path)
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
	setGoFunc(L, t, "export", func(l *lua.State) int {
		path, err := sqlPath(mnt, tmp, argString(l, 1))
		if err != nil {
			panic(err)
		}
		conn, err := reg.conn(path)
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
	return t
}

func addSQLConn(L *lua.State, t int, reg *sqlRegistry, bound string) {
	getPath := func(l *lua.State) string {
		return bound
	}
	req := func(l *lua.State) (string, string, []any) {
		path := getPath(l)
		query := argString(l, 1)
		if query == "" {
			panic(fmt.Errorf("consulta SQL vazia"))
		}
		var args []any
		if l.Top() >= 2 && !l.IsNil(2) {
			for _, v := range luaArrayAny(l, 2) {
				args = append(args, v)
			}
		}
		return path, query, args
	}

	setGoFunc(L, t, "exec", func(l *lua.State) int {
		path, query, args := req(l)
		conn, err := reg.conn(path)
		if err != nil {
			panic(err)
		}
		res, err := conn.Exec(query, args...)
		if err != nil {
			panic(sqlErr(err))
		}
		id, _ := res.LastInsertId()
		n, _ := res.RowsAffected()
		pushAny(l, map[string]any{"lastId": id, "rows": n})
		return 1
	})
	setGoFunc(L, t, "query", func(l *lua.State) int {
		path, query, args := req(l)
		conn, err := reg.conn(path)
		if err != nil {
			panic(err)
		}
		rows, err := sqlQueryRows(conn, query, args)
		if err != nil {
			panic(err)
		}
		pushAny(l, rows)
		return 1
	})
	setGoFunc(L, t, "get", func(l *lua.State) int {
		path, query, args := req(l)
		conn, err := reg.conn(path)
		if err != nil {
			panic(err)
		}
		r, err := conn.Query(query, args...)
		if err != nil {
			panic(sqlErr(err))
		}
		defer r.Close()
		cols, err := r.Columns()
		if err != nil {
			panic(sqlErr(err))
		}
		if !r.Next() {
			l.PushNil()
			return 1
		}
		row, err := scanRow(r, cols)
		if err != nil {
			panic(err)
		}
		pushAny(l, row)
		return 1
	})
	setGoFunc(L, t, "scalar", func(l *lua.State) int {
		path, query, args := req(l)
		conn, err := reg.conn(path)
		if err != nil {
			panic(err)
		}
		var v any
		if err := conn.QueryRow(query, args...).Scan(&v); err != nil {
			if err == sql.ErrNoRows {
				l.PushNil()
				return 1
			}
			panic(sqlErr(err))
		}
		pushAny(l, sqlVal(v))
		return 1
	})
	setGoFunc(L, t, "close", func(l *lua.State) int {
		l.PushBoolean(reg.closePath(getPath(l)))
		return 1
	})
	setGoFunc(L, t, "begin", func(l *lua.State) int {
		if err := reg.begin(getPath(l)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})
	setGoFunc(L, t, "commit", func(l *lua.State) int {
		if err := reg.commit(getPath(l)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})
	setGoFunc(L, t, "rollback", func(l *lua.State) int {
		if err := reg.rollback(getPath(l)); err != nil {
			panic(err)
		}
		l.PushBoolean(true)
		return 1
	})
	setGoFunc(L, t, "tables", func(l *lua.State) int {
		conn, err := reg.conn(getPath(l))
		if err != nil {
			panic(err)
		}
		rows, err := sqlQueryRows(conn, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name", nil)
		if err != nil {
			panic(err)
		}
		names := make([]any, 0, len(rows))
		for _, r := range rows {
			if m, ok := r.(map[string]any); ok {
				if n, ok := m["name"].(string); ok && n != "" {
					names = append(names, n)
				}
			}
		}
		pushAny(l, names)
		return 1
	})
	setGoFunc(L, t, "columns", func(l *lua.State) int {
		tbl := argString(l, 1)
		if tbl == "" {
			panic(fmt.Errorf("nome da tabela vazio"))
		}
		conn, err := reg.conn(getPath(l))
		if err != nil {
			panic(err)
		}
		rows, err := sqlQueryRows(conn, "PRAGMA table_info("+sqlQuote(tbl)+")", nil)
		if err != nil {
			panic(err)
		}
		pushAny(l, cleanColumns(rows))
		return 1
	})
	setGoFunc(L, t, "schema", func(l *lua.State) int {
		conn, err := reg.conn(getPath(l))
		if err != nil {
			panic(err)
		}
		rows, err := sqlQueryRows(conn, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name", nil)
		if err != nil {
			panic(err)
		}
		var names []any
		cols := map[string]any{}
		for _, r := range rows {
			m, _ := r.(map[string]any)
			n, _ := m["name"].(string)
			if n == "" {
				continue
			}
			names = append(names, n)
			cc, err := sqlQueryRows(conn, "PRAGMA table_info("+sqlQuote(n)+")", nil)
			if err != nil {
				panic(err)
			}
			cols[n] = cleanColumns(cc)
		}
		pushAny(l, map[string]any{"tables": names, "columns": cols})
		return 1
	})
	setGoFunc(L, t, "import", func(l *lua.State) int {
		db, err := reg.get(getPath(l))
		if err != nil {
			panic(err)
		}
		n, err := dataSQLImport(db, argString(l, 1), luaArrayAny(l, 2), toAnyMap(l, 3))
		if err != nil {
			panic(err)
		}
		pushAny(l, map[string]any{"imported": n})
		return 1
	})
	setGoFunc(L, t, "export", func(l *lua.State) int {
		conn, err := reg.conn(getPath(l))
		if err != nil {
			panic(err)
		}
		rows, err := dataSQLExport(conn, argString(l, 1))
		if err != nil {
			panic(err)
		}
		pushAny(l, rows)
		return 1
	})
}

func cleanColumns(rows []any) []any {
	out := make([]any, 0, len(rows))
	for _, r := range rows {
		m, _ := r.(map[string]any)
		out = append(out, map[string]any{
			"name":    m["name"],
			"type":    m["type"],
			"notnull": m["notnull"],
			"pk":      m["pk"],
			"dflt":    m["dflt_value"],
		})
	}
	return out
}

func sqlQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func sqlPath(mnt, tmp *Store, p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("caminho do banco vazio")
	}
	if p == ":memory:" {
		return p, nil
	}
	if strings.HasPrefix(p, "tmp:") {
		return tmp.resolve(strings.TrimPrefix(p, "tmp:"))
	}
	return mnt.resolve(p)
}

func openSQLite(path string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" {
		dsn = path + "?_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func sqlQueryRows(conn sqlConn, query string, args []any) ([]any, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, sqlErr(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, sqlErr(err)
	}
	limit := sqlRowLimit()
	out := make([]any, 0)
	for rows.Next() {
		if len(out) >= limit {
			break
		}
		row, err := scanRow(rows, cols)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, sqlErr(err)
	}
	return out, nil
}

func sqlRowLimit() int {
	if v := strings.TrimSpace(os.Getenv("SANDBOX_SQL_MAX_ROWS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultMaxSQLRows
}

func scanRow(rows *sql.Rows, cols []string) (map[string]any, error) {
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, sqlErr(err)
	}
	row := make(map[string]any, len(cols))
	for i, c := range cols {
		row[c] = sqlVal(vals[i])
	}
	return row, nil
}

func sqlVal(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339)
	case int64:
		return float64(x)
	case int:
		return float64(x)
	default:
		return v
	}
}

func sqlErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("sqlite: %w", err)
}
