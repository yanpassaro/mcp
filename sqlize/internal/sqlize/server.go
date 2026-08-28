package sqlize

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	store *store
}

func New(stateDir string) (*Server, error) {
	st, err := newStore(stateDir)
	if err != nil {
		return nil, err
	}
	return &Server{store: st}, nil
}

func (s *Server) Close() error {
	return s.store.Close()
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_import",
		Description: "Import a file into the working SQLite database (stored under .local/state/sqlize). Supported formats: .json, .csv, .tsv, .xlsx, .sql, .sqlite, .db, .xml. For individual tables set 'table'; for .sqlite/.db the file is attached as a schema (use sqlize_structure to list its tables).",
	}, s.importTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_structure",
		Description: "Show the structure of the imported data. Without 'table', lists every table and its columns (sqlite or individual table). With 'table', shows that table's columns.",
	}, s.structureTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_query",
		Description: "Run a SQL statement (SELECT/WITH/INSERT/UPDATE/DELETE) over the local SQLite test database and return a Markdown table (for queries) or the affected-row count (for writes), limited to 200 rows. Do not use ';'. Values must be passed via 'args' as bound parameters - string literals and numeric/boolean literals inside WHERE are rejected (use '?' placeholders + 'args'; IS NULL / column-to-column comparisons are fine). Format/constant strings outside WHERE (TO_CHAR 'YYYY-MM-DD', COALESCE(..,''), CASE) are allowed. Only built-in functions from an allowlist (COUNT, SUM, TO_CHAR, COALESCE, DATE_TRUNC, ...) can be called; user-defined, extension or dangerous functions (pg_sleep, dblink, LOAD_FILE, ...) are rejected. Results are masked (CPF, CNPJ, e-mail, phone, card, dates, IP, PII) by default; set 'redact' false to disable.",
	}, s.queryTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_export",
		Description: "Export the result of a query or a table to a file. Output formats: .json, .csv, .tsv, .xlsx, .sql, .html, .xml (determined by the 'path' extension).",
	}, s.exportTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_compare",
		Description: "Compare two tables or two SELECT queries in the local SQLite database by a key column. Shows a summary per status (only in A / only in B / differing rows) plus the details per key. 'key' is mandatory (comma separated for composite keys); provide one of table_a/query_a and one of table_b/query_b. Results are masked except 'redact': false.",
	}, s.compareTool)

	for _, cfg := range discoverLiveDBs() {
		prefix := cfg.ToolPrefix()
		env := cfg.EnvVar
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_query",
			Description: fmt.Sprintf("Run a read-only SQL query (SELECT or WITH) against the live %s database configured in %s. Executed inside a READ ONLY transaction; writes are impossible. By default a hard LIMIT of 500 rows is enforced. Results are ALWAYS masked (CPF, CNPJ, e-mail, phone, card, IP, plus PII column names; generic dates are NOT masked). Pass dynamic values via 'args' as bound parameters (%s); string and numeric/boolean literals inside WHERE are rejected (use placeholders + 'args'; IS NULL / column-to-column comparisons are fine; format/constant strings outside WHERE are allowed). Only built-in functions from an allowlist (COUNT, SUM, TO_CHAR, COALESCE, DATE_TRUNC, ...) can be called; user-defined, extension or dangerous functions (pg_sleep, dblink, LOAD_FILE, ...) are rejected. 'export_to' writes the full result to a file (.csv, .html, .xlsx, .tsv, .json, .xml, .sql) and 'all' bypasses the 500-row limit, but only together with 'export_to'. For .sql exports 'target_table' sets the name used in the script (defaults to 'exported').", cfg.Engine, env, livePlaceholders(cfg.Engine)),
		}, s.liveQueryHandler(cfg))
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_tables",
			Description: fmt.Sprintf("List tables of the live %s database (%s). Output is always redacted.", cfg.Engine, env),
		}, s.liveTablesHandler(cfg))
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_schema",
			Description: fmt.Sprintf("Describe the columns (name + type) of a %s table (%s). Use 'schema.table' or just 'table'. Output is always redacted.", cfg.Engine, env),
		}, s.liveSchemaHandler(cfg))
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_explain",
			Description: fmt.Sprintf("Explain a SQL query (SELECT or WITH) against the live %s database configured in %s WITHOUT executing it: shows the %s execution plan (EXPLAIN). Read-only and safe — literals are allowed (the query is not executed) but placeholders are not (EXPLAIN has no bound parameters).", cfg.Engine, env, cfg.Engine),
		}, s.liveExplainHandler(cfg))
	}
}

func livePlaceholders(engine string) string {
	if engine == "mysql" {
		return "?"
	}
	return "$1, $2, ..."
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

type importInput struct {
	Path  string `json:"path" jsonschema:"Path of the input file (.json, .csv, .tsv, .xlsx, .sql, .sqlite, .db, .xml)"`
	Table string `json:"table,omitempty" jsonschema:"Name of the destination table (optional; defaults to the file name without extension; with 'sheet' this names the single imported table)"`
	Sheet string `json:"sheet,omitempty" jsonschema:"Excel sheet name to import (optional, Excel only; defaults to all sheets)"`
}

func (s *Server) importTool(ctx context.Context, _ *mcp.CallToolRequest, in importInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Path) == "" {
		return nil, nil, fmt.Errorf("'path' é obrigatório")
	}
	res, err := s.store.importFile(ctx, in.Path, in.Table, in.Sheet)
	if err != nil {
		return nil, nil, err
	}
	return textResult(res)
}

type structureInput struct {
	Table string `json:"table,omitempty" jsonschema:"Specific table (optional). If omitted, lists all tables and columns."`
}

func (s *Server) structureTool(ctx context.Context, _ *mcp.CallToolRequest, in structureInput) (*mcp.CallToolResult, any, error) {
	tables, err := s.store.listTables(ctx)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.Table) != "" {
		var found []tableInfo
		for _, t := range tables {
			if t.Name == in.Table {
				found = append(found, t)
			}
		}
		if len(found) == 0 {
			return nil, nil, fmt.Errorf("tabela %q não encontrada", in.Table)
		}
		var b strings.Builder
		for _, t := range found {
			cols, err := s.store.tableColumns(ctx, t.Schema, t.Name)
			if err != nil {
				return nil, nil, err
			}
			fmt.Fprintf(&b, "### %s (esquema %s)\n", t.Name, schemaLabel(t.Schema))
			for _, c := range cols {
				fmt.Fprintf(&b, "- %s: %s\n", c.Name, c.Type)
			}
			b.WriteString("\n")
		}
		return textResult(b.String())
	}
	if len(tables) == 0 {
		return textResult("Nenhuma tabela importada ainda. Use sqlize_import para carregar um arquivo.")
	}
	var b strings.Builder
	for _, t := range tables {
		cols, err := s.store.tableColumns(ctx, t.Schema, t.Name)
		if err != nil {
			return nil, nil, err
		}
		fmt.Fprintf(&b, "### %s (esquema %s) — %d colunas\n", t.Name, schemaLabel(t.Schema), len(cols))
		names := make([]string, len(cols))
		for i, c := range cols {
			names[i] = c.Name + ": " + c.Type
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n\n")
	}
	return textResult(b.String())
}

func schemaLabel(s string) string {
	if s == "" {
		return "main"
	}
	return s
}

type queryInput struct {
	SQL    string   `json:"sql" jsonschema:"SQL statement (SELECT/WITH/INSERT/UPDATE/DELETE) on the local SQLite test database. Do not use ';'. Values must be passed via 'args' (no inline literals)."`
	Args   []string `json:"args,omitempty" jsonschema:"Bound parameters for the statement (optional). Passed as parameters, never interpolated into the SQL. Required for any value (no inline string literals allowed)."`
	Redact *bool    `json:"redact,omitempty" jsonschema:"Apply partial masking to sensitive data (CPF, CNPJ, e-mail, phone, CEP, RG, card, dates, etc.). Default: true. Set false to disable."`
}

func (s *Server) queryTool(ctx context.Context, _ *mcp.CallToolRequest, in queryInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.SQL) == "" {
		return nil, nil, fmt.Errorf("'sql' é obrigatório")
	}
	doRedact := in.Redact == nil || *in.Redact
	res, err := s.store.runQuery(ctx, in.SQL, in.Args, doRedact)
	if err != nil {
		return nil, nil, err
	}
	return textResult(res)
}

type exportInput struct {
	Path   string `json:"path" jsonschema:"Path of the output file (.json, .csv, .tsv, .xlsx, .sql, .xml)"`
	Query  string `json:"query,omitempty" jsonschema:"Source SQL query (optional if 'table' is provided)"`
	Table  string `json:"table,omitempty" jsonschema:"Source table name (optional if 'query' is provided)"`
	Target string `json:"target_table,omitempty" jsonschema:"Table name used in the exported SQL (optional; defaults to 'exported')"`
	Redact *bool  `json:"redact,omitempty" jsonschema:"Apply partial masking to sensitive data (CPF, CNPJ, e-mail, phone, CEP, RG, card, dates, etc.). Default: true. Set false to disable."`
}

func (s *Server) exportTool(ctx context.Context, _ *mcp.CallToolRequest, in exportInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Path) == "" {
		return nil, nil, fmt.Errorf("'path' de saída é obrigatório")
	}
	target := in.Target
	if strings.TrimSpace(target) == "" {
		target = "exported"
	}
	doRedact := in.Redact == nil || *in.Redact
	res, err := s.store.exportFile(ctx, in.Path, in.Query, in.Table, target, doRedact)
	if err != nil {
		return nil, nil, err
	}
	return textResult(res)
}

type compareInput struct {
	Key    string `json:"key" jsonschema:"Column(s) used as the comparison key, comma separated (mandatory)."`
	TableA string `json:"table_a,omitempty" jsonschema:"First table (alternative to query_a)."`
	QueryA string `json:"query_a,omitempty" jsonschema:"First SELECT/WITH query (alternative to table_a)."`
	TableB string `json:"table_b,omitempty" jsonschema:"Second table (alternative to query_b)."`
	QueryB string `json:"query_b,omitempty" jsonschema:"Second SELECT/WITH query (alternative to table_b)."`
	Redact *bool  `json:"redact,omitempty" jsonschema:"Apply partial masking to sensitive data. Default: true."`
}

func compareSource(table, query, label string) (string, error) {
	t, q := strings.TrimSpace(table), strings.TrimSpace(query)
	switch {
	case t != "" && q != "":
		return "", fmt.Errorf("informe apenas um de %s", label)
	case t != "":
		return "SELECT * FROM " + quoteIdent(t), nil
	case q != "":
		clean, err := sanitizeStoreQuery(q, nil)
		if err != nil {
			return "", err
		}
		return clean, nil
	default:
		return "", fmt.Errorf("informe %s (tabela ou query)", label)
	}
}

func (s *Server) columnNames(ctx context.Context, q string) ([]string, error) {
	cols, _, err := s.store.query(ctx, "SELECT * FROM ("+q+") _src LIMIT 0")
	return cols, err
}

func (s *Server) compareTool(ctx context.Context, _ *mcp.CallToolRequest, in compareInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Key) == "" {
		return nil, nil, fmt.Errorf("'key' é obrigatório (coluna(s) separadas por vírgula)")
	}
	qa, err := compareSource(in.TableA, in.QueryA, "table_a/query_a")
	if err != nil {
		return nil, nil, err
	}
	qb, err := compareSource(in.TableB, in.QueryB, "table_b/query_b")
	if err != nil {
		return nil, nil, err
	}
	colsA, err := s.columnNames(ctx, qa)
	if err != nil {
		return nil, nil, fmt.Errorf("colunas de A: %w", err)
	}
	colsB, err := s.columnNames(ctx, qb)
	if err != nil {
		return nil, nil, fmt.Errorf("colunas de B: %w", err)
	}
	if len(colsA) != len(colsB) {
		return nil, nil, fmt.Errorf("as fontes têm colunas diferentes (A: %s; B: %s)", strings.Join(colsA, ", "), strings.Join(colsB, ", "))
	}
	for i := range colsA {
		if colsA[i] != colsB[i] {
			return nil, nil, fmt.Errorf("coluna %d difere: A tem %q e B tem %q", i+1, colsA[i], colsB[i])
		}
	}
	keyCols := make([]string, 0)
	for k := range strings.SplitSeq(in.Key, ",") {
		if k = strings.TrimSpace(k); k != "" {
			keyCols = append(keyCols, k)
		}
	}
	if len(keyCols) == 0 {
		return nil, nil, fmt.Errorf("'key' vazia")
	}
	keySet := map[string]bool{}
	for _, k := range keyCols {
		keySet[k] = true
		found := false
		for _, c := range colsA {
			if c == k {
				found = true
				break
			}
		}
		if !found {
			return nil, nil, fmt.Errorf("coluna chave %q não existe nas fontes (disponíveis: %s)", k, strings.Join(colsA, ", "))
		}
	}
	dataCols := make([]string, 0)
	for _, c := range colsA {
		if !keySet[c] {
			dataCols = append(dataCols, c)
		}
	}
	parts := make([]string, 0, len(keyCols)+2)
	for _, k := range keyCols {
		parts = append(parts, "COALESCE(a."+quoteIdent(k)+", b."+quoteIdent(k)+") AS "+quoteIdent(k))
	}
	status := "'igual'"
	if len(dataCols) > 0 {
		preds := make([]string, 0, len(dataCols))
		for _, c := range dataCols {
			preds = append(preds, "a."+quoteIdent(c)+" IS NOT b."+quoteIdent(c))
		}
		status = "CASE WHEN (" + strings.Join(preds, " OR ") + ") THEN 'diferente' ELSE 'igual' END"
	}
	status = "CASE WHEN a." + quoteIdent(keyCols[0]) + " IS NULL THEN 'só-em-B' WHEN b." + quoteIdent(keyCols[0]) + " IS NULL THEN 'só-em-A' ELSE " + status + " END AS status"
	joins := make([]string, 0, len(keyCols))
	for _, k := range keyCols {
		joins = append(joins, "a."+quoteIdent(k)+" = b."+quoteIdent(k))
	}
	withClause := "WITH _a AS (" + qa + "), _b AS (" + qb + ") "
	orderAux := ""
	if len(keyCols) > 1 {
		orderAux = ", 2"
	}
	body := "SELECT " + strings.Join(parts, ", ") + ", " + status + " " +
		"FROM _a a FULL OUTER JOIN _b b ON " + strings.Join(joins, " AND ") + " " +
		"ORDER BY 1" + orderAux + ", " + strconv.Itoa(len(keyCols)+1)
	diff := withClause + body

	doRedact := in.Redact == nil || *in.Redact
	agg, err := s.store.runQuery(ctx,
		withClause+"SELECT status, COUNT(*) AS qtd FROM ("+body+") GROUP BY status "+
			"ORDER BY CASE status WHEN 'só-em-A' THEN 1 WHEN 'só-em-B' THEN 2 WHEN 'diferente' THEN 3 ELSE 4 END",
		nil, doRedact)
	if err != nil {
		return nil, nil, err
	}
	detail, err := s.store.runQuery(ctx, diff, nil, doRedact)
	if err != nil {
		return nil, nil, err
	}
	var out strings.Builder
	fmt.Fprintf(&out, "## Comparação (chave: %s)\n\n%s\n\n## Detalhe por chave\n\n%s", strings.Join(keyCols, ", "), agg, detail)
	return textResult(out.String())
}

const liveMaxRows = 200

func (s *Server) renderLiveQuery(ctx context.Context, cfg liveDBConfig, q string, args []string, exportTo, targetTable string, all bool) (string, error) {
	if strings.TrimSpace(q) == "" {
		return "", fmt.Errorf("'sql' é obrigatório")
	}
	if all && strings.TrimSpace(exportTo) == "" {
		return "", fmt.Errorf("'all' só pode ser usado junto com 'export_to'")
	}
	c, err := newLiveDB(cfg)
	if err != nil {
		return "", err
	}
	cols, rows, err := c.query(ctx, q, args, true, all)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(exportTo) != "" {
		target := strings.TrimSpace(targetTable)
		if target == "" {
			target = "exported"
		}
		return exportLiveFile(exportTo, cols, rows, target)
	}
	return renderRedactedTable(cols, rows, liveMaxRows), nil
}

func renderRedactedTable(cols []string, rows [][]string, max int) string {
	shown := rows
	if len(rows) > max {
		shown = rows[:max]
	}
	red := RedactRows(cols, shown)
	var b strings.Builder
	b.WriteString(markdownTable(cols, red))
	if len(rows) > max {
		fmt.Fprintf(&b, "\n... %d linhas no total (mostrando %d, mascaradas).\n", len(rows), max)
	} else {
		fmt.Fprintf(&b, "\n%d linha(s) (mascaradas).\n", len(rows))
	}
	return b.String()
}

func (s *Server) liveQueryHandler(cfg liveDBConfig) func(context.Context, *mcp.CallToolRequest, liveQueryInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in liveQueryInput) (*mcp.CallToolResult, any, error) {
		res, err := s.renderLiveQuery(ctx, cfg, in.SQL, in.Args, strings.TrimSpace(in.ExportTo), in.TargetTable, in.All)
		if err != nil {
			return nil, nil, err
		}
		return textResult(res)
	}
}

func (s *Server) liveTablesHandler(cfg liveDBConfig) func(context.Context, *mcp.CallToolRequest, liveConnInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ liveConnInput) (*mcp.CallToolResult, any, error) {
		c, err := newLiveDB(cfg)
		if err != nil {
			return nil, nil, err
		}
		res, err := c.tables(ctx)
		if err != nil {
			return nil, nil, err
		}
		return textResult(res)
	}
}

func (s *Server) liveSchemaHandler(cfg liveDBConfig) func(context.Context, *mcp.CallToolRequest, liveTableInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in liveTableInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.Table) == "" {
			return nil, nil, fmt.Errorf("'table' é obrigatório")
		}
		c, err := newLiveDB(cfg)
		if err != nil {
			return nil, nil, err
		}
		res, err := c.schema(ctx, in.Table)
		if err != nil {
			return nil, nil, err
		}
		return textResult(res)
	}
}

type liveQueryInput struct {
	SQL         string   `json:"sql" jsonschema:"Read-only SQL query (SELECT or WITH). Do not use ';'. Executed in a READ ONLY transaction; writes are rejected."`
	Args        []string `json:"args,omitempty" jsonschema:"Bound parameters for the query (optional). Passed as parameters, never interpolated into the SQL."`
	ExportTo    string   `json:"export_to,omitempty" jsonschema:"Output path to write the full result to a file instead of the Markdown table. The file extension defines the format: .csv, .html, .xlsx, .tsv, .json, .xml, .sql. The result is always masked."`
	All         bool     `json:"all,omitempty" jsonschema:"Return all rows, bypassing the 500-row hard limit. Only allowed together with 'export_to'."`
	TargetTable string   `json:"target_table,omitempty" jsonschema:"Table name used in the exported SQL script (only relevant for .sql exports; defaults to 'exported')."`
}

type liveExplainInput struct {
	SQL string `json:"sql" jsonschema:"Read-only SQL query (SELECT or WITH) to explain. Do not use ';'."`
}

func (s *Server) liveExplainHandler(cfg liveDBConfig) func(context.Context, *mcp.CallToolRequest, liveExplainInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in liveExplainInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.SQL) == "" {
			return nil, nil, fmt.Errorf("'sql' é obrigatório")
		}
		clean, err := sanitizeReadQuery(in.SQL)
		if err != nil {
			return nil, nil, err
		}
		if rePgPlaceholder.MatchString(clean) || strings.Contains(clean, "?") {
			return nil, nil, fmt.Errorf("EXPLAIN não aceita placeholders; use valores concretos no SQL (a query não é executada, então literais são seguros)")
		}
		c, err := newLiveDB(cfg)
		if err != nil {
			return nil, nil, err
		}
		plan, err := c.explain(ctx, clean)
		if err != nil {
			return nil, nil, fmt.Errorf("executar EXPLAIN: %s", scrubInfra(err))
		}
		return textResult(plan)
	}
}

type liveConnInput struct{}

type liveTableInput struct {
	Table string `json:"table" jsonschema:"Table name, optionally qualified as 'schema.table'."`
}

func cleanCell(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "/")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func markdownTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return "Sem colunas."
	}
	var b strings.Builder
	b.WriteString("| ")
	b.WriteString(strings.Join(headers, " | "))
	b.WriteString(" |\n")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	b.WriteString("| ")
	b.WriteString(strings.Join(seps, " | "))
	b.WriteString(" |\n")
	for _, row := range rows {
		cells := make([]string, len(headers))
		for i := range headers {
			if i < len(row) {
				cells[i] = cleanCell(row[i])
			} else {
				cells[i] = ""
			}
		}
		b.WriteString("| ")
		b.WriteString(strings.Join(cells, " | "))
		b.WriteString(" |\n")
	}
	return b.String()
}
