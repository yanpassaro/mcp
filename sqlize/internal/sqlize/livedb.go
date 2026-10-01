package sqlize

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const LIVE_HARD_LIMIT = 500

func enforceLimit(q string, hard int) string {
	m := reLimit.FindStringSubmatch(q)
	if m != nil {
		n, _ := strconv.Atoi(m[1])
		if n > hard {
			return reLimit.ReplaceAllString(q, fmt.Sprintf("LIMIT %d", hard))
		}
		return q
	}
	if reLimitPH.MatchString(q) {
		return reLimitPH.ReplaceAllString(q, fmt.Sprintf("LIMIT LEAST($1, %d)", hard))
	}
	return fmt.Sprintf("%s LIMIT %d", strings.TrimRight(q, " \n\t;"), hard)
}

var reLimit = regexp.MustCompile(`(?i)\blimit\s+(\d+)`)

var reLimitPH = regexp.MustCompile(`(?i)\blimit\s+(\?|\$\d+)`)

var rePgPlaceholder = regexp.MustCompile(`\$\d+`)

var reInjection = regexp.MustCompile(`(?i)(?:'|")\s*(?:--|#|/\*|union|into)\b`)

var reStringInWhere = regexp.MustCompile(`(?i)\bwhere\b[^;]*?'(?:[^']|'')*'`)

var reWhereLiteral = regexp.MustCompile(`(?i)\bwhere\b[^;]*?(?:=|<>|!=|<=|>=|<|>|like|glob|regexp|regex|between|in)\s*\(?\s*(?:-?\d+(?:\.\d+)?|true|false)\b`)

var reInfraLeak = regexp.MustCompile(`(?i)(dial\s+tcp[^\s:]*[:\s]\S+|tcp\([^)]*\)|\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b)`)

var reFuncCall = regexp.MustCompile(`(?i)\b([a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)*)\s*\(`)

var sqlFuncToken = map[string]bool{
	"in": true, "exists": true, "over": true, "on": true, "filter": true,
	"within": true, "group": true, "order": true, "any": true, "all": true,
	"some": true, "unique": true, "values": true, "table": true,
	"check": true, "references": true, "default": true, "union": true,
	"intersect": true, "except": true, "collate": true, "window": true,
	"rows": true, "range": true, "end": true, "cast": true, "array": true,
	"row": true, "interval": true, "as": true, "using": true,
}

var tableKeywords = map[string]bool{
	"into": true, "from": true, "update": true, "table": true, "with": true,
	"set": true, "join": true, "left": true, "right": true, "full": true,
	"cross": true, "inner": true, "outer": true, "replace": true,
}

var sqlFuncAllowlist = map[string]bool{
	"count": true, "sum": true, "avg": true, "min": true, "max": true,
	"string_agg": true, "array_agg": true, "json_agg": true, "jsonb_agg": true,
	"group_concat": true, "bool_and": true, "bool_or": true, "every": true,
	"stddev": true, "stddev_pop": true, "stddev_samp": true, "variance": true,
	"var_pop": true, "var_samp": true, "corr": true, "covar_pop": true,
	"covar_samp": true,
	"lower":      true, "upper": true, "trim": true, "ltrim": true, "rtrim": true,
	"btrim": true, "length": true, "char_length": true, "character_length": true,
	"octet_length": true, "substr": true, "substring": true, "left": true,
	"right": true, "replace": true, "translate": true, "concat": true,
	"concat_ws": true, "format": true, "lpad": true, "rpad": true,
	"repeat": true, "reverse": true, "initcap": true, "split_part": true,
	"strpos": true, "position": true, "instr": true, "to_char": true,
	"to_number": true, "chr": true, "ascii": true, "quote_literal": true,
	"quote_ident": true, "regexp_replace": true, "regexp_like": true,
	"regexp_instr": true, "regexp_substr": true,
	"abs": true, "round": true, "ceil": true, "ceiling": true, "floor": true,
	"trunc": true, "mod": true, "power": true, "pow": true, "sqrt": true,
	"cbrt": true, "exp": true, "ln": true, "log": true, "log10": true,
	"sign": true, "random": true, "pi": true, "sin": true, "cos": true,
	"tan": true, "asin": true, "acos": true, "atan": true, "atan2": true,
	"degrees": true, "radians": true, "greatest": true, "least": true,
	"div": true, "gcd": true, "lcm": true, "width_bucket": true,
	"extract": true, "date_trunc": true, "date_part": true, "age": true,
	"now": true, "current_date": true, "current_time": true,
	"current_timestamp": true, "localtime": true, "localtimestamp": true,
	"clock_timestamp": true, "statement_timestamp": true, "to_date": true,
	"to_timestamp": true, "make_date": true, "make_time": true,
	"make_timestamp": true, "make_interval": true, "strftime": true,
	"date": true, "time": true, "datetime": true, "julianday": true,
	"unixepoch": true, "date_add": true, "date_sub": true, "datediff": true,
	"date_format": true, "str_to_date": true, "from_unixtime": true,
	"unix_timestamp": true, "adddate": true, "subdate": true, "curdate": true,
	"curtime": true, "year": true, "month": true, "day": true, "hour": true,
	"minute": true, "second": true, "quarter": true, "week": true,
	"weekday": true, "dayname": true, "monthname": true, "last_day": true,
	"coalesce": true, "nullif": true, "ifnull": true, "if": true,
	"iif": true, "convert": true, "hex": true, "unhex": true, "bin": true,
	"oct":          true,
	"json_extract": true, "json_array_length": true, "json_type": true,
	"json_group_array": true, "json_group_object": true, "json_object": true,
	"json_array": true, "json_build_object": true, "json_build_array": true,
	"jsonb_build_object": true, "jsonb_build_array": true,
	"jsonb_extract_path": true, "json_object_agg": true, "jsonb_object_agg": true,
	"row_to_json": true, "to_json": true, "to_jsonb": true, "jsonb_pretty": true,
	"rank": true, "dense_rank": true, "row_number": true, "lag": true,
	"lead": true, "first_value": true, "last_value": true, "nth_value": true,
	"ntile": true, "percent_rank": true, "cume_dist": true,
	"md5": true, "sha1": true, "sha2": true, "sha256": true, "sha512": true,
	"gen_random_uuid": true,
}

func prevWord(q string, start int) string {
	i := start - 1
	for i >= 0 {
		if !isSpace(q[i]) {
			break
		}
		i--
	}
	j := i + 1
	for i >= 0 {
		if !isWordChar(rune(q[i])) {
			break
		}
		i--
	}
	return strings.ToLower(q[i+1 : j])
}

func isSpace(c byte) bool {
	if c == ' ' {
		return true
	}
	if c == '\t' {
		return true
	}
	if c == '\n' {
		return true
	}
	return c == '\r'
}

func isWordChar(r rune) bool {
	if r == '_' {
		return true
	}
	if r >= 'a' {
		if r <= 'z' {
			return true
		}
	}
	if r >= 'A' {
		if r <= 'Z' {
			return true
		}
	}
	return isDigit(r)
}

func checkFuncAllowlist(q string) error {
	for _, m := range reFuncCall.FindAllStringSubmatchIndex(q, -1) {
		start, end := m[2], m[3]
		name := strings.ToLower(q[start:end])
		if i := strings.LastIndexByte(name, '.'); i >= 0 {
			name = name[i+1:]
		}
		if sqlFuncToken[name] {
			continue
		}
		if sqlFuncAllowlist[name] {
			continue
		}
		if tableKeywords[prevWord(q, start)] {
			continue
		}
		return fmt.Errorf("consulta rejeitada: chamada de função %q não está na allowlist; apenas funções embutidas comuns são permitidas (COUNT, SUM, TO_CHAR, COALESCE, DATE_TRUNC...)", q[start:end])
	}
	return nil
}

func hasPlaceholderToken(clean, placeholder string) bool {
	if placeholder == "?" {
		return strings.Contains(clean, "?")
	}
	return rePgPlaceholder.MatchString(clean)
}

func enforceQueryRules(clean string, args []string, placeholder string) error {
	if reStringInWhere.MatchString(clean) {
		return fmt.Errorf("consulta rejeitada: não inclua literais de string na cláusula WHERE; passe valores dinâmicos via 'args' (use %s)", placeholder)
	}
	if reWhereLiteral.MatchString(clean) {
		if !hasPlaceholderToken(clean, placeholder) {
			return fmt.Errorf("consulta rejeitada: cláusula WHERE compara valores; parametrize com %s e passe os valores em 'args'", placeholder)
		}
	}
	if len(args) > 0 {
		if !hasPlaceholderToken(clean, placeholder) {
			return fmt.Errorf("'args' informado mas o SQL não contém placeholders; use %s e passe os valores em 'args'", placeholder)
		}
	}
	if reInjection.MatchString(clean) {
		return fmt.Errorf("consulta rejeitada: padrão suspeito de injeção de SQL (aspas seguidas de comentário ou de UNION/INTO); passe valores dinâmicos via 'args'")
	}
	return nil
}

func validateLiveQuery(q string, args []string, driver string, strict bool) (string, error) {
	clean, err := sanitizeReadQuery(q)
	if err != nil {
		return "", err
	}
	if strict {
		placeholder := "?"
		if driver == "pgx" {
			placeholder = "$1.."
		}
		if err := enforceQueryRules(clean, args, placeholder); err != nil {
			return "", err
		}
		if err := checkFuncAllowlist(clean); err != nil {
			return "", err
		}
	}
	return clean, nil
}

func connErr(op string) error {
	return fmt.Errorf("%s: falha de conexão com o banco de dados (verifique a DSN e a rede)", op)
}

func scrubInfra(err error) string {
	if err == nil {
		return ""
	}
	return reInfraLeak.ReplaceAllString(err.Error(), "<oculto>")
}

type liveDB struct {
	driver string
	dsn    string
}

type liveDBConfig struct {
	Engine string
	Alias  string
	EnvVar string
	Kind   string
	DSN    string
}

func (c liveDBConfig) ToolPrefix() string {
	if c.Alias == "" {
		return c.Engine
	}
	return fmt.Sprintf("%s_%s", c.Engine, c.Alias)
}

func discoverLiveDBs() []liveDBConfig {
	byKey := map[string]liveDBConfig{}
	for _, e := range os.Environ() {
		key, val, _ := strings.Cut(e, "=")
		if strings.TrimSpace(val) == "" {
			continue
		}
		engine, prefix, kind, ok := parseLiveEnv(key)
		if !ok {
			continue
		}
		alias := normalizeAlias(prefix)
		k := fmt.Sprintf("%s|%s", engine, alias)
		prev, exists := byKey[k]
		if exists {
			if prev.Kind == "url" {
				continue
			}
		}
		byKey[k] = liveDBConfig{Engine: engine, Alias: alias, EnvVar: key, Kind: kind, DSN: strings.TrimSpace(val)}
	}
	out := make([]liveDBConfig, 0, len(byKey))
	for _, cfg := range byKey {
		out = append(out, cfg)
	}
	slices.SortFunc(out, func(a, b liveDBConfig) int { return liveConfigLess(a, b) })
	return out
}

func liveConfigLess(a, b liveDBConfig) int {
	if a.Engine != b.Engine {
		return cmp.Compare(a.Engine, b.Engine)
	}
	return cmp.Compare(a.Alias, b.Alias)
}

func parseLiveEnv(key string) (engine, prefix, kind string, ok bool) {
	up := strings.ToUpper(strings.TrimSpace(key))
	switch up {
	case "POSTGRES_URL":
		return "postgres", "", "url", true
	case "POSTGRES_DSN":
		return "postgres", "", "dsn", true
	case "MYSQL_URL":
		return "mysql", "", "url", true
	case "MYSQL_DSN":
		return "mysql", "", "dsn", true
	}
	for _, engine := range []string{"postgres", "mysql"} {
		upper := strings.ToUpper(engine)
		for _, k := range []string{"URL", "DSN"} {
			suffix := fmt.Sprintf("_%s_%s", upper, k)
			if strings.HasSuffix(up, suffix) {
				return engine, strings.TrimSuffix(up, suffix), strings.ToLower(k), true
			}
		}
	}
	return "", "", "", false
}

func normalizeAlias(prefix string) string {
	a := strings.ToLower(strings.TrimSpace(prefix))
	if a == "" {
		return ""
	}
	if a == "db" {
		return ""
	}
	b := strings.Builder{}
	for _, r := range a {
		if isLowerAlphaNum(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	return strings.Trim(b.String(), "_")
}

func isLowerAlphaNum(r rune) bool {
	if r >= 'a' {
		if r <= 'z' {
			return true
		}
	}
	return isDigit(r)
}

func driverFor(engine string) string {
	if engine == "mysql" {
		return "mysql"
	}
	return "pgx"
}

func newLiveDB(cfg liveDBConfig) (*liveDB, error) {
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("defina a variável de ambiente %s para usar o %s", cfg.EnvVar, cfg.Engine)
	}
	return &liveDB{driver: driverFor(cfg.Engine), dsn: cfg.DSN}, nil
}

func (c *liveDB) query(ctx context.Context, q string, args []string, strict, noLimit bool) (columns []string, rows [][]string, err error) {
	clean, err := validateLiveQuery(q, args, c.driver, strict)
	if err != nil {
		return nil, nil, err
	}
	if !noLimit {
		clean = enforceLimit(clean, LIVE_HARD_LIMIT)
	}
	db, err := sql.Open(c.driver, c.dsn)
	if err != nil {
		return nil, nil, connErr("abrir conexão")
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, connErr("iniciar transação read-only")
	}
	defer tx.Rollback()

	params := make([]any, len(args))
	for i, a := range args {
		params[i] = a
	}

	rs, err := tx.QueryContext(ctx, clean, params...)
	if err != nil {
		return nil, nil, fmt.Errorf("executar consulta: %s", scrubInfra(err))
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


func (c *liveDB) tables(ctx context.Context) (string, error) {
	q := qTables
	if c.driver == "mysql" {
		q = qTablesMySQL
	}
	_, rows, err := c.query(ctx, q, nil, false, false)
	if err != nil {
		return "", err
	}
	b := strings.Builder{}
	for _, r := range rows {
		schema := ""
		if len(r) > 0 {
			schema = r[0]
		}
		name := ""
		if len(r) > 1 {
			name = r[1]
		}
		if schema == name {
			fmt.Fprintf(&b, "- %s\n", name)
			continue
		}
		if schema != "" {
			fmt.Fprintf(&b, "- %s.%s\n", schema, name)
			continue
		}
		fmt.Fprintf(&b, "- %s\n", name)
	}
	if b.Len() == 0 {
		return "Nenhuma tabela encontrada (verifique o usuário/conexão).", nil
	}
	return b.String(), nil
}

const qTables = `
	SELECT
		table_schema,
		table_name
	FROM information_schema.tables
	WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		AND table_type = 'BASE TABLE'
	ORDER BY table_schema, table_name
`

const qTablesMySQL = `
	SELECT
		table_schema,
		table_name
	FROM information_schema.tables
	WHERE table_schema = DATABASE()
		AND table_type = 'BASE TABLE'
	ORDER BY table_name
`

func (c *liveDB) structure(ctx context.Context, table string) (string, error) {
	if strings.TrimSpace(table) == "" {
		return c.tables(ctx)
	}
	schema := ""
	if i := strings.Index(table, "."); i >= 0 {
		schema = table[:i]
		table = table[i+1:]
	}
	colsQ, fkQ, idxQ := qColsMySQL, qFKMySQL, qIdxMySQL
	if c.driver == "pgx" {
		colsQ, fkQ, idxQ = qColsPg, qFKPg, qIdxPg
	}
	args := []string{table, schema}

	engine := "postgres"
	if c.driver == "mysql" {
		engine = "mysql"
	}
	display := schema
	if display == "" {
		if engine == "postgres" {
			display = "public"
		}
	}
	if display == "" {
		if engine == "mysql" {
			_, r, e := c.query(ctx, "SELECT DATABASE()", nil, false, true)
			if e == nil {
				if len(r) > 0 {
					if len(r[0]) > 0 {
						display = r[0][0]
					}
				}
			}
		}
	}

	st := structTable{Schema: display, Name: table, Engine: engine}

	_, rows, err := c.query(ctx, colsQ, args, false, false)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return fmt.Sprintf("Tabela %q não encontrada.", table), nil
	}
	for _, r := range rows {
		name := ""
		if len(r) > 0 {
			name = r[0]
		}
		typ := ""
		if len(r) > 1 {
			typ = r[1]
		}
		nullable := ""
		if len(r) > 2 {
			nullable = r[2]
		}
		def := ""
		if len(r) > 3 {
			def = r[3]
		}
		pk := ""
		if len(r) > 4 {
			pk = r[4]
		}
		auto := ""
		if len(r) > 5 {
			auto = r[5]
		}
		st.Cols = append(st.Cols, structColumn{
			Name:    name,
			Type:    typ,
			NotNull: notNull(nullable),
			Default: def,
			PK:      isPkVal(pk),
			Auto:    isTruthy(auto),
		})
	}

	fkByCol := map[string]structColumn{}
	if _, rows, err := c.query(ctx, fkQ, args, false, false); err == nil {
		for _, r := range rows {
			if len(r) == 0 {
				continue
			}
			fk := structColumn{}
			col := r[0]
			if len(r) > 1 {
				fk.RefSchema = r[1]
			}
			if len(r) > 2 {
				fk.RefTable = r[2]
			}
			if len(r) > 3 {
				fk.RefColumn = r[3]
			}
			if _, ok := fkByCol[col]; !ok {
				fkByCol[col] = fk
			}
		}
	}
	for i := range st.Cols {
		if fk, ok := fkByCol[st.Cols[i].Name]; ok {
			st.Cols[i].RefSchema = fk.RefSchema
			st.Cols[i].RefTable = fk.RefTable
			st.Cols[i].RefColumn = fk.RefColumn
		}
	}

	if _, rows, err := c.query(ctx, idxQ, args, false, false); err == nil {
		for _, r := range rows {
			if len(r) == 0 {
				continue
			}
			ix := structIdx{}
			colsStr := ""
			if c.driver == "pgx" {
				if len(r) > 1 {
					ix.Unique = isTruthy(r[1])
				}
				if len(r) > 2 {
					ix.IsPK = isTruthy(r[2])
				}
				if len(r) > 3 {
					colsStr = r[3]
				}
			}
			if c.driver != "pgx" {
				if len(r) > 1 {
					colsStr = r[1]
				}
				if len(r) > 2 {
					ix.Unique = r[2] == "0"
				}
				if len(r) > 3 {
					ix.IsPK = isTruthy(r[3])
				}
			}
			if colsStr != "" {
				for _, col := range strings.Split(colsStr, ",") {
					col = strings.TrimSpace(col)
					if col == "" {
						continue
					}
					ix.Columns = append(ix.Columns, col)
				}
			}
			st.Idx = append(st.Idx, ix)
		}
	}

	return renderStructureTable(st), nil
}

const qColsPg = `
	SELECT
		c.column_name,
		c.data_type,
		c.is_nullable,
		c.column_default,
		EXISTS (
			SELECT 1
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON tc.constraint_name = kcu.constraint_name
				AND tc.table_schema = kcu.table_schema
				AND tc.table_name = kcu.table_name
			WHERE tc.table_name = c.table_name
				AND tc.table_schema = c.table_schema
				AND tc.constraint_type = 'PRIMARY KEY'
				AND kcu.column_name = c.column_name
		) AS is_pk,
		(c.is_identity = 'YES' OR c.column_default LIKE 'nextval(%') AS is_auto
	FROM information_schema.columns c
	WHERE c.table_name = $1
		AND c.table_schema = COALESCE(NULLIF($2, ''), 'public')
	ORDER BY c.ordinal_position
`

const qFKPg = `
	SELECT
		att.attname AS col,
		fns.nspname AS ref_schema,
		ft.relname AS ref_table,
		fatt.attname AS ref_col
	FROM pg_constraint con
	JOIN pg_class rel ON rel.oid = con.conrelid
	JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace
	JOIN pg_attribute att ON att.attrelid = con.conrelid AND att.attnum = ANY(con.conkey)
	JOIN pg_class ft ON ft.oid = con.confrelid
	JOIN pg_namespace fns ON fns.oid = ft.relnamespace
	JOIN pg_attribute fatt ON fatt.attrelid = con.confrelid AND fatt.attnum = ANY(con.confkey)
	WHERE con.contype = 'f'
		AND rel.relname = $1
		AND nsp.nspname = COALESCE(NULLIF($2, ''), 'public')
	ORDER BY con.conname, att.attnum
`

const qIdxPg = `
	SELECT
		i.relname,
		ix.indisunique,
		ix.indisprimary,
		string_agg(a.attname, ',' ORDER BY k.ord)
	FROM pg_index ix
	JOIN pg_class i ON i.oid = ix.indexrelid
	JOIN pg_class t ON t.oid = ix.indrelid
	JOIN pg_namespace n ON n.oid = t.relnamespace
	JOIN LATERAL unnest(ix.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord) ON true
	JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum AND k.attnum <> 0
	WHERE t.relname = $1
		AND n.nspname = COALESCE(NULLIF($2, ''), 'public')
	GROUP BY i.relname, ix.indisunique, ix.indisprimary
	ORDER BY i.relname
`

const qColsMySQL = `
	SELECT
		column_name,
		data_type,
		is_nullable,
		column_default,
		(column_key = 'PRI') AS is_pk,
		(LOWER(extra) LIKE '%auto_increment%') AS is_auto
	FROM information_schema.columns
	WHERE table_name = ?
		AND table_schema = COALESCE(NULLIF(?, ''), DATABASE())
	ORDER BY ordinal_position
`

const qFKMySQL = `
	SELECT
		column_name AS col,
		referenced_table_schema AS ref_schema,
		referenced_table_name AS ref_table,
		referenced_column_name AS ref_col
	FROM information_schema.key_column_usage
	WHERE table_name = ?
		AND table_schema = COALESCE(NULLIF(?, ''), DATABASE())
		AND referenced_table_name IS NOT NULL
	ORDER BY constraint_name, ordinal_position
`

const qIdxMySQL = `
	SELECT
		index_name,
		GROUP_CONCAT(column_name ORDER BY seq_in_index),
		MIN(non_unique),
		(index_name = 'PRIMARY')
	FROM information_schema.statistics
	WHERE table_name = ?
		AND table_schema = COALESCE(NULLIF(?, ''), DATABASE())
	GROUP BY index_name
	ORDER BY index_name
`
