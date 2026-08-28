# SQLize MCP

Servidor MCP em Go para importar, inspecionar, consultar e exportar dados em
diferentes formatos, usando um banco **SQLite em arquivo** como base. É um
projeto **independente** dos outros MCPs deste repositório (tem seu próprio
`go.mod`).

As ferramentas não alteram arquivos de entrada e a consulta é **somente
leitura** (apenas `SELECT`/`WITH`).

## Estrutura

```
sqlize/
  cmd/sqlize-mcp/main.go        # entrypoint (transporte stdio + setupLog)
  internal/sqlize/              # pacote principal (lógica das ferramentas)
    store.go                    # banco SQLite em arquivo + carga/consulta
    import.go                   # parsers de importação
    export.go                   # escrita de exportação
    livedb.go                   # bancos ao vivo (Postgres/MySQL) read-only
    redact.go                   # redator PII (detecção + mascaramento)
    names_br.go                 # deny-list de nomes BR
    xml.go                      # parser XML heurístico
    server.go                   # ferramentas MCP (Register)
  go.mod
  README.md
```

## Requisitos

- Go 1.26 ou mais recente;
- acesso à rede para baixar as dependências (`excelize` e `modernc.org/sqlite`)
  no primeiro `go mod tidy`.

## Banco de estado

O banco de trabalho é um arquivo SQLite persistido na **pasta raiz do usuário**
(resolvida a partir de `USERPROFILE` no Windows ou `HOME` no Linux/macOS),
seguindo o mesmo padrão do `taiga-mcp`:

```
~/.local/state/sqlize/sqlize.db
# Windows: C:\Users\yannp\.local\state\sqlize\sqlize.db
```

(O diretório é criado automaticamente.) Para usar outro local, defina a
variável de ambiente `SQLIZE_STATE_DIR`. As importações de `.sqlite`/`.db` são
anexadas como esquemas e permanecem nesses arquivos originais.

## Build e execução

Via Taskfile (na raiz deste repositório de MCPs):

```powershell
task build:sqlize
```

Direto:

```powershell
cd mcp/sqlize
go mod tidy
go build -o ../dist/sqlize-mcp.exe ./cmd/sqlize-mcp
..\dist\sqlize-mcp.exe
```

## Logs

```text
~/.local/share/mcp/sqlize/logs/sqlize-<AAAA-MM-DD_HH-MM-SS>.log
```

O processo usa **stdio** para o transporte MCP. Integre-o em um cliente MCP
(Zed, Claude Desktop etc.) apontando para o binário compilado.

Exemplo de configuração (Zed `~/.config/zed/settings.json`):

```json
{
  "context_servers": {
    "sqlize": {
      "command": "C:/Users/yannp/nautidesk/mcp/dist/sqlize-mcp.exe"
    }
  }
}
```

(Opcionalmente, para usar um local diferente do padrão `~/.local/state/sqlize`,
defina `SQLIZE_STATE_DIR` no ambiente.)

## Fluxo de uso

1. `sqlize_import` — carrega um arquivo no banco.
   - `.json`, `.csv`, `.tsv`, `.xlsx`, `.xml` → viram tabelas (no Excel, uma por
     aba; use o campo `sheet` para importar só uma aba — opcionalmente com
     `table` nomeando a tabela criada).
   - `.sql` → instruções executadas no banco.
   - `.sqlite`, `.db` → banco anexado como esquema (ex.: `db0`); suas tabelas
     ficam consultáveis diretamente pelo nome.
2. `sqlize_structure` — lista tabelas/colunas; com `table`, detalha uma tabela.
3. `sqlize_query` — consulta SQL (`SELECT`/`WITH`, sem `;`).
4. `sqlize_export` — grava o resultado em `.json`, `.csv`, `.tsv`, `.xlsx`,
   `.sql` ou `.xml` (a extensão do `path` define o formato).
5. `sqlize_compare` — compara duas tabelas/consultas do SQLite local por
   coluna(s) chave (`key`, obrigatória): resumo por status (`só-em-A`,
   `só-em-B`, `diferente`) + detalhe por chave. Fontes: `table_a`/`query_a` e
   `table_b`/`query_b` (informe um de cada). Resultado mascarado por padrão
   (`redact: false` desliga).

Reimportar um arquivo com o mesmo nome de tabela **recria** a tabela
(atualizando os dados).

## Redator (detecção + mascaramento, estilo Presidio)

Por padrão, `sqlize_query` e `sqlize_export` mascaram os dados sensíveis. O
redator é inspirado na arquitetura do [Presidio](https://github.com/data-privacy-stack/presidio)
(analyzer + anonymizer), mas 100% em Go, sem dependências externas:

1. **Análise**: cada célula é varrida por reconhecedores (regex) que devolvem
   *spans* com **entidade + score de confiança**.
2. **Validação**: CPF, CNPJ e cartão passam por **checksum real** (dígito
   verificador / Luhn). Válidos sobem para score 1.0; inválidos caem e **só
   mascaram se a coluna confirmar** (acaba o falso positivo de "qualquer
   número de 11 dígitos vira CPF").
3. **Contexto de coluna (pt + en)**: colunas como `cpf`, `nome`, `email`,
   `senha` — ou `customer`, `phone`, `password`, `address` — recebem score
   1.0 e são sempre mascaradas, mesmo sem bater padrão.
4. **Pessoas**: detecção **sem depender de lista de nomes**: seqüências de
   2+ palavras capitalizadas (`João da Silva`, `Tomasz Kowalski`), nomes
   únicos após honoríficos/preposições (`Sr. Pedro`, `falei com Maria`) e
   células inteiras no formato de nome ganham score por **forma + contexto**.
   A deny-list embutida de nomes BR (`brFirstNames`) virou apenas um **booster**
   (não é mais requisito), e o conjunto usado para **excluir** não-nomes é
   fechado e pequeno: dias/meses, produtos, cargos, logradouros, órgãos,
   geográficos (estados, capitais, países) e sufixos de razão social.
   Aumentáveis por arquivo: `SQLIZE_PII_NAMES` (nomes para reforçar, um por
   linha) e `SQLIZE_PII_WORDS` (palavras comuns do seu domínio para excluir,
   ex.: nomes de produtos internos) — ambos normalizados (lowercase, sem
   acento), com a mesma regra do contexto de coluna.
5. **Limiar** (`score >= 0.5`) e **resolução de sobreposição** decidem o que
   é mascarado; o operador de máscara é escolhido pela entidade.

Exemplos de saída (máscara parcial, mantém pontas):

- CPF válido `529.982.247-25` → `52*******-25`; CPF **inválido** fora de
  coluna `cpf` → fica como está
- e-mail `joao@exemplo.com` → `jo**@***.com` (só o início do usuário e a
  extensão do domínio ficam visíveis)
- telefone `(11) 98765-4321` → `11*******-21`
- cartão (Luhn válido) `4111 1111 1111 1111` → `4111********1111`
- datas **genéricas não** são mascaradas (ex.: `created_at`); colunas de data
  pessoal (`nascimento`, `date of birth`...) continuam mascaradas pelo contexto
  de coluna
- **qualquer URL** vira PII: `https://exemplo.com/pagina?x=1`, `www.site.com.br`
  ou mesmo `exemplo.com.br` → `***`
- **endereços** detectados pelo logradouro: `Rua das Flores, 123` → `R***`
- `nome`/`customer` = `João da Silva` → `J***`; em texto livre, só o nome
  `Joao da Silva` é mascarado dentro da frase
- colunas de segredo (`senha`, `token`, `apikey`...) → `***` inteiro
- entidades tokenizadas (IP, MAC, JWT, hash, BTC) → `***`

Notas:

- Padrões ambíguos (ex.: RG) só mascaram com coluna PII confirmando.
- Para desativar o mascaramento nas tools do SQLite, informe `"redact": false`
  em `sqlize_query` ou `sqlize_export`. As tools de bancos ao vivo são
  **sempre** mascaradas (não há como desligar).

- `sqlize_import` — importa arquivo (formatos acima).
- `sqlize_structure` — estrutura dos dados (todas as tabelas ou uma específica).
- `sqlize_query` — consulta SQL somente leitura (retorna Markdown, até 200 linhas).
- `sqlize_export` — exporta consulta/tabela para arquivo.

## Bancos ao vivo (Postgres / MySQL)

Além do SQLite em arquivo, o sqlize pode consultar **bancos reais** de forma
**estritamente somente leitura** e com **mascaramento sempre ligado** (sem
opção de desligar). As conexões são descobertas no **startup** a partir das
variáveis de ambiente — quantas conexões existirem, tantas tools serão
registradas (adicione/remova bancos nas envs e reinicie o servidor):

| Variável | Descrição |
| --- | --- |
| `{PREFIXO}_POSTGRES_URL` ou `{PREFIXO}_POSTGRES_DSN` | DSN do Postgres (ex.: `postgres://user:pass@host:5432/db?sslmode=disable`) |
| `{PREFIXO}_MYSQL_URL` ou `{PREFIXO}_MYSQL_DSN` | DSN do MySQL (ex.: `user:pass@tcp(host:3306)/db`) |

O `{PREFIXO}` vira o alias da conexão nas tools. O prefixo `DB` (ou ausente:
`POSTGRES_URL`/`MYSQL_URL`) é a conexão **padrão**, sem alias. Quando `_URL` e
`_DSN` existem para o mesmo prefixo, o `_URL` vence.

Cada banco registrado ganha 4 tools, nomeadas `{engine}[_{alias}]_...`:

- `{engine}[_{alias}]_query` — executa `SELECT`/`WITH` dentro de uma transação
  `READ ONLY` (qualquer escrita é rejeitada pelo banco). **LIMIT forçado de 500
  linhas** (anexado se ausente, ou reduzido a 500 se maior). Parâmetros
  opcionais em `args` são passados como *bound parameters* (nunca interpolados).
- `{engine}[_{alias}]_tables` — lista as tabelas do banco.
- `{engine}[_{alias}]_schema` — descreve as colunas de uma tabela
  (`schema.table` ou só `table`).
- `{engine}[_{alias}]_explain` — plano de execução **sem executar a query**
  (`EXPLAIN`; `EXPLAIN (FORMAT TEXT)` no Postgres). Como nada roda, literais
  são permitidos — mas placeholders não (EXPLAIN não tem parâmetros). Útil para
  depurar performance com segurança.

Exemplos: `DB_POSTGRES_DSN` → `postgres_query`, `postgres_tables`,
`postgres_schema`; `PRD_MYSQL_URL` → `mysql_prd_query`, `mysql_prd_tables`,
`mysql_prd_schema`; `LOCAL_POSTGRES_URL` → `postgres_local_query` ...

Toda a saída dessas tools é **sempre mascarada** (CPF, CNPJ, e-mail, telefone,
cartão, datas, IP e colunas consideradas PII por nome).

### `export_to` e `all` (tools de consulta)

As tools `{engine}[_{alias}]_query` aceitam dois campos opcionais:

- **`export_to`** — caminho de um arquivo onde o **resultado completo** é
gravado em vez de devolver a tabela Markdown. A extensão define o formato:
  `.csv`, `.html`, `.xlsx` (excel), `.tsv`, `.json`, `.xml` e `.sql` (script
  com `CREATE TABLE` + `INSERT`s; o campo `target_table` escolhe o nome da
  tabela no script, padrão `exported`). O arquivo é sempre mascarado (mesma
  regra das tools ao vivo).
- **`all`** (booleano) — libera o limite forçado de 500 linhas na consulta.
  **Só pode ser usado quando `export_to` também é informado**; caso contrário a
  chamada é rejeitada. A consulta continua read-only e sem escrita.

### Modo estrito (anti-injeção)

As queries ao vivo seguem regras de segurança, além do `READ ONLY` e do
`LIMIT` forçado:

- **"where sem args"**: a cláusula `WHERE` não aceita valores inline — string
  (`WHERE cidade = 'SP'`), número ou booleano (`WHERE id = 5`, `WHERE ativo =
  true`). Filtros precisam ser parametrizados (`$1..` no Postgres, `?` no
  MySQL/SQLite) com o valor em `args` (ex.: `WHERE id = $1`). Predicados sem
  valor (`IS NULL`, `IS NOT NULL`) e comparações coluna-coluna continuam
  permitidos.
- A restrição vale **só para o WHERE**: strings de formatação/constante fora
  dele passam normalmente (ex.: `TO_CHAR(d, 'YYYY-MM-DD')`, `COALESCE(c, '')`,
  `CASE`).
- **Allowlist de funções**: só funções embutidas comuns podem ser chamadas
  (`COUNT`, `SUM`, `TO_CHAR`, `COALESCE`, `DATE_TRUNC`, `LOWER`, `ROW_NUMBER`,
  `JSON_EXTRACT`, `GEN_RANDOM_UUID`...). Funções definidas pelo usuário, de
  extensões ou administrativas/perigosas são rejeitadas antes de executar:
  `pg_sleep`, `dblink_exec`, `pg_read_file`, `lo_import`/`lo_export`,
  `SLEEP`, `LOAD_FILE`... (a lista é um `map` em `livedb.go` — `sqlFuncAllowlist`).
- Se você passar `args`, o SQL **precisa** conter placeholders (`$1..` no
  Postgres ou `?` no MySQL) — caso contrário a consulta é rejeitada (indica
  valor concatenado inline, não parametrizado).
- Padrões clássicos de injeção são bloqueados: aspas seguidas de comentário
  SQL (`--`, `#`, `/*`) ou de `UNION`/`INTO`. Queries parametrizadas legítimas
  não carregam valores entre aspas no próprio SQL.

Essas regras valem também para o `sqlize_query` (SQLite local — placeholder
`?`). Sempre passe valores dinâmicos em `args` (nunca concatene no `sql`).

Requer os drivers `github.com/jackc/pgx/v5` e `github.com/go-sql-driver/mysql`
(rode `go mod tidy`).

## Observações

- `.sqlite`/`.db` anexados via `ATTACH`; para tabelas com nome duplicado entre
  esquemas, qualifique como `esquema.tabela` na consulta.
- A importação de XML é heurística: identifica elementos repetidos como linhas e
  usa atributos + elementos folha como colunas.
- O Excel suportado é `.xlsx`/`.xlsm` (não `.xls` legado).
