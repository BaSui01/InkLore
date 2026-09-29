// R1 platform source drift checks; this is not a live database acceptance check.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const read=name=>fs.readFileSync(path.join(root,name),'utf8');
const normalize=text=>text.replace(/\r\n/g,'\n');
const domains=['platform','content','outline','worldview','llm'];
for(const [index,domain] of domains.entries()){const version=String(index+1).padStart(6,'0');if(normalize(read('docs/system-design/data/'+domain+'.sql'))!==normalize(read('migrations/r1/'+version+'_'+domain+'.sql')))throw Error('R1 '+domain+' migration differs from frozen DDL');}
const catalog=JSON.parse(read('docs/system-design/api/error-catalog.json'));
const source=new Map();
for(const files of Object.values(catalog.domains))for(const file of files)for(const error of JSON.parse(read('docs/system-design/api/'+file)).errors)source.set(error.code,error.http_status);
const generated=new Map([...read('internal/platform/errors/catalog.go').matchAll(/"([a-z_]+\.[a-z0-9_]+)":\s*(\d+),/g)].map(match=>[match[1],Number(match[2])]));
if(source.size!==generated.size||[...source].some(([code,status])=>generated.get(code)!==status))throw Error('R1 error catalog implementation drift');
console.log('R1 platform source check passed: exact DDL and '+source.size+' registered error mappings.');
