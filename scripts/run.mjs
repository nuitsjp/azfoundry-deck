// The only public command entry. Build order lives in Taskfile.yml.
import { spawnSync } from 'node:child_process';
import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { dirname, delimiter, resolve } from 'node:path';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const requiredNodeVersion = readFileSync(resolve(root, '.nvmrc'), 'utf8').trim();
if (process.versions.node !== requiredNodeVersion) {
  console.error(`Node.js ${requiredNodeVersion} を使用してください（実行中: ${process.versions.node}）。nvm install ${requiredNodeVersion} と nvm use ${requiredNodeVersion} を実行してください。`);
  process.exit(1);
}
process.chdir(root);
const windows = process.platform === 'win32';
const tools = resolve('.tools');
const cli = resolve(tools, windows ? 'wails3.exe' : 'wails3');
const goLinterVersion = '2.4.0';
const goLinter = resolve(tools, windows ? 'golangci-lint.exe' : 'golangci-lint');
const analysisTools = {
  'golangci-lint': { module: 'github.com/golangci/golangci-lint/v2/cmd/golangci-lint', version: goLinterVersion },
  actionlint: { module: 'github.com/rhysd/actionlint/cmd/actionlint', version: '1.7.12' },
  govulncheck: { module: 'golang.org/x/vuln/cmd/govulncheck', version: '1.8.0' },
};
const env = { ...process.env, PATH: tools + delimiter + process.env.PATH };
function run(command, args, cwd = root, extra = {}) {
  // npm.cmd needs cmd.exe on Windows. All args here are fixed by this script.
  const options = { cwd, env: { ...env, ...extra }, stdio: 'inherit' };
  const result = windows && command === 'npm'
    ? spawnSync(process.env.ComSpec || 'cmd.exe', ['/d', '/s', '/c', `npm ${args.join(' ')}`], options)
    : spawnSync(command, args, options);
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
const [command = 'help'] = process.argv.slice(2);
function installCLI() {
  const match = readFileSync('go.mod', 'utf8').match(/github\.com\/wailsapp\/wails\/v3 (\S+)/);
  if (!match) throw new Error('Wails version is missing from go.mod');
  mkdirSync(tools, { recursive: true });
  run('go', ['install', `github.com/wailsapp/wails/v3/cmd/wails3@${match[1]}`], root, {
    GOBIN: tools,
    GOMODCACHE: resolve(tmpdir(), 'azfoundry-deck-wails', 'mod'),
    GOCACHE: resolve(tmpdir(), 'azfoundry-deck-wails', 'build'),
  });
}
function installAnalysisTool(name) {
  const tool = analysisTools[name];
  mkdirSync(tools, { recursive: true });
  const goVersion = spawnSync('go', ['env', '-json', 'GOVERSION', 'GOHOSTOS', 'GOHOSTARCH'], {
    env: { ...env, GOTOOLCHAIN: 'local' }, encoding: 'utf8',
  });
  if (goVersion.error) throw goVersion.error;
  if (goVersion.status !== 0) throw new Error(goVersion.stderr);
  const hostGo = JSON.parse(goVersion.stdout);
  const key = [tool.version, hostGo.GOVERSION, hostGo.GOHOSTOS, hostGo.GOHOSTARCH].join('-');
  const executable = name + (windows ? '.exe' : '');
  const binary = resolve(tools, executable);
  const marker = resolve(tools, `${name}.install-key`);
  if (existsSync(binary) && existsSync(marker) && readFileSync(marker, 'utf8') === key) {
    console.log(`Reusing ${name}: ${key}`);
    return;
  }
  const cached = process.env.RUNNER_TOOL_CACHE
    ? resolve(process.env.RUNNER_TOOL_CACHE, 'azfoundry-deck-tools', name, key, executable)
    : undefined;
  if (cached && existsSync(cached)) {
    copyFileSync(cached, binary);
    console.log(`Restored ${name}: ${key}`);
  } else {
    run('go', ['install', `${tool.module}@v${tool.version}`], root, {
      GOBIN: tools, GOTOOLCHAIN: 'local', GOOS: hostGo.GOHOSTOS, GOARCH: hostGo.GOHOSTARCH,
    });
    if (cached) {
      mkdirSync(dirname(cached), { recursive: true });
      copyFileSync(binary, cached);
    }
  }
  writeFileSync(marker, key);
}
try {
  if (command === 'cli:install') {
    installCLI();
  } else if (command === 'lint:go:install') {
    installAnalysisTool('golangci-lint');
  } else if (command === 'check:workflow') {
    const actionlint = resolve(tools, windows ? 'actionlint.exe' : 'actionlint');
    if (!existsSync(actionlint)) throw new Error('先に mise run setup を実行してください。');
    const workflows = readdirSync('.github/workflows').filter(name => /\.ya?ml$/.test(name));
    run(actionlint, ['-shellcheck=', '-pyflakes=', ...workflows.map(name => `.github/workflows/${name}`)]);
  } else if (command === 'audit' || command === 'audit:go') {
    installAnalysisTool('govulncheck');
    if (command === 'audit') run('npm', ['audit', '--audit-level=low'], resolve('frontend'));
    for (const tags of ['production', 'server,production,e2e']) {
      run(resolve(tools, windows ? 'govulncheck.exe' : 'govulncheck'), ['-tags', tags, '.', './internal/...', './cmd/...']);
    }
  } else if (command === 'lint:go:prepared') {
    if (!existsSync(goLinter)) throw new Error('先に node scripts/run.mjs lint:go:install を実行してください。');
    for (const tags of ['production', 'server,production,e2e']) {
      run(goLinter, ['run', '--config', '.golangci.yml', '--build-tags', tags, '.', './internal/...', './cmd/...']);
    }
  } else if (command === 'format:go' || command === 'format:go:check') {
    run('node', ['scripts/go-format.mjs', command === 'format:go' ? 'write' : 'check']);
  } else if (command === 'setup' || command === 'setup:dependencies') {
    if (command === 'setup') installCLI();
    for (const name of Object.keys(analysisTools)) installAnalysisTool(name);
    // Install exactly the committed go.sum and package-lock.json without rewriting them.
    run('go', ['mod', 'download']);
    run('go', ['mod', 'verify']);
    const reuse = command === 'setup:dependencies' && process.argv.includes('--reuse-node-modules');
    const installKey = createHash('sha256').update(JSON.stringify([
      process.version, process.platform, process.arch, 'npm ci --no-audit --no-fund',
      readFileSync('frontend/package.json', 'utf8'), readFileSync('frontend/package-lock.json', 'utf8'),
    ])).digest('hex');
    const marker = resolve('frontend/node_modules/.ci-install-key');
    if (reuse && existsSync(marker) && readFileSync(marker, 'utf8') === installKey) {
      console.log('Reusing node_modules: installation inputs unchanged.');
    } else {
      rmSync(marker, { force: true });
      run('npm', ['ci', '--no-audit', '--no-fund'], resolve('frontend'));
      if (reuse) writeFileSync(marker, installKey);
    }
    if (command === 'setup') run(cli, ['task', 'generate']);
  } else if (command === 'help') {
    console.log('node scripts/run.mjs setup | dev | build | package | package:prepared | ci | server | server:review | server:review:login | server:review:login-multiple | server:review:foundry-change | server:review:foundry-refresh | server:review:foundry-add | server:review:foundry-delete | server:review:deployment-refresh | server:review:deployment-detail | server:review:deployment-delete | server:review:deployment-add | server:review:deployment-update | server:review:update | server:review:update-untrusted | server:review:tenant-change | server:review:no-foundry | server:review:foundry-empty | server:review:tenant-revisit | verify | verify:e2e | audit | format | format:check | format:go | lint | lint:go | lint:go:install | test:core | test:desktop');
  } else {
    if (!existsSync(cli)) throw new Error('先に node scripts/run.mjs setup を実行してください。');
    if (command === 'dev') {
      if (!windows) throw new Error('Desktop development is Windows-only. Use server for browser verification.');
      // Wails の既定 9245 は他プロダクトの開発サーバーと競合するため、専用ポートを使う。
      run(cli, ['dev'], root, { WAILS_VITE_PORT: '9301' });
    } else if (command === 'test:desktop') {
      // Installs, updates and uninstalls the desktop app; never part of verify:e2e.
      if (!windows) throw new Error('The desktop E2E is Windows-only.');
      run('npm', ['--prefix', 'frontend', 'run', 'test:e2e', '--', '--grep', '@desktop'], root, { DESKTOP_E2E: '1' });
    } else if (['build', 'package', 'package:prepared', 'ci', 'server', 'server:review', 'server:review:login', 'server:review:login-multiple', 'server:review:foundry-change', 'server:review:foundry-refresh', 'server:review:foundry-add', 'server:review:foundry-delete', 'server:review:deployment-refresh', 'server:review:deployment-detail', 'server:review:deployment-delete', 'server:review:deployment-add', 'server:review:deployment-update', 'server:review:update', 'server:review:update-untrusted', 'server:review:tenant-change', 'server:review:no-foundry', 'server:review:foundry-empty', 'server:review:tenant-revisit', 'verify', 'verify:e2e', 'format', 'format:check', 'lint', 'lint:go', 'test:core', 'generate'].includes(command)) {
      run(cli, ['task', command]);

    } else throw new Error(`Unknown command: ${command}`);
  }
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
}
