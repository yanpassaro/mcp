package sqlize

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

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
		Description: "Import a file into the working SQLite database (stored under .local/state/sqlize). Supported formats: .json, .jsonl, .ndjson, .csv, .tsv, .xlsx, .xlsm, .xls, .sql, .sqlite, .db, .xml. For individual tables set 'table'; for .sqlite/.db the file is attached as a schema (use sqlize_structure to list its tables).",
	}, s.importTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_structure",
		Description: "Show the structure of the imported data. Without 'table', lists every table and its columns. With 'table', shows the columns plus foreign keys and indexes of that table.",
	}, s.structureTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_query",
		Description: "Run a SQL statement (SELECT/WITH/INSERT/UPDATE/DELETE) over the local SQLite test database and return a Markdown table (for queries) or the affected-row count (for writes), limited to 200 rows. Do not use ';'. Values must be passed via 'args' as bound parameters - string literals and numeric/boolean literals inside WHERE are rejected (use '?' placeholders + 'args'; IS NULL / column-to-column comparisons are fine). Format/constant strings outside WHERE (TO_CHAR 'YYYY-MM-DD', COALESCE(..,''), CASE) are allowed. Only built-in functions from an allowlist (COUNT, SUM, TO_CHAR, COALESCE, DATE_TRUNC, ...) can be called; user-defined, extension or dangerous functions (pg_sleep, dblink, LOAD_FILE, ...) are rejected. Results are ALWAYS masked (CPF, CNPJ, e-mail, phone, card, dates, IP, PII).",
	}, s.queryTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_export",
		Description: "Export the result of a query or a table to a file. Output formats: .json, .csv, .tsv, .xlsx, .sql, .html, .xml (determined by the 'path' extension). Pass dynamic values via 'args' as bound '?' parameters (required when the WHERE clause compares values; inline literals are rejected). Results are masked (CPF, CNPJ, e-mail, phone, card, dates, IP, PII) by default; set 'redact' false to export without masking.",
	}, s.exportTool)

	for _, cfg := range discoverLiveDBs() {
		prefix := cfg.ToolPrefix()
		env := cfg.EnvVar
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_query",
			Description: fmt.Sprintf("Run a read-only SQL query (SELECT or WITH) against the live %s database configured in %s. Executed inside a READ ONLY transaction; writes are impossible. By default a hard LIMIT of 500 rows is enforced. Results are ALWAYS masked (CPF, CNPJ, e-mail, phone, card, IP, plus PII column names; generic dates are NOT masked). Pass dynamic values via 'args' as bound parameters (%s); string and numeric/boolean literals inside WHERE are rejected (use placeholders + 'args'; IS NULL / column-to-column comparisons are fine; format/constant strings outside WHERE are allowed). Only built-in functions from an allowlist (COUNT, SUM, TO_CHAR, COALESCE, DATE_TRUNC, ...) can be called; user-defined, extension or dangerous functions (pg_sleep, dblink, LOAD_FILE, ...) are rejected. To write the full result to a file use %s_export.", cfg.Engine, env, livePlaceholders(cfg.Engine), prefix),
		}, s.liveQueryHandler(cfg))
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_export",
			Description: fmt.Sprintf("Run a read-only SQL query (SELECT or WITH) against the live %s database (%s) and write the FULL result to a file, always masked. The file extension defines the format: .csv, .html, .xlsx, .tsv, .json, .xml, .sql. Executed in a READ ONLY transaction; writes are impossible. 'all' bypasses the 500-row hard limit (only allowed together with 'export_to'). For .sql exports 'target_table' sets the table name used in the script (defaults to 'exported'). Uses the same security rules as %s_query (args as bound parameters, WHERE literals rejected, function allowlist).", cfg.Engine, env, prefix),
		}, s.liveExportHandler(cfg))
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_structure",
			Description: fmt.Sprintf("Structure of the live %s database (%s): lists tables when 'table' is empty; when 'table' is given (use 'schema.table' or just 'table'), shows columns + foreign keys + indexes. Output is always redacted.", cfg.Engine, env),
		}, s.liveStructureHandler(cfg))

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
	Path  string `json:"path" jsonschema:"Path of the input file (.json, .jsonl, .ndjson, .csv, .tsv, .xlsx, .xlsm, .xls, .sql, .sqlite, .db, .xml)"`
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
				fmt.Fprintf(&b, "- %s: %s\n", short(c.Name), short(c.Type))
			}
			if fks, e := s.store.tableForeignKeys(ctx, t.Schema, t.Name); e == nil && len(fks) > 0 {
				b.WriteString("\nFks:\n")
				for _, fk := range fks {
					fmt.Fprintf(&b, "- %s → %s.%s\n", short(fk.Column), short(fk.RefTable), short(fk.RefColumn))
				}
			}
			if idx, e := s.store.tableIndexes(ctx, t.Schema, t.Name); e == nil && len(idx) > 0 {
				b.WriteString("\nÍndices:\n")
				for _, ix := range idx {
					u := ""
					if ix.Unique {
						u = " (único)"
					}
					fmt.Fprintf(&b, "- %s%s: %s\n", short(ix.Name), u, short(strings.Join(ix.Columns, ", ")))
				}
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
	SQL  string   `json:"sql" jsonschema:"SQL statement (SELECT/WITH/INSERT/UPDATE/DELETE) on the local SQLite test database. Do not use ';'. Values must be passed via 'args' (no inline literals)."`
	Args []string `json:"args,omitempty" jsonschema:"Bound parameters for the statement (optional). Passed as parameters, never interpolated into the SQL. Required for any value (no inline string literals allowed)."`
}

func (s *Server) queryTool(ctx context.Context, _ *mcp.CallToolRequest, in queryInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.SQL) == "" {
		return nil, nil, fmt.Errorf("'sql' é obrigatório")
	}
	res, err := s.store.runQuery(ctx, in.SQL, in.Args)
	if err != nil {
		return nil, nil, err
	}
	return textResult(res)
}

type exportInput struct {
	Path   string   `json:"path" jsonschema:"Path of the output file (.json, .csv, .tsv, .xlsx, .sql, .html, .xml)"`
	Query  string   `json:"query,omitempty" jsonschema:"Source SQL query (optional if 'table' is provided)"`
	Args   []string `json:"args,omitempty" jsonschema:"Bound parameters for the query (optional). Passed as '?' placeholders, never interpolated into the SQL."`
	Table  string   `json:"table,omitempty" jsonschema:"Source table name (optional if 'query' is provided)"`
	Target string   `json:"target_table,omitempty" jsonschema:"Table name used in the exported SQL (optional; defaults to 'exported')"`
	Redact *bool    `json:"redact,omitempty" jsonschema:"Apply partial masking to sensitive data (CPF, CNPJ, e-mail, phone, CEP, RG, card, dates, etc.). Default: true. Set false to export without masking."`
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
	res, err := s.store.exportFile(ctx, in.Path, in.Query, in.Table, target, in.Args, doRedact)
	if err != nil {
		return nil, nil, err
	}
	return textResult(res)
}


const liveMaxRows = 200

func (s *Server) renderLiveQuery(ctx context.Context, cfg liveDBConfig, q string, args []string) (string, error) {
	if strings.TrimSpace(q) == "" {
		return "", fmt.Errorf("'sql' é obrigatório")
	}
	c, err := newLiveDB(cfg)
	if err != nil {
		return "", err
	}
	cols, rows, err := c.query(ctx, q, args, true, false)
	if err != nil {
		return "", err
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
		res, err := s.renderLiveQuery(ctx, cfg, in.SQL, in.Args)
		if err != nil {
			return nil, nil, err
		}
		return textResult(res)
	}
}

type liveExportInput struct {
	Query       string   `json:"query" jsonschema:"Read-only SQL query (SELECT or WITH). Do not use ';'. Executed in a READ ONLY transaction; writes are rejected."`
	Args        []string `json:"args,omitempty" jsonschema:"Bound parameters for the query (optional). Passed as parameters, never interpolated into the SQL."`
	ExportTo    string   `json:"export_to" jsonschema:"Output path to write the full result to a file. The file extension defines the format: .csv, .html, .xlsx, .tsv, .json, .xml, .sql. The result is always masked."`
	All         bool     `json:"all,omitempty" jsonschema:"Return all rows, bypassing the 500-row hard limit. Only allowed together with 'export_to'."`
	TargetTable string   `json:"target_table,omitempty" jsonschema:"Table name used in the exported SQL script (only relevant for .sql exports; defaults to 'exported')."`
}

func (s *Server) liveExportHandler(cfg liveDBConfig) func(context.Context, *mcp.CallToolRequest, liveExportInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in liveExportInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.ExportTo) == "" {
			return nil, nil, fmt.Errorf("'export_to' é obrigatório")
		}
		if strings.TrimSpace(in.Query) == "" {
			return nil, nil, fmt.Errorf("'query' é obrigatório")
		}
		c, err := newLiveDB(cfg)
		if err != nil {
			return nil, nil, err
		}
		cols, rows, err := c.query(ctx, in.Query, in.Args, true, in.All)
		if err != nil {
			return nil, nil, err
		}
		target := strings.TrimSpace(in.TargetTable)
		if target == "" {
			target = "exported"
		}
		res, err := exportLiveFile(in.ExportTo, cols, rows, target)
		if err != nil {
			return nil, nil, err
		}
		return textResult(res)
	}
}

func (s *Server) liveStructureHandler(cfg liveDBConfig) func(context.Context, *mcp.CallToolRequest, liveStructureInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in liveStructureInput) (*mcp.CallToolResult, any, error) {
		c, err := newLiveDB(cfg)
		if err != nil {
			return nil, nil, err
		}
		res, err := c.structure(ctx, in.Table)
		if err != nil {
			return nil, nil, err
		}
		return textResult(res)
	}
}

type liveQueryInput struct {
	SQL  string   `json:"sql" jsonschema:"Read-only SQL query (SELECT or WITH). Do not use ';'. Executed in a READ ONLY transaction; writes are rejected."`
	Args []string `json:"args,omitempty" jsonschema:"Bound parameters for the query (optional). Passed as parameters, never interpolated into the SQL."`
}


type liveStructureInput struct {
	Table string `json:"table,omitempty" jsonschema:"Table name, optionally qualified as 'schema.table'. If empty, lists all tables."`
}

const maxCellLen = 200

func isBinaryText(s string) bool {
	if strings.IndexByte(s, 0) >= 0 {
		return true
	}
	return !utf8.ValidString(s)
}

func short(s string) string {
	if isBinaryText(s) {
		return "[binário]"
	}
	rs := []rune(s)
	if len(rs) <= maxCellLen {
		return s
	}
	return string(rs[:maxCellLen]) + "…"
}

func cleanCell(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "/")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return short(strings.TrimSpace(s))
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
