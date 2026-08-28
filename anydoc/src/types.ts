export const DOCUMENT_FORMATS = ["md", "pdf", "docx", "xlsx", "pptx"] as const;
export type DocumentFormat = (typeof DOCUMENT_FORMATS)[number];

const FORMAT_EXT: Record<DocumentFormat, string> = {
  md: "md",
  pdf: "pdf",
  docx: "docx",
  xlsx: "xlsx",
  pptx: "pptx",
};
export { FORMAT_EXT };

export interface CreatedDocument {
  path: string;
  bytes: number;
  losses: number;
}
