// Package content owns novels, volumes, chapters, immutable body versions,
// chapter import/export jobs (TXT/Markdown/DOCX/EPUB3), full-text search and
// Beijing-time writing statistics.
// R1 transport calls application ports; infrastructure accesses only content tables
// through the shared opaque UnitOfWork.
package content
