# Fetch MCP

Servidor MCP em Go para o agente **testar endpoints HTTP** com segurança: só
hosts da allowlist `FETCH_ALLOW_HOST` podem ser chamados (proteção anti-SSRF no
nível do servidor). Envia JSON/XML e devolve **timing, status, headers e corpo**
(JSON e XML "pretty-printed") formatados em Markdown, além de um comando `curl`
para reproduzir a chamada fora do MCP. Cookies de `Set-Cookie` são salvos e
reenviados automaticamente. Módulo independente (`module ntdsk.com/mcp/fetch`).

## Tools

| Tool | Descrição |
| --- | --- |
| `fetch_request` | Envia um HTTP request (GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS) para um host permitido: método, URL, headers opcionais, corpo (JSON/XML/texto), timeout, `noCookies`, `followRedirects` e `noBody`. Retorna timing (DNS/conexão/TLS/primeiro byte), status, lista leve de headers (data em hora local; `Set-Cookie` resumido ao **nome** dos cookies), corpo pretty-printed e o `curl` equivalente. Respostas binárias são resumidas (não despejam bytes). |
| `cookie_list` | Lista os cookies salvos (domínio, nome, valor, path, expiração, flags). |
| `cookie_clear` | Limpa cookies: tudo, um domínio, ou um cookie específico (`domain` + `name`). |
| `fetch_allowlist` | Lista os hosts liberados por `FETCH_ALLOW_HOST`. |
| `fetch_history` | Devolve as últimas N requisições (padrão 20, máx 200) gravadas no log da sessão — reaproveitáveis para reproduzir chamadas sem redigitar. |

## Recursos

- **Timing**: DNS, conexão TCP, handshake TLS e tempo até o primeiro byte —
  medidos no 1º hop (mostrados quando > 0). O total aparece na linha de status.
- **`noBody`**: pula a leitura/exibição do corpo (só status + headers + timing);
  o tamanho é obtido do header `Content-Length` quando presente.
- **Binário**: respostas não-texto (image/*, octet-stream, pdf, zip, NUL no
  body...) mostram `_(resposta binária — N; corpo não exibido)_` em vez de
  despejar bytes.
- **Curl**: cada resposta traz o comando `curl` equivalente (com os headers e
  o corpo enviados) no bloco "Reproduzir".
- **HTML**: respostas `text/html` são **extraídas para texto** (título, texto
  visível colapsado e até 20 links resolvidos contra a URL) — nada de markup
  pesado no contexto. O texto vem limitado a **1.200 chars** por padrão
  (ajustável por chamada com `htmlMaxChars`, ex.: um resumo de 400 ou 3.000);
  o HTML cru fica disponível com `htmlRaw: true` (e o corpo ainda obedece o
  `FETCH_MAX_BODY_KB`).

## Allowlist de hosts

```
FETCH_ALLOW_HOST=localhost,golang.org,.example.com
```

- Separados por vírgula, sem esquema. `localhost:8080` pode incluir porta.
- Entrada começando com `.` libera o domínio e **subdomínios**
  (`.example.com` → `api.example.com`).
- Sem a env, o padrão é `localhost,127.0.0.1,::1`.
- URLs são validadas: apenas `http`/`https` e host deve bater na allowlist
  (o servidor recusa com erro claro caso contrário).

## Cookies

- `Set-Cookie` das respostas são salvos (domínio = atributo `Domain` ou o host
  da requisição) e reenviados automaticamente nas próximas chamadas para o
  mesmo domínio (respeitando `Path`, `Secure` e expiração).
- Persistidos em `~/.local/share/mcp/fetch/cookies.json` (sobrevivem ao
  restart; defina `FETCH_COOKIE_FILE` para outro local).
- Gerencie com `cookie_list` e `cookie_clear`.

## Variáveis de ambiente

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `FETCH_ALLOW_HOST` | `localhost,127.0.0.1,::1` | Hosts permitidos (vírgulas; `.domínio` libera subdomínios) |
| `FETCH_TIMEOUT_SECONDS` | `30` | Timeout padrão de cada requisição |
| `FETCH_MAX_BODY_KB` | `1024` | Teto do corpo da resposta (truncado acima disso) |
| `FETCH_COOKIE_FILE` | `~/.local/share/mcp/fetch/cookies.json` | Arquivo de persistência dos cookies |

## Build

Via Taskfile (na raiz deste repositório de MCPs):

```powershell
task build:fetch
```

Direto:

```powershell
cd fetch
go mod tidy
go build -o ../dist/fetch-mcp.exe ./cmd/fetch-mcp
```

## Exemplo

```powershell
$env:FETCH_ALLOW_HOST = "localhost,example.com"
.\dist\fetch-mcp.exe
```

Uso pelo agente: `fetch_request(method: POST, url: http://localhost:8080/api/login, body: {"user":"a","pass":"b"})` → resposta com headers + JSON formatado; se vier `Set-Cookie`, o próximo `fetch_request` já manda o cookie de volta.

Cada `fetch_request` bem-sucedido também é gravado no log da sessão (arquivo em
`~/.local/share/mcp/fetch/logs/`), e o `fetch_history` devolve essas chamadas a
partir do arquivo mais recente (as entradas incluem o `curl` para reproduzir).

## Logs

```text
~/.local/share/mcp/fetch/logs/fetch-<AAAA-MM-DD_HH-MM-SS>.log
```
