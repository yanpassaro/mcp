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

- `github_search_code` — busca código (`/search/code`). Aceita os qualificadores
  do GitHub: `repo:`, `org:`, `user:`, `language:`, `path:`, `extension:`,
  `filename:`, `in:file`. Ideal para achar trechos de código e arquivos de docs.
- `github_search_repositories` — busca repositórios (`/search/repositories`),
  com qualificadores `language:`, `stars:`, `topics:`, `user:`, `pushed:`, etc.
- `github_search_issues` — busca issues **e** pull requests (`/search/issues`),
  com qualificadores `is:issue`, `is:pr`, `label:`, `author:`, `assignee:`. A
  saída mostra tipo (Issue/PR), estado (aberto/fechado/mergeado), autor,
  repositório, labels, responsáveis, nº de comentários, datas e o trecho do
  match.
- `github_search_commits` — busca commits (`/search/commits`), com qualificadores
  `repo:`, `author:`, `committer:`, `author-date:`, `merge:`. A saída mostra
  autor/committer, data, repositório, detecção de merge (nº de pais), status de
  assinatura e o trecho da mensagem que motivou o match.
- `github_search_pull_requests` — busca pull requests (`/search/issues` com
  `is:pr` adicionado automaticamente). Qualificadores: `repo:`, `org:`, `is:open`,
  `is:closed`, `is:merged`, `label:`, `author:`, `assignee:`, `base:`, `head:`,
  `review:`. Reusa o formatador de issues (tipo PR, estado, mergeado, autor,
  repositório, labels, responsáveis, comentários, datas e trecho do match).
- `github_list_releases` — lista as releases de um repositório
  (`/repos/{owner}/{repo}/releases`): tag, nome, branch alvo, autor, data,
  rascunho/pré-release, notas e assets.
- `github_get_release_latest` — última release publicada
  (`/repos/{owner}/{repo}/releases/latest`).
- `github_list_wiki` — lista as páginas da wiki de um repositório (acessa o repo
  git `{repo}.wiki` via git trees). Mostra a árvore de arquivos da wiki
  (Home.md, Sidebar.md etc.).
- `github_fetch_wiki` — lê o conteúdo de uma página da wiki
  (`/repos/{owner}/{repo}.wiki/contents/{path}`), retornando o texto em UTF-8
  (normalmente markdown). Use `github_list_wiki` para descobrir os caminhos.
- `github_get_insights` — métricas de Insights de um repositório (stats da API):
  `contributors` (lista de contribuidores e nº de commits), `commit_activity`
  (commits por semana), `code_frequency` (adições/remoções por semana),
  `participation` (commits do dono vs todos nas 52 semanas) e `punch_card`
  (dia/hora com mais commits). Algumas métricas demoram a calcular (HTTP 202)
  e pedem retry.
- `github_search_users` — busca usuários (`/search/users`).
- `github_get_tree` — lista a árvore de arquivos de um repositório (git trees),
  útil para mapear onde estão os docs/código antes de buscar o conteúdo.
- `github_fetch_file` — lê o conteúdo completo de um arquivo
  (`/repos/{owner}/{repo}/contents/{path}`), retornando o texto em UTF-8.

## Observações

- A busca de código no GitHub exige autenticação e tem limites de taxa mais
  restritos que os outros tipos de busca; em caso de erro 403/429, aguarde e
  tente novamente.
- Comandos de busca aceitam o `query` livre com os qualificadores do GitHub.
  Quanto mais específico (ex.: `extension:md path:docs`), melhores os resultados.
- `github_fetch_file` trunca arquivos acima de 200KB.
