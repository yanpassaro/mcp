package sqlize

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const liveHardLimit = 500

func enforceLimit(q string, hard int) string {
	if m := reLimit.FindStringSubmatch(q); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n > hard {
			return reLimit.ReplaceAllString(q, fmt.Sprintf("LIMIT %d", hard))
		}
		return q
	}
	if reLimitPH.MatchString(q) {
		return reLimitPH.ReplaceAllString(q, fmt.Sprintf("LIMIT LEAST($1, %d)", hard))
	}
	return strings.TrimRight(q, " \n\t;") + " LIMIT " + strconv.Itoa(hard)
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
	for i >= 0 && (q[i] == ' ' || q[i] == '\t' || q[i] == '\n' || q[i] == '\r') {
		i--
	}
	j := i + 1
	for i >= 0 && (isWordChar(rune(q[i]))) {
		i--
	}
	return strings.ToLower(q[i+1 : j])
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func checkFuncAllowlist(q string) error {
	for _, m := range reFuncCall.FindAllStringSubmatchIndex(q, -1) {
		start, end := m[2], m[3]
		name := strings.ToLower(q[start:end])
		if i := strings.LastIndexByte(name, '.'); i >= 0 {
			name = name[i+1:]
		}
		if sqlFuncToken[name] || sqlFuncAllowlist[name] {
			continue
		}
		if tableKeywords[prevWord(q, start)] {
			continue
		}
		return fmt.Errorf("consulta rejeitada: chamada de função %q não está na allowlist; apenas funções embutidas comuns são permitidas (COUNT, SUM, TO_CHAR, COALESCE, DATE_TRUNC...)", q[start:end])
	}
	return nil
}

func hasPlaceholders(q, driver string) bool {
	if driver == "mysql" {
		return strings.Contains(q, "?")
	}
	return rePgPlaceholder.MatchString(q)
}

func validateLiveQuery(q string, args []string, driver string, strict bool) (string, error) {
	clean, err := sanitizeReadQuery(q)
	if err != nil {
		return "", err
	}
	if strict && reStringInWhere.MatchString(clean) {
		return "", fmt.Errorf("consulta rejeitada: não inclua literais de string na cláusula WHERE; passe valores dinâmicos via 'args' (use ? no MySQL ou $1.. no Postgres)")
	}
	if strict && reWhereLiteral.MatchString(clean) && !hasPlaceholders(clean, driver) {
		return "", fmt.Errorf("consulta rejeitada: cláusula WHERE compara valores; parametrize com ? (MySQL) ou $1.. (Postgres) e passe os valores em 'args' (ex.: WHERE id = $1)")
	}
	if len(args) > 0 && !hasPlaceholders(clean, driver) {
		return "", fmt.Errorf("'args' informado mas o SQL não contém placeholders; use $1.. (Postgres) ou ? (MySQL) e passe os valores em 'args'")
	}
	if reInjection.MatchString(clean) {
		return "", fmt.Errorf("consulta rejeitada: padrão suspeito de injeção de SQL (aspas seguidas de comentário ou de UNION/INTO); passe valores dinâmicos via 'args'")
	}
	if strict {
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
	name := c.Engine
	if c.Alias != "" {
		name += "_" + c.Alias
	}
	return name
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
		k := engine + "|" + alias
		if prev, exists := byKey[k]; exists && prev.Kind == "url" {
			continue
		}
		byKey[k] = liveDBConfig{Engine: engine, Alias: alias, EnvVar: key, Kind: kind, DSN: strings.TrimSpace(val)}
	}
	out := make([]liveDBConfig, 0, len(byKey))
	for _, cfg := range byKey {
		out = append(out, cfg)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Engine != out[j].Engine {
			return out[i].Engine < out[j].Engine
		}
		return out[i].Alias < out[j].Alias
	})
	return out
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
			suffix := "_" + upper + "_" + k
			if strings.HasSuffix(up, suffix) {
				return engine, strings.TrimSuffix(up, suffix), strings.ToLower(k), true
			}
		}
	}
	return "", "", "", false
}

func normalizeAlias(prefix string) string {
	a := strings.ToLower(strings.TrimSpace(prefix))
	if a == "" || a == "db" {
		return ""
	}
	var b strings.Builder
	for _, r := range a {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
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
		clean = enforceLimit(clean, liveHardLimit)
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
	var q string
	switch c.driver {
	case "pgx":
		q = "SELECT table_schema, table_name FROM information_schema.tables " +
			"WHERE table_schema NOT IN ('pg_catalog','information_schema') AND table_type='BASE TABLE' ORDER BY table_schema, table_name"
	case "mysql":
		q = "SELECT table_schema, table_name FROM information_schema.tables " +
			"WHERE table_schema = DATABASE() AND table_type='BASE TABLE' ORDER BY table_name"
	}
	cols, rows, err := c.query(ctx, q, nil, false, false)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, r := range RedactRows(cols, rows) {
		schema, name := "", ""
		if len(r) > 0 {
			schema = r[0]
		}
		if len(r) > 1 {
			name = r[1]
		}
		if schema != "" && schema != name {
			fmt.Fprintf(&b, "- %s.%s\n", schema, name)
		} else {
			fmt.Fprintf(&b, "- %s\n", name)
		}
	}
	if b.Len() == 0 {
		return "Nenhuma tabela encontrada (verifique o usuário/conexão).", nil
	}
	return b.String(), nil
}

func (c *liveDB) structure(ctx context.Context, table string) (string, error) {
	if strings.TrimSpace(table) == "" {
		return c.tables(ctx)
	}
	schema := ""
	if i := strings.Index(table, "."); i >= 0 {
		schema = table[:i]
		table = table[i+1:]
	}
	var colsQ, fkQ, idxQ string
	var args []string
	if c.driver == "pgx" {
		schemaExpr := "COALESCE(NULLIF($2,''), 'public')"
		colsQ = "SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1 AND table_schema = " + schemaExpr + " ORDER BY ordinal_position"
		fkQ = "SELECT att.attname AS col, fns.nspname AS ref_schema, ft.relname AS ref_table, fatt.attname AS ref_col " +
			"FROM pg_constraint con " +
			"JOIN pg_class rel ON rel.oid = con.conrelid " +
			"JOIN pg_namespace nsp ON nsp.oid = rel.relnamespace " +
			"JOIN pg_attribute att ON att.attrelid = con.conrelid AND att.attnum = ANY(con.conkey) " +
			"JOIN pg_class ft ON ft.oid = con.confrelid " +
			"JOIN pg_namespace fns ON fns.oid = ft.relnamespace " +
			"JOIN pg_attribute fatt ON fatt.attrelid = con.confrelid AND fatt.attnum = ANY(con.confkey) " +
			"WHERE con.contype = 'f' AND rel.relname = $1 AND nsp.nspname = " + schemaExpr + " ORDER BY con.conname, att.attnum"
		idxQ = "SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = " + schemaExpr + " AND tablename = $1 ORDER BY indexname"
		args = []string{table, schema}
	} else {
		schemaExpr := "COALESCE(NULLIF(?,''), DATABASE())"
		colsQ = "SELECT column_name, data_type FROM information_schema.columns WHERE table_name = ? AND table_schema = " + schemaExpr + " ORDER BY ordinal_position"
		fkQ = "SELECT column_name AS col, referenced_table_schema AS ref_schema, referenced_table_name AS ref_table, referenced_column_name AS ref_col " +
			"FROM information_schema.key_column_usage WHERE table_name = ? AND table_schema = " + schemaExpr + " AND referenced_table_name IS NOT NULL " +
			"ORDER BY constraint_name, ordinal_position"
		idxQ = "SELECT index_name, GROUP_CONCAT(column_name ORDER BY seq_in_index) AS cols, MIN(non_unique) AS non_unique " +
			"FROM information_schema.statistics WHERE table_name = ? AND table_schema = " + schemaExpr + " GROUP BY index_name ORDER BY index_name"
		args = []string{table, schema}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n", table)
	cols, rows, err := c.query(ctx, colsQ, args, false, false)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return fmt.Sprintf("Tabela %q não encontrada.", table), nil
	}
	for _, r := range RedactRows(cols, rows) {
		name, typ := "", ""
		if len(r) > 0 {
			name = r[0]
		}
		if len(r) > 1 {
			typ = r[1]
		}
		fmt.Fprintf(&b, "- %s: %s\n", short(name), short(typ))
	}

	if cols, rows, err := c.query(ctx, fkQ, args, false, false); err == nil && len(rows) > 0 {
		b.WriteString("\nFks:\n")
		seen := map[string]bool{}
		for _, r := range RedactRows(cols, rows) {
			if len(r) == 0 {
				continue
			}
			col, refSC, refT, refC := r[0], "", "", ""
			if len(r) > 1 {
				refSC = r[1]
			}
			if len(r) > 2 {
				refT = r[2]
			}
			if len(r) > 3 {
				refC = r[3]
			}
			key := col + "|" + refSC + "|" + refT + "|" + refC
			if seen[key] {
				continue
			}
			seen[key] = true
			ref := refC
			if refT != "" {
				ref = refT + "." + refC
			}
			if refSC != "" && ref != "" {
				ref = refSC + "." + ref
			}
			fmt.Fprintf(&b, "- %s → %s\n", short(col), short(ref))
		}
	}

	if cols, rows, err := c.query(ctx, idxQ, args, false, false); err == nil && len(rows) > 0 {
		b.WriteString("\nÍndices:\n")
		for _, r := range RedactRows(cols, rows) {
			if len(r) == 0 {
				continue
			}
			name := r[0]
			if c.driver == "pgx" {
				fmt.Fprintf(&b, "- %s\n", short(name))
				if len(r) > 1 && r[1] != "" {
					fmt.Fprintf(&b, "  `%s`\n", short(r[1]))
				}
			} else {
				colsStr := ""
				uniq := ""
				if len(r) > 1 {
					colsStr = short(r[1])
				}
				if len(r) > 2 && r[2] == "0" {
					uniq = " (único)"
				}
				fmt.Fprintf(&b, "- %s%s: %s\n", short(name), uniq, colsStr)
			}
		}
	}
	return b.String(), nil
}
