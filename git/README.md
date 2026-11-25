# Git MCP

Inspeciona e altera repositórios Git locais. Toda chamada usa `repo`, `action` e `data`.

## `git_search`

| `action` | `data` |
| --- | --- |
| `commits` | `query`, `author`, `path`, `since`, `until`, `stat`, `maxCount` |
| `files` | `ref`, `path`, `pattern` |

## `git_read`

| `action` | `data` |
| --- | --- |
| `info` | — |
| `status` | — |
| `refs` | `type` (`branch`/`remote`/`tag`), `all` |
| `file` | `path`, `ref` |
| `show` | `ref` |
| `diff` | `base`, `head`, `staged`, `path`, `context`, `stat` |
| `blame` | `path`, `maxLines` |
| `grep` | `pattern` (regexp), `ref`, `path` (regexp), `max` |

## `git_act`

Só é registrada com `GIT_ALLOW_ACT`.

| `action` | `data` |
| --- | --- |
| `add` | `paths`, `all` |
| `restore` | `paths`, `staged` |
| `commit` | `message`, `all`, `amend`, `allowEmpty`, `coAuthors` |
| `pull` | `remote`, `branch`, `force` |
| `push` | `remote`, `branch`, `tags` (`--follow-tags`) |
| `fetch` | `remote`, `all`, `tags`, `prune`, `depth` |
| `merge` | `ref` (só fast-forward; exige working tree limpo) |
| `reset` | `mode` (`soft`/`mixed`/`hard`/`keep`), `ref` |
| `checkout` | `branch`, `create`, `ref`, `force`, `keep` |
| `branch` | `branch`, `ref`, `delete` |
| `tag` | `name`, `ref`, `message`, `delete` |
| `clone` | `url`, `branch`, `depth` (`repo` = destino) |
| `clean` | — |

```json
{ "repo": "C:/dev/app", "action": "commit", "data": { "message": "feat: cache" } }
```

## Auth

SSH usa o `ssh-agent`. HTTPS usa `GIT_TOKEN` (token/PAT; usuário via `GIT_HTTP_USER`, default `oauth2`) ou credenciais na própria URL; sem token, só repositório público. `GIT_USER`/`GIT_EMAIL` definem o autor dos commits.

## Env

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `GIT_ALLOW_ACT` | desligado | habilita `git_act` (`1`, `true`, `yes`, `on`) |
| `GIT_TIMEOUT_SECONDS` | `60` | timeout local (rede: `300`) |
| `GIT_USER` | `user.name` | autor dos commits |
| `GIT_EMAIL` | `user.email` | e-mail do autor |
| `GIT_COAUTHOR` | — | co-autores fixos, `Nome <email>` separados por vírgula |
| `GIT_TOKEN` | — | token/PAT para remotos HTTPS |
| `GIT_HTTP_USER` | `oauth2` | usuário do HTTPS (com `GIT_TOKEN`) |
