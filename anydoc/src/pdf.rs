use anyhow::{Context, Result};
use printpdf::{
    BuiltinFont, Color, IndirectFontRef, Mm, Op, PdfDocument, PdfDocumentReference, Point,
    Positioning, TextItem,
};

use crate::markdown::{Block, ParsedMarkdown};
use crate::render::{pdf_safe_parsed, runs_text};

const PAGE_W: f32 = 210.0;
const PAGE_H: f32 = 297.0;
const MARGIN: f32 = 18.0;
const BODY_SIZE: f32 = 11.0;
const LEADING: f32 = 5.2;
const HEADING_SIZES: [f32; 6] = [20.0, 16.0, 14.0, 12.5, 11.5, 11.0];

pub struct Page {
    ops: Vec<Op>,
    y: f32,
}

impl Page {
    fn new() -> Self {
        Self {
            ops: Vec::new(),
            y: PAGE_H - MARGIN,
        }
    }

    fn break_if_needed(&mut self, needed: f32, pages: &mut Vec<Page>) {
        if self.y - needed < MARGIN {
            pages.push(std::mem::replace(self, Page::new()));
        }
    }
}

pub fn create_pdf(path: &std::path::Path, parsed: &ParsedMarkdown) -> Result<usize> {
    let parsed = pdf_safe_parsed(parsed);
    let (mut doc, pages) = build(&parsed)?;
    doc.add_pages(
        pages
            .iter()
            .map(|p| (Mm(PAGE_W), Mm(PAGE_H)), p.ops.as_slice()),
    );
    let mut writer = std::io::BufWriter::new(
        std::fs::File::create(path).with_context(|| format!("criar {}", path.display()))?,
    );
    doc.save(&mut writer).context("gravar PDF")?;
    Ok(std::fs::metadata(path)?.len() as usize)
}

fn build(parsed: &ParsedMarkdown) -> Result<(PdfDocumentReference, Vec<Page>)> {
    let mut doc = PdfDocument::new();
    let font = doc.add_font(BuiltinFont::Helvetica);
    let bold = doc.add_font(BuiltinFont::HelveticaBold);

    let mut pages: Vec<Page> = Vec::new();
    let mut page = Page::new();

    if let Some(title) = &parsed.title {
        write_wrapped(&mut page, &mut pages, title, 20.0, MARGIN, &font);
    }
    if let Some(subtitle) = &parsed.subtitle {
        write_wrapped(&mut page, &mut pages, subtitle, 14.0, MARGIN, &font);
    }

    for block in &parsed.blocks {
        match block {
            Block::Heading { text, level } => {
                let idx = (*level).clamp(1, 6) - 1;
                write_wrapped(&mut page, &mut pages, text, HEADING_SIZES[idx as usize], MARGIN, &bold);
            }
            Block::Paragraph { runs, indent, .. } => {
                let x = MARGIN + (*indent as f32) * 6.0;
                write_wrapped(&mut page, &mut pages, &runs_text(runs), BODY_SIZE, x, &font);
            }
            Block::List { items } => {
                for item in items {
                    write_wrapped(
                        &mut page,
                        &mut pages,
                        &format!("• {}", item),
                        BODY_SIZE,
                        MARGIN + 6.0,
                        &font,
                    );
                }
            }
            Block::Blockquote { items } => {
                for item in items {
                    write_wrapped(
                        &mut page,
                        &mut pages,
                        item,
                        BODY_SIZE,
                        MARGIN + 6.0,
                        &font,
                    );
                }
            }
            Block::CodeBlock { text, .. } => {
                for line in text.lines() {
                    write_wrapped(
                        &mut page,
                        &mut pages,
                        line,
                        BODY_SIZE - 1.0,
                        MARGIN + 6.0,
                        &font,
                    );
                }
            }
            Block::Image { text, .. } => {
                write_wrapped(
                    &mut page,
                    &mut pages,
                    &format!("[imagem: {}]", text),
                    BODY_SIZE,
                    MARGIN,
                    &font,
                );
            }
            Block::Definition { text, items } => {
                write_wrapped(&mut page, &mut pages, text, BODY_SIZE, MARGIN, &bold);
                for item in items {
                    write_wrapped(
                        &mut page,
                        &mut pages,
                        &format!("  {}", item),
                        BODY_SIZE - 1.0,
                        MARGIN + 6.0,
                        &font,
                    );
                }
            }
            Block::Break => {
                page.break_if_needed(LEADING * 2.0, &mut pages);
                page.y -= LEADING * 2.0;
            }
            Block::Table { columns, rows, .. } => {
                write_table(&mut page, &mut pages, columns, rows, &font, &bold);
            }
        }
    }

    pages.push(page);
    while pages.len() > 1 && pages[pages.len() - 1].ops.is_empty() {
        pages.pop();
    }
    Ok((doc, pages))
}

fn write_wrapped(
    page: &mut Page,
    pages: &mut Vec<Page>,
    text: &str,
    size: f32,
    x: f32,
    font: &IndirectFontRef,
) {
    let leading = if size > 14.0 { LEADING * 1.6 } else { LEADING };
    let max_chars = (((PAGE_W - MARGIN - x) / (size * 0.5)).floor() as usize).max(8);
    for chunk in wrap(text, max_chars) {
        page.break_if_needed(leading, pages);
        page.ops.push(Op::WriteText {
            font: font.clone(),
            size: Point(size),
            color: Color::Black,
            text: TextItem {
                text: chunk,
                positioning: Positioning {
                    x: Point(x),
                    y: Point(page.y),
                },
            },
            ..Default::default()
        });
        page.y -= leading;
    }
}

fn write_table(
    page: &mut Page,
    pages: &mut Vec<Page>,
    columns: &[String],
    rows: &[Vec<String>],
    font: &IndirectFontRef,
    bold: &IndirectFontRef,
) {
    let col_width = if columns.is_empty() {
        PAGE_W - 2.0 * MARGIN
    } else {
        (PAGE_W - 2.0 * MARGIN) / columns.len() as f32
    };
    let max_chars = ((col_width / ((BODY_SIZE - 1.0) * 0.5)).floor() as usize).max(4);

    let mut emit = |page: &mut Page, pages: &mut Vec<Page>, cells: &[String], f: &IndirectFontRef| {
        for (i, cell) in cells.iter().enumerate() {
            let x = MARGIN + col_width * i as f32;
            let text: String = if cell.chars().count() > max_chars {
                let cut = cell.chars().take(max_chars.saturating_sub(1)).collect::<String>();
                format!("{}…", cut)
            } else {
                cell.clone()
            };
            page.break_if_needed(LEADING, pages);
            page.ops.push(Op::WriteText {
                font: f.clone(),
                size: Point(BODY_SIZE - 1.0),
                color: Color::Black,
                text: TextItem {
                    text,
                    positioning: Positioning {
                        x: Point(x),
                        y: Point(page.y),
                    },
                },
                ..Default::default()
            });
        }
        page.y -= LEADING;
    };

    emit(page, pages, columns, bold);
    for row in rows {
        let cells: Vec<String> = columns
            .iter()
            .enumerate()
            .map(|(i, _)| row.get(i).cloned().unwrap_or_default())
            .collect();
        emit(page, pages, &cells, font);
    }
    page.y -= LEADING;
}

fn wrap(text: &str, max_chars: usize) -> Vec<String> {
    if text.trim().is_empty() {
        return vec![String::new()];
    }
    let mut out: Vec<String> = Vec::new();
    let mut current = String::new();
    for word in text.split_whitespace() {
        if current.is_empty() {
            current.push_str(word);
            continue;
        }
        if current.chars().count() + 1 + word.chars().count() <= max_chars {
            current.push(' ');
            current.push_str(word);
            continue;
        }
        out.push(std::mem::take(&mut current));
        current.push_str(word);
    }
    if !current.is_empty() {
        out.push(current);
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wraps_long_text() {
        let out = wrap("aaa bbb ccc ddd", 8);
        assert!(out.len() > 1);
        assert!(out.iter().all(|l| l.chars().count() <= 8));
    }

    #[test]
    fn builds_a_pdf_file() {
        let path = std::env::temp_dir().join("anydoc-test-pdf.pdf");
        let parsed = crate::markdown::parse_markdown("# T\n\n|a|b|\n|---|---|\n|1|2|\n");
        let n = create_pdf(&path, &parsed).expect("pdf");
        assert!(n > 0);
        let _ = std::fs::remove_file(path);
    }
}
