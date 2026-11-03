// INK-143: structure and seal validation; no network and no dependencies.
import path from 'node:path';
import { readFile, readdir, lstat } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { normalize, prose, safePath, checkLinks } from './agent-note-links.mjs';
export const lifecycles = ['proposed', 'implemented', 'rejected', 'archived'];
export const classes = ['feature', 'bug-fix', 'simplification', 'architecture', 'process', 'testing'];
const notePattern = /^(proposed|implemented|rejected|archived)\/(feature|bug-fix|simplification|architecture|process|testing)\/(\d{4}-\d{2}-\d{2})-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$/;
export const hash = text => createHash('sha256').update(normalize(text), 'utf8').digest('hex');
export const label = (root, file) => path.relative(root, file).split(path.sep).join('/');
export function notePath(relative) {
  const match = notePattern.exec(relative);
  if (!match) return null;
  const date = new Date(match[3] + 'T00:00:00Z');
  if (!Number.isFinite(date.getTime()) || date.toISOString().slice(0, 10) !== match[3] || match[3].startsWith('0000')) return null;
  return { lifecycle: match[1], category: match[2], basename: path.posix.basename(relative) };
}
export function manifestOf(text) {
  let data;
  try { data = JSON.parse(text); } catch { throw new Error('archive-manifest.json: invalid JSON'); }
  if (!data || Array.isArray(data) || data.version !== 1 || !data.files || typeof data.files !== 'object' || Array.isArray(data.files)
      || Object.keys(data).sort().join(',') !== 'files,version') throw new Error('archive-manifest.json: expected {"version":1,"files":{...}}');
  for (const [file, digest] of Object.entries(data.files)) {
    if (notePath(file)?.lifecycle !== 'archived' || typeof digest !== 'string' || !/^[0-9a-f]{64}$/.test(digest)) throw new Error('archive-manifest.json: invalid archived path or lowercase SHA-256');
  }
  return data;
}
export async function utf8(file, limit = Infinity) {
  const data = await readFile(file);
  if (data.length > limit) throw new Error('Markdown exceeds 50000 bytes');
  try { return new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(data); }
  catch { throw new Error('invalid UTF-8'); }
}
const aliases = new Map([
  ['问题', 'problem'], ['问题背景', 'problem'], ['提案', 'proposal'], ['方案', 'proposal'],
  ['备选方案', 'alternatives considered'], ['考虑过的替代方案', 'alternatives considered'], ['替代方案', 'alternatives considered'],
  ['验收标准', 'acceptance criteria'], ['风险', 'risks'], ['决策', 'decision'], ['后果', 'consequences'], ['影响', 'consequences'],
  ['验证', 'verification'], ['验证结果', 'verification'], ['计划', 'plan'], ['实施计划', 'plan'], ['迁移计划', 'migration plan'],
]);
const headingName = value => {
  const key = value.replace(/\s+#+\s*$/, '').trim().toLowerCase();
  return aliases.get(key) ?? key;
};
function substantive(body) {
  const lines = body.split('\n').filter(line => !/^\s*#{1,6}\s/.test(line)).map(line => line.replace(/^\s*(?:[-*+>]\s*|\d+[.)]\s*)/, '').replace(/\[[ xX]\]/g, '').trim()).filter(Boolean);
  return lines.some(line => {
    const plain = line.replace(/[*_\x60<>\[\]{}().。,:：;；!！?？…\-]/g, '').trim();
    return plain && !/^(?:(?:todo|tbd|fixme|xxx|placeholder|pending|n\/?a|none|待办|待补充|待完善|待填写|占位(?:文本)?|暂无|无|略|未定|待定|在此填写|填写此处)(?:\s|\/|、)*)$/i.test(plain)
      && !/^(?:todo|tbd|fixme|placeholder|待补充|待填写|占位文本)\s*[:：]/i.test(line);
  });
}
function validateNote(text, meta, tasks) {
  if (/\p{Extended_Pictographic}/u.test(text)) throw new Error('Extended_Pictographic is forbidden');
  const lines = normalize(text).replace(/^\uFEFF/, '').split('\n');
  if (!/^# Agent Note: \S.*$/.test(lines[0]) || lines[1] !== '') throw new Error('expected title and blank first two lines');
  const status = meta.lifecycle === 'rejected' ? /^Status: rejected — \S.*$/ : new RegExp('^Status: ' + meta.lifecycle + '$');
  if (!status.test(lines[2] ?? '')) throw new Error('Status must match lifecycle (rejected needs an em-dash reason)');
  if (lines[3] !== '') throw new Error('line 4 must be blank');
  const task = /^Task: (INK-\d{3,})$/.exec(lines[4] ?? '');
  if (!task || !tasks.has(task[1])) throw new Error('line 5 Task must name a backlog table task INK-<at least three digits>');
  if (!/^Scope: \S.*$/.test(lines[5] ?? '')) throw new Error('line 6 Scope must be nonempty');
  const body = prose(lines.slice(6).join('\n'));
  const headings = [...body.matchAll(/^ {0,3}(#{1,6})[ \t]+(.+?)[ \t]*$/gm)];
  // Setext headings are H1/H2 too; they cannot bypass terminal-state rules.
  for (const match of body.matchAll(/^ {0,3}([^\n]+)\n {0,3}(={3,}|-{3,})[ \t]*$/gm)) {
    if (/^\s*(?:#|>|[-*+]\s)/.test(match[1])) continue;
    const entry = [match[0], match[2][0] === '=' ? '#' : '##', match[1]];
    entry.index = match.index;
    headings.push(entry);
  }
  headings.sort((a, b) => a.index - b.index);
  const sections = new Map();
  const terminal = ['implemented', 'archived'].includes(meta.lifecycle);
  for (let i = 0; i < headings.length; i++) {
    const entry = headings[i], key = headingName(entry[2]);
    if (terminal && ['proposal', 'plan', 'migration plan', 'acceptance criteria', 'risks'].includes(key)) throw new Error('implemented/archived forbids planning heading: ' + key);
    if (entry[1].length !== 2) continue;
    let end = body.length;
    for (let j = i + 1; j < headings.length; j++) if (headings[j][1].length <= 2) { end = headings[j].index; break; }
    const entries = sections.get(key) ?? [];
    entries.push(body.slice(entry.index + entry[0].length, end));
    sections.set(key, entries);
  }
  const required = terminal ? ['problem', 'decision', 'alternatives considered', 'consequences', 'verification']
    : ['problem', 'proposal', 'alternatives considered', 'acceptance criteria', 'risks'];
  for (const key of required) {
    const entries = sections.get(key) ?? [];
    if (entries.length !== 1 || !substantive(entries[0])) throw new Error('required H2 must occur once with substantive content: ' + key);
  }
}
export async function checkTree(root) {
  const notes = path.join(root, '.agents', 'notes');
  const files = new Map(), identities = new Set(), errors = [];
  const report = (file, message) => errors.push(label(root, file) + ': ' + message);
  await safePath(root, notes);
  const backlog = await utf8(path.join(root, 'docs', 'backlog.md'));
  const tasks = new Set([...prose(backlog).matchAll(/^\s*\|\s*(INK-\d{3,})\s*\|.+\|\s*$/gm)].map(match => match[1]));
  async function walk(directory, parts = []) {
    for (const entry of (await readdir(directory, { withFileTypes: true })).sort((a, b) => a.name.localeCompare(b.name))) {
      const file = path.join(directory, entry.name), next = [...parts, entry.name];
      if (entry.isSymbolicLink()) { report(file, 'symlink is forbidden'); continue; }
      if (entry.isDirectory()) {
        const valid = next[0] === 'reference' || (next.length === 1 ? lifecycles.includes(entry.name) : next.length === 2 && classes.includes(entry.name));
        if (!valid) report(file, 'invalid note directory');
        else await walk(file, next);
        continue;
      }
      if (!entry.isFile()) { report(file, 'only regular files allowed'); continue; }
      // Reference files are preserved ADRs, not lifecycle notes.
      if (next[0] === 'reference') {
        if (!/\.(?:md|json)$/i.test(entry.name)) report(file, 'unsupported reference file');
        continue;
      }
      const relative = next.join('/');
      if (relative === 'archive-manifest.json') continue;
      const meta = notePath(relative);
      if (relative !== 'README.md' && !meta) { report(file, 'invalid note path/date (INDEX.md is forbidden)'); continue; }
      try {
        const text = await utf8(file, 50000);
        if (/\p{Extended_Pictographic}/u.test(text)) throw new Error('Extended_Pictographic is forbidden');
        if (meta) {
          const identity = meta.category + '/' + meta.basename;
          if (identities.has(identity)) report(file, 'duplicate class and basename across lifecycles');
          identities.add(identity);
          validateNote(text, meta, tasks);
          files.set(relative, { file, text, ...meta });
        }
        await checkLinks(root, file, text);
      } catch (error) { report(file, error.code ? 'cannot read required file (' + error.code + ')' : error.message); }
    }
  }
  await walk(notes);
  const manifestFile = path.join(notes, 'archive-manifest.json');
  let manifest, manifestText;
  try {
    await safePath(root, manifestFile);
    if (!(await lstat(manifestFile)).isFile()) throw new Error('manifest must be a regular file');
    manifestText = await utf8(manifestFile);
    manifest = manifestOf(manifestText);
    for (const [relative, entry] of files) if (entry.lifecycle === 'archived' && manifest.files[relative] !== hash(entry.text)) report(entry.file, 'missing or mismatched archive SHA-256');
    for (const relative of Object.keys(manifest.files)) if (files.get(relative)?.lifecycle !== 'archived') report(manifestFile, 'orphan archived entry: ' + relative);
  } catch (error) { report(manifestFile, error.code ? 'cannot read required manifest (' + error.code + ')' : error.message); }
  if (errors.length) throw new Error(errors.join('\n'));
  return { notes, files, manifest, manifestFile, manifestText };
}
