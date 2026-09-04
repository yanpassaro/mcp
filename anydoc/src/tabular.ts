export const TABULAR_EXTS = [".csv", ".tsv", ".json", ".xml", ".html", ".htm"];

export function isTabular(ext: string): boolean {
  return TABULAR_EXTS.includes(ext.toLowerCase());
}

function cleanCell(raw: string): string {
  return raw.replace(/\|/g, "¦").replace(/\r?\n/g, " ").trim();
}

function mdTable(columns: string[], rows: string[][]): string {
  const header = "| " + columns.map(cleanCell).join(" | ") + " |";
  const sep = "| " + columns.map(() => "---").join(" | ") + " |";
  const body = rows.map((r) => {
    const cells = columns.map((_c, i) => cleanCell(String(r[i] ?? "")));
    return "| " + cells.join(" | ") + " |";
  });
  return [header, sep, ...body].join("\n");
}

function parseCSV(text: string): string[][] {
  const rows: string[][] = [];
  let field = "";
  let row: string[] = [];
  let inQuotes = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (inQuotes) {
      if (c === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += c;
      }
    } else if (c === '"') {
      inQuotes = true;
    } else if (c === ",") {
      row.push(field);
      field = "";
    } else if (c === "\n" || c === "\r") {
      if (c === "\r" && text[i + 1] === "\n") i++;
      row.push(field);
      field = "";
      if (row.some((x) => x !== "")) rows.push(row);
      row = [];
    } else {
      field += c;
    }
  }
  if (field !== "" || row.length) {
    row.push(field);
    if (row.some((x) => x !== "")) rows.push(row);
  }
  return rows.filter((r) => r.some((x) => x !== ""));
}

function parseTSV(text: string): string[][] {
  return text
    .split(/\r?\n/)
    .map((l) => l.split("\t"))
    .filter((r) => r.some((x) => x !== ""));
}

function parseJSONTable(text: string): string {
  const data = JSON.parse(text);
  if (Array.isArray(data)) {
    if (data.length === 0) return "";
    const first = data[0];
    if (first && typeof first === "object" && !Array.isArray(first)) {
      const columns = Array.from(
        new Set(data.flatMap((o) => Object.keys(o as Record<string, unknown>))),
      );
      const rows = data.map((o) =>
        columns.map((c) => String((o as Record<string, unknown>)[c] ?? ""))
      );
      return mdTable(columns, rows);
    }
    if (Array.isArray(first)) {
      const columns = first.map((_: unknown, i: number) => `col${i + 1}`);
      const rows = data.map((r) =>
        (r as unknown[]).map((v) => String(v ?? ""))
      );
      return mdTable(columns, rows);
    }
    return mdTable(["value"], data.map((v) => [String(v ?? "")]));
  }
  if (typeof data === "object" && data !== null) {
    const columns = Object.keys(data);
    return mdTable(columns, [
      columns.map((c) => String((data as Record<string, unknown>)[c] ?? "")),
    ]);
  }
  return "";
}

function stripTags(html: string): string {
  return html
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<[^>]+>/g, "")
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/gi, "'")
    .trim();
}

function parseHTMLTables(text: string): string[] {
  const tables: string[] = [];
  const re = /<table[\s\S]*?<\/table>/gi;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    const rows: string[][] = [];
    const trRe = /<tr[\s\S]*?<\/tr>/gi;
    let tr: RegExpExecArray | null;
    let header: string[] | null = null;
    while ((tr = trRe.exec(m[0])) !== null) {
      const cells: string[] = [];
      const cellRe = /<(th|td)[^>]*>([\s\S]*?)<\/\1>/gi;
      let c: RegExpExecArray | null;
      while ((c = cellRe.exec(tr[0])) !== null) {
        cells.push(stripTags(c[2]));
      }
      if (cells.length === 0) continue;
      if (/<th/i.test(tr[0]) && header === null) {
        header = cells;
      } else {
        rows.push(cells);
      }
    }
    if (header) tables.push(mdTable(header, rows));
    else if (rows.length) tables.push(mdTable(rows[0], rows.slice(1)));
  }
  return tables;
}

function parseXMLTable(text: string): string {
  const counts = new Map<string, number>();
  const tagRe = /<([A-Za-z_][\w.-]*)(?:\s[^>]*)?>[\s\S]*?<\/\1>/g;
  let m: RegExpExecArray | null;
  while ((m = tagRe.exec(text)) !== null) {
    counts.set(m[1], (counts.get(m[1]) ?? 0) + 1);
  }
  let rowTag = "";
  let best = 1;
  for (const [tag, n] of counts) {
    if (n > best) {
      best = n;
      rowTag = tag;
    }
  }
  if (!rowTag) return "";

  const rows: string[][] = [];
  const cols = new Set<string>();
  const re = new RegExp(`<${rowTag}([^>]*)>([\\s\\S]*?)<\\/${rowTag}>`, "g");
  let rm: RegExpExecArray | null;
  while ((rm = re.exec(text)) !== null) {
    const attrs = rm[1] ?? "";
    const inner = rm[2] ?? "";
    const fields = new Map<string, string>();
    const attrRe = /([A-Za-z_][\w.-]*)\s*=\s*["']([^"']*)["']/g;
    let am: RegExpExecArray | null;
    while ((am = attrRe.exec(attrs)) !== null) {
      fields.set(am[1], am[2]);
      cols.add(am[1]);
    }
    const childRe = /<([A-Za-z_][\w.-]*)(?:\s[^>]*)?>([\s\S]*?)<\/\1>/g;
    let cm: RegExpExecArray | null;
    while ((cm = childRe.exec(inner)) !== null) {
      fields.set(cm[1], stripTags(cm[2]));
      cols.add(cm[1]);
    }
    if (cols.size === 0) continue;
    const colList = Array.from(cols);
    rows.push(colList.map((c) => fields.get(c) ?? ""));
  }
  if (rows.length === 0) return "";
  return mdTable(Array.from(cols), rows);
}

export function tabularToMarkdown(bytes: Uint8Array, ext: string): string {
  const text = new TextDecoder().decode(bytes);
  const e = ext.toLowerCase();
  if (e === ".csv") {
    const rows = parseCSV(text);
    if (rows.length === 0) return "";
    return mdTable(rows[0], rows.slice(1));
  }
  if (e === ".tsv") {
    const rows = parseTSV(text);
    if (rows.length === 0) return "";
    return mdTable(rows[0], rows.slice(1));
  }
  if (e === ".json") return parseJSONTable(text);
  if (e === ".html" || e === ".htm") return parseHTMLTables(text).join("\n\n");
  if (e === ".xml") return parseXMLTable(text);
  return "";
}
