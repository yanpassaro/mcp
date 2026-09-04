import { Ream, writeDocx } from "@reamkit";
import { type DocumentBlock, jsonToFlowDoc, withUtcDates } from "./flow.ts";
import { saveFile } from "./save.ts";
import { loadImages, loadMermaidImages } from "./images.ts";
import type { ParsedMarkdown } from "./markdown.ts";
import type { CreatedDocument } from "./types.ts";

const EMOJI_TO_GLYPH: Record<string, string> = {
  "✅": "✔",
  "❌": "✗",
  "❤": "♥",
  "❤️": "♥",
  "💜": "♥",
  "💙": "♥",
  "⭐": "★",
  "🌟": "★",
  "✨": "★",
};

const PDF_FORMAT_RE = /[\uFE0E\uFE0F\u200D\u{1F3FB}-\u{1F3FF}]/gu;

function pdfSafeText(text: string | undefined): string {
  if (!text) return "";
  let s = text.replace(PDF_FORMAT_RE, "");
  for (const [emoji, glyph] of Object.entries(EMOJI_TO_GLYPH)) {
    if (s.includes(emoji)) s = s.split(emoji).join(glyph);
  }
  return s;
}

function pdfSafeBlocks(blocks: DocumentBlock[]): DocumentBlock[] {
  const safe = (s?: string) => (s === undefined ? undefined : pdfSafeText(s));
  return blocks.map((b) => {
    if (
      b.kind === "image" || (b.kind === "codeblock" && b.text !== undefined)
    ) {
      return b;
    }
    const out: DocumentBlock = { ...b };
    out.text = safe(b.text);
    if (b.items) out.items = b.items.map((i) => safe(i) ?? "");
    if (b.rows) out.rows = b.rows.map((r) => r.map((c) => safe(c) ?? ""));
    if (b.runs) {
      out.runs = b.runs.map((r) => (r.code ? r : { ...r, text: safe(r.text) }));
    }
    return out;
  });
}

export async function createPdf(
  path: string,
  parsed: ParsedMarkdown,
): Promise<CreatedDocument> {
  const images = await loadImages(parsed.blocks);
  const mermaid = await loadMermaidImages(parsed.blocks);
  const flow = jsonToFlowDoc(
    {
      title: pdfSafeText(parsed.title),
      subtitle: pdfSafeText(parsed.subtitle),
      blocks: pdfSafeBlocks(parsed.blocks),
    },
    images,
    { hardWrap: true },
    mermaid,
  );
  const r = withUtcDates(() => writeDocx(flow));
  const converted = Ream.parse(r.bytes);
  const bytes = await converted.convert("pdf");
  const losses = r.losses.length + (converted.losses?.length ?? 0);
  return saveFile(path, bytes, losses);
}
