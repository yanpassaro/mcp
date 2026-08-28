// Gera o PNG do diagrama mermaid SEM dependências externas: o SVG produzido
// pelo diagram.ts é rasterizado pelo nosso raster.ts (scanline + supersampling)
// e codificado como PNG (zlib via fflate, já usado pelo projeto). A única
// coisa lida do sistema é a fonte TrueType (contornos → paths, svg-text.ts).
// QUALQUER falha aqui retorna null — o fluxo cai no ASCII (nunca quebra).
import { mermaidToSvg } from "./diagram.ts";
import { svgToPng } from "./raster.ts";
import { loadGlyphShaper, type GlyphShaper } from "./svg-text.ts";

export interface MermaidPng {
  bytes: Uint8Array;
  /** Dimensões de design (antes do 2x da rasterização). */
  width: number;
  height: number;
}

let shaperCache: GlyphShaper | null | undefined; // undefined = ainda não tentou
async function loadShaper(): Promise<GlyphShaper | null> {
  if (shaperCache !== undefined) return shaperCache;
  shaperCache = await loadGlyphShaper();
  return shaperCache;
}

/** Gera o PNG do diagrama mermaid; null se algo falhar (fallback ASCII). */
export async function mermaidToPng(source: string): Promise<MermaidPng | null> {
  const shaper = await loadShaper();
  // Texto vira path via shaper; sem shaper (nenhuma fonte TrueType lida) não há
  // como ter texto → melhor o fallback ASCII (que tem texto).
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

/** Diagnóstico rápido do pipeline PNG (para o teste scratch). */
export async function mermaidPngDiagnose(): Promise<string> {
  const out: string[] = [];
  const shaper = await loadShaper();
  out.push(shaper ? "shaper de texto: OK (contornos TrueType)" : "shaper de texto: NENHUM (fontes TrueType não encontradas)");
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
      out.push(`prova do shaper: ${shaped.slice(0, 90)}… (${shaped.length} caracteres)`);
    } catch (e) {
      out.push(`shaper ERRO → ${String(e).slice(0, 200)}`);
    }
  }
  out.push("rasterizador: próprio (raster.ts) — sem wasm/dependências");
  return out.join("\n");
}
