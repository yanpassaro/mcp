# MCP servers

Servidores MCP (Model Context Protocol) para agentes/clientes como Zed e Claude
Desktop. Cada um é um módulo **independente** na sua pasta (os Go têm `go.mod`
próprio; o único fora do Go é o `anydoc`, em Deno/TypeScript). Todos falam o
protocolo por **stdio** e logam em `~/.local/share/mcp/<server>/logs/`.

## Servidores

| Servidor | O que faz | Tech | Env principal |
| --- | --- | --- | --- |
| `git-mcp` | Inspeção **read-only** de repositórios Git locais (go-git, sem CLI): log, diff, branches, blame, refs, árvore, arquivos | Go | — |
| `github-mcp` | Consulta **read-only** à API do GitHub: busca, arquivos, issues/PRs, releases, wiki | Go | `GITHUB_TOKEN` |
| `sqlize-mcp` | Importa, consulta, exporta e compara dados (SQLite + Postgres/MySQL read-only), com redator de PII | Go | `SQLIZE_STATE_DIR` |
| `anydoc` | Converte/exporta documentos (Word, PDF, Excel, OpenDocument, RTF, EPUB, CSV) com redator de PII embutido | Deno/TS | — |
| `sandbox-mcp` | Sandbox **não-destrutivo** de scripts **Lua** (go-lua): API `std` (filesystem único `fs`, JSON, data, texto, listas, números, assert, encode) e rede opcional via `std.fetch` (allowlist). Sem acesso a processo/SO | Go | `SANDBOX_FS_DIR`, `SANDBOX_SCRIPTS_DIR` |

## Tools principais

- **git**: `git_repo_info`, `git_status`, `git_log`, `git_show`, `git_diff`, `git_refs`, `git_blame`, `git_tree`, `git_read_file`, `git_find_commits`
- **github**: `github_search`, `github_get_tree`, `github_read_file`, `github_repo_info`, `github_get_item`
- **sqlize**: `sqlize_import`, `sqlize_structure`, `sqlize_query`, `sqlize_export`, `sqlize_clear` (+ `postgres_*`/`mysql_*`)
- **anydoc**: `anydoc_import`, `anydoc_export`
- **sandbox**: `sandbox_read`, `sandbox_write`, `sandbox_del`, `sandbox_run`

## Build

Gera os executáveis em `dist/` (`GOOS=windows`, `GOARCH=amd64`, `CGO_ENABLED=0`):

```powershell
task build        # todos os servidores
task build:git    # um servidor (git, github, sqlize, anydoc, sandbox)
task clean        # remove dist/
```

Os servidores Go também compilam direto (`cd <server> && go build -o ../dist/<server>-mcp.exe ./cmd/<server>-mcp`); o `anydoc` via `cd anydoc && deno task compile`.

## Logs

Todos os servidores Go gravam o log apenas em stderr/arquivo (nunca stdout, para
não quebrar o stdio):

```text
~/.local/share/mcp/<server>/logs/<server>-<AAAA-MM-DD_HH-MM-SS>.log
# Windows: C:\Users\<usuário>\.local\share\mcp\<server>\logs\
```

O `anydoc` usa só stderr. Dados persistidos: `sqlize` → `~/.local/state/sqlize/sqlize.db`.

## Variáveis de ambiente

| Servidor | Variável | Padrão | Descrição |
| --- | --- | --- | --- |
| `github` | `GITHUB_TOKEN` | obrigatório | Personal Access Token |
| | `GITHUB_BASE_URL` | `https://api.github.com` | URL da API (GitHub Enterprise) |
| | `GITHUB_TIMEOUT_SECONDS` | `60` | timeout por requisição |
| `sqlize` | `SQLIZE_STATE_DIR` | `~/.local/state/sqlize` | pasta do banco de estado |
| | `{PREFIXO}_POSTGRES_URL` / `_DSN` | — | conexão Postgres read-only (por prefixo) |
| | `{PREFIXO}_MYSQL_URL` / `_DSN` | — | conexão MySQL read-only (por prefixo) |
| `sandbox` | `SANDBOX_FS_DIR` | `~/.local/share/mcp/sandbox/fs` | filesystem do script |
| | `SANDBOX_SCRIPTS_DIR` | `~/.local/share/mcp/sandbox/scripts` | scripts do agente |
| | `SANDBOX_FETCH_ALLOW_HOST` | `localhost,127.0.0.1,::1` | allowlist do `std.fetch` |

Veja o README de cada servidor para as variáveis e comportamentos específicos.

## Exemplo de configuração (Zed)

```json
{
  "context_servers": {
    "git":     { "command": "~/nautidesk/mcp/dist/git-mcp.exe" },
    "github":  { "command": "~/nautidesk/mcp/dist/github-mcp.exe", "env": { "GITHUB_TOKEN": "<token>" } },
    "sqlize":  { "command": "~/nautidesk/mcp/dist/sqlize-mcp.exe" },
    "sandbox": { "command": "~/nautidesk/mcp/dist/sandbox-mcp.exe" },
    "anydoc":  { "command": "~/nautidesk/mcp/dist/anydoc.exe" }
  }
}
```
