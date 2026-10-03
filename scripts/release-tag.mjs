import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

process.chdir(resolve(dirname(fileURLToPath(import.meta.url)), '..'));

function git(args, inherit = false) {
  const result = spawnSync('git', args, {
    encoding: 'utf8',
    stdio: inherit ? 'inherit' : 'pipe',
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`git ${args.join(' ')} failed${result.stderr ? `:\n${result.stderr.trim()}` : ''}`);
  }
  return result.stdout?.trim() || '';
}

function versionParts(version) {
  if (typeof version !== 'string' || !/^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/.test(version)) {
    throw new Error('Version must be X.Y.Z without leading zeros.');
  }
  const parts = version.split('.').map(Number);
  if (parts.some(part => part > 65535)) {
    throw new Error('Version components must be between 0 and 65535.');
  }
  return parts;
}

try {
  const args = process.argv.slice(2);
  if (args.length > 1) throw new Error('Usage: mise run release:tag -- [X.Y.Z]');
  const app = JSON.parse(readFileSync('build/app.json', 'utf8'));
  const current = versionParts(app.version);
  const nextVersion = args[0] ?? `${current[0]}.${current[1]}.${current[2] + 1}`;
  const next = versionParts(nextVersion);
  const different = next.findIndex((part, index) => part !== current[index]);
  if (different === -1 || next[different] < current[different]) {
    throw new Error(`Version must be greater than ${app.version}.`);
  }
  if (git(['status', '--porcelain', '--untracked-files=all'])) {
    throw new Error('Working tree must be clean before releasing.');
  }
  const branch = git(['symbolic-ref', '--short', 'HEAD']);
  const tag = `v${nextVersion}`;
  if (git(['tag', '--list', tag])) throw new Error(`Local tag ${tag} already exists.`);
  if (git(['ls-remote', '--tags', 'origin', `refs/tags/${tag}`, `refs/tags/${tag}^{}`])) {
    throw new Error(`Origin tag ${tag} already exists.`);
  }

  app.version = nextVersion;
  writeFileSync('build/app.json', JSON.stringify(app, null, 2) + '\n');
  git(['add', '--', 'build/app.json'], true);
  git(['commit', '-m', `release: ${tag}`, '--', 'build/app.json'], true);
  git(['tag', tag], true);
  git(['push', '--atomic', 'origin', `HEAD:refs/heads/${branch}`, `refs/tags/${tag}`], true);
  console.log(`Pushed ${tag}; GitHub Actions will build and publish the installer.`);
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
}
