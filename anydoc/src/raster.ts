// Rasterizador SVG mínimo para os diagramas gerados pelo diagram.ts.
//
// Não usa resvg nem qualquer dependência externa: o SVG que produzimos só
// contém um subconjunto pequeno e determinístico (rect/círculo/elipse/
// polígono/linha/path com M·L·Q·Z, gradiente linear vertical e opacity), que
// este módulo interpreta com preenchimento scanline (regra even-odd), contorno
// via quads+junções redondas e antialiasing por supersampling 4x + biratamento
// 2x. O PNG é codificado com zlib (fflate, já usado pelo projeto) + CRC32.
import { zlibSync } from "@fflate";

export interface RasterImage {
  width: number; // dimensões do PNG (2x o design)
  height: number;
  rgba: Uint8Array; // 24-bit RGB (PNG color type 2)
}

const SS = 4; // supersampling (buffer de trabalho)
const DS = 2; // downsampling → PNG = design * 2

// ---------------------------------------------------------------------------
// Parse do SVG
// ---------------------------------------------------------------------------
interface RGBA { r: number; g: number; b: number; a: number; }

function hexColor(s: string): RGBA | null {
  const m = /^#([0-9a-fA-F]{6})$/.exec(s.trim());
  if (!m) return null;
  const n = parseInt(m[1], 16);
  return { r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255, a: 255 };
}

interface Stop { t: number; c: RGBA; }
interface Grad { a: Stop[]; b: Stop[]; } // a=topo, b=base (gradiente vertical)

// Desenho de uma forma retangular (rect/rrect) como OPIMIZAÇÃO — os casos mais
// comuns (painel, caixas) não precisam de scanline de polígono por linha.
interface RectInstr {
  kind: "rect";
  x: number; y: number; w: number; h: number; rx: number;
  color: RGBA | Grad;
  opacity: number;
  stroke: RGBA | null;
  strokeWidth: number;
}

type Instr =
  | RectInstr
  | { kind: "shape"; contours: { x: number; y: number }[][]; color: RGBA | Grad; opacity: number; stroke: RGBA | null; strokeWidth: number }
  | { kind: "path"; contours: { x: number; y: number }[][]; close: boolean[]; fill: RGBA | Grad | null; stroke: RGBA | null; strokeWidth: number; opacity: number };

function attrMap(tag: string): Map<string, string> {
  const m = new Map<string, string>();
  const re = /([A-Za-z0-9_:.-]+)="([^"]*)"/g;
  let mm: RegExpExecArray | null;
  while ((mm = re.exec(tag))) m.set(mm[1].toLowerCase(), mm[2]);
  return m;
}

interface Pt { x: number; y: number; }

// Achata path SVG (M/L/Q/Z) em contornos de segmentos retos.
function flattenPathD(d: string): { contours: Pt[][]; close: boolean[] } {
  const tokens = d.match(/[MLQZmlqz]|[-+]?(?:\d+\.?\d*|\.\d+)(?:[eE][-+]?\d+)?/g) ?? [];
  const contours: Pt[][] = [];
  const closed: boolean[] = [];
  let open: Pt[] = [];
  let i = 0;
  let x = 0;
  let y = 0;
  let sx = 0;
  let sy = 0;
  const num = (): number => parseFloat(tokens[i++] ?? "0");
  while (i < tokens.length) {
    const t = tokens[i];
    if (!/[-+]?(?:\d|\.)/.test(t)) {
      i++;
      if (t === "M" || t === "m") {
        if (open.length) contours.push(open);
        open = [];
        x = num();
        y = num();
        sx = x;
        sy = y;
        open.push({ x, y });
      } else if (t === "L" || t === "l") {
        x = num();
        y = num();
        open.push({ x, y });
      } else if (t === "Q" || t === "q") {
        const qx = num();
        const qy = num();
        const ex = num();
        const ey = num();
        const x0 = open.length ? open[open.length - 1].x : x;
        const y0 = open.length ? open[open.length - 1].y : y;
        // Número de segmentos adaptativo ao comprimento da curva (~0.4px):
        // suficiente para curvas de glifo ficarem lisas em qualquer escala.
        const L = Math.hypot(qx - x0, qy - y0) + Math.hypot(ex - qx, ey - qy);
        const n = Math.max(6, Math.min(48, Math.ceil(L / 60)));
        for (let s = 1; s <= n; s++) {
          const u = s / n;
          const v = 1 - u;
          open.push({
            x: v * v * x0 + 2 * v * u * qx + u * u * ex,
            y: v * v * y0 + 2 * v * u * qy + u * u * ey,
          });
        }
        x = ex;
        y = ey;
      } else if (t === "Z" || t === "z") {
        if (open.length) {
          open.push({ x: sx, y: sy });
          contours.push(open);
          closed.push(true);
        }
        open = [];
      }
      continue;
    }
    // Número solto (notação compacta sem comando) — ignora, não geramos assim.
    i++;
  }
  if (open.length) {
    contours.push(open);
    closed.push(false);
  }
  return { contours, closed };
}

function parseTransform(s: string | undefined): { a: number; b: number; c: number; d: number; e: number; f: number } | null {
  if (!s) return null;
  const m = /translate\(\s*([-\d.]+)\s*[, ]\s*([-\d.]+)\s*\)\s*scale\(\s*([-\d.]+)\s*[, ]\s*([-\d.]+)\s*\)/.exec(s);
  if (!m) return null;
  return { a: parseFloat(m[3]), b: 0, c: 0, d: parseFloat(m[4]), e: parseFloat(m[1]), f: parseFloat(m[2]) };
}

function applyTransform(pts: Pt[], tf: { a: number; b: number; c: number; d: number; e: number; f: number } | null): Pt[] {
  if (!tf) return pts;
  const out: Pt[] = new Array(pts.length);
  for (let k = 0; k < pts.length; k++) {
    const p = pts[k];
    out[k] = { x: tf.a * p.x + tf.c * p.y + tf.e, y: tf.b * p.x + tf.d * p.y + tf.f };
  }
  return out;
}

// Amostra uma curva de cor entre dois stops.
function gradAt(g: Grad, t: number): RGBA {
  t = Math.max(0, Math.min(1, t));
  const stops = g.a.length ? g.a : [];
  if (stops.length === 0) return { r: 0, g: 0, b: 0, a: 255 };
  if (stops.length === 1) return stops[0].c;
  for (let i = 0; i < stops.length - 1; i++) {
    const s0 = stops[i];
    const s1 = stops[i + 1];
    if (t <= s1.t) {
      const u = s1.t === s0.t ? 0 : (t - s0.t) / (s1.t - s0.t);
      return {
        r: s0.c.r + (s1.c.r - s0.c.r) * u,
        g: s0.c.g + (s1.c.g - s0.c.g) * u,
        b: s0.c.b + (s1.c.b - s0.c.b) * u,
        a: 255,
      };
    }
  }
  return stops[stops.length - 1].c;
}

// ---------------------------------------------------------------------------
// Rasterização scanline (even-odd + supersampling)
// ---------------------------------------------------------------------------
export function renderSvg(svg: string, designW: number, designH: number): RasterImage | null {
  const W = Math.round(designW * SS);
  const H = Math.round(designH * SS);
  if (W <= 0 || H <= 0) return null;
  // Fundo = cor do painel (primeiro <rect fill="#..">): assim a margem fora do
  // painel fica contínua (sem filete preto nas bordas do PNG) e os cantos
  // arredondados “recortam” sobre o mesmo tom.
  const panelHex = /<rect[^>]*fill="#([0-9a-fA-F]{6})"/.exec(svg)?.[1] ?? "1A1D23";
  const panelBg = hexColor("#" + panelHex) ?? { r: 26, g: 29, b: 35, a: 255 };
  const buf = new Uint8Array(W * H * 3); // RGB opaco
  for (let i = 0; i < buf.length; i += 3) {
    buf[i] = panelBg.r;
    buf[i + 1] = panelBg.g;
    buf[i + 2] = panelBg.b;
  }

  const setPx = (x: number, y: number, c: RGBA) => {
    if (x < 0 || y < 0 || x >= W || y >= H) return;
    const o = (y * W + x) * 3;
    const a = c.a / 255;
    buf[o] = c.r * a + buf[o] * (1 - a);
    buf[o + 1] = c.g * a + buf[o + 1] * (1 - a);
    buf[o + 2] = c.b * a + buf[o + 2] * (1 - a);
  };

  // Preenchimento even-odd com TODOS os contornos do elemento juntos: os furos
  // (caso de letras como a/e/o — contorno externo + buraco) ficam vazios porque
  // os cruzamentos são casados em sequência (1-2, 3-4...).
  const fillScanlines = (contours: Pt[][], colorFn: (y: number) => RGBA) => {
    let ymin = Infinity;
    let ymax = -Infinity;
    for (const pts of contours) {
      for (const p of pts) {
        if (p.y < ymin) ymin = p.y;
        if (p.y > ymax) ymax = p.y;
      }
    }
    if (ymin === Infinity) return;
    const ys = Math.max(0, Math.floor(ymin));
    const ye = Math.min(H - 1, Math.ceil(ymax) - 1);
    const xsArr: number[] = [];
    for (let y = ys; y <= ye; y++) {
      const yy = y + 0.5;
      xsArr.length = 0;
      for (const pts of contours) {
        const n = pts.length;
        for (let k = 0; k < n; k++) {
          const p = pts[k];
          const q = pts[(k + 1) % n];
          if (Math.abs(q.y - p.y) < 1e-9) continue;
          if ((p.y <= yy && q.y > yy) || (q.y <= yy && p.y > yy)) {
            xsArr.push(p.x + (yy - p.y) * (q.x - p.x) / (q.y - p.y));
          }
        }
      }
      xsArr.sort((a, b) => a - b);
      const c = colorFn(yy);
      for (let i = 0; i + 1 < xsArr.length; i += 2) {
        const x0p = Math.max(0, Math.ceil(xsArr[i]));
        const x1p = Math.min(W - 1, Math.floor(xsArr[i + 1]));
        if (x1p < x0p) continue;
        for (let x = x0p; x <= x1p; x++) setPx(x, y, c);
      }
    }
  };

  const isFlat = (c: unknown): c is RGBA => typeof c === "object" && c !== null && "r" in (c as Record<string, unknown>);

  const fillRect = (ri: RectInstr) => {
    const x0 = ri.x;
    const y0 = ri.y;
    const x1 = ri.x + ri.w;
    const y1 = ri.y + ri.h;
    const rx = Math.min(ri.rx, ri.w / 2, ri.h / 2);
    const flat: RGBA | null = isFlat(ri.color) ? ri.color : null;
    const grad = flat ? null : ri.color as Grad;
    const ys = Math.max(0, Math.floor(y0));
    const ye = Math.min(H - 1, Math.ceil(y1) - 1);
    for (let y = ys; y <= ye; y++) {
      const yy = y + 0.5;
      const c = flat ?? gradAt(grad!, (yy - y0) / Math.max(1, ri.h));
      let left = x0;
      let right = x1 - 1;
      if (rx > 0) {
        // Cantos arredondados: distância ao centro do arco (v = yy - cy) define
        // o quanto a faixa encolhe; aplica para os quatro cantos.
        for (const cy of [y0 + rx, y1 - rx]) {
          const v = yy - cy;
          if (Math.abs(v) <= rx) {
            const dx = Math.sqrt(rx * rx - v * v);
            left = Math.max(left, x0 + rx - dx);
            right = Math.min(right, x1 - rx + dx);
          }
        }
      }
      left = Math.max(0, Math.ceil(left));
      right = Math.min(W - 1, Math.floor(right));
      // Opacidade do retângulo também vale (o preenchimento pode ser suave,
      // como o selo "DIAGRAMA" com 13% de tinta).
      const alpha = { ...c, a: Math.round(c.a * ri.opacity) };
      for (let x = left; x <= right; x++) setPx(x, y, alpha);
    }
  };

  // Preenche uma forma (flat ou gradiente vertical), todos os contornos juntos.
  const fillShape = (ptsArr: Pt[][], color: RGBA | Grad, opacity: number, anchorY: number, anchorH: number) => {
    const c0: RGBA | null = isFlat(color) ? color : null;
    fillScanlines(ptsArr, (yy) => {
      const cc: RGBA = c0 ?? gradAt(color as Grad, (yy - anchorY) / Math.max(1, anchorH));
      return { ...cc, a: Math.round(cc.a * opacity) };
    });
  };

  const strokeContour = (pts: Pt[], color: RGBA, opacity: number, width: number, closed: boolean) => {
    const r = width / 2;
    const cc = { ...color, a: Math.round(color.a * opacity) };
    // caps/junções redondos: círculos em cada vértice
    for (const p of pts) {
      const y0p = Math.max(0, Math.floor(p.y - r));
      const y1p = Math.min(H - 1, Math.ceil(p.y + r) - 1);
      for (let y = y0p; y <= y1p; y++) {
        const dy = y + 0.5 - p.y;
        const dx = Math.sqrt(Math.max(0, r * r - dy * dy));
        const x0p = Math.max(0, Math.ceil(p.x - dx));
        const x1p = Math.min(W - 1, Math.floor(p.x + dx));
        for (let x = x0p; x <= x1p; x++) setPx(x, y, cc);
      }
    }
    // segmentos como quads espessos (contorno fechado inclui o lado de volta)
    const last = pts.length - 1;
    for (let k = 0; k < last + (closed ? 1 : 0); k++) {
      const p = pts[k];
      const q = closed && k === last ? pts[0] : pts[k + 1];
      const dx = q.x - p.x;
      const dy = q.y - p.y;
      const len = Math.hypot(dx, dy);
      if (len < 1e-9) continue;
      const nx = (-dy / len) * r;
      const ny = (dx / len) * r;
      fillQuad(p.x + nx, p.y + ny, q.x + nx, q.y + ny, q.x - nx, q.y - ny, p.x - nx, p.y - ny, cc);
    }
  };

  const fillQuad = (ax: number, ay: number, bx: number, by: number, cx2: number, cy2: number, dx2: number, dy2: number, c: RGBA) => {
    let ymin = Math.min(ay, by, cy2, dy2);
    let ymax = Math.max(ay, by, cy2, dy2);
    const ys = Math.max(0, Math.floor(ymin));
    const ye = Math.min(H - 1, Math.ceil(ymax) - 1);
    const edges: [Pt, Pt][] = [
      [{ x: ax, y: ay }, { x: bx, y: by }],
      [{ x: bx, y: by }, { x: cx2, y: cy2 }],
      [{ x: cx2, y: cy2 }, { x: dx2, y: dy2 }],
      [{ x: dx2, y: dy2 }, { x: ax, y: ay }],
    ];
    const xsArr: number[] = [];
    for (let y = ys; y <= ye; y++) {
      const yy = y + 0.5;
      xsArr.length = 0;
      for (const [p, q] of edges) {
        if (Math.abs(q.y - p.y) < 1e-9) continue;
        if ((p.y <= yy && q.y > yy) || (q.y <= yy && p.y > yy)) {
          xsArr.push(p.x + (yy - p.y) * (q.x - p.x) / (q.y - p.y));
        }
      }
      xsArr.sort((a, b) => a - b);
      for (let i = 0; i + 1 < xsArr.length; i += 2) {
        const x0p = Math.max(0, Math.ceil(xsArr[i]));
        const x1p = Math.min(W - 1, Math.floor(xsArr[i + 1]));
        for (let x = x0p; x <= x1p; x++) setPx(x, y, c);
      }
    }
  };

  // ---- Parse: converte o SVG em instruções na ordem do documento ----------
  const instrs: Instr[] = [];
  const grads = new Map<string, Grad>();
  let curGrad: { id: string; a: Stop[]; b: Stop[] } | null = null;
  const tagRe = /<(\/)?(svg|defs|linearGradient|stop|rect|circle|ellipse|polygon|line|path|text)([^>]*)>/g;
  let m: RegExpExecArray | null;
  while ((m = tagRe.exec(svg))) {
    const close = !!m[1];
    const name = m[2];
    const body = m[3];
    if (name === "linearGradient" && !close) {
      const a = attrMap(body);
      curGrad = { id: a.get("id") ?? "", a: [], b: [] };
      continue;
    }
    if (name === "stop" && curGrad) {
      const a = attrMap(body);
      const c = hexColor(a.get("stop-color") ?? "");
      if (c) curGrad.a.push({ t: parseFloat(a.get("offset") ?? "0"), c });
      continue;
    }
    if (name === "linearGradient" && close && curGrad) {
      grads.set(curGrad.id, { a: curGrad.a, b: curGrad.a });
      curGrad = null;
      continue;
    }
    if (name === "rect") {
      const a = attrMap(body);
      const x = parseFloat(a.get("x") ?? "0") * SS;
      const y = parseFloat(a.get("y") ?? "0") * SS;
      const w = parseFloat(a.get("width") ?? "0") * SS;
      const h = parseFloat(a.get("height") ?? "0") * SS;
      const rx = parseFloat(a.get("rx") ?? "0") * SS;
      const fill = a.get("fill") ?? "";
      const strokeH = hexColor(a.get("stroke") ?? "");
      const color = fill.startsWith("url(#")
        ? (grads.get(fill.slice(5, -1)) ?? null)
        : hexColor(fill);
      if (grads.has(fill.slice(5, -1))) {
        instrs.push({
          kind: "rect",
          x, y, w, h, rx,
          color: grads.get(fill.slice(5, -1))!,
          opacity: parseFloat(a.get("opacity") ?? "1"),
          stroke: strokeH,
          strokeWidth: parseFloat(a.get("stroke-width") ?? "0") * SS,
        });
      } else if (color) {
        instrs.push({
          kind: "rect",
          x, y, w, h, rx,
          color,
          opacity: parseFloat(a.get("opacity") ?? "1"),
          stroke: strokeH,
          strokeWidth: parseFloat(a.get("stroke-width") ?? "0") * SS,
        });
      }
      continue;
    }
    if (name === "circle") {
      const a = attrMap(body);
      const cx2 = parseFloat(a.get("cx") ?? "0") * SS;
      const cy2 = parseFloat(a.get("cy") ?? "0") * SS;
      const r = parseFloat(a.get("r") ?? "0") * SS;
      const pts: Pt[] = [];
      for (let k = 0; k < 48; k++) {
        const ang = (k / 48) * Math.PI * 2;
        pts.push({ x: cx2 + r * Math.cos(ang), y: cy2 + r * Math.sin(ang) });
      }
      pushShape(a, [pts], grads) && undefined;
      continue;
    }
    if (name === "ellipse") {
      const a = attrMap(body);
      const cx2 = parseFloat(a.get("cx") ?? "0") * SS;
      const cy2 = parseFloat(a.get("cy") ?? "0") * SS;
      const rx = parseFloat(a.get("rx") ?? "0") * SS;
      const ry = parseFloat(a.get("ry") ?? "0") * SS;
      const pts: Pt[] = [];
      for (let k = 0; k < 48; k++) {
        const ang = (k / 48) * Math.PI * 2;
        pts.push({ x: cx2 + rx * Math.cos(ang), y: cy2 + ry * Math.sin(ang) });
      }
      pushShape(a, [pts], grads) && undefined;
      continue;
    }
    if (name === "polygon") {
      const a = attrMap(body);
      const pts: Pt[] = [];
      const re = /([-\d.]+)[, ]\s*([-\d.]+)/g;
      let pm: RegExpExecArray | null;
      while ((pm = re.exec(a.get("points") ?? ""))) {
        pts.push({ x: parseFloat(pm[1]) * SS, y: parseFloat(pm[2]) * SS });
      }
      pushShape(a, [pts], grads) && undefined;
      continue;
    }
    if (name === "line") {
      const a = attrMap(body);
      const strokeH = a.get("stroke") ? hexColor(a.get("stroke")!) : null;
      if (strokeH) {
        instrs.push({
          kind: "path",
          contours: [[
            { x: parseFloat(a.get("x1") ?? "0") * SS, y: parseFloat(a.get("y1") ?? "0") * SS },
            { x: parseFloat(a.get("x2") ?? "0") * SS, y: parseFloat(a.get("y2") ?? "0") * SS },
          ]],
          close: [false],
          fill: null,
          stroke: strokeH,
          strokeWidth: parseFloat(a.get("stroke-width") ?? "1") * SS,
          opacity: parseFloat(a.get("opacity") ?? "1"),
        });
      }
      continue;
    }
    if (name === "path") {
      const a = attrMap(body);
      const tf = parseTransform(a.get("transform"));
      const fl = flattenPathD(a.get("d") ?? "");
      // Converte para o espaço de trabalho (SS) — as formas já vêm * SS aqui,
      // então os paths (arestas/triângulos/texto) precisam do mesmo fator.
      const contours = fl.contours.map((c) =>
        applyTransform(c, tf).map((p) => ({ x: p.x * SS, y: p.y * SS })),
      );
      const fill = a.get("fill");
      const strokeH = a.get("stroke") ? hexColor(a.get("stroke")!) : null;
      instrs.push({
        kind: "path",
        contours,
        close: fl.closed,
        fill: fill === "none" || !fill ? null : fill.startsWith("url(#") ? (grads.get(fill.slice(5, -1)) ?? null) : hexColor(fill),
        stroke: strokeH,
        strokeWidth: parseFloat(a.get("stroke-width") ?? "0") * SS,
        opacity: parseFloat(a.get("opacity") ?? "1"),
      });
      continue;
    }
    // <text> só existe no fallback sem shaper — o caminho PNG sempre usa paths.
  }

  function pushShape(a: Map<string, string>, contours: Pt[][], g: Map<string, Grad>): boolean {
    const fill = a.get("fill") ?? "";
    const color = fill.startsWith("url(#") ? g.get(fill.slice(5, -1)) ?? null : hexColor(fill);
    if (!color) return false;
    const strokeH = a.get("stroke") ? hexColor(a.get("stroke")!) : null;
    instrs.push({
      kind: "shape",
      contours,
      color,
      opacity: parseFloat(a.get("opacity") ?? "1"),
      stroke: strokeH,
      strokeWidth: parseFloat(a.get("stroke-width") ?? "0") * SS,
    });
    return true;
  }

  // ---- Render: painter's algorithm ----------------------------------------
  for (const op of instrs) {
    if (op.kind === "rect") {
      fillRect(op);
      if (op.stroke && op.strokeWidth > 0) {
        strokeContour(roundedRectOutline(op.x, op.y, op.w, op.h, op.rx), op.stroke, op.opacity, op.strokeWidth, true);
      }
      continue;
    }
    if (op.kind === "shape") {
      const anchorY = minY(op.contours[0] ?? []);
      const anchorH = Math.max(1, boundsH(op.contours[0] ?? []));
      const flatC = isFlat(op.color) ? op.color : null;
      if (flatC) {
        fillScanlines(op.contours, () => ({ ...flatC, a: Math.round(flatC.a * op.opacity) }));
      } else {
        fillShape(op.contours, op.color as Grad, op.opacity, anchorY, anchorH);
      }
      if (op.stroke && op.strokeWidth > 0) strokeContour(op.contours[0], op.stroke, op.opacity, op.strokeWidth, true);
      continue;
    }
    // path
    if (op.fill) {
      const flatC = isFlat(op.fill) ? op.fill : null;
      if (flatC) {
        fillScanlines(op.contours, () => ({ ...flatC, a: Math.round(flatC.a * op.opacity) }));
      } else {
        fillShape(op.contours, op.fill as Grad, op.opacity, minY(op.contours[0] ?? []), Math.max(1, boundsH(op.contours[0] ?? [])));
      }
    }
    if (op.stroke && op.strokeWidth > 0) {
      for (let ci = 0; ci < op.contours.length; ci++) {
        strokeContour(op.contours[ci], op.stroke, op.opacity, op.strokeWidth, op.close[ci] ?? false);
      }
    }
  }

  // ---- Downsample 2x2 → PNG RGB ---------------------------------------------
  const pw = Math.round(designW * DS);
  const ph = Math.round(designH * DS);
  const px = new Uint8Array(pw * ph * 3);
  for (let y = 0; y < ph; y++) {
    for (let x = 0; x < pw; x++) {
      let r = 0;
      let g = 0;
      let b = 0;
      const bx = x * (SS / DS);
      const by = y * (SS / DS);
      for (let dy = 0; dy < SS / DS; dy++) {
        for (let dx = 0; dx < SS / DS; dx++) {
          const o = ((by + dy) * W + (bx + dx)) * 3;
          r += buf[o];
          g += buf[o + 1];
          b += buf[o + 2];
        }
      }
      const oo = (y * pw + x) * 3;
      px[oo] = (r / 4) | 0;
      px[oo + 1] = (g / 4) | 0;
      px[oo + 2] = (b / 4) | 0;
    }
  }
  return { width: pw, height: ph, rgba: px };
}

// Perímetro de um retângulo arredondado: arestas retas + arcos dos cantos,
// encadeados em um contorno fechado (para traçado).
function roundedRectOutline(x: number, y: number, w: number, h: number, rx: number): Pt[] {
  const r = Math.min(rx, w / 2, h / 2);
  const n = 28; // arcos suaves — sem facetamento visível no contorno
  const pts: Pt[] = [];
  const arc = (cx: number, cy: number, a0: number) => {
    for (let k = 0; k <= n; k++) {
      const ang = a0 + (Math.PI / 2) * (k / n);
      pts.push({ x: cx + Math.cos(ang) * r, y: cy + Math.sin(ang) * r });
    }
  };
  if (r <= 0) {
    pts.push({ x, y }, { x: x + w, y }, { x: x + w, y: y + h }, { x, y: y + h });
    return pts;
  }
  pts.push({ x: x + r, y }, { x: x + w - r, y });
  arc(x + w - r, y + r, -Math.PI / 2); // topo → direita
  pts.push({ x: x + w, y: y + h - r });
  arc(x + w - r, y + h - r, 0); // direita → baixo
  pts.push({ x: x + r, y: y + h });
  arc(x + r, y + h - r, Math.PI / 2); // baixo → esquerda
  pts.push({ x, y: y + r });
  arc(x + r, y + r, Math.PI); // esquerda → topo
  return pts;
}

function minY(pts: Pt[]): number {
  let ymin = Infinity;
  for (const p of pts) if (p.y < ymin) ymin = p.y;
  return ymin === Infinity ? 0 : ymin;
}

function maxY(pts: Pt[]): number {
  let ymax = -Infinity;
  for (const p of pts) if (p.y > ymax) ymax = p.y;
  return ymax === -Infinity ? 0 : ymax;
}

function boundsH(pts: Pt[]): number {
  return Math.max(0, maxY(pts) - minY(pts));
}

// ---------------------------------------------------------------------------
// Encoder PNG (24-bit RGB)
// ---------------------------------------------------------------------------
const CRC_TABLE = (() => {
  const t = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[n] = c >>> 0;
  }
  return t;
})();

function crc32(bytes: Uint8Array): number {
  let c = 0xffffffff;
  for (let i = 0; i < bytes.length; i++) c = CRC_TABLE[(c ^ bytes[i]) & 255] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

function chunk(type: string, data: Uint8Array): Uint8Array {
  const out = new Uint8Array(12 + data.length);
  const dv = new DataView(out.buffer);
  dv.setUint32(0, data.length);
  for (let i = 0; i < 4; i++) out[4 + i] = type.charCodeAt(i);
  out.set(data, 8);
  const crc = new Uint8Array(4);
  new DataView(crc.buffer).setUint32(0, crc32(out.subarray(4, 8 + data.length)));
  out.set(crc, 8 + data.length);
  return out;
}

export function encodePng(img: RasterImage): Uint8Array {
  const { width, height, rgba } = img;
  const raw = new Uint8Array(height * (1 + width * 3));
  for (let y = 0; y < height; y++) {
    const rowStart = y * (1 + width * 3);
    raw[rowStart] = 0; // filtro None
    raw.set(rgba.subarray(y * width * 3, (y + 1) * width * 3), rowStart + 1);
  }
  const idat = zlibSync(raw, { level: 9 });

  const ihdr = new Uint8Array(13);
  const d = new DataView(ihdr.buffer);
  d.setUint32(0, width);
  d.setUint32(4, height);
  ihdr[8] = 8; // bit depth
  ihdr[9] = 2; // color type RGB
  ihdr[10] = 0;
  ihdr[11] = 0;
  ihdr[12] = 0;

  const parts: Uint8Array[] = [
    new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", ihdr),
    chunk("IDAT", idat),
    chunk("IEND", new Uint8Array(0)),
  ];
  let len = 0;
  for (const p of parts) len += p.length;
  const out = new Uint8Array(len);
  let o = 0;
  for (const p of parts) {
    out.set(p, o);
    o += p.length;
  }
  return out;
}

/** Conveniência: SVG → PNG em uma chamada (mesma convenção de diagram-png). */
export function svgToPng(svg: string, designW: number, designH: number): Uint8Array | null {
  const img = renderSvg(svg, designW, designH);
  if (!img) return null;
  return encodePng(img);
}
