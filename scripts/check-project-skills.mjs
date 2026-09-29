#!/usr/bin/env node
// INK-142: dependency-free, deliberately scoped project-skill validation.
import { readdir, readFile, realpath, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const skillsRoot = path.join(root, '.agents', 'skills');
const requiredNames = ['inklore-development', 'inklore-decisions', 'inklore-maintenance'];
const errors = [];
const names = new Map();
const skillFiles = new Set();
let fileCount = 0;
let linkCount = 0;
let realRoot;
const label = (file) => path.relative(root, file).split(path.sep).join('/') || '.';
const issue = (file, line, message) => errors.push(
  label(file) + (line ? ':' + line : '') + ': ' + message,
);
const within = (base, file) => {
  const relative = path.relative(base, file);
  return relative !== '..' && !relative.startsWith('..' + path.sep) && !path.isAbsolute(relative);
};
const linesOf = (text) => {
  const lines = text.replace(/\r\n?/g, '\n').split('\n');
  if (lines.at(-1) === '') lines.pop();
  return lines;
};
const lineAt = (text, offset) => text.slice(0, offset).split('\n').length;
const blank = (text) => text.replace(/[^\n]/g, ' ');

async function collect(directory, ancestors = new Set()) {
  try {
    const actual = await realpath(directory);
    if (!within(realRoot, actual)) {
      issue(directory, 0, 'skill directory resolves outside the repository');
      return;
    }
    if (ancestors.has(actual)) {
      issue(directory, 0, 'cyclic directory symlink');
      return;
    }
    const branch = new Set([...ancestors, actual]);
    const entries = await readdir(directory, { withFileTypes: true });
    entries.sort((a, b) => a.name.localeCompare(b.name));
    for (const entry of entries) {
      const file = path.join(directory, entry.name);
      try {
        const resolved = await realpath(file);
        if (!within(realRoot, resolved)) {
          issue(file, 0, 'skill entry resolves outside the repository');
          continue;
        }
        const info = await stat(file);
        if (info.isDirectory()) await collect(file, branch);
        else if (info.isFile() && /\.md$/i.test(entry.name)) skillFiles.add(file);
      } catch (error) {
        issue(file, 0, 'cannot inspect skill entry: ' + error.message);
      }
    }
  } catch (error) {
    issue(directory, 0, 'cannot read skill directory: ' + error.message);
  }
}

// Only single-line YAML strings are accepted; block, collection, tagged and
// implicitly typed scalars are intentionally rejected without a YAML dependency.
function scalar(source) {
  const value = source.trim();
  if (value.startsWith("'")) {
    const match = /^'((?:[^']|'')*)'\s*(?:#.*)?$/.exec(value);
    if (!match) throw new Error('invalid single-quoted string');
    return match[1].replace(/''/g, "'");
  }
  if (value.startsWith('"')) {
    const match = /^"((?:[^"\\]|\\.)*)"\s*(?:#.*)?$/.exec(value);
    if (!match) throw new Error('invalid double-quoted string');
    const escapes = { '0': '\0', a: '\x07', b: '\b', t: '\t', n: '\n', v: '\v', f: '\f', r: '\r', e: '\x1b', ' ': ' ', '"': '"', '/': '/', '\\': '\\', N: '\u0085', _: '\u00a0', L: '\u2028', P: '\u2029' };
    return match[1].replace(/\\(x[0-9a-fA-F]{2}|u[0-9a-fA-F]{4}|U[0-9a-fA-F]{8}|.)/g, (_, code) => {
      if (/^[xuU]/.test(code) && code.length > 1) return String.fromCodePoint(parseInt(code.slice(1), 16));
      if (Object.hasOwn(escapes, code)) return escapes[code];
      throw new Error('unsupported YAML escape: \\' + code);
    });
  }
  const plain = value.replace(/\s+#.*$/, '').trim();
  if (!plain || /^[\[\]{}&*!|>'"%@\x60#]/.test(plain) || /^[-?:](?:\s|$)/.test(plain)
      || /:\s/.test(plain) || /^(?:null|~|true|false|yes|no|on|off|[-+]?\.(?:inf|nan))$/i.test(plain)
      || /^[-+]?(?:0x[\da-f]+|0o[0-7]+|0b[01]+|(?:\d[\d_]*(?:\.[\d_]*)?|\.[\d_]+)(?:e[-+]?\d+)?)$/i.test(plain)
      || /^\d{4}-\d{2}-\d{2}(?:$|[Tt \t])/.test(plain)) {
    throw new Error('expected a simple YAML string (quote ambiguous values)');
  }
  return plain;
}

function metadata(file, text) {
  const lines = linesOf(text);
  if (lines[0] !== '---') {
    issue(file, 1, 'missing opening YAML front matter delimiter ---');
    return;
  }
  const end = lines.findIndex((line, index) => index > 0 && /^(?:---|\.\.\.)\s*$/.test(line));
  if (end < 0) {
    issue(file, 1, 'missing closing YAML front matter delimiter');
    return;
  }
  const fields = new Map();
  for (let index = 1; index < end; index++) {
    const match = /^(name|description)\s*:(?:\s+(.*)|$)/.exec(lines[index]);
    if (!match) continue;
    const key = match[1];
    if (fields.has(key)) issue(file, index + 1, 'duplicate YAML field ' + key);
    try {
      fields.set(key, { value: scalar(match[2] ?? ''), line: index + 1 });
    } catch (error) {
      fields.set(key, { value: '', line: index + 1 });
      issue(file, index + 1, key + ': ' + error.message);
    }
  }
  for (const key of ['name', 'description']) {
    if (!fields.has(key)) issue(file, 1, 'missing simple YAML string field ' + key);
    else if (!fields.get(key).value.trim()) issue(file, fields.get(key).line, key + ' must not be empty');
  }
  const name = fields.get('name');
  if (name?.value) {
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(name.value)) issue(file, name.line, 'name must be lowercase kebab-case');
    if (name.value !== path.basename(path.dirname(file))) issue(file, name.line, 'name must equal its containing directory name');
    if (names.has(name.value)) issue(file, name.line, 'duplicate skill name ' + name.value + ' (also ' + label(names.get(name.value)) + ')');
    else names.set(name.value, file);
  }
  const bodyLines = lines.length - end - 1;
  if (bodyLines >= 500) issue(file, end + 2, 'skill body has ' + bodyLines + ' lines; must be < 500');
}

// Mask code without deleting newlines or changing offsets used in diagnostics.
function prose(text) {
  let fence = null;
  return text.split('\n').map((line) => {
    // Also recognize fences inside block quotes and ordinary list indentation.
    const candidate = line.replace(/^\s*(?:>\s*)+/, '').replace(/^\s*[-+*]\s+/, '').trimStart();
    const marker = /^(\x60{3,}|~{3,})(.*)$/.exec(candidate);
    if (fence) {
      if (marker && marker[1][0] === fence.char && marker[1].length >= fence.length && !marker[2].trim()) fence = null;
      return blank(line);
    }
    if (marker && (marker[1][0] !== '\x60' || !marker[2].includes('\x60'))) {
      fence = { char: marker[1][0], length: marker[1].length };
      return blank(line);
    }
    return line;
  }).join('\n').replace(/(\x60+)([^\x60]|(?!\1)\x60)*?\1/g, blank);
}

const escaped = (text, index) => {
  let count = 0;
  while (index > 0 && text[--index] === '\\') count++;
  return count % 2 === 1;
};

// Read angle-bracket destinations or unquoted destinations with balanced
// parentheses and Markdown escapes. Optional titles are outside the destination.
function destination(text, start) {
  let index = start;
  while (/\s/.test(text[index] ?? '') && index < text.length) index++;
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
    if (char === ')') {
      if (depth === 0) break;
      depth--;
    }
    index++;
  }
  return depth === 0 ? { value: text.slice(begin, index), end: index } : null;
}

async function checkLink(file, text, offset, raw) {
  let url = raw.replace(/\\([!"#$%&'()*+,\-./:;<=>?@[\]\\^_\x60{|}~])/g, '$1')
    .replace(/&amp;/gi, '&').replace(/&#(x[\da-f]+|\d+);/gi, (match, code) => {
      const value = code[0].toLowerCase() === 'x' ? parseInt(code.slice(1), 16) : Number(code);
      return value > 0 && value <= 0x10ffff ? String.fromCodePoint(value) : match;
    });
  if (!url || url.startsWith('#') || url.startsWith('?') || url.startsWith('//')) return;
  const line = lineAt(text, offset);
  if (/^[a-z]:[\\/]/i.test(url) || url.startsWith('\\\\')) {
    issue(file, line, 'absolute filesystem link is not repository-relative: ' + raw);
    return;
  }
  if (/^[a-z][a-z0-9+.-]*:/i.test(url)) return;
  linkCount++;
  try {
    url = decodeURIComponent(url.split(/[?#]/, 1)[0]);
    // Backslashes are separators on every host, so Windows-style traversal
    // cannot silently pass when validation runs on Linux.
    url = url.replace(/\\/g, '/');
    if (url.includes('\0') || path.win32.isAbsolute(url) || path.posix.isAbsolute(url)) {
      issue(file, line, 'link must be relative to its Markdown file: ' + raw);
      return;
    }
    const target = path.resolve(path.dirname(file), url);
    if (!within(root, target)) {
      issue(file, line, 'link escapes the repository: ' + raw);
      return;
    }
    const actual = await realpath(target);
    if (!within(realRoot, actual)) issue(file, line, 'link resolves outside the repository: ' + raw);
    else await stat(target); // Both files and directories are valid targets.
  } catch (error) {
    issue(file, line, 'invalid or missing link target ' + JSON.stringify(raw) + ': ' + error.message);
  }
}

async function links(file, source) {
  let text = prose(source);
  // Reference definitions are checked once each, even if reused by many links.
  const definitions = /^ {0,3}\[((?:\\.|[^\]\\\n])+)\]:[ \t]*(?:\n[ \t]*)?/gm;
  const ranges = [];
  for (const match of text.matchAll(definitions)) {
    const found = destination(text, match.index + match[0].length);
    if (found) {
      await checkLink(file, text, match.index, found.value);
      const end = text.indexOf('\n', found.end);
      ranges.push([match.index, end < 0 ? text.length : end]);
    }
  }
  for (const [start, end] of ranges.reverse()) text = text.slice(0, start) + blank(text.slice(start, end)) + text.slice(end);
  // A bracket stack permits nested labels, images and escaped brackets.
  const stack = [];
  for (let index = 0; index < text.length; index++) {
    if (escaped(text, index)) continue;
    if (text[index] === '[') stack.push(index);
    if (text[index] !== ']' || !stack.length) continue;
    const start = stack.pop();
    if (text[index + 1] !== '(') continue;
    const found = destination(text, index + 2);
    if (!found) continue;
    const tail = text.slice(found.end);
    if (!/^(?:\s+(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|\((?:[^)\\]|\\.)*\)))?\s*\)/.test(tail)) continue;
    await checkLink(file, text, start, found.value);
    index = found.end - 1;
  }
}

async function scan(file, isSkill) {
  try {
    const actual = await realpath(file);
    if (!within(realRoot, actual)) {
      issue(file, 0, 'Markdown file resolves outside the repository');
      return;
    }
    const bytes = await readFile(file);
    fileCount++;
    const text = bytes.toString('utf8').replace(/^\uFEFF/, '').replace(/\r\n?/g, '\n');
    if (isSkill) {
      if (bytes.length >= 50000) issue(file, 0, 'Markdown file is ' + bytes.length + ' bytes; must be < 50000');
      for (const match of text.matchAll(/\p{Extended_Pictographic}/gu)) {
        issue(file, lineAt(text, match.index), 'forbidden Unicode Extended_Pictographic U+' + match[0].codePointAt(0).toString(16).toUpperCase());
      }
      if (path.basename(file) === 'SKILL.md') metadata(file, text);
    }
    await links(file, text);
  } catch (error) {
    issue(file, 0, 'cannot scan Markdown file: ' + error.message);
  }
}

try {
  realRoot = await realpath(root);
  await collect(skillsRoot);
  for (const file of skillFiles) await scan(file, true);
  for (const name of requiredNames) {
    const expected = path.join(skillsRoot, name, 'SKILL.md');
    if (!skillFiles.has(expected)) issue(expected, 0, 'required skill file is missing');
    else if (names.get(name) !== expected) issue(expected, 1, 'required skill name is missing or not unique at its expected path: ' + name);
  }
  await scan(path.join(root, 'docs', 'development-guide.md'), false);
} catch (error) {
  issue(root, 0, 'validation failed: ' + error.message);
}
if (errors.length) {
  for (const error of errors) console.error(error);
  console.error('Project skills validation failed: ' + errors.length + ' issue(s), ' + fileCount + ' Markdown file(s), ' + linkCount + ' relative link destination(s).');
  process.exitCode = 1;
} else {
  console.log('Project skills validation passed: ' + fileCount + ' Markdown file(s), ' + linkCount + ' relative link destination(s).');
}
