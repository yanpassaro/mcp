import type { DocumentBlock } from "./flow.ts";
import { mermaidToPng } from "./diagram-png.ts";
import type { MermaidPng } from "./diagram-png.ts";

async function resolveImage(url: string): Promise<Uint8Array | undefined> {
  if (/^https?:\/\//i.test(url)) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 20000);
    try {
      const res = await fetch(url, { signal: controller.signal });
      if (!res.ok) return undefined;
      return new Uint8Array(await res.arrayBuffer());
    } catch {
      return undefined;
    } finally {
      clearTimeout(timer);
    }
  }
  try {
    return await Deno.readFile(url);
  } catch {
    return undefined;
  }
}

export async function loadImages(blocks: DocumentBlock[]): Promise<Map<string, Uint8Array>> {
  const map = new Map<string, Uint8Array>();
  for (const b of blocks) {
    if (b.kind !== "image" || !b.url) continue;
    if (map.has(b.url)) continue;
    const data = await resolveImage(b.url);
    if (data) map.set(b.url, data);
  }
  return map;
}

// Pré-renderiza os diagramas mermaid como PNG (índice do bloco → imagem).
// Se o resvg/não estiver disponível, o bloco fica de fora do mapa e o fluxo
// usa o fallback ASCII — nada quebra.
export async function loadMermaidImages(blocks: DocumentBlock[]): Promise<Map<number, MermaidPng>> {
  const map = new Map<number, MermaidPng>();
  let i = 0;
  for (const b of blocks) {
    if (b.kind === "codeblock" && String(b.language ?? "").toLowerCase() === "mermaid") {
      const png = await mermaidToPng(String(b.text ?? ""));
      if (png) map.set(i, png);
    }
    i++;
  }
  return map;
}
