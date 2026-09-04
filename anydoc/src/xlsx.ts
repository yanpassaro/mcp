import ExcelJS from "exceljs";
import { saveFile } from "./save.ts";
import type { ParsedMarkdown } from "./markdown.ts";
import type { CreatedDocument } from "./types.ts";

interface XlsxSheet {
  name?: string;
  columns: string[];
  widths?: number[];
  rows: unknown[][];
  style?: {
    headerBackground?: string;
    headerColor?: string;
    striped?: boolean;
    borders?: boolean;
    formats?: Record<string, string>;
    total?: boolean;
    freezeHeader?: boolean;
  };
}

function argb(color?: string): string | undefined {
  if (!color) return undefined;
  const h = color.trim().replace(/^#/, "");
  return h.length === 6 ? `FF${h.toUpperCase()}` : h.toUpperCase();
}

function thinBorder(color: string) {
  return { style: "thin" as const, color: { argb: color } };
}

function safeSheetName(raw: string, fallback: string): string {
  const clean = raw.replace(/[\\/:*?[\]]/g, "").trim().slice(0, 31);
  return clean || fallback;
}

function buildSheets(parsed: ParsedMarkdown): XlsxSheet[] {
  if (parsed.tables.length > 0) {
    return parsed.tables.map((t, i) => ({
      name: safeSheetName(t.columns[0] ?? `Sheet ${i + 1}`, `Sheet ${i + 1}`),
      columns: t.columns,
      rows: t.rows,
      style: {
        headerBackground: "#17171c",
        headerColor: "#ffffff",
        striped: true,
        borders: true,
        freezeHeader: true,
      },
    }));
  }

  const rows = parsed.blocks
    .filter((b) =>
      b.kind === "paragraph" || b.kind === "heading" || b.kind === "list" ||
      b.kind === "codeblock"
    )
    .map((b) => {
      if (b.kind === "list") return [(b.items ?? []).join("; ")];
      if (b.kind === "heading") return [b.text ?? ""];
      if (b.kind === "codeblock") return [b.text ?? ""];
      return [(b.runs ?? []).map((r) => r.text ?? "").join("")];
    });
  return [{
    name: safeSheetName(parsed.title ?? "Content", "Content"),
    columns: ["Text"],
    rows,
    style: { borders: true },
  }];
}

export async function createXlsx(
  path: string,
  parsed: ParsedMarkdown,
): Promise<CreatedDocument> {
  const sheets = buildSheets(parsed);
  const wb = new ExcelJS.Workbook();
  wb.creator = "Pizinho";

  sheets.forEach((sheet, si) => {
    const columns = Array.isArray(sheet.columns)
      ? sheet.columns.map(String)
      : [];
    const rows = Array.isArray(sheet.rows) ? sheet.rows : [];
    if (columns.length === 0) return;
    const style = sheet.style ?? {};
    const sheetName = (sheet.name || `Sheet ${si + 1}`).slice(0, 31);
    const ws = wb.addWorksheet(sheetName);

    const widths = Array.isArray(sheet.widths) ? sheet.widths : [];
    columns.forEach((_col, i) => {
      ws.getColumn(i + 1).width = Number(widths[i]) || 16;
    });

    const headerBackground = argb(style.headerBackground);
    const headerColor = argb(style.headerColor);
    const zebraBackground = "FFF2F2F4";
    const border = style.borders ? thinBorder("FFC9C9CF") : undefined;

    columns.forEach((col, ci) => {
      const cell = ws.getCell(1, ci + 1);
      cell.value = col;
      if (headerBackground) {
        cell.fill = {
          type: "pattern",
          pattern: "solid",
          fgColor: { argb: headerBackground },
        };
      }
      cell.font = {
        bold: true,
        ...(headerColor ? { color: { argb: headerColor } } : {}),
      };
      if (border) {
        cell.border = {
          top: border,
          right: border,
          bottom: border,
          left: border,
        };
      }
    });
    ws.getRow(1).height = 22;

    rows.forEach((rowData, ri) => {
      const row = ws.getRow(ri + 2);
      const zebra = style.striped && ri % 2 === 1;
      columns.forEach((col, ci) => {
        const cell = row.getCell(ci + 1);
        const v = (rowData as unknown[])[ci];
        cell.value = v === null || v === undefined
          ? null
          : (v as string | number | boolean | null | Date);
        if (zebra) {
          cell.fill = {
            type: "pattern",
            pattern: "solid",
            fgColor: { argb: zebraBackground },
          };
        }
        if (border) {
          cell.border = {
            top: border,
            right: border,
            bottom: border,
            left: border,
          };
        }
        const fmt = style.formats?.[col];
        if (fmt) cell.numFmt = fmt;
      });
    });

    if (style.total && rows.length > 0) {
      const totalRow = ws.getRow(rows.length + 2);
      columns.forEach((col, ci) => {
        const cell = totalRow.getCell(ci + 1);
        let sum = 0;
        let numeric = true;
        for (const rowData of rows) {
          const v = (rowData as unknown[])[ci];
          if (typeof v === "number") {
            sum += v;
          } else if (v !== null && v !== undefined && v !== "") {
            numeric = false;
            break;
          }
        }
        cell.value = ci === 0 ? "TOTAL" : numeric ? sum : "";
        cell.font = {
          bold: true,
          ...(headerColor ? { color: { argb: headerColor } } : {}),
        };
        if (headerBackground) {
          cell.fill = {
            type: "pattern",
            pattern: "solid",
            fgColor: { argb: headerBackground },
          };
        }
        const fmt = style.formats?.[col];
        if (fmt && numeric) cell.numFmt = fmt;
        if (border) {
          cell.border = {
            top: border,
            right: border,
            bottom: border,
            left: border,
          };
        }
      });
    }

    if (style.freezeHeader !== false) {
      ws.views = [{ state: "frozen", ySplit: 1 }];
    }
    if (rows.length > 0) {
      ws.autoFilter = {
        from: { row: 1, column: 1 },
        to: { row: rows.length + 1, column: columns.length },
      };
    }
  });

  const buf = await wb.xlsx.writeBuffer();
  return saveFile(path, new Uint8Array(buf));
}
