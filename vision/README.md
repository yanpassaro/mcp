# Vision MCP

Servidor MCP em Go que usa um modelo de visão **OpenAI-compatible** para ler
imagens e vídeos. Expõe três ferramentas:

- `vision_image_analysis` — análise geral de imagem quando nenhuma outra
  ferramenta específica se aplica.
- `vision_diff` — compara duas capturas de UI e aponta divergência visual ou
  de implementação.
- `vision_video_analysis` — inspeciona um vídeo curto (≤ 8 MB; MP4/MOV/M4V) e
  descreve cenas, momentos e entidades.

O modelo recebe um **prompt (em inglês)** e o **caminho/URL** da imagem ou do
vídeo. Imagens e vídeos podem ser locais ou remotos (URL `http/https`).

## Requisitos

- Go 1.26 ou mais recente;
- um endpoint OpenAI-compatible (`/chat/completions`) e uma chave de API
  (opcional para servidores locais como LM Studio/Ollama).

## Configuração

Variáveis de ambiente:

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `VISION_BASE_URL` | `https://api.openai.com/v1` | Base da API OpenAI-compatible |
| `VISION_API_KEY` | vazio | Token Bearer (omitível em servidores locais) |
| `VISION_MODEL` | `gpt-4o` | Nome do modelo de visão |
| `VISION_TIMEOUT_SECONDS` | `120` | Timeout de cada requisição HTTP |
| `VISION_MAX_TOKENS` | `1500` | `max_tokens` da geração |
| `VISION_MAX_IMAGE_MB` | `25` | Limite de tamanho de imagem (leitura) |

## Build e execução

Via Taskfile (na raiz deste repositório de MCPs):

```powershell
task build:vision
```

Direto:

```powershell
go mod tidy
go build -o ../dist/vision-mcp.exe ./cmd/vision-mcp
$env:VISION_API_KEY = "sua-chave"
$env:VISION_MODEL   = "gpt-4o"
..\dist\vision-mcp.exe
```

O processo usa **stdio** para o transporte MCP. Integre-o em um cliente MCP
(Zed, Claude Desktop etc.) apontando para o binário compilado com as variáveis
de ambiente acima.

Estrutura: `cmd/vision-mcp/main.go` (entrypoint), `internal/vision/` (cliente
OpenAI-compatible) e `internal/mcpserver/` (registro das tools + formatação).

## Logs

```text
~/.local/share/mcp/vision/logs/vision-<AAAA-MM-DD_HH-MM-SS>.log
```

## Tools

- `vision_image_analysis(image_path, prompt)` — lê a imagem (local/URL) e
  retorna a análise do modelo para o `prompt` informado.
- `vision_diff(image_a_path, image_b_path, prompt?)` — envia as duas imagens e
  solicita a detecção de divergência visual/implementação. Se `prompt` for
  omitido, usa um prompt padrão de comparação de UI.
- `vision_video_analysis(video_path, prompt?)` — baixa/valida o vídeo
  (≤ 8 MB; MP4/MOV/M4V) e o envia diretamente ao modelo de visão,
  retornando a descrição de cenas, momentos e entidades. Se `prompt` for
  omitido, usa um prompt padrão de inspeção de vídeo.
