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
  keepNext?: boolean;
  pageBreakBefore?: boolean;
  code?: boolean;
  verticalAlign?: "superscript" | "subscript" | "baseline";
  anchor?: string;
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
  indent?: number;
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

function cleanText(s: string): string {
  return s.replace(
    /[\p{Cc}]/gu,
    (ch) => (ch === "\t" || ch === "\n" || ch === "\r" ? ch : ""),
  );
}

const ZWSP = "\u200B";

const BREAK_AFTER = /[\/\-_.:@?&=+#,%~]/;

let HARD_WRAP = false;

function allowWrapping(text: string, threshold = 48, hardEvery = 20): string {
  if (!text) return text;

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
    codeColor?: string;
  } = {},
): BodyElement[] {
  const shading = hex(opts.shading);
  const out: FlowRun[] = [];
  for (const r of runs) {
    if (r.link && r.url) {
      const url = r.url;
      if (url.startsWith("#")) {
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
        color: r.code ? (opts.codeColor ?? "D85131") : (r.color ?? opts.color),
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
  function cellRenderWidth(cell: string): number {
    let w = 0;
    const codeSegments: string[] = [];
    const plain = cell.replace(/`([^`]+)`/g, (_, m) => {
      codeSegments.push(m);
      return m;
    });
    for (const ch of plain) w += /\P{ASCII}/u.test(ch) ? 1.2 : 1;
    for (const seg of codeSegments) w += seg.length * 0.55;
    return Math.max(w, 1);
  }
  const charToPt = 5.6;

  const CELL_PADDING = 12;
  function longestWordWidth(cell: string): number {
    let w = 0;
    for (const word of String(cell).split(/\s+/)) {
      const ww = cellRenderWidth(word);
      if (ww > w) w = ww;
    }
    return w;
  }
  const desired = columns.map((_, ci) => {
    let maxLen = cellRenderWidth(columns[ci] ?? "");
    for (const row of rows) {
      const w = cellRenderWidth(row[ci] ?? "");
      if (w > maxLen) maxLen = w;
    }
    return Math.max(maxLen, 1) * charToPt + CELL_PADDING;
  });

  const minWidths = columns.map((_, ci) => {
    let floor = longestWordWidth(columns[ci] ?? "");
    for (const row of rows) {
      const w = longestWordWidth(row[ci] ?? "");
      if (w > floor) floor = w;
    }
    return floor * charToPt + CELL_PADDING;
  });

  let widths = desired.slice();
  let widthSum = widths.reduce((a, b) => a + b, 0);
  if (widthSum > PAGE_CONTENT_WIDTH) {
    const minSum = minWidths.reduce((a, b) => a + b, 0);
    if (minSum >= PAGE_CONTENT_WIDTH) {
      const k = PAGE_CONTENT_WIDTH / minSum;
      widths = minWidths.map((w) => w * k);
    } else {
      const slack = widths.map((w, i) => w - minWidths[i]);
      const slackSum = slack.reduce((a, b) => a + b, 0);
      const excess = widthSum - PAGE_CONTENT_WIDTH;
      widths = widths.map((w, i) =>
        slackSum > 0 ? w - (slack[i] / slackSum) * excess : w
      );
    }
    widthSum = widths.reduce((a, b) => a + b, 0);
  }
  if (widthSum < PAGE_CONTENT_WIDTH && widths.length > 0) {
    widths[widths.length - 1] += PAGE_CONTENT_WIDTH - widthSum;
  }
  widths = widths.map((w) => Math.floor(w));
  const drift = PAGE_CONTENT_WIDTH - widths.reduce((a, b) => a + b, 0);
  if (drift !== 0 && widths.length > 0) widths[widths.length - 1] += drift;

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
        const rowBackground = isHeader
          ? headerBackground
          : opts.striped && ri % 2 === 1
          ? zebraBackground
          : undefined;
        return {
          properties: {
            ...(isHeader ? { keepNext: true } : {}),
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

function codeBlockShell(
  meta: { label: string; accent: string },
  content: BodyElement[],
  attachedBelow = false,
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
      keepNext: true,
    },
  );
  return [
    {
      kind: "table",
      table: {
        properties: {
          layout: "fixed",
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
            properties: { cantSplit: true },
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
    ...(attachedBelow
      ? []
      : [run("", { spacingBefore: 0, spacingAfter: 0, size: 1 })]),
  ];
}

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

function renderCodeBlock(
  b: DocumentBlock,
  attachedBelow = false,
): BodyElement[] {
  const meta = langMeta(b.language);
  const fg = "ABB2BF";
  const MAX_CODE_COLS = 88;
  const content: BodyElement[] = [];

  const lang = (b.language ?? "").toLowerCase();
  if (lang === "mermaid") {
    const diagram = renderMermaidDiagram(String(b.text ?? ""));
    if (diagram) {
      const n = diagram.lines.length;
      for (let k = 0; k < n; k++) {
        const runs = diagram.runs?.[k] ?? colorDiagram(diagram.lines[k]);
        content.push(...paragraphWithRuns(
          [{ text: " " }, ...runs],
          {
            fontFamily: "Courier New",
            size: 8.5,
            color: "E6EDF3",
            spacingBefore: 0,
            spacingAfter: k === n - 1 ? 2 : 0,
          },
        ));
      }
      return codeBlockShell(meta, content, attachedBelow);
    }
  }

  const code = String(b.text ?? "");
  const lines = code.length === 0 ? [""] : code.split("\n");

  for (let li = 0; li < lines.length; li++) {
    const line = lines[li];
    const subs = splitLongLine(line, MAX_CODE_COLS);
    for (let k = 0; k < subs.length; k++) {
      const isLast = k === subs.length - 1;

      content.push(...paragraphWithRuns(
        [{ text: isLast ? " " : "  " }, ...highlight(subs[k], b.language)],
        {
          fontFamily: "Courier New",
          size: 8.5,
          color: fg,
          spacingBefore: 0,
          spacingAfter: isLast ? 2 : 1,
        },
      ));
    }
  }
  return codeBlockShell(meta, content, attachedBelow);
}

export interface FlowDocOptions {
  hardWrap?: boolean;
}

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

export interface MermaidImageData {
  bytes: Uint8Array;
  width: number;
  height: number;
}

export interface LoadedImage {
  bytes: Uint8Array;
  width: number;
  height: number;
}

const IMAGE_MAX_W = 430;
const IMAGE_MAX_H = 440;

function fitToPage(w: number, h: number): { w: number; h: number } {
  if (w > IMAGE_MAX_W || h > IMAGE_MAX_H) {
    const k = Math.min(
      w > IMAGE_MAX_W ? IMAGE_MAX_W / w : 1,
      h > IMAGE_MAX_H ? IMAGE_MAX_H / h : 1,
    );
    w *= k;
    h *= k;
  }
  return { w, h };
}

function imageSizeFor(
  img: LoadedImage,
  requestedWidth?: number,
  requestedHeight?: number,
): { w: number; h: number } {
  const iw = img.width > 0 ? img.width : 0;
  const ih = img.height > 0 ? img.height : 0;

  if (requestedWidth && requestedHeight) {
    return fitToPage(requestedWidth, requestedHeight);
  }
  if (requestedWidth) {
    return fitToPage(
      requestedWidth,
      iw > 0
        ? (requestedWidth * ih) / iw
        : requestedHeight ?? requestedWidth * 0.75,
    );
  }
  if (requestedHeight) {
    return fitToPage(
      ih > 0
        ? (requestedHeight * iw) / ih
        : requestedWidth ?? requestedHeight * 1.5,
      requestedHeight,
    );
  }
  if (iw > 0 && ih > 0) {
    return fitToPage(iw, ih);
  }
  return { w: 300, h: 200 };
}

export function jsonToFlowDoc(
  content: unknown,
  images?: Map<string, LoadedImage>,
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
        const task = /^\[([ xX])\]\s+(.*)$/.exec(item);
        if (task) {
          const done = task[1].toLowerCase() === "x";
          body.push(
            ...paragraphWithRuns([
              {
                text: done ? "\u2611  " : "\u2610  ",
                color: done ? "2E7D32" : "6B7280",
              },
              ...parseInline(task[2]).map((r) =>
                done ? { ...r, color: r.color ?? "6B7280" } : r
              ),
            ], { ...style, indent: 1 }),
          );
          continue;
        }
        body.push(
          ...paragraphWithRuns([{ text: "\u2022  " }, ...parseInline(item)], {
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
      const img = images?.get(String(b.url ?? ""));
      if (img) {
        const size = imageSizeFor(
          img,
          Number(b.width) || undefined,
          Number(b.height) || undefined,
        );
        body.push({
          kind: "image",
          image: {
            resource: resources.put(img.bytes),
            width: pt(size.w),
            height: pt(size.h),
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

      if (lang === "mermaid") {
        const png = mermaidImages?.get(bi);
        if (png) {
          body.push(mermaidImageBlock(png, resources));
          continue;
        }
      }
      const nextIsQuote = String(blocks[bi + 1]?.kind ?? "") === "blockquote";
      body.push(...renderCodeBlock(b, nextIsQuote));
    } else if (kind === "blockquote") {
      const prev = blocks[bi - 1];
      const afterCode = String(prev?.kind ?? "") === "codeblock";

      const accent = afterCode ? langMeta(prev?.language).accent : undefined;
      const bg = afterCode ? "1F2D22" : "F5F5F7";
      const bar = afterCode ? (accent ?? "3E5C42") : "D85131";
      const textColor = afterCode ? "C8D6C4" : "374151";
      const PAGE_CONTENT_WIDTH = 468;
      const content: BodyElement[] = [];
      const lines = (b.items ?? []).length ? (b.items ?? []) : [""];
      lines.forEach((line, idx) => {
        const icon = afterCode && idx === 0
          ? [{ text: "\u263C  ", color: accent ?? "E8C547", size: 11 }]
          : [];
        content.push(...paragraphWithRuns(
          [...icon, ...parseInline(line || " ")],
          {
            size: afterCode ? 9.5 : 10.5,
            color: textColor,
            spacingBefore: 0,
            spacingAfter: afterCode ? 1 : 3,
            ...(afterCode ? { codeColor: "9CCFA0" } : {}),
          },
        ));
      });

      const border = afterCode
        ? { style: "single" as const, width: pt(0.75), colorHex: "3E5C42" }
        : { style: "single" as const, width: pt(2), colorHex: bar };

      const leftBar = afterCode
        ? { style: "single" as const, width: pt(2), colorHex: bar }
        : border;

      if (!afterCode) {
        body.push(run("", { spacingBefore: 0, spacingAfter: 0, size: 1 }));
      }
      body.push({
        kind: "table",
        table: {
          properties: { layout: "fixed" },
          grid: [pt(PAGE_CONTENT_WIDTH)],
          rows: [{
            properties: { isHeader: false },
            cells: [{
              properties: {
                shading: { colorHex: bg },
                borders: afterCode
                  ? {
                    top: border,
                    right: border,
                    bottom: border,
                    left: leftBar,
                  }
                  : { left: leftBar },
              },
              content,
            }],
          }],
        },
      });
      body.push(run("", { spacingBefore: 0, spacingAfter: 0, size: 1 }));
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
