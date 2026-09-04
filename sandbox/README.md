# sandbox-mcp

Servidor MCP (stdio) em Go que dá à IA um **sandbox de scripts não-destrutivo**:
a IA **escreve** scripts, **lê** os que já existem, **roda** e consulta **documentação** — tudo isolado.

Cada script é uma pequena **ferramenta** com contrato claro: metadados (`name`,
`desc`) e uma função `main(std)` que retorna `std.return.ok(...)` ou
`std.return.err(...)`. O script roda numa VM JavaScript (**goja**) dentro do
servidor, então **não tem acesso ao SO** — sem subprocessos, sem variáveis de
ambiente; rede apenas via `std.fetch` (allowlist). O acesso a arquivos é
**confinado a um filesystem único** (pasta `fs/`, via `std.fs`). A saída é
devolvida em **Markdown**
(objetos viram linhas `**chave:** valor`).

## Tools

| Tool | O que faz |
| --- | --- |
| `sandbox_write_script` | Grava/sobrescreve um script (envolve o corpo), ou remove (`delete`). |
| `sandbox_read_script` | Lista todos os scripts (sem `name`) ou lê um (`name`). |
| `sandbox_run_script` | Executa um script salvo (`name`) ou inline (`code`). |
| `sandbox_help` | Documentação em **inglês** do `std`. |

## Formato do script

```js
const name = "gerador.js";
const desc = "Gera nomes a partir das wordlists.";

export function main(std) {
  const a = std.fs.lines('first.txt');
  const b = std.fs.lines('last.txt');
  const nomes = [];
  for (let i = 0; i < 5; i++)
    nomes.push(std.random.pick(a) + ' ' + std.random.pick(b));
  std.fs.write('nomes.txt', nomes.join('\n'));
  return std.return.ok({ nomes, salvos: std.fs.dir() });
}
```

## Escrevendo via `sandbox_write_script`

Você passa **só o corpo** de `main(std)`; o servidor envolve com `name`/`description`:

```jsonc
{
  "name": "gerador.js",
  "description": "Gera nomes e salva em fs/.",
  "code": "const a = std.fs.lines('first.txt');\nconst b = std.fs.lines('last.txt');\nconst nomes = [];\nfor (let i = 0; i < 5; i++) nomes.push(std.random.pick(a) + ' ' + std.random.pick(b));\nstd.fs.write('nomes.txt', nomes.join('\\n'));\nreturn std.return.ok({ nomes });"
}
```

## API `std`

| Namespace | Funções |
| --- | --- |
| `return` | `ok(dados)` (sucesso) · `err(msg)` (falha + `IsError`) |
| `log` | `ok(...)` / `err(...)` (imprime) · `console.log/error` são alias |
| `args` | o que você passa no campo `args` (JSON é interpretado) |
| `fs` | `read` · `lines` · `json` · `write` · `append` · `del` · `exists` · `stat` · `dir` (filesystem único do sandbox — leitura **e** escrita) |
| `random` | `pick(arr)` · `shuffle(arr)` · `int(min,max)` · `seed(v)` (deixa `std.random`/`Math.random` determinístico) |
| `date` | `now()` · `iso([data])` · `format("YYYY-MM-DD"[ ,data])` · `parse(str)` · `add(data,n,unidade)` · `unix([data])` · `diff(a,b,unidade)` |
| `str` | `normalize(s)` · `slug(s)` · `title(s)` · `camel(s)` · `pascal(s)` · `snake(s)` · `kebab(s)` · `wrap(s,n)` · `summarize(s,max)` · `format(tpl,ctx)` · `count(s,sub)` · `split(s,sep[,n])` |
| `list` | `chunk(arr,n)` · `groupBy(arr,chave|fn)` · `unique(arr)` · `flatten(arr)` · `sortBy(arr,chave|fn)` · `countBy(arr,chave|fn)` · `first(arr[,n])` · `last(arr[,n])` |
| `num` | `round(n,d)` · `clamp(n,a,b)` · `percent(a,b)` · `sum(arr)` · `avg(arr)` · `parse(s)` · `fmt(n[,loc][,dec])` |
| `json` | `parse(s)` · `stringify(obj[,indent])` · `format(obj)` · `minify(s)` · `path(obj,"a.b.c")` |
| `assert` | `ok(v[,msg])` · `equal(a,b[,msg])` · `throws(fn)` |
| `fetch` | `request(url[,opts])` · `get(url[,opts])` · `post(url[,body][,opts])` · `cookies.list()` · `cookies.clear([dom])` · `cookies.set(dom,nome,val[,opts])` |
| `encode` | `crc32(s)` · `md5(s)` · `sha256(s)` · `base64(s[,modo])` · `hex(s[,modo])` |

- **`std.args`** → o que você passa na chamada (campo `args`; JSON é interpretado).
- **`std.fs.*`** → opera no filesystem único do sandbox: `read/lines/json` (conteúdo), `write/append/writeJson/del` (gravação), `exists/stat/dir` (inspeção). A mesma pasta serve para ler e escrever.
- **Para gravar JSON** use `std.fs.write(nome, std.json.stringify(obj))`; **`fs.json`** lê e parseia — use `std.json.stringify`/`std.json.parse` no lugar de `JSON.stringify`/`JSON.parse` nativos.
- **`str.normalize`** remove acentos e espaços; `slug` gera identificadores kebab-case; `format` substitui `{{chave}}`.
- **`list.groupBy` / `list.sortBy`** aceitam uma chave string (propriedade do item) **ou** uma função `(item) => valor`.
- **`encode`** é determinístico (sem rede/SO), útil para checksums, de-dup e anonimização estável. `base64`/`hex` aceitam `modo`: `"encode"` (padrão) ou `"decode"`; o `base64` também `"url"`/`"urlDecode"`.
- **`fetch`** é a única via de **rede** do sandbox: só hosts da allowlist `SANDBOX_FETCH_ALLOW_HOST` (padrão `localhost,127.0.0.1,::1`), sem proxy. `opts` aceita `method`, `headers`, `body`, `timeout` (ms), `noCookies`, `followRedirects`. Cookies `Set-Cookie` são salvos e reenviados automaticamente e persistem em `SANDBOX_FETCH_COOKIE_FILE`.
- **`assert`** lança exceção quando falha (o script termina com erro); `equal` é comparação estrita (`===`).

Padrões JS (`Math`, `JSON`, `Array`, `Date`, ...) ficam **desativados por padrão** — o script só enxerga `std` (e `console`), e a lib nativa some. Para reativá-los (deixar `Math`, `JSON`, `Array`, `Date` disponíveis), defina `SANDBOX_DISABLE_JS_STDLIB=0` (ou use `FullStdlib: true` no script). Sem `require`, `process` nem `Buffer`; o filesystem é só via `std.fs` e a rede via `std.fetch`.

## Limites padrão

| Recurso | Valor |
| --- | --- |
| Tempo de execução | 30 s (fixo) |
| Saída (stdout) | 256 KiB |
| Arquivo lido (fs) | 2 MB (por arquivo) |
| Arquivo gravado (fs) | 1 MiB |

Loop infinito é interrompido pelo timeout; `while(true){}` não trava o servidor.

## Pastas

- **Filesystem (`std.fs`, leitura e escrita)**: `~/.local/share/mcp/sandbox/fs`
  (Windows: `C:\Users\<usuário>\.local\share\mcp\sandbox\fs`). Se vazia, o
  servidor cria wordlists de exemplo (nunca sobrescreve o que existe).
- **Scripts** (do agente): `…/share/mcp/sandbox/scripts`.

Por padrão tudo fica sob `~/.local/share/mcp/sandbox/`. As variáveis
`SANDBOX_FS_DIR`/`SANDBOX_SCRIPTS_DIR` são **opcionais** (precedem o padrão),
mas o uso típico é só registrar o binário:

```json
{
  "context_servers": {
    "sandbox": {
      "command": "~/nautidesk/mcp/dist/sandbox-mcp.exe"
    }
  }
}
```

## Build

> Após mudanças de dependência (goja), rode `go mod tidy` no primeiro build.

```powershell
cd sandbox
go mod tidy
go test ./...
go build -o ../dist/sandbox-mcp.exe ./cmd/sandbox-mcp
```

Ou via Task: `task build:sandbox`.

## Variáveis de ambiente

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `SANDBOX_FS_DIR` | `~/.local/share/mcp/sandbox/fs` | Pasta do filesystem do script (leitura **e** escrita). |
| `SANDBOX_SCRIPTS_DIR` | `~/.local/share/mcp/sandbox/scripts` | Pasta de scripts do agente. |
| `SANDBOX_FETCH_ALLOW_HOST` | `localhost,127.0.0.1,::1` | Hosts permitidos para `std.fetch` (vírgulas; `.domínio` libera subdomínios). |
| `SANDBOX_FETCH_TIMEOUT_SECONDS` | `30` | Timeout padrão de cada requisição do `std.fetch`. |
| `SANDBOX_FETCH_MAX_BODY_KB` | `1024` | Teto do corpo da resposta do `std.fetch`. |
| `SANDBOX_FETCH_COOKIE_FILE` | `~/.local/share/mcp/sandbox/cookies.json` | Arquivo de persistência dos cookies do `std.fetch`. |
| `SANDBOX_DISABLE_JS_STDLIB` | `1` (on) | Por padrão remove os built-ins JS nativos (`Math`, `JSON`, `Array`, `Date`, ...) do sandbox, deixando só `std`/`console`. Defina `0` para reativar a lib nativa. |
