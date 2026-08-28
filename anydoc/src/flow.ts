import { pt, ResourceStore } from "@reamkit";
import type { FlowDoc } from "@reamkit";
import type { BodyElement } from "@reamkit/document-model";
import { highlight } from "./highlight.ts";
import { parseInline, slug } from "./markdown.ts";
import { colorDiagram, renderMermaidDiagram } from "./mermaid.ts";

type Alignment = NonNullable<
  Extract<
    BodyElement,
    { kind: "paragraph" }
  >["paragraph"]["properties"]["alignment"]
>;

type FlowRun = ReturnType<typeof makeRun>;

interface RunOpts {
  bold?: boolean;
  italic?: boolean;
  size?: number;
  align?: string;
  color?: string;
  shading?: string;
  underline?: boolean;
  strike?: boolean;
  fontFamily?: string;
  letterSpacing?: number;
  spacingBefore?: number;
  spacingAfter?: number;
  // Mantém o parágrafo colado ao próximo (w:keepNext) — evita órfãos.
  keepNext?: boolean;
  // Começa o parágrafo em uma página nova (w:pageBreakBefore).
  pageBreakBefore?: boolean;
  code?: boolean;
  verticalAlign?: "superscript" | "subscript" | "baseline";
  // Internal link target (a bookmark name in this document, §17.16.22
  // w:hyperlink @w:anchor). When set, reamkit wraps the run in
  // <w:hyperlink w:anchor="…"> and the DOCX->PDF converter keeps the jump.
  anchor?: string;
  // External link target (§17.16.22). Only http/https/mailto are ever set;
  // sanitizeHref rejects everything else for safety.
  href?: string;
}

export interface DocumentBlock {
  kind: string;
  language?: string;
  text?: string;
  bold?: boolean;
  italic?: boolean;
  size?: number;
  align?: string;
  columns?: string[];
  rows?: string[][];
  alignCols?: string[];
  items?: string[];
  url?: string;
  width?: number;
  height?: number;
  color?: string;
  background?: string;
  underline?: boolean;
  strike?: boolean;
  font?: string;
  letterSpacing?: number;
  headerBackground?: string;
  headerColor?: string;
  striped?: boolean;
  borders?: boolean;
  // Left indent in levels (used by the auto-generated table of contents to
  // reflect heading depth). Multiplied by 14pt when emitted.
  indent?: number;
  // Heading depth (1 = H1 … 6 = H6). Used to style headings hierarchically.
  level?: number;
  runs?: Array<{
    text?: string;
    bold?: boolean;
    italic?: boolean;
    strike?: boolean;
    underline?: boolean;
    color?: string;
    code?: boolean;
    link?: boolean;
    url?: string;
  }>;
}

export interface RunSpec {
  text?: string;
  bold?: boolean;
  italic?: boolean;
  strike?: boolean;
  underline?: boolean;
  color?: string;
  code?: boolean;
  link?: boolean;
  url?: string;
  verticalAlign?: "superscript" | "subscript" | "baseline";
}

function hex(color?: string): string | undefined {
  if (!color) return undefined;
  const h = color.trim().replace(/^#/, "");
  return /^[0-9a-fA-F]{6}$/.test(h) ? h.toUpperCase() : undefined;
}

// slug() is defined in ./markdown.ts and imported above.

function cleanText(s: string): string {
  // Remove apenas caracteres de controle (preservando tab/nova linha); emojis e
  // símbolos passam intactos para o DOCX/PDF.
  return s.replace(
    /[\p{Cc}]/gu,
    (ch) => (ch === "\t" || ch === "\n" || ch === "\r" ? ch : ""),
  );
}

const ZWSP = "\u200B";
// Characters where it is natural to allow a line break inside a long token
// (URLs, file paths, identifiers). Inserting a break after them lets the
// renderer wrap the line without changing how the text looks.
const BREAK_AFTER = /[\/\-_.:@?&=+#,%~]/;

// Wrapping mode. Soft wrapping inserts zero-width spaces, which Word/LibreOffice
// honor nicely (invisible break). Hard wrapping inserts real line breaks, which
// also work in the DOCX->PDF converter that ignores zero-width spaces.
let HARD_WRAP = false;

// Insert break opportunities into long unbreakable tokens so they wrap instead
// of overflowing the page. Short words and normal sentences (spaces) are
// untouched.
function allowWrapping(text: string, threshold = 48, hardEvery = 20): string {
  if (!text) return text;

  // Soft mode: invisible zero-width spaces at every delimiter. Word/LibreOffice
  // only break where needed, so this stays clean and doesn't change the look.
  if (!HARD_WRAP) {
    let out = "";
    let tokenLen = 0;
    let sinceBreak = 0;
    for (let i = 0; i < text.length; i++) {
      const ch = text[i];
      if (ch === " " || ch === "\t" || ch === "\n" || ch === "\r") {
        out += ch;
        tokenLen = 0;
        sinceBreak = 0;
        continue;
      }
      tokenLen++;
      out += ch;
      if (tokenLen > threshold) {
        sinceBreak++;
        if (BREAK_AFTER.test(ch) || sinceBreak >= hardEvery) {
          out += ZWSP;
          sinceBreak = 0;
        }
      } else {
        sinceBreak = 0;
      }
    }
    return out;
  }

  // Hard mode: real line breaks (for the DOCX->PDF converter). Only break once a
  // segment is long enough (MIN_SEG), at the nearest delimiter, so the line
  // isn't shredded into tiny pieces; force a break past MAX_SEG to fit the page.
  const MIN_SEG = 55;
  const MAX_SEG = 70;
  let out = "";
  let seg = 0;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (ch === " " || ch === "\t" || ch === "\n" || ch === "\r") {
      out += ch;
      seg = 0;
      continue;
    }
    out += ch;
    seg++;
    if ((seg >= MIN_SEG && BREAK_AFTER.test(ch)) || seg >= MAX_SEG) {
      out += "\n";
      seg = 0;
    }
  }
  return out;
}

function baseRunProps(font = "Arial") {
  return {
    bold: false,
    italic: false,
    strike: false,
    underline: "none" as const,
    verticalAlign: "baseline" as const,
    fontSizePt: pt(11),
    colorHex: "000000",
    fontFamily: { ascii: font, hAnsi: font },
    rtl: false,
  };
}

function makeRun(text: string, opts: RunOpts = {}) {
  const font = opts.fontFamily ?? "Arial";
  const color = hex(opts.color);
  const shading = hex(opts.shading);
  const size = Number(opts.size);
  const fontSizePt = Number.isFinite(size) && size > 0 ? pt(size) : pt(11);
  return {
    text: cleanText(allowWrapping(text)),
    properties: {
      ...baseRunProps(opts.code ? "Courier New" : font),
      bold: opts.bold ?? false,
      italic: opts.italic ?? false,
      fontSizePt,
      colorHex: color ?? "000000",
      underline: opts.underline ? ("single" as const) : ("none" as const),
      strike: opts.strike ?? false,
      verticalAlign: opts.verticalAlign ?? "baseline",
      ...(shading ? { shadingColorHex: shading } : {}),
      ...(opts.letterSpacing !== undefined
        ? { letterSpacingPt: pt(opts.letterSpacing) }
        : {}),
    },
    // Internal/external hyperlink. These are first-class Run fields in
    // reamkit's document model (not a separate body element), so a clickable
    // link is just a run that carries anchor/href alongside its text.
    ...(opts.anchor !== undefined ? { anchor: opts.anchor } : {}),
    ...(opts.href !== undefined ? { href: opts.href } : {}),
  };
}

function run(
  text: string,
  opts: RunOpts & { bookmarks?: string[] } = {},
): BodyElement {
  const shading = hex(opts.shading);
  return {
    kind: "paragraph",
    paragraph: {
      ...(opts.bookmarks && opts.bookmarks.length
        ? { bookmarks: opts.bookmarks }
        : {}),
      properties: {
        spacingBefore: pt(opts.spacingBefore ?? (opts.bold ? 10 : 4)),
        spacingAfter: pt(opts.spacingAfter ?? 6),
        alignment: (opts.align ?? "left") as unknown as Alignment,
        ...(shading ? { shading: { colorHex: shading } } : {}),
        ...(opts.pageBreakBefore ? { pageBreakBefore: true } : {}),
        ...(opts.keepNext ? { keepNext: true } : {}),
      },
      runs: [makeRun(text, opts)],
    },
  };
}

// Only these URL schemes become clickable external links. Anything else is
// rendered as plain text (mirrors reamkit's own sanitizeHref allowlist, so an
// untrusted `javascript:`/`data:` target can never become a clickable link).
const ALLOWED_LINK_SCHEMES = new Set(["http", "https", "mailto"]);
function sanitizeHref(href: string): string | undefined {
  const url = href.trim();
  const m = /^([a-zA-Z][a-zA-Z0-9+.-]*):/.exec(url);
  if (!m) return undefined;
  return ALLOWED_LINK_SCHEMES.has(m[1]!.toLowerCase()) ? url : undefined;
}

function paragraphWithRuns(
  runs: RunSpec[],
  opts: {
    align?: string;
    color?: string;
    shading?: string;
    size?: number;
    fontFamily?: string;
    spacingBefore?: number;
    spacingAfter?: number;
    italic?: boolean;
    bold?: boolean;
    indent?: number;
    keepNext?: boolean;
    pageBreakBefore?: boolean;
  } = {},
): BodyElement[] {
  const shading = hex(opts.shading);
  const out: FlowRun[] = [];
  for (const r of runs) {
    if (r.link && r.url) {
      const url = r.url;
      if (url.startsWith("#")) {
        // Internal link: the run's `anchor` points at a heading's bookmark
        // (§17.16.22 w:hyperlink @w:anchor). reamkit groups runs that share
        // the same anchor into <w:hyperlink w:anchor="…">, so this is exactly
        // the structure a clickable in-document link needs — no separate body
        // element (which reamkit does not model and would drop silently).
        out.push(
          makeRun(r.text ?? "", {
            underline: true,
            color: "1A0DAB",
            anchor: slug(url.slice(1)),
          }),
        );
      } else {
        const safe = sanitizeHref(url);
        if (safe !== undefined) {
          out.push(
            makeRun(r.text ?? "", {
              underline: true,
              color: "1A0DAB",
              href: safe,
            }),
          );
        } else {
          // Disallowed scheme: keep the words, surface the raw target, no jump.
          out.push(makeRun(r.text ?? "", { color: opts.color }));
          out.push(makeRun(` (${url})`, { color: "6B7280", size: 9 }));
        }
      }
      continue;
    }
    out.push(
      makeRun(r.text ?? "", {
        bold: r.bold ?? opts.bold,
        italic: r.italic ?? opts.italic,
        strike: r.strike,
        code: r.code,
        fontFamily: r.code ? "Courier New" : opts.fontFamily,
        color: r.code ? "D85131" : (r.color ?? opts.color),
        size: opts.size,
        shading: opts.shading,
        verticalAlign: r.verticalAlign,
      }),
    );
  }
  return [
    {
      kind: "paragraph",
      paragraph: {
        properties: {
          spacingBefore: pt(opts.spacingBefore ?? 4),
          spacingAfter: pt(opts.spacingAfter ?? 6),
          alignment: (opts.align ?? "left") as unknown as Alignment,
          ...(shading ? { shading: { colorHex: shading } } : {}),
          ...(opts.indent ? { indentLeft: pt(opts.indent * 14) } : {}),
          ...(opts.pageBreakBefore ? { pageBreakBefore: true } : {}),
          ...(opts.keepNext ? { keepNext: true } : {}),
        },
        runs: out,
      },
    } as BodyElement,
  ];
}

interface TableOpts {
  headerBackground?: string;
  headerColor?: string;
  striped?: boolean;
  borders?: boolean;
  alignCols?: string[];
}

function tableBlock(
  columns: string[],
  rows: string[][],
  opts: TableOpts = {},
): BodyElement {
  const headerBackground = hex(opts.headerBackground);
  const headerColor = hex(opts.headerColor);
  const border = opts.borders
    ? { style: "single" as const, width: pt(0.5), colorHex: "c9c9cf" }
    : undefined;
  const cellProps = (background?: string) => ({
    ...(background ? { shading: { colorHex: background } } : {}),
    ...(border
      ? {
        borders: { top: border, right: border, bottom: border, left: border },
      }
      : {}),
  });
  const zebraBackground = "f3f3f5";
  const all = [columns, ...rows];

  const PAGE_CONTENT_WIDTH = 468;
  // Measure the *rendered* width of each cell by simulating font metrics.
  // Courier New (code) is ~1.6x wider per character than Arial.
  function cellRenderWidth(cell: string): number {
    let w = 0;
    // Strip inline markdown to get plain text, but account for code spans
    const codeSegments: string[] = [];
    const plain = cell.replace(/`([^`]+)`/g, (_, m) => {
      codeSegments.push(m);
      return m;
    });
    for (const ch of plain) w += /\P{ASCII}/u.test(ch) ? 1.2 : 1; // accented chars slightly wider
    for (const seg of codeSegments) w += seg.length * 0.55; // extra cost for code runs already counted in plain
    return Math.max(w, 1);
  }
  const maxLens = columns.map((_, ci) => {
    let maxLen = cellRenderWidth(columns[ci] ?? "") * 1.15; // header em negrito é ~15% mais largo
    for (const row of rows) {
      const w = cellRenderWidth(row[ci] ?? "");
      if (w > maxLen) maxLen = w;
    }
    return Math.max(maxLen, 1);
  });
  const totalLen = maxLens.reduce((a, b) => a + b, 0);
  // Larguras proporcionais ao conteúdo, com piso de 36pt. Se a soma passar da
  // largura útil da página (ex.: tabela com muitas colunas), abandona o piso e
  // reescala — a grade nunca estoura no PDF nem no Word (a última coluna
  // absorve o arredondamento).
  let widths = maxLens.map((len) =>
    Math.max(Math.floor((len / totalLen) * PAGE_CONTENT_WIDTH), 36)
  );
  let widthSum = widths.reduce((a, b) => a + b, 0);
  if (widthSum > PAGE_CONTENT_WIDTH) {
    widths = maxLens.map((len) =>
      Math.floor((len / totalLen) * PAGE_CONTENT_WIDTH)
    );
    widthSum = widths.reduce((a, b) => a + b, 0);
  }
  if (widthSum < PAGE_CONTENT_WIDTH && widths.length > 0) {
    widths[widths.length - 1] += PAGE_CONTENT_WIDTH - widthSum;
  }

  return {
    kind: "table",
    table: {
      properties: {
        layout: "auto",
        ...(opts.borders
          ? {
            borders: {
              top: border,
              right: border,
              bottom: border,
              left: border,
            },
          }
          : {}),
      },
      grid: widths.map(pt),
      rows: all.map((cells, ri) => {
        const isHeader = ri === 0;
        const isLast = ri === all.length - 1;
        const rowBackground = isHeader
          ? headerBackground
          : opts.striped && ri % 2 === 1
          ? zebraBackground
          : undefined;
        return {
          // Mantém as linhas juntas (keepNext) e inteiras (cantSplit): tabela
          // pequena passa inteira para a próxima página em vez de quebrar no
          // meio — sem header “órfão/duplicado” na quebra.
          properties: {
            ...(isHeader ? { isHeader: true } : {}),
            ...(isLast ? {} : { keepNext: true }),
            cantSplit: true,
          },
          cells: cells.map((c, ci) => ({
            properties: cellProps(rowBackground),
            content: paragraphWithRuns(parseInline(String(c ?? "")), {
              bold: isHeader,
              color: isHeader ? headerColor : undefined,
              align: opts.alignCols?.[ci] ?? "left",
            }),
          })),
        };
      }),
    },
  };
}

// Split a long code line into segments that each fit within `maxCols` columns.
// Prefers a break at a space, then at a natural delimiter (BREAK_AFTER), then
// hard-cuts, so long unbreakable tokens (URLs, file paths, package names) still
// wrap into real lines instead of being shredded or clipped by the PDF layout
// engine. Each segment becomes its own paragraph, which the renderer always
// starts on a fresh line — unlike a bare `\n` inside a run, whose break depends
// on the output format.
// Shell/common operators that make natural break points: when a long command
// line must wrap, prefer a break just before one of these (e.g. `docker … &&`
// then `docker compose …`) so each wrapped fragment stays a coherent command
// rather than splitting mid-word. Longest forms first so `>>` wins over `>`,
// `2>>` over `2>`, `&&` over `&`, etc.
const SHELL_OPS =
  /<<<|2>>|2>|>>|&>|>&|\|&|\[\[|\]\]|<<&&\|\||<|>|\||;|&|==|!=|=|\$\(|\$\{|\$@|\$\*|\$\#|\$\?|\$\$|\$!|\[|\]|`/g;

function lastOperatorIndex(s: string, max: number): number {
  let best = -1;
  for (const m of s.matchAll(SHELL_OPS)) {
    const idx = m.index ?? 0;
    if (idx > max) break;
    if (idx > best) best = idx;
  }
  return best;
}

function splitLongLine(line: string, maxCols: number): string[] {
  if (line.length <= maxCols) return [line];
  const out: string[] = [];
  let rest = line;
  while (rest.length > maxCols) {
    // Prefer a break right before a shell operator, then a space, then any
    // delimiter, then a hard cut — so the rendered line never overflows.
    let cut = lastOperatorIndex(rest, maxCols);
    if (cut <= 0) cut = rest.lastIndexOf(" ", maxCols);
    if (cut <= 0) cut = lastDelimiterIndex(rest, maxCols);
    if (cut <= 0) cut = maxCols;
    out.push(rest.slice(0, cut).replace(/\s+$/, ""));
    rest = rest.slice(cut).replace(/^\s+/, "");
  }
  if (rest.length) out.push(rest);
  return out;
}

function lastDelimiterIndex(s: string, max: number): number {
  for (let i = max; i >= 1; i--) {
    if (BREAK_AFTER.test(s[i - 1])) return i;
  }
  return -1;
}

// Per-language identity: each code block gets its own accent color, used in
// the language badge, the header tint and the left edge. This is what keeps
// JSON / YAML / TOML / XML (and the common languages) visually distinct while
// still feeling like one family.
const LANG_META: Record<string, { label: string; accent: string }> = {
  json: { label: "JSON", accent: "E5C07B" },
  yaml: { label: "YAML", accent: "E06C75" },
  yml: { label: "YAML", accent: "E06C75" },
  toml: { label: "TOML", accent: "D19A66" },
  xml: { label: "XML", accent: "56B6C2" },
  html: { label: "HTML", accent: "E06C75" },
  css: { label: "CSS", accent: "61AFEF" },
  js: { label: "JS", accent: "61AFEF" },
  javascript: { label: "JS", accent: "61AFEF" },
  jsx: { label: "JSX", accent: "61AFEF" },
  ts: { label: "TS", accent: "61AFEF" },
  typescript: { label: "TS", accent: "61AFEF" },
  tsx: { label: "TSX", accent: "61AFEF" },
  python: { label: "PY", accent: "98C379" },
  py: { label: "PY", accent: "98C379" },
  bash: { label: "BASH", accent: "E5C07B" },
  sh: { label: "SH", accent: "E5C07B" },
  shell: { label: "SHELL", accent: "E5C07B" },
  zsh: { label: "ZSH", accent: "E5C07B" },
  sql: { label: "SQL", accent: "C678DD" },
  postgres: { label: "SQL", accent: "C678DD" },
  postgresql: { label: "SQL", accent: "C678DD" },
  mysql: { label: "SQL", accent: "C678DD" },
  sqlite: { label: "SQL", accent: "C678DD" },
  powershell: { label: "PS", accent: "C678DD" },
  ps1: { label: "PS1", accent: "C678DD" },
  ruby: { label: "RB", accent: "E06C75" },
  rb: { label: "RB", accent: "E06C75" },
  go: { label: "GO", accent: "56B6C2" },
  rust: { label: "RUST", accent: "D19A66" },
  php: { label: "PHP", accent: "C678DD" },
  java: { label: "JAVA", accent: "D19A66" },
  c: { label: "C", accent: "61AFEF" },
  cpp: { label: "C++", accent: "61AFEF" },
  csharp: { label: "C#", accent: "61AFEF" },
  cs: { label: "C#", accent: "61AFEF" },
  markdown: { label: "MD", accent: "56B6C2" },
  md: { label: "MD", accent: "56B6C2" },
  dockerfile: { label: "DOCKER", accent: "61AFEF" },
  make: { label: "MAKE", accent: "E06C75" },
  ini: { label: "INI", accent: "8B949E" },
  env: { label: "ENV", accent: "8B949E" },
  nginx: { label: "NGINX", accent: "98C379" },
  cron: { label: "CRON", accent: "E5C07B" },
  mermaid: { label: "DIAGRAMA", accent: "56B6C2" },
};

function langMeta(lang?: string): { label: string; accent: string } {
  const l = (lang ?? "").toLowerCase();
  return LANG_META[l] ?? LANG_META[l.replace(/[^a-z]/g, "")] ??
    { label: (lang ?? "code").toUpperCase(), accent: "8B949E" };
}

// Blend a hex color toward black; factor 1 keeps the color, 0 makes it black.
function shadeHex(color: string, factor: number): string {
  const h = color.replace(/^#/, "");
  if (!/^[0-9a-fA-F]{6}$/.test(h)) return color;
  const n = parseInt(h, 16);
  const mix = (shift: number) => Math.round(((n >> shift) & 255) * factor);
  return (((mix(16) << 16) | (mix(8) << 8) | mix(0)).toString(16).padStart(
    6,
    "0",
  ));
}

// Casca visual de um bloco de código/diagrama: badge do idioma em cima (com a
// cor de identidade), fundo escuro e a borda esquerda grossa na mesma cor.
function codeBlockShell(
  meta: { label: string; accent: string },
  content: BodyElement[],
): BodyElement[] {
  const bg = "1A1D23";
  const borderColor = "2B303B";
  const PAGE_CONTENT_WIDTH = 468;
  const border = {
    style: "single" as const,
    width: pt(0.5),
    colorHex: borderColor,
  };
  const accentBorder = {
    style: "single" as const,
    width: pt(2),
    colorHex: meta.accent,
  };
  const headerBorder = {
    style: "single" as const,
    width: pt(0.5),
    colorHex: meta.accent,
  };
  const header = paragraphWithRuns(
    [{ text: " " }, { text: meta.label, color: meta.accent, bold: true }],
    {
      fontFamily: "Courier New",
      size: 9,
      color: "8B949E",
      spacingBefore: 2,
      spacingAfter: 2,
    },
  );
  return [
    {
      kind: "table",
      table: {
        properties: {
          borders: {
            top: border,
            right: border,
            bottom: border,
            left: accentBorder,
          },
        },
        grid: [pt(PAGE_CONTENT_WIDTH)],
        rows: [
          {
            // isHeader: true repete o badge do idioma quando um bloco longo
            // quebra entre páginas (o conversor respeita w:tblHeader).
            properties: { isHeader: true },
            cells: [{
              properties: {
                shading: { colorHex: shadeHex(meta.accent, 0.14) },
                borders: { bottom: headerBorder },
              },
              content: header,
            }],
          },
          {
            properties: { isHeader: false },
            cells: [{
              properties: { shading: { colorHex: bg } },
              content,
            }],
          },
        ],
      },
    },
    run("", { spacingBefore: 2, spacingAfter: 10 }),
  ];
}

// Diagrama mermaid embutido como imagem centralizada, limitada à largura da
// página (dimensões de design → pt; o PNG rasterizado em 2x fica nítido).
function mermaidImageBlock(
  png: MermaidImageData,
  resources: ResourceStore,
): BodyElement {
  const MAX_W = 430;
  const MAX_H = 440;
  const MIN_W = 150;
  let wPt = Math.max(MIN_W, Math.min(MAX_W, png.width));
  let hPt = (wPt * png.height) / Math.max(png.width, 1);
  if (hPt > MAX_H) {
    // Diagrama alto: reduz a largura para caber na página sem cortar.
    const k = MAX_H / hPt;
    wPt *= k;
    hPt = MAX_H;
  }
  return {
    kind: "image",
    image: {
      resource: resources.put(png.bytes),
      width: pt(wPt),
      height: pt(hPt),
      paragraphProperties: {
        alignment: "center" as unknown as Alignment,
        spacingBefore: pt(8),
        spacingAfter: pt(8),
      },
      altText: "Diagrama",
    },
  };
}

function renderCodeBlock(b: DocumentBlock): BodyElement[] {
  const meta = langMeta(b.language);
  const fg = "ABB2BF";
  const MAX_CODE_COLS = 88;
  const content: BodyElement[] = [];

  // Mermaid: converte o diagrama em arte de caixas/setas (simples e robusta);
  // se a sintaxe for complexa demais, cai no rendering comum de código.
  const lang = (b.language ?? "").toLowerCase();
  if (lang === "mermaid") {
    const diagram = renderMermaidDiagram(String(b.text ?? ""));
    if (diagram) {
      const n = diagram.lines.length;
      for (let k = 0; k < n; k++) {
        // O layout enfileirado já entrega os runs por linha (caixas ciano,
        // fios âmbar, rótulos em neve); a árvore textual cai no colorDiagram.
        const runs = diagram.runs?.[k] ?? colorDiagram(diagram.lines[k]);
        content.push(...paragraphWithRuns(
          [{ text: " " }, ...runs],
          // Rótulos em neve para ler bem sobre o fundo escuro.
          {
            fontFamily: "Courier New",
            size: 8.5,
            color: "E6EDF3",
            spacingBefore: 0,
            spacingAfter: k === n - 1 ? 2 : 0,
            keepNext: true,
          },
        ));
      }
      return codeBlockShell(meta, content);
    }
  }

  const code = String(b.text ?? "");
  const lines = code.length === 0 ? [""] : code.split("\n");
  // One solid paragraph per source line: the inline comment stays glued to its
  // code (the highlighter dims it), empty lines are preserved, and wrapped
  // continuations are indented so the block reads as a single unit.
  for (const line of lines) {
    const subs = splitLongLine(line, MAX_CODE_COLS);
    for (let k = 0; k < subs.length; k++) {
      const isLast = k === subs.length - 1;
      content.push(...paragraphWithRuns(
        [{ text: isLast ? " " : "  " }, ...highlight(subs[k], b.language)],
        // keepNext: as linhas do bloco viajam juntas entre páginas.
        {
          fontFamily: "Courier New",
          size: 8.5,
          color: fg,
          spacingBefore: 0,
          spacingAfter: isLast ? 2 : 1,
          keepNext: true,
        },
      ));
    }
  }
  return codeBlockShell(meta, content);
}

export interface FlowDocOptions {
  /** Use real line breaks instead of zero-width spaces (needed for PDF). */
  hardWrap?: boolean;
}

// Escala tipográfica dos títulos. H1 carrega a cor de marca (a mesma da barra
// do blockquote) e abre capítulo em página nova; os demais níveis escurecem e
// encolhem até se aproximarem do texto corrido.
function headingProps(
  level: number,
): {
  size: number;
  color: string;
  spacingBefore: number;
  spacingAfter: number;
} {
  switch (level) {
    case 1:
      return { size: 18, color: "D85131", spacingBefore: 6, spacingAfter: 8 };
    case 2:
      return { size: 14, color: "1F2937", spacingBefore: 18, spacingAfter: 6 };
    case 3:
      return { size: 12, color: "374151", spacingBefore: 14, spacingAfter: 4 };
    default:
      return { size: 11, color: "4B5563", spacingBefore: 12, spacingAfter: 4 };
  }
}

/** Imagem PNG pronta de um diagrama (dimensões de design em px). */
export interface MermaidImageData {
  bytes: Uint8Array;
  width: number;
  height: number;
}

export function jsonToFlowDoc(
  content: unknown,
  images?: Map<string, Uint8Array>,
  options: FlowDocOptions = {},
  mermaidImages?: Map<number, MermaidImageData>,
): FlowDoc {
  HARD_WRAP = !!options.hardWrap;
  const c = (content ?? {}) as Record<string, unknown>;
  const body: BodyElement[] = [];
  const resources = new ResourceStore();

  if (typeof c.title === "string" && c.title) {
    body.push(
      run(c.title, {
        bold: true,
        size: 20,
        align: "center",
        bookmarks: [slug(c.title)],
      }),
    );
  }
  if (typeof c.subtitle === "string" && c.subtitle) {
    body.push(run(c.subtitle, { size: 13, align: "center" }));
  }

  const blocks = Array.isArray(c.blocks) ? c.blocks as DocumentBlock[] : [];
  let blockIndex = 0;
  for (const b of blocks) {
    const kind = String(b.kind ?? "paragraph");
    const bi = blockIndex++;
    const style = {
      color: b.color,
      shading: b.background,
      underline: b.underline,
      strike: b.strike,
      fontFamily: b.font,
      letterSpacing: b.letterSpacing,
    };
    if (kind === "heading") {
      const level = Math.min(6, Math.max(1, Number(b.level ?? 2)));
      const hp = headingProps(level);
      body.push(run(String(b.text ?? ""), {
        ...style,
        ...hp,
        bold: level <= 3,
        align: b.align,
        bookmarks: [slug(String(b.text ?? ""))],
        // Só um H1 de verdade (e que não seja a primeira coisa do documento)
        // abre capítulo em página nova.
        pageBreakBefore: level === 1 && body.length > 0,
      }));
    } else if (kind === "paragraph") {
      if (Array.isArray(b.runs) && b.runs.length > 0) {
        body.push(...paragraphWithRuns(b.runs as RunSpec[], {
          align: b.align,
          color: b.color,
          shading: b.background,
          size: b.size,
          fontFamily: b.font,
          indent: b.indent,
        }));
      } else {
        body.push(run(String(b.text ?? ""), {
          ...style,
          bold: b.bold,
          italic: b.italic,
          size: b.size,
          align: b.align,
        }));
      }
    } else if (kind === "list") {
      for (const item of b.items ?? []) {
        body.push(
          ...paragraphWithRuns([{ text: "•  " }, ...parseInline(item)], {
            ...style,
          }),
        );
      }
    } else if (kind === "table") {
      body.push({
        kind: "paragraph",
        paragraph: {
          properties: { spacingBefore: pt(8), spacingAfter: pt(2) },
          runs: [{ text: "", properties: baseRunProps() }],
        },
      });
      body.push(tableBlock(b.columns ?? [], b.rows ?? [], {
        headerBackground: b.headerBackground,
        headerColor: b.headerColor,
        striped: b.striped,
        borders: b.borders,
        alignCols: b.alignCols,
      }));
    } else if (kind === "break") {
      body.push({
        kind: "paragraph",
        paragraph: {
          properties: { pageBreakBefore: true },
          runs: [{ text: "", properties: baseRunProps() }],
        },
      });
    } else if (kind === "image") {
      const bytes = images?.get(String(b.url ?? ""));
      if (bytes) {
        body.push({
          kind: "image",
          image: {
            resource: resources.put(bytes),
            width: pt(Number(b.width) || 300),
            height: pt(Number(b.height) || 200),
            paragraphProperties: {
              alignment: (b.align ?? "center") as unknown as Alignment,
              spacingBefore: pt(6),
              spacingAfter: pt(6),
            },
            altText: String(b.text ?? "image"),
          },
        });
      } else {
        body.push(
          run(`[image not loaded: ${b.url}]`, { italic: true, size: 9 }),
        );
      }
    } else if (kind === "codeblock") {
      const lang = String(b.language ?? "").toLowerCase();
      // Mermaid renderizado como imagem PNG (resvg) quando disponível — visual
      // de diagrama de verdade; sem PNG, cai no ASCII dentro da casca de código.
      if (lang === "mermaid") {
        const png = mermaidImages?.get(bi);
        if (png) {
          body.push(mermaidImageBlock(png, resources));
          continue;
        }
      }
      body.push(...renderCodeBlock(b));
    } else if (kind === "blockquote") {
      const bg = "F5F5F7";
      const bar = "D85131";
      const textColor = "374151";
      const PAGE_CONTENT_WIDTH = 468;
      const content: BodyElement[] = [];
      const lines = (b.items ?? []).length ? (b.items ?? []) : [""];
      for (const line of lines) {
        content.push(...paragraphWithRuns(
          parseInline(line || " "),
          { size: 10.5, color: textColor, spacingBefore: 0, spacingAfter: 3 },
        ));
      }
      const border = { style: "single" as const, width: pt(2), colorHex: bar };
      body.push(run("", { spacingBefore: 10, spacingAfter: 0 }));
      body.push({
        kind: "table",
        table: {
          properties: {},
          grid: [pt(PAGE_CONTENT_WIDTH)],
          rows: [{
            properties: { isHeader: false },
            cells: [{
              properties: {
                shading: { colorHex: bg },
                borders: { left: border },
              },
              content,
            }],
          }],
        },
      });
      body.push(run("", { spacingBefore: 0, spacingAfter: 10 }));
    }
  }

  if (Array.isArray(c.paragraphs)) {
    for (const p of c.paragraphs as unknown[]) {
      body.push(run(String(p)));
    }
  }
  if (c.table && typeof c.table === "object") {
    const t = c.table as { columns?: string[]; rows?: string[][] };
    body.push(tableBlock(t.columns ?? [], t.rows ?? []));
  }

  if (body.length === 0) body.push(run(""));

  return {
    kind: "flow",
    body,
    styles: {
      defaultParagraphProperties: {},
      defaultRunProperties: {},
      styles: new Map(),
    },
    sections: [{
      endIndex: body.length,
      properties: { headers: [], footers: [] },
    }],
    resources,
  };
}

interface DateProto {
  getFullYear(): number;
  getMonth(): number;
  getDate(): number;
  getHours(): number;
  getMinutes(): number;
  getSeconds(): number;
  getUTCFullYear(): number;
  getUTCMonth(): number;
  getUTCDate(): number;
  getUTCHours(): number;
  getUTCMinutes(): number;
  getUTCSeconds(): number;
}

export function withUtcDates<T>(fn: () => T): T {
  const proto = Date.prototype as unknown as DateProto;
  const originals = {
    getFullYear: proto.getFullYear,
    getMonth: proto.getMonth,
    getDate: proto.getDate,
    getHours: proto.getHours,
    getMinutes: proto.getMinutes,
    getSeconds: proto.getSeconds,
  };
  proto.getFullYear = function (this: DateProto) {
    return this.getUTCFullYear();
  };
  proto.getMonth = function (this: DateProto) {
    return this.getUTCMonth();
  };
  proto.getDate = function (this: DateProto) {
    return this.getUTCDate();
  };
  proto.getHours = function (this: DateProto) {
    return this.getUTCHours();
  };
  proto.getMinutes = function (this: DateProto) {
    return this.getUTCMinutes();
  };
  proto.getSeconds = function (this: DateProto) {
    return this.getUTCSeconds();
  };
  try {
    return fn();
  } finally {
    Object.assign(proto, originals);
  }
}
