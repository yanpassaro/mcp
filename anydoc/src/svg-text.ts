

export interface GlyphTextOpts {
  text: string;
  x: number;
  y: number;
  size: number;
  anchor: "start" | "middle" | "end";
  color: string;
}

export type GlyphShaper = (opts: GlyphTextOpts) => string;

const FONT_CANDIDATES = [
  "C:\\Windows\\Fonts\\arial.ttf",
  "C:\\Windows\\Fonts\\segoeui.ttf",
  "C:\\Windows\\Fonts\\tahoma.ttf",
  "C:\\Windows\\Fonts\\consola.ttf",
  "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
  "/System/Library/Fonts/Supplemental/Arial.ttf",
];

function findTable(bytes: Uint8Array, tag: string): { off: number; len: number } | null {
  const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const numTables = dv.getUint16(4);
  for (let i = 0; i < numTables; i++) {
    const r = 12 + i * 16;
    let t = "";
    for (let k = 0; k < 4; k++) t += String.fromCharCode(bytes[r + k]);
    if (t === tag) return { off: dv.getUint32(r + 8), len: dv.getUint32(r + 12) };
  }
  return null;
}

class Ttf {
  readonly dv: DataView;
  readonly unitsPerEm: number;
  readonly numGlyphs: number;
  readonly glyphOffsets: number[];
  readonly glyfOff: number;
  readonly advances: number[];
  readonly charGlyph = new Map<number, number>();
  private pathCache = new Map<number, string>();

  constructor(readonly bytes: Uint8Array) {
    this.dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    const head = findTable(bytes, "head");
    const maxp = findTable(bytes, "maxp");
    const hhea = findTable(bytes, "hhea");
    const hmtx = findTable(bytes, "hmtx");
    const loca = findTable(bytes, "loca");
    const glyf = findTable(bytes, "glyf");
    if (!head || !maxp || !hhea || !hmtx || !loca || !glyf) throw new Error("fonte sem tabelas TrueType");
    this.unitsPerEm = this.dv.getUint16(head.off + 18);
    if (!this.unitsPerEm) throw new Error("unitsPerEm inválido");
    this.numGlyphs = this.dv.getUint16(maxp.off + 4);
    this.glyfOff = glyf.off;


    const short = this.dv.getInt16(head.off + 50) === 0;
    this.glyphOffsets = new Array(this.numGlyphs + 1);
    for (let g = 0; g <= this.numGlyphs; g++) {
      this.glyphOffsets[g] = short
        ? this.dv.getUint16(loca.off + g * 2) * 2
        : this.dv.getUint32(loca.off + g * 4);
    }


    const numMetrics = this.dv.getUint16(hhea.off + 34);
    const lastAdv = numMetrics ? this.dv.getUint16(hmtx.off + (numMetrics - 1) * 4) : 0;
    this.advances = new Array(this.numGlyphs);
    for (let g = 0; g < this.numGlyphs; g++) {
      this.advances[g] = g < numMetrics
        ? this.dv.getUint16(hmtx.off + g * 4)
        : lastAdv;
    }


    const cmap = findTable(bytes, "cmap");
    if (cmap) this.parseCmap(cmap.off, cmap.len);
  }

  private parseCmap(off: number, _len: number): void {
    const dv = this.dv;
    const count = dv.getUint16(off + 2);

    const recs: { pid: number; eid: number; sub: number }[] = [];
    for (let i = 0; i < count; i++) {
      recs.push({
        pid: dv.getUint16(off + 4 + i * 8),
        eid: dv.getUint16(off + 6 + i * 8),
        sub: off + dv.getUint32(off + 8 + i * 8),
      });
    }
    const rank = (r: { pid: number; eid: number }) =>
      (r.pid === 3 && r.eid === 1 ? 0 : r.pid === 0 ? 1 : 9);
    recs.sort((a, b) => rank(a) - rank(b));
    for (const r of recs) {
      const format = dv.getUint16(r.sub);
      if (format === 4) { this.parseCmap4(r.sub); return; }
      if (format === 12) { this.parseCmap12(r.sub, r.sub + dv.getUint32(r.sub + 12)); return; }
    }
  }

  private parseCmap4(sub: number): void {
    const dv = this.dv;
    const segCountX2 = dv.getUint16(sub + 6);
    const segCount = segCountX2 >>> 1;
    const ends = sub + 14;
    const starts = ends + segCountX2 + 2;
    const deltas = starts + segCountX2;
    const rangeOffs = deltas + segCountX2;
    for (let s = 0; s < segCount; s++) {
      const end = dv.getUint16(ends + s * 2);
      const start = dv.getUint16(starts + s * 2);
      const delta = dv.getInt16(deltas + s * 2);
      const ro = dv.getUint16(rangeOffs + s * 2);
      if (ro === 0) {
        for (let c = start; c <= end && c <= 0xffff; c++) {
          const gid = (c + delta) & 0xffff;
          if (gid) this.charGlyph.set(c, gid);
        }
      } else {
        for (let c = start; c <= end && c <= 0xffff; c++) {
          const idx = rangeOffs + s * 2 + ro + (c - start) * 2;
          const g = dv.getUint16(idx);
          const gid = g ? (g + delta) & 0xffff : 0;
          if (gid) this.charGlyph.set(c, gid);
        }
      }
    }
  }

  private parseCmap12(sub: number, end: number): void {
    const dv = this.dv;
    const groups = dv.getUint32(sub + 12);
    let p = sub + 16;
    for (let i = 0; i < groups && p + 12 <= end; i++) {
      const start = dv.getUint32(p);
      const last = dv.getUint32(p + 4);
      const gid0 = dv.getUint32(p + 8);
      p += 12;
      for (let c = start; c <= last; c++) {
        const gid = (gid0 + (c - start)) & 0xffff;
        if (gid && c <= 0xffff) this.charGlyph.set(c, gid);
      }
    }
  }

  glyphPath(gid: number): string {
    if (gid <= 0 || gid >= this.numGlyphs) return "";
    const cached = this.pathCache.get(gid);
    if (cached !== undefined) return cached;

    const contours = this.contoursOf(gid, new Set());
    let d = "";
    if (contours) for (const c of contours) d += contourToPath(c);
    this.pathCache.set(gid, d);
    return d;
  }

  advance(gid: number): number {
    return gid > 0 && gid < this.numGlyphs ? this.advances[gid] : 0;
  }


  private contoursOf(gid: number, visiting: Set<number>): GlyphPoint[][] | null {
    if (gid <= 0 || gid >= this.numGlyphs || visiting.has(gid)) return null;
    visiting.add(gid);
    try {
      const start = this.glyfOff + this.glyphOffsets[gid];
      const end = this.glyfOff + this.glyphOffsets[gid + 1];
      if (end - start < 10) return null;
      const numContours = this.dv.getInt16(start);
      if (numContours >= 0) {

        const endPts: number[] = [];
        let p = start + 10;
        for (let c = 0; c < numContours; c++) {
          endPts.push(this.dv.getUint16(p));
          p += 2;
        }
        const insLen = this.dv.getUint16(p);
        p += 2 + insLen;
        const total = endPts[numContours - 1] + 1;

        const flags: number[] = new Array(total);
        for (let i = 0; i < total;) {
          const f = this.dv.getUint8(p++);
          flags[i++] = f;
          if (f & 8) {
            const rep = this.dv.getUint8(p++);
            for (let k = 0; k < rep && i < total; k++) flags[i++] = f;
          }
        }
        const xs: number[] = new Array(total);
        let x = 0;
        for (let i = 0; i < total; i++) {
          const f = flags[i];
          if (f & 2) {
            const b = this.dv.getUint8(p++);
            x += f & 16 ? b : -b;
          } else if (!(f & 16)) {
            x += this.dv.getInt16(p);
            p += 2;
          }
          xs[i] = x;
        }
        const ys: number[] = new Array(total);
        let y = 0;
        for (let i = 0; i < total; i++) {
          const f = flags[i];
          if (f & 4) {
            const b = this.dv.getUint8(p++);
            y += f & 32 ? b : -b;
          } else if (!(f & 32)) {
            y += this.dv.getInt16(p);
            p += 2;
          }
          ys[i] = y;
        }

        const out: GlyphPoint[][] = [];
        let idx = 0;
        for (const lastIdx of endPts) {
          const pts: GlyphPoint[] = [];
          for (; idx <= lastIdx; idx++) {
            pts.push({ x: xs[idx], y: ys[idx], on: (flags[idx] & 1) !== 0 });
          }
          out.push(pts);
        }
        return out;
      }


      const parts = this.compositeParts(start, end);
      const out: GlyphPoint[][] = [];
      for (const part of parts) {
        const sub = this.contoursOf(part.gid, visiting);
        if (!sub) continue;
        for (const contour of sub) {
          out.push(contour.map((pt) => ({
            x: part.a * pt.x + part.b * pt.y + part.dx,
            y: part.c * pt.x + part.d * pt.y + part.dy,
            on: pt.on,
          })));
        }
      }
      return out;
    } finally {
      visiting.delete(gid);
    }
  }


  private compositeParts(
    start: number,
    end: number,
  ): { gid: number; a: number; b: number; c: number; d: number; dx: number; dy: number }[] {
    const dv = this.dv;
    const parts: { gid: number; a: number; b: number; c: number; d: number; dx: number; dy: number }[] = [];
    let p = start + 10;
    for (let guard = 0; guard < 32 && p + 4 <= end; guard++) {
      const flags = dv.getUint16(p);
      const compGid = dv.getUint16(p + 2);
      p += 4;
      let arg1: number;
      let arg2: number;
      if (flags & 0x0001) {
        arg1 = dv.getInt16(p);
        arg2 = dv.getInt16(p + 2);
        p += 4;
      } else {
        arg1 = dv.getInt8(p);
        arg2 = dv.getInt8(p + 1);
        p += 2;
      }
      let a = 1;
      let b = 0;
      let c = 0;
      let d = 1;
      if (flags & 0x0008) {
        const s = dv.getUint16(p) / 16384;
        p += 2;
        a = d = s;
      } else if (flags & 0x0040) {
        a = dv.getUint16(p) / 16384;
        d = dv.getUint16(p + 2) / 16384;
        p += 4;
      } else if (flags & 0x0080) {
        a = dv.getUint16(p) / 16384;
        b = dv.getUint16(p + 2) / 16384;
        c = dv.getUint16(p + 4) / 16384;
        d = dv.getUint16(p + 6) / 16384;
        p += 8;
      }

      const dx = arg1;
      const dy = arg2;
      parts.push({
        gid: compGid,
        a, b, c, d,
        dx: (flags & 0x0002) ? dx : 0,
        dy: (flags & 0x0002) ? dy : 0,
      });
      if (!(flags & 0x0020)) break;
    }
    return parts;
  }
}

interface GlyphPoint { x: number; y: number; on: boolean; }


function contourToPath(pts: { x: number; y: number; on: boolean }[]): string {
  const n = pts.length;
  if (n < 2) return "";
  let s = pts.findIndex((p) => p.on);
  if (s < 0) s = 0;
  const rot: { x: number; y: number; on: boolean }[] = [];
  for (let k = 0; k < n; k++) rot.push(pts[(s + k) % n]);

  let d = `M${rot[0].x.toFixed(1)} ${rot[0].y.toFixed(1)}`;
  let ctrl: { x: number; y: number } | null = null;
  for (let k = 1; k < n; k++) {
    const p = rot[k];
    if (p.on) {
      if (ctrl) {
        d += `Q${ctrl.x.toFixed(1)} ${ctrl.y.toFixed(1)} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`;
        ctrl = null;
      } else {
        d += `L${p.x.toFixed(1)} ${p.y.toFixed(1)}`;
      }
    } else if (ctrl) {
      const mx = (ctrl.x + p.x) / 2;
      const my = (ctrl.y + p.y) / 2;
      d += `Q${ctrl.x.toFixed(1)} ${ctrl.y.toFixed(1)} ${mx.toFixed(1)} ${my.toFixed(1)}`;
      ctrl = p;
    } else {
      ctrl = p;
    }
  }
  if (ctrl) {
    const mx = (ctrl.x + rot[0].x) / 2;
    const my = (ctrl.y + rot[0].y) / 2;
    d += `Q${ctrl.x.toFixed(1)} ${ctrl.y.toFixed(1)} ${mx.toFixed(1)} ${my.toFixed(1)}`;
  }
  return d + "Z";
}

export async function loadGlyphShaper(): Promise<GlyphShaper | null> {
  let ttf: Ttf | null = null;
  for (const p of FONT_CANDIDATES) {
    try {
      const bytes = await Deno.readFile(p);
      try {
        ttf = new Ttf(bytes);
        break;
      } catch {
        continue;
      }
    } catch {
      continue;
    }
  }
  if (!ttf) return null;

  return (opts: GlyphTextOpts): string => {
    const scale = opts.size / ttf!.unitsPerEm;
    const glyphs: number[] = [];
    let w = 0;
    for (const ch of opts.text) {
      const gid = ttf!.charGlyph.get(ch.codePointAt(0) ?? 0) ?? 0;
      glyphs.push(gid);
      w += ttf!.advance(gid) * scale;
    }
    let cx = opts.anchor === "middle" ? opts.x - w / 2 : opts.anchor === "end" ? opts.x - w : opts.x;
    const parts: string[] = [];
    for (const gid of glyphs) {
      const d = ttf!.glyphPath(gid);
      if (d) {
        parts.push(
          `<path transform="translate(${cx.toFixed(2)},${opts.y.toFixed(2)}) scale(${scale.toFixed(5)},${(-scale).toFixed(5)})" d="${d}" fill="${opts.color}"/>`,
        );
      }
      cx += ttf!.advance(gid) * scale;
    }
    return parts.join("");
  };
}
