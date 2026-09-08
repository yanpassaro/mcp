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
		Description: "Import a file into the working SQLite database. Formats: .json, .jsonl, .ndjson, .csv, .tsv, .xlsx, .xlsm, .xls, .sql, .sqlite, .db, .xml. 'table' names the destination table; .sqlite/.db are attached as a schema.",
	}, s.importTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_structure",
		Description: "List tables and columns. With 'table', show columns, foreign keys and indexes.",
	}, s.structureTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_query",
		Description: "Run any SQL statement on the local SQLite database; returns Markdown (up to 200 rows) for queries. Pass values via 'args' (use ? placeholders).",
	}, s.queryTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "sqlize_export",
		Description: "Export a query or table to a raw (unredacted) file inside the shared filesystem root ~/.local/state/mcp/mnt (sandbox mnt, out of the AI's reach). Only the path is returned. Formats by path extension (.json, .csv, .tsv, .xlsx, .sql, .html, .xml). Values in 'args'.",
	}, s.exportTool)

	for _, cfg := range discoverLiveDBs() {
		prefix := cfg.ToolPrefix()
		env := cfg.EnvVar
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_query",
			Description: fmt.Sprintf("Run a read-only SQL query (SELECT/WITH) against the live %s database (%s), limited to 500 rows. Pass values via 'args' (%s).", cfg.Engine, env, livePlaceholders(cfg.Engine)),
		}, s.liveQueryHandler(cfg))
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_export",
			Description: fmt.Sprintf("Run a read-only query (SELECT/WITH) on the live %s database (%s) and write the full (raw, unredacted) result to a file inside the shared ~/.local/state/mcp/mnt; the extension sets the format (.csv, .html, .xlsx, .tsv, .json, .xml, .sql).", cfg.Engine, env),
		}, s.liveExportHandler(cfg))
		mcp.AddTool(server, &mcp.Tool{
			Name:        prefix + "_structure",
			Description: fmt.Sprintf("Structure of the live %s database (%s): tables (no 'table') or columns + FKs + indexes ('table').", cfg.Engine, env),
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
	Path  string `json:"path" jsonschema:"Input file path."`
	Table string `json:"table,omitempty" jsonschema:"Destination table name (optional; defaults to file name)."`
	Sheet string `json:"sheet,omitempty" jsonschema:"Excel sheet to import (optional, Excel only)."`
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
	Table string `json:"table,omitempty" jsonschema:"Table (optional). Empty = list all."`
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
				b.WriteString(columnLine(c.Name, c.Type, c.NotNull, c.Default, c.PK))
				b.WriteString("\n")
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
	SQL  string   `json:"sql" jsonschema:"SQL statement. Values via 'args' (use ? placeholders)."`
	Args []string `json:"args,omitempty" jsonschema:"Bound parameters for '?' placeholders."`
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
	Path   string   `json:"path" jsonschema:"Output file name (.json, .csv, .tsv, .xlsx, .sql, .html, .xml). Always saved inside the shared ~/.local/state/mcp/mnt."`
	Query  string   `json:"query,omitempty" jsonschema:"Source SQL (optional if 'table' given)."`
	Args   []string `json:"args,omitempty" jsonschema:"Bound parameters for the query."`
	Table  string   `json:"table,omitempty" jsonschema:"Source table (optional if 'query' given)."`
	Target string   `json:"target_table,omitempty" jsonschema:"Table name in exported .sql (default 'exported')."`
}

func (s *Server) exportTool(ctx context.Context, _ *mcp.CallToolRequest, in exportInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Path) == "" {
		return nil, nil, fmt.Errorf("'path' de saída é obrigatório")
	}
	target := in.Target
	if strings.TrimSpace(target) == "" {
		target = "exported"
	}
	res, err := s.store.exportFile(ctx, in.Path, in.Query, in.Table, target, in.Args)
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
	Query       string   `json:"query" jsonschema:"Read-only SQL query (SELECT/WITH)."`
	Args        []string `json:"args,omitempty" jsonschema:"Bound parameters."`
	ExportTo    string   `json:"export_to" jsonschema:"Output file name (.csv, .html, .xlsx, .tsv, .json, .xml, .sql). Always saved inside the shared ~/.local/state/mcp/mnt."`
	All         bool     `json:"all,omitempty" jsonschema:"Bypass the 500-row limit (only with 'export_to')."`
	TargetTable string   `json:"target_table,omitempty" jsonschema:"Table name in exported .sql (default 'exported')."`
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
	SQL  string   `json:"sql" jsonschema:"Read-only SQL query (SELECT/WITH)."`
	Args []string `json:"args,omitempty" jsonschema:"Bound parameters."`
}


type liveStructureInput struct {
	Table string `json:"table,omitempty" jsonschema:"Table (optional). Empty = list all."`
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

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "t", "true", "1", "yes", "y":
		return true
	}
	return false
}

func notNull(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "no", "1", "true", "y", "yes":
		return true
	}
	return false
}

func columnLine(name, typ, nullable, def, pk string) string {
	line := "- " + short(name) + ": " + short(typ)
	var flags []string
	if isTruthy(pk) {
		flags = append(flags, "PK")
	}
	if notNull(nullable) {
		flags = append(flags, "NOT NULL")
	}
	if def != "" {
		line += " DEFAULT " + short(strings.TrimSpace(def))
	}
	if len(flags) > 0 {
		line += " · " + strings.Join(flags, " · ")
	}
	return line
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
