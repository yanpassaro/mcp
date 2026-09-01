# MCP servers

Servidores MCP (Model Context Protocol) para agentes e clientes MCP em geral
(Zed, Claude Desktop etc.). Cada servidor é um módulo **independente** (próprio
`go.mod`, `module ntdsk.com/mcp/<server>`) na sua pasta; o único fora do Go é o
`anydoc` (Deno/TypeScript).

Todos os servidores falam o protocolo MCP por **stdio** e gravam os logs em
`~/.local/share/mcp/<server>/logs/` (detalhes em [Logs](#logs-e-dados-persistidos)).

## Servidores

| Servidor | O que faz | Tech | Variável de ambiente principal |
| --- | --- | --- | --- |
| `git-mcp` | Inspeção **read-only** de repositórios Git locais com [go-git](https://github.com/go-git/go-git) (sem `git` CLI): log, diff, branches, blame, stash, submodules | Go | — |
| `github-mcp` | Consulta à API do GitHub (somente leitura): busca de código/commits/issues/PRs/usuários, releases, wiki, insights e árvore de arquivos | Go | `GITHUB_TOKEN` |
| `sqlize-mcp` | Importa, consulta, exporta e compara dados (SQLite em arquivo + Postgres/MySQL read-only), com redator de PII | Go | `SQLIZE_STATE_DIR` (opcional) |
| `fetch-mcp` | Testa endpoints HTTP da allowlist (`FETCH_ALLOW_HOST`, anti-SSRF): timing, status, headers, corpo JSON/XML pretty-printed, cookies e comando `curl` equivalente | Go | `FETCH_ALLOW_HOST` |
| `anydoc` | Converte e exporta documentos (Word, PDF, Excel, OpenDocument, RTF, EPUB, CSV) com redator de PII embutido | Deno/TypeScript | — |

## Tools principais (resumo)

A lista completa está no README de cada servidor:

- **git**: `git_repo_info`, `git_status`, `git_log`, `git_show`, `git_diff`,
  `git_blame`, `git_tree`, `git_read_file`, `git_find_files`, `git_contributors`
- **github**: `github_search_code`, `github_search_issues`, `github_search_commits`,
  `github_list_releases`, `github_list_wiki`/`github_fetch_wiki`,
  `github_get_insights`, `github_get_tree`, `github_fetch_file`
- **sqlize**: `sqlize_import`, `sqlize_structure`, `sqlize_query`,
  `sqlize_export`, `sqlize_compare` + tools de bancos ao vivo
  (`postgres_*`/`mysql_*`)
- **fetch**: `fetch_request`, `cookie_list`, `cookie_clear`, `fetch_allowlist`,
  `fetch_history`
- **anydoc**: `anydoc_convert_to_markdown`, `anydoc_export_to_pdf`,
  `anydoc_export_to_docx`, `anydoc_export_to_xlsx`

## Layout padrão

```text
<server>/
├── cmd/<server>-mcp/main.go     # entrypoint: env vars, setupLog, registro das tools
├── internal/
│   ├── <api>/client.go          # cliente da API (pacote com o nome da API)
│   └── mcpserver/               # pacote mcpserver
│       ├── server.go            # registro das tools + handlers
│       └── format.go            # formatação da saída (Markdown)
├── go.mod                       # module ntdsk.com/mcp/<server>
└── README.md
```

## Requisitos

- Go 1.26+ (os `go.mod` declaram `go 1.26.0`);
- [Task](https://taskfile.dev/) v3 (opcional — para build via `Taskfile.yml`);
- Deno 1.x+ (apenas para o `anydoc`).

## Build

O `Taskfile.yml` fixa `GOOS=windows`, `GOARCH=amd64` e `CGO_ENABLED=0` e gera
os executáveis em `dist/` (pasta criada automaticamente e ignorada pelo git):

```powershell
task build          # todos os servidores
task build:git      # apenas um servidor (git, github, sqlize, fetch, anydoc)
task clean          # remove dist/
```

Direto, por servidor (exemplo do `git`):

```powershell
cd git
go mod tidy
go build -o ../dist/git-mcp.exe ./cmd/git-mcp
```

O `anydoc` compila via Deno (o binário também vai para `dist/anydoc.exe`):

```powershell
cd anydoc
deno task compile   # ou "deno task start" para rodar sem compilar
```

## CI / Release

O workflow [`.github/workflows/build-windows.yml`](./.github/workflows/build-windows.yml)
compila todos os servidores para Windows (`dist/*.exe`) no GitHub Actions e
publica os binários no **GitHub Release**, prontos para baixar (junto com um
arquivo `SHA256SUMS` para conferência):

- **Push para `main`** — atualiza a release rolante **`latest`**, sem precisar de
  tag: o link `https://github.com/yanpassaro/mcp/releases/latest` sempre aponta
  para os binários mais recentes;
- **Push de tag `v*`** (ex.: `git tag v1.0.0 && git push origin v1.0.0`) — cria a
  release versionada da tag;
- **Workflow manual** (aba *Actions* → *Build Windows* → *Run workflow*) — build
  sob demanda; se informar uma **tag** no input, cria/atualiza a release dela
  (vazio = atualiza a `latest`).

## Logs e dados persistidos

Todos os servidores Go gravam o log (apenas em stderr/arquivo, **nunca stdout**,
para não quebrar o protocolo stdio do MCP) em:

```text
~/.local/share/mcp/<server>/logs/<server>-<AAAA-MM-DD_HH-MM-SS>.log
# Windows: C:\Users\<usuário>\.local\share\mcp\<server>\logs\<server>-<data>.log
```

Se a pasta/arquivo não puder ser criado, o log cai para stderr com um aviso
(o servidor não quebra). O `anydoc` (Deno) também usa apenas stderr, sem arquivo.

Dados persistidos ficam em locais específicos de cada servidor:

- **fetch** — cookies em `~/.local/share/mcp/fetch/cookies.json`
  (configurável via `FETCH_COOKIE_FILE`);
- **sqlize** — banco de estado em `~/.local/state/sqlize/sqlize.db`
  (configurável via `SQLIZE_STATE_DIR`).

## Variáveis de ambiente

| Servidor | Variável | Padrão | Descrição |
| --- | --- | --- | --- |
| `git` | — | — | sem variáveis obrigatórias (repositório informado por chamada) |
| `github` | `GITHUB_TOKEN` | obrigatório | Personal Access Token |
| | `GITHUB_BASE_URL` | `https://api.github.com` | URL da API (GitHub Enterprise) |
| | `GITHUB_TIMEOUT_SECONDS` | `60` | timeout de cada requisição HTTP |
| `sqlize` | `SQLIZE_STATE_DIR` | `~/.local/state/sqlize` | pasta do banco de estado |
| | `{PREFIXO}_POSTGRES_URL` / `_DSN` | — | conexão Postgres read-only (1 por prefixo) |
| | `{PREFIXO}_MYSQL_URL` / `_DSN` | — | conexão MySQL read-only (1 por prefixo) |
| | `SQLIZE_PII_NAMES` / `SQLIZE_PII_WORDS` | — | reforços do redator PII (nomes/termos do domínio) |
| `fetch` | `FETCH_ALLOW_HOST` | `localhost,127.0.0.1,::1` | allowlist de hosts (`.domínio` libera subdomínios) |
| | `FETCH_TIMEOUT_SECONDS` | `30` | timeout padrão de cada requisição |
| | `FETCH_MAX_BODY_KB` | `1024` | teto do corpo da resposta |
| | `FETCH_COOKIE_FILE` | `~/.local/share/mcp/fetch/cookies.json` | persistência dos cookies |
| `anydoc` | — | — | sem variáveis de ambiente |

## Exemplo de configuração

Gere os executáveis com `task build` e registre os servidores no seu cliente
MCP no Zed (bloco `context_servers` em `~/.config/zed/settings.json`), com
caminhos no padrão Linux (`~` expande para o home do usuário):

```json
{
  "context_servers": {
    "git": {
      "command": "~/nautidesk/mcp/dist/git-mcp.exe"
    },
    "github": {
      "command": "~/nautidesk/mcp/dist/github-mcp.exe",
      "env": {
        "GITHUB_TOKEN": "<seu-token>"
      }
    },
    "sqlize": {
      "command": "~/nautidesk/mcp/dist/sqlize-mcp.exe"
    },
    "fetch": {
      "command": "~/nautidesk/mcp/dist/fetch-mcp.exe",
      "env": {
        "FETCH_ALLOW_HOST": "localhost,example.com"
      }
    },
    "anydoc": {
      "command": "~/nautidesk/mcp/dist/anydoc.exe"
    }
  }
}
```

Veja o README de cada servidor para as variáveis de ambiente específicas.
