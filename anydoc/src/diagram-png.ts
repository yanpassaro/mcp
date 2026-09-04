import { mermaidToSvg } from "./diagram.ts";
import { svgToPng } from "./raster.ts";
import { type GlyphShaper, loadGlyphShaper } from "./svg-text.ts";

export interface MermaidPng {
  bytes: Uint8Array;

  width: number;
  height: number;
}

let shaperCache: GlyphShaper | null | undefined;
async function loadShaper(): Promise<GlyphShaper | null> {
  if (shaperCache !== undefined) return shaperCache;
  shaperCache = await loadGlyphShaper();
  return shaperCache;
}

export async function mermaidToPng(source: string): Promise<MermaidPng | null> {
  const shaper = await loadShaper();

  if (!shaper) return null;
  try {
    const box = mermaidToSvg(source, shaper);
    if (!box) return null;
    const bytes = svgToPng(box.svg, box.width, box.height);
    if (!bytes) return null;
    return { bytes, width: box.width, height: box.height };
  } catch {
    return null;
  }
}

export async function mermaidPngDiagnose(): Promise<string> {
  const out: string[] = [];
  const shaper = await loadShaper();
  out.push(
    shaper
      ? "shaper de texto: OK (contornos TrueType)"
      : "shaper de texto: NENHUM (fontes TrueType não encontradas)",
  );
  if (shaper) {
    try {
      const shaped = shaper({
        text: "Ag123 ãção",
        x: 10,
        y: 16,
        size: 14,
        anchor: "start",
        color: "#ffffff",
      });
      out.push(
        `prova do shaper: ${
          shaped.slice(0, 90)
        }… (${shaped.length} caracteres)`,
      );
    } catch (e) {
      out.push(`shaper ERRO → ${String(e).slice(0, 200)}`);
    }
  }
  out.push("rasterizador: próprio (raster.ts) — sem wasm/dependências");
  return out.join("\n");
}
