# Git MCP

Inspeciona repositórios Git locais (**somente leitura**) com `go-git` (não precisa do `git` CLI). O repositório é informado por chamada no campo `repo`.

## Tools

| Tool | O que faz |
| --- | --- |
| `git_repo_info` | Info do repositório: raiz, HEAD, commits, branches, tags, top contribuidores, linguagens |
| `git_status` | Estado da working tree (modificados, adicionados, removidos, conflitos) |
| `git_log` | Histórico de commits em tabela Markdown (autor, path, `since`/`until`, `stat`) |
| `git_show` | Detalhes de um commit (autor, data, mensagem, diff contra o pai) |
| `git_diff` | Diff: working tree vs HEAD, index (`staged`), ou duas refs (`base`+`head`) |
| `git_refs` | Lista refs por `type` (`branch`, `remote`, `tag`) |
| `git_blame` | Blame linha a linha (SHA + autor) |
| `git_tree` | Lista arquivos rastreados numa ref; `path`/`pattern` filtram |
| `git_read_file` | Conteúdo de um arquivo (working tree ou numa ref) |
| `git_find_commits` | Busca commits por texto na mensagem |

Sem variáveis de ambiente.
