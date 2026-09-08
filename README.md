# MCP servers

Servidores MCP (stdio) para agentes/clientes como Zed e Claude Desktop. Cada um é um módulo independente; os Go têm `go.mod` próprio, o `anydoc` roda em Deno.

| Servidor | O que faz |
| --- | --- |
| `git-mcp` | Inspeção **read-only** de repositórios Git locais (go-git, sem CLI) |
| `github-mcp` | Consulta **read-only** à API do GitHub (busca, arquivos, issues/PRs, releases) |
| `sqlize-mcp` | Importa, consulta e exporta dados (SQLite + Postgres/MySQL read-only) |
| `anydoc` | Converte/exporta documentos (Word, PDF, Excel, ODT, RTF, EPUB, CSV) |
| `sandbox-mcp` | Sandbox **não-destrutivo** de scripts Lua (API `std`, filesystem único, rede via allowlist) |

## Tools

- **git**: `git_repo_info`, `git_status`, `git_log`, `git_show`, `git_diff`, `git_refs`, `git_blame`, `git_tree`, `git_read_file`, `git_find_commits`
- **github**: `github_search`, `github_get_tree`, `github_read_file`, `github_repo_info`, `github_get_item`
- **sqlize**: `sqlize_import`, `sqlize_structure`, `sqlize_query`, `sqlize_export` (+ `postgres_*`/`mysql_*`)
- **anydoc**: `anydoc_import`, `anydoc_export`
- **sandbox**: `sandbox_run` (run por `path`/`code` + `args`), `sandbox_doc` (API `std`), `sandbox_os` (copy/mount/del/stat/list)

## Variáveis de ambiente

| Servidor | Variável | Padrão | Descrição |
| --- | --- | --- | --- |
| `github` | `GITHUB_TOKEN` | obrigatório | Personal Access Token |
| | `GITHUB_BASE_URL` | `https://api.github.com` | URL da API (GitHub Enterprise) |
| | `GITHUB_TIMEOUT_SECONDS` | `60` | timeout por requisição |
| `sqlize` | `SQLIZE_STATE_DIR` | `~/.local/state/sqlize` | pasta do banco de estado |
| | `{PREFIXO}_POSTGRES_URL` / `_DSN` | — | conexão Postgres read-only |
| | `{PREFIXO}_MYSQL_URL` / `_DSN` | — | conexão MySQL read-only |
| `sandbox` | `SANDBOX_MEM_LIMIT_MB` | `512` | teto de RAM do processo sandbox |
| | `SANDBOX_MNT_SPACE_MB` | `256` | teto de espaço em `mnt/` |
| | `SANDBOX_SQL_MAX_ROWS` | `10000` | teto de linhas no `std.sql.query` |
| | `SANDBOX_FETCH_ALLOW_HOST` | `localhost,127.0.0.1,::1` | allowlist do `std.fetch` |
| | `SECRET_*` | — | secrets lidas via `std.secrets.get("chave")`; valores são redigidos em toda saída |

O filesystem do sandbox (`mnt`) e a exportação do sqlize ficam **fixos** em `~/.local/state/mcp/mnt` (pasta compartilhada, sem env): o `mnt` do sandbox é `~/.local/state/mcp/mnt` e `tmp` é `~/.local/state/mcp/tmp`.

## Exemplo (Zed)

```json
{
  "context_servers": {
    "git":     { "command": "~/.local/bin/git-mcp.exe" },
    "github":  { "command": "~/.local/bin/github-mcp.exe", "env": { "GITHUB_TOKEN": "<token>" } },
    "sqlize":  { "command": "~/.local/bin/sqlize-mcp.exe" },
    "sandbox": { "command": "~/.local/bin/sandbox-mcp.exe" },
    "anydoc":  { "command": "~/.local/bin/anydoc.exe" }
  }
}
```

Build: `task build` gera os executáveis em `dist/`. Logs: `~/.local/share/mcp/<server>/logs/`.
