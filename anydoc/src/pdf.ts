import { Ream, writeDocx } from "@reamkit";
import { jsonToFlowDoc, withUtcDates, type DocumentBlock } from "./flow.ts";
import { saveFile } from "./save.ts";
import { loadImages, loadMermaidImages } from "./images.ts";
import type { ParsedMarkdown } from "./markdown.ts";
import type { CreatedDocument } from "./types.ts";

// O conversor DOCX→PDF do reamkit embute apenas fontes com um subconjunto de
// glifos: símbolos como ✔(2714), ✗(2717), ♥(2665), ★(2605), ⚠(26A0) passam,
// mas vários emojis não (ex.: ✅ 2705 e todo o plano astral 1F000+). Por isso o
// caminho PDF normaliza: remove marcadores de apresentação/ligação (FE0F, 200D)
// e tons de pele, e troca os emojis mais comuns por glifos equivalentes que a
// fonte aguenta. O DOCX não passa por isso — Word/Google Docs renderizam emoji
// colorido de verdade.
const EMOJI_TO_GLYPH: Record<string, string> = {
  "✅": "✔", // 2705 → 2714
  "❌": "✗", // 274C → 2717
  "❤": "♥", // 2764 → 2665
  "❤️": "♥",
  "💜": "♥",
  "💙": "♥",
  "⭐": "★", // 2B50 → 2605
  "🌟": "★", // 1F31F → 2605
  "✨": "★", // 2728 → 2605
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

// Aplica a normalização de emoji apenas nos blocos de texto. Blocos de código
// ficam intocados (conteúdo verbatim), imagens/mermaid também.
function pdfSafeBlocks(blocks: DocumentBlock[]): DocumentBlock[] {
  const safe = (s?: string) => (s === undefined ? undefined : pdfSafeText(s));
  return blocks.map((b) => {
    if (b.kind === "image" || (b.kind === "codeblock" && b.text !== undefined)) {
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
  const flow = jsonToFlowDoc({
    title: pdfSafeText(parsed.title),
    subtitle: pdfSafeText(parsed.subtitle),
    blocks: pdfSafeBlocks(parsed.blocks),
  }, images, { hardWrap: true }, mermaid);
  const r = withUtcDates(() => writeDocx(flow));
  const converted = Ream.parse(r.bytes);
  const bytes = await converted.convert("pdf");
  const losses = r.losses.length + (converted.losses?.length ?? 0);
  return saveFile(path, bytes, losses);
}
