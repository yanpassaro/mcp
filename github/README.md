# GitHub MCP

Servidor MCP em Go (somente leitura) para consultar a API do GitHub, focado em
**pesquisa de documentação e código para agentes**. É um projeto **independente**
(`module ntdsk.com/mcp/github`) e não importa nada dos outros MCPs do repositório.

As tools retornam o conteúdo formatado em Markdown (tabelas ou texto) para
facilitar a leitura pelo assistente.

## Requisitos

- Go 1.26 ou mais recente;
- um Personal Access Token do GitHub (escopo `read` ou `repo`, dependendo dos
  repositórios — `public_repo`/`repo` para repositórios privados).

## Configuração

Variáveis de ambiente:

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `GITHUB_TOKEN` | obrigatório | Personal Access Token |
| `GITHUB_BASE_URL` | `https://api.github.com` | URL da API (use para GitHub Enterprise) |
| `GITHUB_TIMEOUT_SECONDS` | `60` | Timeout de cada requisição HTTP |

## Build e execução

Via Taskfile (na raiz deste repositório de MCPs):

```powershell
task build:github
```

Direto:

```powershell
cd github
go mod tidy
go build -o ../dist/github-mcp.exe ./cmd/github-mcp
$env:GITHUB_TOKEN = "seu-token"
..\dist\github-mcp.exe
```

O processo usa **stdio** para o transporte MCP. Integre-o em um cliente MCP
(Zed, Claude Desktop etc.) apontando para o binário compilado com as variáveis
de ambiente acima.

Estrutura: `cmd/github-mcp/main.go` (entrypoint), `internal/github/client.go`
(cliente da API REST) e `internal/mcpserver/` (registro das tools + formatação).

## Logs

```text
~/.local/share/mcp/github/logs/github-<AAAA-MM-DD_HH-MM-SS>.log
```

## Tools

- `github_search` — busca unificada. O campo `type` escolhe o endpoint:
  `code` (`/search/code`), `repo` (`/search/repositories`), `issue`
  (`/search/issues`), `pr` (o mesmo com `is:pr` automático), `commit`
  (`/search/commits`; com `repo:owner/name` lista os commits do repo) e `user`
  (`/search/users`). O `query` livre aceita os qualificadores do GitHub de cada
  tipo (ex.: `extension:go repo:owner/name`, `topic:llm stars:>100`,
  `bug is:issue is:open label:bug`); `sort`/`order`/`perPage`/`page` opcionais.

- `github_repo_info` — informações de um repositório (`/repos/{owner}/{repo}`):
  stars, forks, licença, branch padrão, topics, descrição, datas — e as **últimas
  5 releases**.
- `github_get_tree` — lista a árvore de arquivos de um repositório (git trees),
  útil para mapear onde estão os docs/código antes de buscar o conteúdo.
- `github_read_file` — lê o conteúdo completo de um arquivo
  (`/repos/{owner}/{repo}/contents/{path}`), retornando o texto em UTF-8.
- `github_get_item` — lê uma issue ou PR específico por número, com `type`
  (`issue` ou `pr`).

## Observações

- A busca de código no GitHub exige autenticação e tem limites de taxa mais
  restritos que os outros tipos de busca; em caso de erro 403/429, aguarde e
  tente novamente.
- Comandos de busca aceitam o `query` livre com os qualificadores do GitHub.
  Quanto mais específico (ex.: `extension:md path:docs`), melhores os resultados.
- `github_read_file` trunca arquivos acima de 200KB.
