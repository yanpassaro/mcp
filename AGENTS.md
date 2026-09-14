# AGENTS.md

Contexto para agentes que trabalham neste repositório.

## Estrutura

Repositório de **servidores MCP (stdio)**, cada um em um módulo independente:

- `git/` · `github/` · `sqlize/` · `anydoc/` (Deno) · `sandbox/` — cada um com `go.mod` próprio.
- `Taskfile.yml` — build via `task` (`task build` gera os `.exe` em `dist/`).
- O `sandbox/` é um interpretador Lua isolado: expõe a API `std.*` (módulos como `pipe`, `infer`, `missing`, `text`, `path`, `stats`, `schema`, `diff`, `seq`, `duration`, `unit`, `random`, `io`, `encode`, …). Cada módulo é um `buildXxx(L *lua.State) int` registrado em `internal/sandbox/runner.go` e documentado em `internal/mcpserver/lunadoc.go` (`var lunaXxx = &lunaMod{...}` + `lunaOrder`).

## Convenções de estilo (Go)

- **Não comente o código** — sem `//` nem doc comments. Nomeie bem funções/variáveis; o código deve se explicar.
- **Prefira `for ... range`** em vez de loops por índice como `for i := 0; i < n; i++`. Ex.: `for i, x := range d { ... }` em vez de `for i := 0; i < len(d); i++ { d[i] ... }`.
- Use `range` também sobre a fonte de dados quando possível (ex.: iterar a própria slice, não o comprimento).
- `gofmt` é obrigatório; rode `task format` no módulo alterado (`gofmt -w .`, `go mod tidy`, `go vet ./...`).

## Convenções do sandbox

- Módulos `std.*` retornam valores via `pushAny(l, ...)`; erros de tipo/uso usam `panic(...)` (o sandbox captura como erro do script).
- Valores inválidos retornam `nil`/`false`, não `panic`, a menos que seja erro de *tipo*.
- Datas/durações em **ms** (`std.date`/`std.duration`); timezone IANA (`std.date`, `std.cron`).
- Ao adicionar um módulo: criar `buildXxx` em `internal/sandbox/xxx.go`, registrar em `runner.go`, e documentar em `lunadoc.go` (adicionar em `lunaOrder` + `var lunaXxx`).
