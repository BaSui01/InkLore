#!/usr/bin/env node
// INK-143. Offline Node 22+ CLI. Root is anchored to this script, never cwd.
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, openSync, closeSync, readFileSync, rmSync } from 'node:fs';
import { readdir, lstat, mkdir, mkdtemp, rename, unlink, rm, open } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
import { checkTree, manifestOf, notePath, hash, label, utf8 } from './lib/agent-note-validation.mjs';
import { destinations, resolveLink, checkLinks, safePath, samePath } from './lib/agent-note-links.mjs';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const usage = 'Usage: node scripts/agent-notes.mjs check [--base <git-ref>]\n       node scripts/agent-notes.mjs archive <repository-relative implemented note path>';
// OS files, not pipes: works under Windows runtimes that forbid named pipes.
function git(args) {
  const directory = mkdtempSync(path.join(os.tmpdir(), 'agent-note-git-'));
  let stdout, stderr;
  try {
    stdout = openSync(path.join(directory, 'stdout'), 'w');
    stderr = openSync(path.join(directory, 'stderr'), 'w');
    const result = spawnSync('git', ['--no-pager', ...args], {
      cwd: root, shell: false, stdio: ['ignore', stdout, stderr], windowsHide: true,
      env: { ...process.env, GIT_TERMINAL_PROMPT: '0', GIT_OPTIONAL_LOCKS: '0', GIT_NO_LAZY_FETCH: '1' },
    });
    if (result.error || result.status !== 0) throw new Error('read-only git operation failed (output withheld)');
    return readFileSync(path.join(directory, 'stdout'), 'utf8');
  } finally {
    if (stdout !== undefined) closeSync(stdout);
    if (stderr !== undefined) closeSync(stderr);
    rmSync(directory, { recursive: true, force: true });
  }
}
function checkBase(base, manifest) {
  if (!base || base.startsWith('-') || /[\0\r\n]/.test(base)) throw new Error('invalid --base ref');
  const oid = git(['rev-parse', '--verify', '--end-of-options', base + '^{commit}']).trim();
  if (!/^[0-9a-f]{40,64}$/.test(oid)) throw new Error('git returned an invalid commit object');
  const location = '.agents/notes/archive-manifest.json';
  const entries = git(['ls-tree', '-z', oid, '--', location]).split('\0').filter(Boolean);
  if (!entries.length) {
    console.log('bootstrap: base commit exists but has no archive manifest; allowing first introduction.');
    return;
  }
  if (entries.length !== 1 || !/^100(?:644|755) blob [0-9a-f]+\t/.test(entries[0])) throw new Error('base manifest must be a regular file');
  const previous = manifestOf(git(['show', oid + ':' + location]));
  for (const [file, digest] of Object.entries(previous.files)) {
    if (!Object.hasOwn(manifest.files, file) || manifest.files[file] !== digest) throw new Error('base archive seal removed or changed: ' + file);
  }
  console.log('base archive seals unchanged: ' + Object.keys(previous.files).length);
}
async function governanceFiles() {
  const result = [];
  async function walk(file) {
    let info;
    try { info = await lstat(file); } catch (error) { if (error.code === 'ENOENT') return; throw error; }
    if (info.isSymbolicLink()) throw new Error(label(root, file) + ': governance symlink prevents safe inbound-reference scan');
    if (info.isDirectory()) {
      for (const name of await readdir(file)) await walk(path.join(file, name));
    } else if (info.isFile() && /\.md$/i.test(file)) result.push(file);
  }
  for (const name of ['.agents', 'docs', 'AGENTS.md', 'README.md', 'CHANGELOG.md']) await walk(path.join(root, name));
  return result;
}
async function durableWrite(file, contents) {
  const handle = await open(file, 'wx');
  try { await handle.writeFile(contents); await handle.sync(); }
  finally { await handle.close(); }
}
async function replaceManifest(file, contents) {
  const staged = file + '.' + randomUUID() + '.tmp';
  try { await durableWrite(staged, contents); await rename(staged, file); }
  finally { await rm(staged, { force: true }); }
}
async function archive(relative) {
  // Serialize the entire read/validate/write/rollback transaction across CLI runs.
  const lockFile = path.join(root, '.agents', 'notes-archive.lock');
  await safePath(root, path.dirname(lockFile));
  let lock;
  try { lock = await open(lockFile, 'wx'); }
  catch (error) {
    if (error.code === 'EEXIST') throw new Error('archive lock already exists: .agents/notes-archive.lock; do not steal it. After a crash, manually inspect source, manifest and recovery copies before removing the stale lock.');
    throw error;
  }
  try {
    await lock.writeFile(JSON.stringify({ pid: process.pid, started: new Date().toISOString() }) + '\n');
    await lock.sync();
    await archiveUnderLock(relative);
  } finally {
    await lock.close();
    await unlink(lockFile);
  }
}
async function archiveUnderLock(relative) {
  if (!relative || relative.includes('\\') || !relative.startsWith('.agents/notes/implemented/') || !notePath(relative.slice('.agents/notes/'.length))) throw new Error('archive requires a canonical repository-relative implemented note path');
  const state = await checkTree(root);
  const sourceKey = relative.slice('.agents/notes/'.length), source = state.files.get(sourceKey);
  if (!source || source.lifecycle !== 'implemented') throw new Error('source must be an implemented note');
  const targetKey = sourceKey.replace(/^implemented\//, 'archived/'), target = path.join(state.notes, ...targetKey.split('/'));
  await safePath(root, source.file);
  await safePath(root, target, true);
  try { await lstat(target); throw new Error('archive destination already exists; refusing overwrite'); }
  catch (error) { if (error.code !== 'ENOENT') throw error; }
  const inbound = [];
  for (const file of await governanceFiles()) {
    // Include existing archived documents: their references are immutable too.
    // The source itself is checked below at its prospective destination.
    if (file === source.file) continue;
    for (const raw of destinations(await utf8(file))) {
      let destination;
      try { destination = resolveLink(root, file, raw); }
      catch { throw new Error(label(root, file) + ': invalid local link prevents inbound-reference scan'); }
      if (samePath(destination, source.file)) { inbound.push(label(root, file)); break; }
    }
  }
  if (inbound.length) throw new Error('archive blocked by inbound local links:\n' + inbound.join('\n') + '\nFirst redirect active references to a successor note; the successor may link to the archived path after archival.');
  // Replace only the third line; preserve BOM, CRLF/CR/LF and all other bytes.
  const chunks = source.text.split(/(\r\n|\r|\n)/);
  chunks[4] = 'Status: archived';
  const finalText = chunks.join('');
  try { await checkLinks(root, target, finalText, source.file); }
  catch { throw new Error('links would fail from archived location; manually normalize cross-note links before archival'); }
  const next = { version: 1, files: { ...state.manifest.files, [targetKey]: hash(finalText) } };
  // Durable recovery copies survive process interruption; printed before mutations.
  const recovery = await mkdtemp(path.join(os.tmpdir(), 'agent-note-archive-'));
  let createdTarget = false, replacedManifest = false, removedSource = false, committed = false;
  try {
    await durableWrite(path.join(recovery, 'source.md'), source.text);
    await durableWrite(path.join(recovery, 'archive-manifest.json'), state.manifestText);
    await durableWrite(path.join(recovery, 'recovery.json'), JSON.stringify({ source: relative, target: label(root, target), manifest: label(root, state.manifestFile) }, null, 2));
    console.log('Recovery copies (removed on success): ' + recovery);
    // Catch edits made while scanning, before touching either authoritative file.
    if (await utf8(source.file) !== source.text || await utf8(state.manifestFile) !== state.manifestText) throw new Error('source or manifest changed during archive; retry');
    await safePath(root, source.file);
    await safePath(root, state.manifestFile);
    await safePath(root, target, true);
    await mkdir(path.dirname(target), { recursive: true });
    await safePath(root, path.dirname(target));
    // Exclusive creation, never rename over an existing archive.
    const handle = await open(target, 'wx');
    createdTarget = true;
    try { await handle.writeFile(finalText); await handle.sync(); } finally { await handle.close(); }
    await replaceManifest(state.manifestFile, JSON.stringify(next, null, 2) + '\n');
    replacedManifest = true;
    await unlink(source.file);
    removedSource = true;
    await checkTree(root);
    committed = true;
  } catch (error) {
    try {
      // Restore source before removing target: never discard the only note copy.
      if (removedSource) await durableWrite(source.file, source.text);
      if (replacedManifest) await replaceManifest(state.manifestFile, state.manifestText);
      if (createdTarget) await unlink(target);
    } catch {
      throw new Error('archive failed and rollback needs manual recovery from ' + recovery);
    }
    throw error;
  } finally {
    if (committed) await rm(recovery, { recursive: true, force: true });
  }
  console.log('Archived and sealed: ' + label(root, target));
  console.log('Recommended: run check --base HEAD before and after archival.');
}
try {
  if (Number(process.versions.node.split('.')[0]) < 22) throw new Error('Node.js 22+ is required');
  const [command, ...args] = process.argv.slice(2);
  if (command === 'check' && (args.length === 0 || (args.length === 2 && args[0] === '--base'))) {
    const state = await checkTree(root);
    if (args.length) checkBase(args[1], state.manifest);
    console.log('Agent notes check passed: ' + state.files.size + ' notes; working-tree structure, links and seals consistent.');
    if (!args.length) console.log('Historical immutability requires --base; this check does not enforce note-per-change or semantic correctness.');
  } else if (command === 'archive' && args.length === 1) await archive(args[0]);
  else throw new Error(usage);
} catch (error) {
  console.error(error.code ? 'Agent notes failed: filesystem operation failed (' + error.code + ')' : 'Agent notes failed: ' + error.message);
  process.exitCode = 1;
}
