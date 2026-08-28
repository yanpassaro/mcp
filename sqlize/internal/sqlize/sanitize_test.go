package sqlize

import (
	"strings"
	"testing"
)

func TestSanitizeStoreQueryWhereArgs(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		args []string
		want string
	}{
		{"select simples sem where", "SELECT * FROM clientes", nil, "SELECT * FROM clientes"},
		{"where parametrizado", "SELECT * FROM clientes WHERE id = ?", []string{"5"}, "SELECT * FROM clientes WHERE id = ?"},
		{"where is null sem args", "SELECT * FROM clientes WHERE deleted_at IS NULL", nil, "SELECT * FROM clientes WHERE deleted_at IS NULL"},
		{"where coluna-coluna", "SELECT * FROM t WHERE a = b", nil, "SELECT * FROM t WHERE a = b"},
		{"where numero inline rejeitado", "SELECT * FROM t WHERE id = 5", nil, ""},
		{"where booleano inline rejeitado", "SELECT * FROM t WHERE ativo = true", nil, ""},
		{"where maior que rejeitado", "SELECT * FROM t WHERE preco > 100.50", nil, ""},
		{"where in inline rejeitado", "SELECT * FROM t WHERE id IN (1, 2, 3)", nil, ""},
		{"where between inline rejeitado", "SELECT * FROM t WHERE id BETWEEN 10 AND 20", nil, ""},
		{"where is null e literal rejeitado", "SELECT * FROM t WHERE a IS NULL AND id = 5", nil, ""},
		{"update where inline rejeitado", "UPDATE t SET x = 1 WHERE id = 5", nil, ""},
		{"update where parametrizado", "UPDATE t SET x = ? WHERE id = ?", []string{"a", "5"}, "UPDATE t SET x = ? WHERE id = ?"},
		{"delete where inline rejeitado", "DELETE FROM t WHERE id = 5", nil, ""},
		{"string literal no where rejeitado", "SELECT * FROM t WHERE cidade = 'SP'", nil, ""},
		{"string literal em subquery do where rejeitado", "SELECT * FROM t WHERE id IN (SELECT x FROM u WHERE y = 'v')", nil, ""},
		{"insert sem where passa", "INSERT INTO t (a) VALUES (1)", nil, "INSERT INTO t (a) VALUES (1)"},
		{"string de formato no select permitida", "SELECT TO_CHAR(d, 'YYYY-MM-DD') FROM t WHERE id = ?", []string{"5"}, "SELECT TO_CHAR(d, 'YYYY-MM-DD') FROM t WHERE id = ?"},
		{"constante no select permitida", "SELECT 'x' AS lbl, COALESCE(c, '') FROM t", nil, "SELECT 'x' AS lbl, COALESCE(c, '') FROM t"},
		{"constante no update set permitida", "UPDATE t SET c = 'x' WHERE id = ?", []string{"5"}, "UPDATE t SET c = 'x' WHERE id = ?"},
		{"insert com lista de colunas", "INSERT INTO t (a, b) VALUES (?, ?)", []string{"1", "2"}, "INSERT INTO t (a, b) VALUES (?, ?)"},
		{"insert com colunas e numeros", "INSERT INTO t (a) VALUES (1)", nil, "INSERT INTO t (a) VALUES (1)"},
		{"cte com colunas", "WITH c (a, b) AS (SELECT 1, 2) SELECT * FROM c", nil, "WITH c (a, b) AS (SELECT 1, 2) SELECT * FROM c"},
		{"cte com as", "WITH c AS (SELECT 1) SELECT * FROM c", nil, "WITH c AS (SELECT 1) SELECT * FROM c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sanitizeStoreQuery(tc.sql, tc.args)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("sanitizeStoreQuery(%q) deveria rejeitar, mas passou (%q)", tc.sql, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("sanitizeStoreQuery(%q) = erro inesperado: %v", tc.sql, err)
			}
			if got != tc.want {
				t.Errorf("sanitizeStoreQuery(%q) = %q; want %q", tc.sql, got, tc.want)
			}
		})
	}
}

func TestValidateLiveQueryWhereArgs(t *testing.T) {
	cases := []struct {
		name   string
		sql    string
		args   []string
		driver string
		strict bool
		want   string
	}{
		{"pg where parametrizado", "SELECT * FROM t WHERE id = $1", []string{"5"}, "pgx", true, "SELECT * FROM t WHERE id = $1"},
		{"pg where inline rejeitado", "SELECT * FROM t WHERE id = 5", nil, "pgx", true, ""},
		{"pg is null ok", "SELECT * FROM t WHERE deleted_at IS NULL", nil, "pgx", true, "SELECT * FROM t WHERE deleted_at IS NULL"},
		{"mysql where parametrizado", "SELECT * FROM t WHERE id = ?", []string{"5"}, "mysql", true, "SELECT * FROM t WHERE id = ?"},
		{"mysql where inline rejeitado", "SELECT * FROM t WHERE id = 5", nil, "mysql", true, ""},
		{"catalogo interno strict false", "SELECT table_schema FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema')", nil, "pgx", false, "SELECT table_schema FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema')"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateLiveQuery(tc.sql, tc.args, tc.driver, tc.strict)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("validateLiveQuery(%q) deveria rejeitar, mas passou (%q)", tc.sql, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateLiveQuery(%q) = erro inesperado: %v", tc.sql, err)
			}
			if got != tc.want {
				t.Errorf("validateLiveQuery(%q) = %q; want %q", tc.sql, got, tc.want)
			}
		})
	}
}

func TestReWhereLiteral(t *testing.T) {
	rejects := []string{
		"SELECT * FROM t WHERE id = 5",
		"SELECT * FROM t WHERE preco >= 9.99",
		"SELECT * FROM t WHERE nome <> 0",
		"SELECT * FROM t WHERE ativo = false",
		"SELECT * FROM t WHERE id IN (1,2)",
		"SELECT * FROM t WHERE a = b AND c = 3",
	}
	allows := []string{
		"SELECT * FROM t",
		"SELECT * FROM t WHERE id = ?",
		"SELECT * FROM t WHERE deleted_at IS NULL",
		"SELECT * FROM t WHERE a = b",
		"SELECT * FROM t WHERE id IN (SELECT x FROM u WHERE y = $1)",
		"SELECT 1",
	}
	for _, q := range rejects {
		if !reWhereLiteral.MatchString(q) {
			t.Errorf("reWhereLiteral deveria casar %q", q)
		}
	}
	for _, q := range allows {
		if reWhereLiteral.MatchString(q) {
			t.Errorf("reWhereLiteral não deveria casar %q", q)
		}
	}
}

func TestFuncAllowlist(t *testing.T) {
	allows := []string{
		"SELECT count(*) FROM t",
		"SELECT TO_CHAR(d, 'YYYY-MM-DD'), COALESCE(c, ''), SUM(v) FROM t GROUP BY 1",
		"SELECT date_trunc('month', d) FROM t WHERE id = ?",
		"SELECT row_number() OVER (ORDER BY x) FROM t",
		"SELECT * FROM t WHERE x IN (1, 2) AND id = ?",
		"SELECT CAST(x AS TEXT) FROM t",
		"SELECT strftime('%Y', d) FROM t",
		"SELECT json_extract(j, '$.a') FROM t",
		"UPDATE t SET c = ? WHERE id = ?",
	}
	rejects := []string{
		"SELECT minha_funcao(1, 2)",
		"SELECT * FROM t WHERE id = (SELECT fn_aux(x) FROM u)",
		"SELECT pg_sleep(10)",
		"SELECT dblink_exec('x')",
		"SELECT pg_read_file('/etc/passwd')",
		"SELECT lo_import('/tmp/f')",
		"SELECT SLEEP(10)",
		"SELECT LOAD_FILE('/etc/passwd')",
		"SELECT public.minha_funcao()",
	}
	for _, q := range allows {
		if err := checkFuncAllowlist(q); err != nil {
			t.Errorf("checkFuncAllowlist(%q) = %v; queria permitir", q, err)
		}
	}
	for _, q := range rejects {
		if err := checkFuncAllowlist(q); err == nil {
			t.Errorf("checkFuncAllowlist(%q) deveria rejeitar", q)
		}
	}
}

func TestQueryDoUsuario(t *testing.T) {
	q := `SELECT TO_CHAR(sd.dt_recebimento::DATE, 'YYYY-MM-DD') AS DATA,
       si.num_sinistro AS GTAI,
       s.numero_autorizacao AS ACIONAMENTO,
       CASE WHEN si.cd_item IN (97,98,99,1094) THEN si.vl_item_pago ELSE 0 END AS FEE,
       '' AS [AUT],
       COALESCE(si.movimento_economico, '') AS [AUT2],
       si.vl_item_pago AS TOTAL
FROM sinistro_item si
JOIN sinistro s
  ON s.num_sinistro = si.num_sinistro AND s.tp_sinistro = si.tp_sinistro
LEFT JOIN sinistro_data sd
  ON sd.num_sinistro = si.num_sinistro AND sd.tp_sinistro = si.tp_sinistro AND sd.cd_tp_data = 1
WHERE si.num_sinistro = $1
ORDER BY si.nu_sequencial`
	if _, err := validateLiveQuery(q, []string{"442064"}, "pgx", true); err != nil {
		t.Fatalf("validateLiveQuery da query do usuário = %v; queria passar", err)
	}
}

func TestWhereErrorMessage(t *testing.T) {
	_, err := sanitizeStoreQuery("SELECT * FROM t WHERE id = 5", nil)
	if err == nil {
		t.Fatal("esperava erro")
	}
	if !strings.Contains(err.Error(), "?)") && !strings.Contains(err.Error(), "'args'") {
		t.Errorf("mensagem deveria citar '?' e 'args'; got: %v", err)
	}
}

func TestCompareSource(t *testing.T) {
	q, err := compareSource("clientes", "", "table_a/query_a")
	if err != nil || q != `SELECT * FROM "clientes"` {
		t.Errorf("compareSource(tabela) = %q, %v; want SELECT * FROM literal", q, err)
	}
	q, err = compareSource("", "SELECT * FROM t WHERE id = ?", "table_a/query_a")
	if err == nil {
		t.Errorf("query com placeholder deveria falhar na sanitização, passou: %q", q)
	}
	q, err = compareSource("", "SELECT * FROM t", "table_a/query_a")
	if err != nil || q != "SELECT * FROM t" {
		t.Errorf("compareSource(query) = %q, %v", q, err)
	}
	if _, err := compareSource("", "", "table_a/query_a"); err == nil {
		t.Error("sem fonte deveria falhar")
	}
	if _, err := compareSource("t", "SELECT 1", "table_a/query_a"); err == nil {
		t.Error("tabela + query juntos deveriam falhar")
	}
}
