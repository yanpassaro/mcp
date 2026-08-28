# MCP servers

Servidores MCP usados pelos agentes do Zed, padronizados conforme o
[`PADRÃO.md`](./PADRÃO.md) em `%USERPROFILE%\.local\share\mcp\<server>\logs`.

## Servidores

| Servidor | O que faz | Tech | Variável de ambiente principal |
| --- | --- | --- | --- |
| `git-mcp` | Inspeção read-only de repositórios Git locais (go-git, sem git CLI) | Go | — |
| `github-mcp` | Busca de código, docs, issues, PRs, releases e árvore de arquivos na API do GitHub | Go | `GITHUB_TOKEN` |
| `search-mcp` | Busca web e fetch de páginas via Exa Search API | Go | `EXA_API_KEY` |
| `sqlize-mcp` | Importa, consulta, exporta e compara dados (SQLite local + Postgres/MySQL read-only) | Go | `SQLIZE_STATE_DIR` (opcional) |
| `vision-mcp` | Análise de imagens/vídeos com modelo de visão OpenAI-compatible | Go | `VISION_API_KEY` |
| `fetch-mcp` | Testa endpoints HTTP da allowlist (`FETCH_ALLOW_HOST`): status, headers, corpo JSON/XML; gerencia cookies | Go | `FETCH_ALLOW_HOST` |
| `anydoc` | Converte e exporta documentos (Word, PDF, Excel, etc.) | Deno/TypeScript | — |

Cada servidor é um módulo Go **independente** (próprio `go.mod`, `module
ntdsk.com/mcp/<server>`) na sua pasta. O único servidor fora do Go é o `anydoc` (Deno).

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

- Go 1.25+ (os módulos usam `go 1.26`);
- [Task](https://taskfile.dev/) v3 (opcional — para build via `Taskfile.yml`);
- Deno (apenas para o `anydoc`).

## Build

Com Task (gera os `.exe` em `dist/`):

```powershell
task build
```

Direto, por servidor:

```powershell
cd git
go build -o ../dist/git-mcp.exe ./cmd/git-mcp
```

## Logs

Todos os servidores Go gravam o log (apenas stderr/arquivo, **nunca stdout**,
para não quebrar o protocolo stdio do MCP) em:

```text
~/.local/share/mcp/<server>/logs/<server>-<AAAA-MM-DD_HH-MM-SS>.log
# Windows: C:\Users\<usuário>\.local\share\mcp\<server>\logs\<server>-<data>.log
```

Se a pasta/arquivo não puder ser criado, o log cai para stderr com um aviso
(o servidor não quebra). Dados persistidos (ex.: banco do `sqlize`) também
ficam em `~/.local/share/mcp/<server>/`.

## Exemplo de configuração (Zed)

Adicione os servidores em `~/.config/zed/settings.json` (bloco `context_servers`):

```json
{
  "context_servers": {
    "git": {
      "command": "C:/Users/<usuario>/nautidesk/mcp/dist/git-mcp.exe"
    },
    "github": {
      "command": "C:/Users/<usuario>/nautidesk/mcp/dist/github-mcp.exe",
      "env": {
        "GITHUB_TOKEN": "<seu-token>"
      }
    },
    "search": {
      "command": "C:/Users/<usuario>/nautidesk/mcp/dist/search-mcp.exe",
      "env": {
        "EXA_API_KEY": "<sua-chave>"
      }
    },
    "sqlize": {
      "command": "C:/Users/<usuario>/nautidesk/mcp/dist/sqlize-mcp.exe"
    },
    "vision": {
      "command": "C:/Users/<usuario>/nautidesk/mcp/dist/vision-mcp.exe",
      "env": {
        "VISION_API_KEY": "<sua-chave>"
      }
    },
    "fetch": {
      "command": "C:/Users/<usuario>/nautidesk/mcp/dist/fetch-mcp.exe",
      "env": {
        "FETCH_ALLOW_HOST": "localhost,example.com"
      }
    },
    "anydoc": {
      "command": "C:/Users/<usuario>/nautidesk/mcp/dist/anydoc.exe",
    }
  }
}
```

Veja o README de cada servidor para as variáveis de ambiente específicas.
