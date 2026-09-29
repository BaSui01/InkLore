/**
 * 正文纯文本规范与码点工具 (03分册§2、00契约基线§2)
 * 正文规范为 UTF-8 纯文本、LF 换行，字数统计 Unicode 码点（不含空白）。
 */

/** 统计不含空白字符的 Unicode 码点数量 */
export function countCodePoints(text: string): number {
  if (!text) return 0;
  let count = 0;
  for (const char of text) {
    if (!/\s/.test(char)) {
      count++;
    }
  }
  return count;
}

/** 换行符统一为 LF */
export function normalizeLf(text: string): string {
  return text.replace(/\r\n?/g, "\n");
}

/** 将 UTF-16 偏移转换为 Unicode 码点偏移 */
export function utf16ToCodePointOffset(text: string, utf16Offset: number): number {
  const boundedOffset = Math.max(0, Math.min(utf16Offset, text.length));
  const slice = text.slice(0, boundedOffset);
  return Array.from(slice).length;
}

/** 将 Unicode 码点偏移转换为 UTF-16 偏移 */
export function codePointToUtf16Offset(text: string, codePointOffset: number): number {
  const chars = Array.from(text);
  const boundedOffset = Math.max(0, Math.min(codePointOffset, chars.length));
  return chars.slice(0, boundedOffset).join("").length;
}

/** 本地草稿显式下载为纯文本文件 */
export function downloadEditorDraft(content: string, chapterId: string): void {
  const normalized = normalizeLf(content);
  const blob = new Blob([normalized], { type: "text/plain;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `${chapterId}.draft.txt`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
