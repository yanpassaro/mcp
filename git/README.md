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

## `git_act`

Só é registrada com `GIT_ALLOW_ACT`. Ações destrutivas exigem `data.confirm`.

| `action` | `data` |
| --- | --- |
| `add` | `paths`, `all` |
| `restore` | `paths`, `staged` |
| `commit` | `message`, `all`, `amend`, `allowEmpty` |
| `push` | `remote`, `branch`, `force`, `followTags`, `prune` |
| `pull` | `remote`, `branch`, `force` |
| `fetch` | `remote`, `all`, `tags`, `prune`, `depth` |
| `merge` | `ref`, `noFF` |
| `reset` | `mode` (`soft`/`mixed`/`hard`/`keep`), `ref` |
| `checkout` | `branch`, `create`, `ref`, `force` |
| `branch` | `branch`, `ref`, `delete` |
| `tag` | `name`, `ref`, `message`, `delete` |
| `clone` | `url`, `branch`, `depth` (`repo` = destino) |
| `clean` | — |

```json
{ "repo": "C:/dev/app", "action": "commit", "data": { "message": "feat: cache" } }
```

## Auth

SSH usa o `ssh-agent` (usuário da URL). HTTPS só funciona com repositório público. `GIT_USER`/`GIT_EMAIL` definem o autor dos commits.

## Env

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `GIT_ALLOW_ACT` | desligado | habilita `git_act` (`1`, `true`, `yes`, `on`) |
| `GIT_TIMEOUT_SECONDS` | `60` | timeout local (rede: `300`) |
| `GIT_USER` | `user.name` | autor dos commits |
| `GIT_EMAIL` | `user.email` | e-mail do autor |
