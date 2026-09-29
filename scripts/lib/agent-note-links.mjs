// Dependency-free Markdown destinations; code fences and inline code are ignored.
import path from 'node:path';
import { lstat, realpath } from 'node:fs/promises';
export const normalize = text => text.replace(/\r\n?/g, '\n');
export const samePath = (left, right) => typeof left === 'string' && typeof right === 'string'
  && (process.platform === 'win32' ? left.toLowerCase() === right.toLowerCase() : left === right);
export const within = (root, file) => {
  const rel = path.relative(root, file);
  return rel !== '..' && !rel.startsWith('..' + path.sep) && !path.isAbsolute(rel);
};
const blank = text => text.replace(/[^\n]/g, ' ');
export function prose(source, inline = false) {
  let fence;
  const text = normalize(source).split('\n').map(line => {
    const candidate = line.replace(/^\s*(?:>\s*)+/, '').replace(/^\s*[-+*]\s+/, '').trimStart();
    const marker = /^(\x60{3,}|~{3,})(.*)$/.exec(candidate);
    if (fence) {
      if (marker && marker[1][0] === fence[0] && marker[1].length >= fence.length && !marker[2].trim()) fence = null;
      return blank(line);
    }
    if (marker && (marker[1][0] !== '\x60' || !marker[2].includes('\x60'))) { fence = marker[1]; return blank(line); }
    return line;
  }).join('\n').replace(/<!--[\s\S]*?-->/g, blank);
  return inline ? text.replace(/(\x60+)([^\x60]|(?!\1)\x60)*?\1/g, blank) : text;
}
const escaped = (text, index) => {
  let count = 0;
  while (index > 0 && text[--index] === '\\') count++;
  return count % 2 === 1;
};
function destination(text, start) {
  let index = start;
  while (index < text.length && /\s/.test(text[index])) index++;
  if (text[index] === '<') {
    for (let end = index + 1; end < text.length && text[end] !== '\n'; end++) {
      if (text[end] === '>' && !escaped(text, end)) return { value: text.slice(index + 1, end), end: end + 1 };
    }
    return null;
  }
  const begin = index;
  let depth = 0;
  while (index < text.length) {
    const char = text[index];
    if (char === '\\' && index + 1 < text.length) { index += 2; continue; }
    if (/\s/.test(char)) break;
    if (char === '(') depth++;
    if (char === ')') { if (!depth) break; depth--; }
    index++;
  }
  return depth === 0 ? { value: text.slice(begin, index), end: index } : null;
}
export function destinations(source) {
  let text = prose(source, true);
  const result = [], ranges = [];
  for (const match of text.matchAll(/^ {0,3}\[((?:\\.|[^\]\\\n])+)\]:[ \t]*(?:\n[ \t]*)?/gm)) {
    const found = destination(text, match.index + match[0].length);
    if (found) {
      result.push(found.value);
      const end = text.indexOf('\n', found.end);
      ranges.push([match.index, end < 0 ? text.length : end]);
    }
  }
  for (const [start, end] of ranges.reverse()) text = text.slice(0, start) + blank(text.slice(start, end)) + text.slice(end);
  const stack = [];
  for (let index = 0; index < text.length; index++) {
    if (escaped(text, index)) continue;
    if (text[index] === '[') stack.push(index);
    if (text[index] !== ']' || !stack.length) continue;
    stack.pop();
    if (text[index + 1] !== '(') continue;
    const found = destination(text, index + 2);
    if (!found || !/^(?:\s+(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|\((?:[^)\\]|\\.)*\)))?\s*\)/.test(text.slice(found.end))) continue;
    result.push(found.value);
    index = found.end - 1;
  }
  return result;
}
export function resolveLink(root, file, raw) {
  let url = raw.replace(/\\([!"#$%&'()*+,\-./:;<=>?@[\]\\^_\x60{|}~])/g, '$1')
    .replace(/&amp;/gi, '&').replace(/&#(x[\da-f]+|\d+);/gi, (match, code) => {
      const n = code[0].toLowerCase() === 'x' ? parseInt(code.slice(1), 16) : Number(code);
      return n > 0 && n <= 0x10ffff ? String.fromCodePoint(n) : match;
    });
  if (!url || /^[#?]/.test(url) || url.startsWith('//')) return null;
  if (/^[a-z]:[\\/]/i.test(url) || url.startsWith('\\\\')) throw new Error('absolute filesystem link');
  if (/^[a-z][a-z0-9+.-]*:/i.test(url)) return null;
  try { url = decodeURIComponent(url.split(/[?#]/, 1)[0]).replace(/\\/g, '/'); }
  catch { throw new Error('invalid URL encoding'); }
  if (url.includes('\0') || path.win32.isAbsolute(url) || path.posix.isAbsolute(url)) throw new Error('link must be relative to its Markdown file');
  const target = path.resolve(path.dirname(file), url);
  if (!within(root, target)) throw new Error('link escapes repository');
  return target;
}
// Reject every symlink component, including directory junctions on Windows.
export async function safePath(root, file, missing = false) {
  if (!within(root, file)) throw new Error('path escapes repository');
  let current = root;
  for (const part of path.relative(root, file).split(path.sep).filter(Boolean)) {
    current = path.join(current, part);
    try { if ((await lstat(current)).isSymbolicLink()) throw new Error('symlink is forbidden'); }
    catch (error) { if (missing && error.code === 'ENOENT') return; throw error; }
  }
  if (!within(await realpath(root), await realpath(file))) throw new Error('path resolves outside repository');
}
export async function checkLinks(root, file, text, absent) {
  for (const raw of destinations(text)) {
    const target = resolveLink(root, file, raw);
    if (!target) continue;
    if (samePath(target, absent)) throw new Error('link would target the removed implemented note');
    try { await safePath(root, target); }
    catch { throw new Error('invalid, missing or symlink local link target (destination omitted)'); }
  }
}
