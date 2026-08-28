# Git MCP

Servidor MCP em Go (**somente leitura**) para inspecionar repositórios Git
locais usando [go-git](https://github.com/go-git/go-git) (Go puro, sem precisar
do `git` CLI). É um módulo independente (`module ntdsk.com/mcp/git`), com estrutura
padrão `cmd/` + `internal/`.

## Tools

| Tool | Descrição |
| --- | --- |
| `git_repo_info` | Info do repositório: raiz, HEAD (branch/SHA), total de commits, branches, tags e remotes |
| `git_status` | Estado da working tree: modificados, adicionados, removidos, não rastreados e conflitos |
| `git_log` | Histórico de commits (tabela Markdown) com filtros de autor, path, `since`/`until` e `stat` |
| `git_show` | Detalhes de um commit (autor, data, mensagem, arquivos e diff contra o pai) |
| `git_diff` | Diff flexível: working tree vs HEAD, index vs HEAD (`staged`) ou duas refs (`base`+`head`) |
| `git_contributors` | Top contribuidores no estilo GitHub (com barrinha e pódio) |
| `git_branch_list` | Lista branches (com `all=true` inclui remotas) |
| `git_branch_compare` | Compara duas refs: ahead/behind, arquivos alterados e commits de diferença |
| `git_remote_list` | Lista os remotes configurados |
| `git_tag_list` | Lista tags com SHA e data |
| `git_blame` | Blame linha a linha (SHA + autor) de um arquivo |
| `git_file_history` | Histórico de um arquivo (`git log -- <path>`) |
| `git_tree` | Lista arquivos rastreados em uma ref |
| `git_read_file` | Lê o conteúdo de um arquivo (working tree ou em uma ref) |
| `git_find_files` | Busca arquivos por glob (`*`) ou substring |
| `git_find_commits` | Busca commits por texto na mensagem |
| `git_check_ignore` | Informa se um path está ignorado (`.gitignore`) |
| `git_stash_list` | Lista os stashes |
| `git_submodule_status` | Estado dos submodules |

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
