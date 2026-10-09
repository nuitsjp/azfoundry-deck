import { spawnSync } from 'node:child_process';
import { readdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const mode = process.argv[2];
if (!['check', 'write'].includes(mode)) {
  throw new Error('Use check or write.');
}
const files = [];
function collect(directory, recursive) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name);
    if (entry.isFile() && entry.name.endsWith('.go')) files.push(path);
    else if (recursive && entry.isDirectory()) collect(path, true);
  }
}
collect(root, false);
collect(resolve(root, 'internal'), true);
collect(resolve(root, 'cmd'), true);

const result = spawnSync('gofmt', [mode === 'write' ? '-w' : '-l', ...files.sort()], {
  cwd: root,
  encoding: 'utf8',
});
if (result.error) throw result.error;
process.stdout.write(result.stdout);
process.stderr.write(result.stderr);
if (result.status !== 0) process.exit(result.status || 1);
if (mode === 'check' && result.stdout.trim()) {
  console.error('Goの整形が必要です。node scripts/run.mjs format:go を実行してください。');
  process.exit(1);
}
