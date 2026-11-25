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
| `pull` | `remote`, `branch`, `force` (via `git pull`) |
| `push` | `remote`, `branch`, `tags` (via `git push`) |
| `fetch` | `remote`, `all`, `tags`, `prune`, `depth` (via `git fetch`) |
| `merge` | `ref`, `ff`/`ffOnly`, `squash`, `message`, `strategy`, `strategyOption`, `noCommit`, `edit`, `abort`, `continue` (via `git merge`) |
| `reset` | `mode` (`soft`/`mixed`/`hard`/`keep`), `ref` |
| `checkout` | `branch`, `create`, `ref`, `force`, `keep` |
| `branch` | `branch`, `ref`, `delete` |
| `tag` | `name`, `ref`, `message`, `delete` |
| `clone` | `url`, `branch`, `depth` (`repo` = destino; via `git clone`) |
| `clean` | — |

```json
{ "repo": "C:/dev/app", "action": "commit", "data": { "message": "feat: cache" } }
```

## Auth

`clone`, `pull`, `fetch`, `push` e `merge` rodam o binário **`git`** e usam a auth do próprio git (credential helper, ssh-agent) — inclusive HTTPS privado. As ações locais (`add`, `restore`, `commit`, `reset`, `checkout`, `branch`, `tag`, `clean`) usam go-git. `GIT_USER`/`GIT_EMAIL` definem o autor dos commits.

## Env

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `GIT_ALLOW_ACT` | desligado | habilita `git_act` (`1`, `true`, `yes`, `on`) |
| `GIT_TIMEOUT_SECONDS` | `60` | timeout local (rede: `300`) |
| `GIT_USER` | `user.name` | autor dos commits |
| `GIT_EMAIL` | `user.email` | e-mail do autor |
| `GIT_COAUTHOR` | — | co-autores fixos, `Nome <email>` separados por vírgula |
