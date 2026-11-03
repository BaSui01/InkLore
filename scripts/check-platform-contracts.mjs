// 错误目录与实现须一致，数据库演进由真实迁移验收证明。
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const read=name=>fs.readFileSync(path.join(root,name),'utf8');
const catalog=JSON.parse(read('docs/system-design/api/error-catalog.json'));
const source=new Map();
for(const files of Object.values(catalog.domains))for(const file of files)for(const error of JSON.parse(read('docs/system-design/api/'+file)).errors)source.set(error.code,error.http_status);
const generated=new Map([...read('internal/platform/errors/catalog.go').matchAll(/"([a-z_]+\.[a-z0-9_]+)":\s*(\d+),/g)].map(match=>[match[1],Number(match[2])]));
if(source.size!==generated.size||[...source].some(([code,status])=>generated.get(code)!==status))throw Error('Error catalog implementation drift');
console.log('Platform contract check passed: '+source.size+' registered error mappings.');
