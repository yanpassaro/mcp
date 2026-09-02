# Git MCP

Servidor MCP em Go (**somente leitura**) para inspecionar repositórios Git
locais usando [go-git](https://github.com/go-git/go-git) (Go puro, sem precisar
do `git` CLI). É um módulo independente (`module ntdsk.com/mcp/git`), com estrutura
padrão `cmd/` + `internal/`.

## Tools

| Tool | Descrição |
| --- | --- |
| `git_repo_info` | Info do repositório: raiz, HEAD (branch/SHA), total de commits, branches, tags, top 5 contribuidores e linguagens (share, sem linhas) |
| `git_status` | Estado da working tree: modificados, adicionados, removidos, não rastreados e conflitos |
| `git_log` | Histórico de commits (tabela Markdown) com filtros de autor, path, `since`/`until` e `stat` |
| `git_show` | Detalhes de um commit (autor, data, mensagem, arquivos e diff contra o pai) |
| `git_diff` | Diff flexível: working tree vs HEAD, index vs HEAD (`staged`) ou duas refs (`base`+`head`) |
| `git_refs` | Lista refs do repositório pelo `type`: `branch` (com `all=true` inclui remotas), `remote` (nome + URLs) ou `tag` (SHA + data) |
| `git_blame` | Blame linha a linha (SHA + autor) de um arquivo |
| `git_tree` | Lista arquivos rastreados em uma ref; `path` filtra por prefixo e `pattern` por glob (`**/*.go`) ou substring (`*test*`) |
| `git_read_file` | Lê o conteúdo de um arquivo (working tree ou em uma ref) |
| `git_find_commits` | Busca commits por texto na mensagem |

## Build

Via Taskfile (na raiz deste repositório de MCPs):

```powershell
task build:git
```

Direto:

```powershell
cd git
go mod tidy
go build -o ../dist/git-mcp.exe ./cmd/git-mcp
```

O processo usa **stdio** para o transporte MCP. Não há variáveis de ambiente
obrigatórias — o caminho do repositório é informado por chamada (campo `repo`).

## Logs

```text
~/.local/share/mcp/git/logs/git-<AAAA-MM-DD_HH-MM-SS>.log
```
