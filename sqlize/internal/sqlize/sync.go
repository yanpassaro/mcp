package sqlize

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type syncInput struct {
	Key     string `json:"key" jsonschema:"Key column(s) used to match rows, comma separated (mandatory)."`
	TableA  string `json:"table_a,omitempty" jsonschema:"Source table that represents the desired state (alternative to query_a)."`
	QueryA  string `json:"query_a,omitempty" jsonschema:"Source SELECT/WITH query that represents the desired state (alternative to table_a). No 'args' here — keep it free of WHERE value filters."`
	TableB  string `json:"table_b,omitempty" jsonschema:"Target table that will be updated (alternative to query_b)."`
	QueryB  string `json:"query_b,omitempty" jsonschema:"Target SELECT/WITH query (alternative to table_b). No 'args' here — keep it free of WHERE value filters."`
	Target  string `json:"target,omitempty" jsonschema:"Name of the table targeted by the generated DML (defaults to the table_b name, or 'target')."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max rows processed (default 500, cap 5000)."`
	Redact  *bool  `json:"redact,omitempty" jsonschema:"Mask value literals in the generated script. Default: true. Set false for an executable script (keys are never masked)."`
}

func (s *Server) syncTool(ctx context.Context, _ *mcp.CallToolRequest, in syncInput) (*mcp.CallToolResult, any, error) {
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
		k = strings.TrimSpace(k)
		if k != "" {
			keyCols = append(keyCols, k)
		}
	}
	if len(keyCols) == 0 {
		return nil, nil, fmt.Errorf("'key' vazia")
	}
	keySet := map[string]bool{}
	for _, k := range keyCols {
		keySet[k] = true
		if !slices.Contains(colsA, k) {
			return nil, nil, fmt.Errorf("coluna chave %q não existe nas fontes (disponíveis: %s)", k, strings.Join(colsA, ", "))
		}
	}
	dataCols := make([]string, 0, len(colsA)-len(keyCols))
	for _, c := range colsA {
		if !keySet[c] {
			dataCols = append(dataCols, c)
		}
	}

	target := strings.TrimSpace(in.Target)
	if target == "" {
		target = strings.TrimSpace(in.TableB)
	}
	if target == "" {
		target = "target"
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}

	doRedact := in.Redact == nil || *in.Redact

	cols, rows, err := s.store.query(ctx, syncDiffQuery(qa, qb, keyCols, dataCols, limit))
	if err != nil {
		return nil, nil, fmt.Errorf("sync: %w", err)
	}
	return textResult(syncMarkdown(target, keyCols, dataCols, cols, rows, doRedact, limit))
}

func syncDiffQuery(qa, qb string, keyCols, dataCols []string, limit int) string {
	keyParts := make([]string, 0, len(keyCols))
	joins := make([]string, 0, len(keyCols))
	for _, k := range keyCols {
		keyParts = append(keyParts, "COALESCE(a."+quoteIdent(k)+", b."+quoteIdent(k)+") AS "+quoteIdent(k))
		joins = append(joins, "a."+quoteIdent(k)+" = b."+quoteIdent(k))
	}
	parts := append(keyParts, "__status")
	status := "'igual'"
	if len(dataCols) > 0 {
		preds := make([]string, 0, len(dataCols))
		for _, c := range dataCols {
			preds = append(preds, "a."+quoteIdent(c)+" IS NOT b."+quoteIdent(c))
		}
		status = "CASE WHEN (" + strings.Join(preds, " OR ") + ") THEN 'diferente' ELSE 'igual' END"
	}
	statusExpr := "CASE WHEN a." + quoteIdent(keyCols[0]) + " IS NULL THEN 'só-em-B' WHEN b." + quoteIdent(keyCols[0]) + " IS NULL THEN 'só-em-A' ELSE " + status + " END AS __status"
	parts[len(keyParts)] = statusExpr

	for j := range dataCols {
		parts = append(parts, "a."+quoteIdent(dataCols[j])+" AS \"a__"+strconv.Itoa(j)+"\"")
	}
	for j := range dataCols {
		parts = append(parts, "b."+quoteIdent(dataCols[j])+" AS \"b__"+strconv.Itoa(j)+"\"")
	}
	orderAux := ""
	if len(keyCols) > 1 {
		orderAux = ", 2"
	}
	body := "SELECT " + strings.Join(parts, ", ") +
		" FROM _a a FULL OUTER JOIN _b b ON " + strings.Join(joins, " AND ") +
		" ORDER BY 1" + orderAux + ", " + strconv.Itoa(len(keyCols)+1)
	return "WITH _a AS (" + qa + "), _b AS (" + qb + ") " + body + " LIMIT " + strconv.Itoa(limit)
}

func syncMarkdown(target string, keyCols, dataCols []string, cols []string, rows [][]string, redact bool, limit int) string {
	statusIdx := len(keyCols)
	idxOf := map[string]int{}
	for i, c := range cols {
		idxOf[c] = i
	}
	aIdx := make([]int, len(dataCols))
	bIdx := make([]int, len(dataCols))
	for j := range dataCols {
		aIdx[j] = idxOf["a__"+strconv.Itoa(j)]
		bIdx[j] = idxOf["b__"+strconv.Itoa(j)]
	}

	var b strings.Builder
	var ins, upd, del int
	trunc := len(rows) >= limit
	fmt.Fprintf(&b, "## Sync — alvo: %s\n\n```sql\nBEGIN;\n", quoteIdent(target))
	for _, r := range rows {
		status := ""
		if statusIdx < len(r) {
			status = r[statusIdx]
		}
		keyVals := make([]string, len(keyCols))
		for i := range keyCols {
			if i < len(r) {
				keyVals[i] = r[i]
			}
		}
		where := keyWhere(keyCols, keyVals)
		switch status {
		case "só-em-A":
			ins++
			allCols := append(append([]string{}, keyCols...), dataCols...)
			vals := make([]string, 0, len(allCols))
			for i, _ := range keyCols {
				vals = append(vals, sqlLit(keyVals[i]))
			}
			for j := range dataCols {
				vals = append(vals, syncVal(dataCols[j], argAt(r, aIdx[j]), redact))
			}
			fmt.Fprintf(&b, "INSERT INTO %s (%s) VALUES (%s);\n", quoteIdent(target), quoteJoin(allCols), strings.Join(vals, ", "))
		case "só-em-B":
			del++
			fmt.Fprintf(&b, "DELETE FROM %s WHERE %s;\n", quoteIdent(target), where)
		case "diferente":
			upd++
			sets := make([]string, 0, len(dataCols))
			for j := range dataCols {
				av := argAt(r, aIdx[j])
				bv := argAt(r, bIdx[j])
				if av == bv {
					continue
				}
				sets = append(sets, quoteIdent(dataCols[j])+" = "+syncVal(dataCols[j], av, redact))
			}
			if len(sets) == 0 {
				continue
			}
			fmt.Fprintf(&b, "UPDATE %s SET %s WHERE %s;\n", quoteIdent(target), strings.Join(sets, ", "), where)
		}
	}
	b.WriteString("COMMIT;\n```\n")
	fmt.Fprintf(&b, "\nResumo: %d INSERT, %d UPDATE, %d DELETE.\n", ins, upd, del)
	if trunc {
		fmt.Fprintf(&b, "(processado até o limite de %d linhas; aumente 'limit' para cobrir tudo.)\n", limit)
	}
	return b.String()
}

func keyWhere(keyCols []string, keyVals []string) string {
	parts := make([]string, len(keyCols))
	for i := range keyCols {
		parts[i] = quoteIdent(keyCols[i]) + " = " + sqlLit(keyVals[i])
	}
	return strings.Join(parts, " AND ")
}

func quoteJoin(cols []string) string {
	q := make([]string, len(cols))
	for i, c := range cols {
		q[i] = quoteIdent(c)
	}
	return strings.Join(q, ", ")
}

func syncVal(col, v string, redact bool) string {
	if v == "" {
		return "NULL"
	}
	if redact {
		v = redactCell(col, v)
	}
	return sqlLit(v)
}

func argAt(r []string, i int) string {
	if i < 0 || i >= len(r) {
		return ""
	}
	return r[i]
}
