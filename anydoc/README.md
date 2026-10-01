# AnyDoc MCP

Converte documentos para Markdown e para PDF.
Escrito em **Rust**, usando o crate [`anydoc`](https://github.com/firecrawl/anydoc) (nativo, sem WASM).

## Tools

| Tool | O que faz |
| --- | --- |
| `anydoc_import` | Converte um documento para Markdown ao lado do original (mesmo nome, `.md`; `-extraido.md` se já existir) |
| `anydoc_export` | Converte um documento para PDF na mesma pasta (mesmo nome, `.pdf`), passando por Markdown |

Ambos devolvem o caminho absoluto do arquivo gerado.

O único formato de saída do `anydoc_export` é **PDF**.

## Formatos de entrada

A conversão usa o crate `anydoc`, que detecta o formato pelo conteúdo dos bytes
(assinatura do PDF, grupo de abertura do RTF, streams OLE, mimetype do pacote ZIP),
e não pela extensão — arquivos mal nomeados convertem certo.

| Formato | Extensões |
| --- | --- |
| Word | `.doc`, `.docx`, `.docm` |
| PowerPoint | `.ppt`, `.pps`, `.pot`, `.pptx`, `.pptm`, `.ppsx`, `.ppsm` |
| Excel | `.xls`, `.xlsx`, `.xlsm`, `.xlsb` |
| OpenDocument | `.odt`, `.ods`, `.odp` |
| Rich Text | `.rtf` |
| EPUB | `.epub` |
| CSV | `.csv` |
| PDF | `.pdf` |

CSV/TSV/JSON/XML/HTML-de-tabela passam por um conversor próprio (`src/tabular.rs`):
são formatos tabulares simples e não vale o custo de abrir o modelo de documento
completo.

## Redação de PII

Todo Markdown gerado passa por `src/pii.rs` antes de ser usado:

- padrões com validação real (CPF, CNPJ, cartão via Luhn) só mascaram se o dígito
  verificador fecha;
- tabelas mascaram célula a célula pela entidade do cabeçalho;
- campos no formato `chave: valor` são mascarados pelo nome da chave;
- endereços, e-mails, telefones, IPs, MAC, JWT, hashes e URLs viram rótulos;
- blocos de código são preservados intactos.

## Estrutura

```
src/
├── main.rs      # servidor MCP (stdio) e as duas tools
├── pii.rs       # redação de PII
├── tabular.rs   # CSV/TSV/JSON/XML/HTML -> tabela Markdown
├── markdown.rs  # parser de Markdown -> modelo de blocos
├── render.rs    # helpers compartilhados (texto puro, saneamento para PDF)
└── pdf.rs       # Markdown -> .pdf (printpdf)
```

## Build

```bash
cargo build --release        # anydoc/target/release/anydoc-mcp
cargo test
```

Ou pelo Taskfile, junto com os outros módulos:

```bash
task build:anydoc            # gera dist/anydoc-mcp.exe
```

## Limitações

- **Sem OCR.** PDF digitalizado falha com `NeedsOcr`; a versão em Rust do crate
  `anydoc` não manda o arquivo para nenhum serviço externo.
- **Imagens não são embutidas no PDF**: entram como o texto alternativo.
- **Mermaid** é preservado como bloco de código, não desenhado.
- O PDF usa as fontes base Helvetica; texto muito largo pode ser truncado.
- Sem variáveis de ambiente (saída de log vai para stderr).