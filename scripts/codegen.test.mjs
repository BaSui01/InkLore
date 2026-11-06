import assert from 'node:assert/strict';
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

test('a missing generator fails the process and preserves existing contracts', () => {
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
  const copy = mkdtempSync(path.join(tmpdir(), 'inklore-codegen-'));
  try {
    mkdirSync(path.join(copy, 'scripts'), { recursive: true });
    cpSync(path.join(root, 'scripts/codegen.mjs'), path.join(copy, 'scripts/codegen.mjs'));
    cpSync(path.join(root, 'docs/system-design/api'), path.join(copy, 'docs/system-design/api'), { recursive: true });
    const outputs = ['frontend/packages/contracts/src/generated/index.ts', 'internal/platform/contracts/llm/contracts.gen.go'];
    for (const file of outputs) {
      mkdirSync(path.dirname(path.join(copy, file)), { recursive: true });
      writeFileSync(path.join(copy, file), 'existing generated contract\n');
    }
    // The real child process cannot resolve npx; no fake success or compiler
    // output is supplied. A failure must propagate through the CLI boundary.
    const result = spawnSync(process.execPath, ['scripts/codegen.mjs'], { cwd: copy, env: { ...process.env, PATH: '' }, encoding: 'utf8' });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /TS generation failed/);
    assert.doesNotMatch(result.stdout, /completed successfully/);
    for (const file of outputs) assert.equal(readFileSync(path.join(copy, file), 'utf8'), 'existing generated contract\n');
  } finally {
    rmSync(copy, { recursive: true, force: true });
  }
});
