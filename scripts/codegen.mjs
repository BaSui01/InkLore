// Unified Full-Domain OpenAPI 3.x Codegen Pipeline for InkLore
// Generates:
// 1. Frontend TypeScript contracts via openapi-typescript into frontend/packages/contracts/src/generated/
// 2. Backend Go structs & Gin Server interfaces via oapi-codegen into internal/platform/contracts/
// 3. Syncs embedded resources and triggers requestschema generator for backward compatibility.

import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const root = path.resolve(__dirname, '..');
const apiDir = path.join(root, 'docs/system-design/api');
const codegenDir = path.join(root, '.codegen');
const specsDir = path.join(codegenDir, 'specs');
const frontendContractsDir = path.join(root, 'frontend/packages/contracts/src/generated');
const backendContractsDir = path.join(root, 'internal/platform/contracts');

fs.mkdirSync(specsDir, { recursive: true });
fs.mkdirSync(frontendContractsDir, { recursive: true });
fs.mkdirSync(backendContractsDir, { recursive: true });

const read = f => JSON.parse(fs.readFileSync(f, 'utf8'));
const allFiles = fs.readdirSync(apiDir).filter(f => f.endsWith('.json') && !['openapi.json', 'error-catalog.json'].includes(f));

// 1. Build dictionary of REAL schemas (prefer definitions with properties, oneOf, allOf, enum over pure $ref)
const realSchemas = {};
for (const f of allFiles) {
  const doc = read(path.join(apiDir, f));
  if (doc.components?.schemas) {
    for (const [name, s] of Object.entries(doc.components.schemas)) {
      if (!s.$ref) {
        realSchemas[name] = s;
      }
    }
  }
}
for (const f of allFiles) {
  const doc = read(path.join(apiDir, f));
  if (doc.components?.schemas) {
    for (const [name, s] of Object.entries(doc.components.schemas)) {
      if (!realSchemas[name]) {
        realSchemas[name] = s;
      }
    }
  }
}

const commonDoc = read(path.join(apiDir, 'common.json'));

function sanitizeForCodegen(node) {
  if (Array.isArray(node)) return node.map(sanitizeForCodegen);
  if (!node || typeof node !== 'object') return node;
  const res = {};
  for (let [k, v] of Object.entries(node)) {
    if (k === 'type' && Array.isArray(v)) {
      if (v.includes('null')) {
        const nonNull = v.filter(t => t !== 'null');
        if (nonNull.length === 1) {
          res['type'] = nonNull[0];
          res['nullable'] = true;
          continue;
        }
      }
      // Multiple union types -> omit type so it maps to any/interface{}
      continue;
    }
    if (k === 'const') {
      if (!node.type) {
        res['type'] = typeof v;
      }
      res['enum'] = [v];
      continue;
    }
    if (k === '$ref' && typeof v === 'string') {
      const match = v.match(/#\/components\/(schemas|responses|parameters|securitySchemes)\/(.+)$/);
      if (match) {
        res[k] = `#/components/${match[1]}/${match[2]}`;
        continue;
      }
    }
    res[k] = sanitizeForCodegen(v);
  }
  return res;
}

const domains = ['llm', 'agent', 'compliance', 'content', 'inspiration', 'outline', 'platform', 'worldview'];

console.log('📦 1/3 Bundling OpenAPI domain specifications...');

for (const domain of domains) {
  const domainFiles = allFiles.filter(f => f.startsWith(domain));
  const domainSchemas = {};

  // Add common foundation schemas
  for (const [name, s] of Object.entries(commonDoc.components?.schemas || {})) {
    domainSchemas[name] = realSchemas[name] || s;
  }

  for (const f of domainFiles) {
    const doc = read(path.join(apiDir, f));
    if (doc.components?.schemas) {
      for (const [name, s] of Object.entries(doc.components.schemas)) {
        domainSchemas[name] = realSchemas[name] || s;
      }
    }
  }

  // Name conflict resolutions
  if (domain === 'worldview' && domainSchemas['Conflict']) {
    domainSchemas['Conflict']['x-go-name'] = 'WorldviewConflict';
  }
  if (domain === 'inspiration' && domainSchemas['CardState']) {
    domainSchemas['CardState']['x-go-name'] = 'InspirationCardState';
  }
  if (domain === 'agent') {
    // Include events.json schemas for SSE contract
    const eventsDoc = read(path.join(apiDir, 'events.json'));
    if (eventsDoc.components?.schemas) {
      Object.assign(domainSchemas, eventsDoc.components.schemas);
    }
    if (domainSchemas['JsonValue']) {
      domainSchemas['JsonValue'] = {
        description: 'Arbitrary JSON value'
      };
    }
    if (domainSchemas['JSONSchema']) {
      domainSchemas['JSONSchema'] = {
        type: 'object',
        description: 'Dynamic JSON Schema Draft 2020-12',
        'x-go-type': 'json.RawMessage'
      };
    }
    if (domainSchemas['CreateRun']) {
      domainSchemas['CreateRun']['x-go-type'] = 'json.RawMessage';
    }
  }

  // Resolve missing transitive referenced schemas
  let added = true;
  while (added) {
    added = false;
    const text = JSON.stringify(domainSchemas);
    const matches = text.matchAll(/components\/schemas\/([A-Za-z0-9_]+)/g);
    for (const m of matches) {
      const name = m[1];
      if (!domainSchemas[name] && realSchemas[name]) {
        domainSchemas[name] = realSchemas[name];
        added = true;
      }
    }
  }

  const domainPaths = {};
  for (const f of domainFiles) {
    const doc = read(path.join(apiDir, f));
    if (doc.paths) Object.assign(domainPaths, doc.paths);
  }

  const bundled = {
    openapi: '3.0.3',
    info: { title: `InkLore ${domain.toUpperCase()} API`, version: '1.0.0' },
    paths: domainPaths,
    components: {
      schemas: domainSchemas,
      responses: commonDoc.components?.responses,
      parameters: commonDoc.components?.parameters,
      securitySchemes: commonDoc.components?.securitySchemes
    }
  };

  const clean = sanitizeForCodegen(bundled);
  fs.writeFileSync(path.join(specsDir, `${domain}.json`), JSON.stringify(clean, null, 2));
  console.log(`  ✓ Bundled [${domain}]: ${Object.keys(domainPaths).length} paths, ${Object.keys(domainSchemas).length} schemas`);
}

// 2. Generate Frontend TypeScript Contracts
// Publish generated files only after every domain and both generators succeed.
// A missing tool or failed compiler must neither truncate Go files nor replace
// the TypeScript barrel with a partial set of exports.
const outputDir = fs.mkdtempSync(path.join(codegenDir, 'output-'));
process.on('exit', () => fs.rmSync(outputDir, { recursive: true, force: true }));
console.log('🎨 2/3 Generating Frontend TypeScript contracts...');
const tsExports = [];
const npxCommand = process.platform === 'win32' ? 'npx.cmd' : 'npx';

for (const domain of domains) {
  const specPath = path.join(specsDir, `${domain}.json`);
  const tsOutPath = path.join(outputDir, `${domain}.gen.ts`);
  try {
    const npxArgs = ['--yes', 'openapi-typescript', specPath, '-o', tsOutPath];
    // Windows 上 Node 不允许未启用 shell 时直接 spawn .cmd/.bat（EINVAL），而 shell:true 又会触发 DEP0190。
    // 这里经 cmd.exe 显式调用，参数仍以数组传递且全部来自本仓库路径。
    if (process.platform === 'win32') {
      execFileSync('cmd.exe', ['/d', '/s', '/c', npxCommand, ...npxArgs], { cwd: path.join(root, 'frontend'), stdio: 'pipe' });
    } else {
      execFileSync(npxCommand, npxArgs, { cwd: path.join(root, 'frontend'), stdio: 'pipe' });
    }
    if (fs.statSync(tsOutPath).size === 0) throw new Error('empty TypeScript output');
    console.log(`  ✓ TS generated: [${domain}] -> frontend/packages/contracts/src/generated/${domain}.gen.ts`);
    tsExports.push(`export * as ${domain.charAt(0).toUpperCase() + domain.slice(1)}API from './${domain}.gen';`);
  } catch (err) {
    console.error(`  ✗ TS generation failed for [${domain}]:`, err.message);
    throw err;
  }
}

const indexContent = `/**\n * Code generated by scripts/codegen.mjs via openapi-typescript. DO NOT EDIT.\n */\n\n` + tsExports.join('\n') + '\n';

// 3. Generate Backend Go Contracts
console.log('🐹 3/3 Generating Backend Go contracts...');
const gopath = process.env.GOPATH || path.join(process.env.HOME || process.env.USERPROFILE || '', 'go');
const oapiBin = path.join(gopath, 'bin', process.platform === 'win32' ? 'oapi-codegen.exe' : 'oapi-codegen');

for (const domain of domains) {
  const specPath = path.join(specsDir, `${domain}.json`);
  const goOutPath = path.join(outputDir, `${domain}.gen.go`);
  const pkgName = `${domain}contracts`;
  try {
    const generated = execFileSync(oapiBin, ['--generate', 'types,gin', '-package', pkgName, specPath], { stdio: 'pipe' });
    if (generated.length === 0) throw new Error('empty Go output');
    fs.writeFileSync(goOutPath, generated);
    const lineCount = fs.readFileSync(goOutPath, 'utf8').split('\n').length;
    console.log(`  ✓ Go generated: [${domain}] -> internal/platform/contracts/${domain}/contracts.gen.go (${lineCount} lines)`);
  } catch (err) {
    console.error(`  ✗ Go generation failed for [${domain}]:`, err.message);
    throw err;
  }
}

for (const domain of domains) {
  const domainOutDir = path.join(backendContractsDir, domain);
  fs.mkdirSync(domainOutDir, { recursive: true });
  fs.copyFileSync(path.join(outputDir, `${domain}.gen.ts`), path.join(frontendContractsDir, `${domain}.gen.ts`));
  fs.copyFileSync(path.join(outputDir, `${domain}.gen.go`), path.join(domainOutDir, 'contracts.gen.go'));
}
fs.writeFileSync(path.join(frontendContractsDir, 'index.ts'), indexContent);
console.log('🎉 Full-domain OpenAPI 3.x Codegen completed successfully!');
