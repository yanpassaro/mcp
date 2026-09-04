import type { DocumentBlock, LoadedImage } from "./flow.ts";
import { mermaidToPng } from "./diagram-png.ts";
import type { MermaidPng } from "./diagram-png.ts";

function u16be(b: Uint8Array, o: number): number {
  return (b[o] << 8) | b[o + 1];
}

function u32be(b: Uint8Array, o: number): number {
  return ((b[o] << 24) | (b[o + 1] << 16) | (b[o + 2] << 8) | b[o + 3]) >>> 0;
}

function u16le(b: Uint8Array, o: number): number {
  return b[o] | (b[o + 1] << 8);
}

function u24le(b: Uint8Array, o: number): number {
  return b[o] | (b[o + 1] << 8) | (b[o + 2] << 16);
}

function u32le(b: Uint8Array, o: number): number {
  return (b[o] | (b[o + 1] << 8) | (b[o + 2] << 16) | (b[o + 3] << 24)) >>> 0;
}

// Reads the intrinsic pixel size of the most common image formats so the
// exported image keeps its real aspect ratio instead of being squashed.
export function imageDimensions(
  bytes: Uint8Array,
): { width: number; height: number } | undefined {
  if (bytes.length < 24) return undefined;

  // PNG
  if (
    bytes[0] === 0x89 && bytes[1] === 0x50 && bytes[2] === 0x4e && bytes[3] === 0x47
  ) {
    if (bytes.length < 33) return undefined;
    return { width: u32be(bytes, 16), height: u32be(bytes, 20) };
  }

  // GIF
  if (bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46) {
    return { width: u16le(bytes, 6), height: u16le(bytes, 8) };
  }

  // BMP
  if (bytes[0] === 0x42 && bytes[1] === 0x4d) {
    if (bytes.length < 26) return undefined;
    return { width: u32le(bytes, 18), height: Math.abs(u32le(bytes, 22)) };
  }

  // JPEG (scan markers for a SOF frame, skipping APPn/metadata segments)
  if (bytes[0] === 0xff && bytes[1] === 0xd8) {
    let o = 2;
    while (o + 9 < bytes.length) {
      if (bytes[o] !== 0xff) {
        o++;
        continue;
      }
      const marker = bytes[o + 1];
      if (marker === 0xff || marker === 0x00) {
        o += marker === 0xff ? 1 : 2;
        continue;
      }
      if (
        marker === 0xd8 || marker === 0x01 ||
        (marker >= 0xd0 && marker <= 0xd7)
      ) {
        o += 2;
        continue;
      }
      if (marker === 0xd9) break;
      const len = u16be(bytes, o + 2);
      if (len < 2) break;
      if (marker === 0xda) break;
      if (
        marker >= 0xc0 && marker <= 0xcf &&
        marker !== 0xc4 && marker !== 0xc8 && marker !== 0xcc
      ) {
        return { height: u16be(bytes, o + 5), width: u16be(bytes, o + 7) };
      }
      o += 2 + len;
    }
    return undefined;
  }

  // WebP
  if (
    bytes[0] === 0x52 && bytes[1] === 0x49 && bytes[2] === 0x46 && bytes[3] === 0x46 &&
    bytes[8] === 0x57 && bytes[9] === 0x45 && bytes[10] === 0x42 && bytes[11] === 0x50
  ) {
    const kind = String.fromCharCode(bytes[12], bytes[13], bytes[14], bytes[15]);
    if (kind === "VP8X") {
      if (bytes.length < 30) return undefined;
      return { width: 1 + u24le(bytes, 24), height: 1 + u24le(bytes, 27) };
    }
    if (kind === "VP8L") {
      if (bytes.length < 25) return undefined;
      const b0 = bytes[21];
      const b1 = bytes[22];
      const b2 = bytes[23];
      const b3 = bytes[24];
      return {
        width: 1 + (((b1 & 0x3f) << 8) | b0),
        height: 1 + (((b2 & 0x0f) << 10) | (b3 << 2) | ((b1 >> 6) & 0x03)),
      };
    }
    if (kind === "VP8 ") {
      if (bytes.length < 30) return undefined;
      if (bytes[23] !== 0x9d || bytes[24] !== 0x01 || bytes[25] !== 0x2a) return undefined;
      return {
        width: u16le(bytes, 26) & 0x3fff,
        height: u16le(bytes, 28) & 0x3fff,
      };
    }
    return undefined;
  }

  return undefined;
}

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

export async function loadImages(blocks: DocumentBlock[]): Promise<Map<string, LoadedImage>> {
  const map = new Map<string, LoadedImage>();
  for (const b of blocks) {
    if (b.kind !== "image" || !b.url) continue;
    if (map.has(b.url)) continue;
    const data = await resolveImage(b.url);
    if (!data) continue;
    const dim = imageDimensions(data) ?? { width: 0, height: 0 };
    map.set(b.url, { bytes: data, width: dim.width, height: dim.height });
  }
  return map;
}


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
