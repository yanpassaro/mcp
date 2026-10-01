mod pii;
mod tabular;

use std::path::{Path, PathBuf};

use rmcp::{
    ServerHandler, ServiceExt,
    handler::server::{router::tool::ToolRouter, wrapper::Parameters},
    transport::stdio,
    tool, tool_handler, tool_router,
};
use schemars::JsonSchema;
use serde::Deserialize;

use anyhow::{Context, Result};

#[derive(Debug, Deserialize, JsonSchema)]
pub struct ReadArgs {
    #[schemars(description = "Path to the document to convert.")]
    pub path: String,
}

#[derive(Debug, Clone, Default)]
pub struct AnydocServer {
    tool_router: ToolRouter<Self>,
}

#[tool_router(router = tool_router)]
impl AnydocServer {
    #[tool(
        name = "anydoc_read",
        description = "Read a document and save it as Markdown next to the source (same base name). Supported formats: Word, PowerPoint, Excel, OpenDocument, RTF, EPUB, CSV, JSON, XML and PDF. PII is redacted. Returns the absolute output path."
    )]
    pub async fn read(&self, args: Parameters<ReadArgs>) -> String {
        let path = args.0.path.trim().to_string();
        match read_document(Path::new(&path)) {
            Ok(out) => out,
            Err(e) => format!("Erro: {:#}", e),
        }
    }
}

#[tool_handler(router = self.tool_router)]
impl ServerHandler for AnydocServer {}

fn read_document(path: &Path) -> Result<String> {
    let meta = std::fs::metadata(path)
        .with_context(|| format!("arquivo não encontrado: {}", path.display()))?;
    if !meta.is_file() {
        anyhow::bail!("não é um arquivo: {}", path.display());
    }

    let md = read_markdown(path)?;
    let out = sibling_with_ext(path, "md")?;
    if same_path(&out, path) {
        anyhow::bail!("a origem já é um .md: {}", path.display());
    }
    let out = if out.exists() {
        sibling_with_ext(path, "extraido.md")?
    } else {
        out
    };
    std::fs::write(&out, md).with_context(|| format!("gravar {}", out.display()))?;
    Ok(out.display().to_string())
}

fn read_markdown(path: &Path) -> Result<String> {
    let ext = extension_of(path);
    let bytes = std::fs::read(path).with_context(|| format!("ler {}", path.display()))?;
    let md = if ext == ".md" {
        String::from_utf8_lossy(&bytes).into_owned()
    } else if tabular::is_tabular(&ext) {
        tabular::tabular_to_markdown(&bytes, &ext)
    } else {
        anydoc::to_markdown_bytes(&bytes, None).map_err(|e| anyhow::anyhow!("{e}"))?
    };
    Ok(pii::redact_pii(&md))
}

fn extension_of(path: &Path) -> String {
    path.extension()
        .map(|e| format!(".{}", e.to_string_lossy().to_lowercase()))
        .unwrap_or_default()
}

fn sibling_with_ext(path: &Path, suffix: &str) -> Result<PathBuf> {
    let dir = path.parent().unwrap_or_else(|| Path::new("."));
    let stem = path
        .file_stem()
        .map(|s| s.to_string_lossy().into_owned())
        .unwrap_or_else(|| "documento".to_string());

    if let Some((head, tail)) = suffix.split_once('.') {
        return Ok(dir.join(format!("{}-{}{}", stem, head, format!(".{}", tail))));
    }
    Ok(dir.join(format!("{}.{}", stem, suffix)))
}

fn same_path(a: &Path, b: &Path) -> bool {
    let da = std::fs::canonicalize(a).unwrap_or_else(|_| a.to_path_buf());
    let db = std::fs::canonicalize(b).unwrap_or_else(|_| b.to_path_buf());
    let ka = da.to_string_lossy().to_lowercase();
    let kb = db.to_string_lossy().to_lowercase();
    ka == kb
}

#[tokio::main]
async fn main() -> Result<()> {
    let service = AnydocServer::default().serve(stdio()).await?;
    service.waiting().await?;
    Ok(())
}