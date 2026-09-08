# SQLize MCP

Importa, consulta e exporta dados usando um banco **SQLite em arquivo** como base. Também consulta **Postgres/MySQL read-only** (configurados por env).

## Tools

| Tool | O que faz |
| --- | --- |
| `sqlize_import` | Importa um arquivo: `.json/.jsonl/.csv/.tsv/.xlsx/.xls/.xml` → tabelas; `.sql` → executa; `.sqlite/.db` → anexa como esquema |
| `sqlize_structure` | Lista tabelas/colunas; com `table`, detalha colunas + FKs + índices |
| `sqlize_query` | Qualquer SQL no banco de trabalho; retorna Markdown (até 200 linhas) com PII mascarada por nome real de coluna |
| `sqlize_export` | Exporta consulta/tabela **crua** (sem máscara) sempre para `<home>/.local/state/mcp/mnt` (o mesmo mnt do sandbox) |

Para cada banco ao vivo configurado nas envs, existem `{engine}[_{alias}]_query`, `{engine}[_{alias}]_export` e `{engine}[_{alias}]_structure`.

## Variáveis de ambiente

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `SQLIZE_STATE_DIR` | `~/.local/state/sqlize` | pasta do banco de estado |
| `{PREFIXO}_POSTGRES_URL` / `_DSN` | — | conexão Postgres read-only (por prefixo) |
| `{PREFIXO}_MYSQL_URL` / `_DSN` | — | conexão MySQL read-only (por prefixo) |

Valores dinâmicos vão sempre em `args` (placeholder `?`, nunca concatenados no SQL).

## PII

- **Leitura** (`sqlize_query`, `{engine}_query`, `{engine}_structure`): a saída mascarada — tanto por padrão (CPF, CNPJ, e-mail, telefone, cartão, endereço...) quanto por nome real de coluna (`cpf`, `email`, `senha`, `celular`, `nome`, `cliente`, `razao_social`...).
- **Exportação** (`sqlize_export`, `{engine}_export`): os dados são gravados **sem redação** em `<home>/.local/state/mcp/mnt` — a mesma pasta do `mnt` do sandbox (fixa, sem env). Como fica dentro do mnt, os arquivos ficam imediatamente acessíveis aos scripts do sandbox (`std.io`, `std.sql`), mas fora do alcance da leitura da IA — só o caminho é retornado.
