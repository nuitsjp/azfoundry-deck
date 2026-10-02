import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
process.chdir(root);
const windows = process.platform === 'win32';
const cli = resolve('.tools', windows ? 'wails3.exe' : 'wails3');
const app = JSON.parse(readFileSync('build/app.json', 'utf8'));
const arch = process.env.GOARCH || (process.arch === 'arm64' ? 'arm64' : 'amd64');
const target = resolve('bin', app.executable);
const server = resolve('bin', app.executable.slice(0, -4) + '-server' + (windows ? '.exe' : ''));
// E2E-only server: the e2e tag swaps Entra ID / ARM and the Credential Manager for fixed boundaries.
const serverE2E = resolve('bin', app.executable.slice(0, -4) + '-server-e2e' + (windows ? '.exe' : ''));
function run(cmd, args, extra = {}) {
  const result = spawnSync(cmd, args, { stdio: 'inherit', ...extra });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
function safeNSIS(value) {
  if (typeof value !== 'string' || /[\r\n"$]/.test(value)) throw new Error('App metadata contains unsupported NSIS characters');
  return value;
}
function check() {
  if (!/^[a-zA-Z][a-zA-Z0-9_.-]+$/.test(app.id) || !/^[a-zA-Z0-9_.-]+\.exe$/.test(app.executable)) throw new Error('Invalid id/executable in build/app.json');
  if (!/^\d+\.\d+\.\d+$/.test(app.version) || app.version.split('.').some(x => Number(x) > 65535)) throw new Error('version must be major.minor.patch (0..65535)');
  if (!['amd64', 'arm64'].includes(arch)) throw new Error('Unsupported architecture');
}
try {
  check(); mkdirSync('bin', { recursive: true });
  const command = process.argv[2];
  if (command === 'desktop' || command === 'desktop-dev') {
    if (!windows) throw new Error('Windows desktop build must be run on Windows.');
    const production = command === 'desktop';
    // Identity and version come from app.json; the manifest file is a template.
    const manifest = readFileSync('build/windows/app.manifest', 'utf8').replaceAll('__APP_ID__', app.id).replaceAll('__APP_VERSION__', app.version);
    writeFileSync('bin/app.manifest', manifest);
    run(cli, ['generate', 'syso', '-manifest', 'bin/app.manifest', '-icon', 'build/windows/app.ico', '-arch', arch, '-out', `rsrc_windows_${arch}.syso`]);
    run('go', ['build', '-trimpath', ...(production ? ['-tags', 'production'] : []), '-ldflags', '-H windowsgui', '-o', target, '.'], { env: { ...process.env, GOOS: 'windows', GOARCH: arch, CGO_ENABLED: '0' } });
  } else if (command === 'server') {
    run('go', ['build', '-trimpath', '-tags', 'server,production', '-o', server, '.'], { env: { ...process.env, CGO_ENABLED: '0' } });
  } else if (command === 'server-e2e') {
    run('go', ['build', '-trimpath', '-tags', 'server,production,e2e', '-o', serverE2E, '.'], { env: { ...process.env, CGO_ENABLED: '0' } });
  } else if (['run-server-review', 'run-server-review-foundry-change', 'run-server-review-foundry-revisit', 'run-server-review-foundry-refresh'].includes(command)) {
    // Screen review only, not a production path: the e2e build starts signed in
    // from a fixed record in a fixed temporary data directory.
    const foundryChangeReview = command === 'run-server-review-foundry-change';
    const foundryRevisitReview = command === 'run-server-review-foundry-revisit';
    const foundryRefreshReview = command === 'run-server-review-foundry-refresh';
    const dataDir = join(tmpdir(), `${app.id}-${foundryRefreshReview ? 'foundry-refresh-review' : foundryRevisitReview ? 'foundry-revisit-review' : foundryChangeReview ? 'foundry-change-review' : 'review'}`);
    // The refresh review resets its saved state on each launch.
    if (foundryRefreshReview) rmSync(dataDir, { recursive: true, force: true });
    mkdirSync(dataDir, { recursive: true });
    writeFileSync(join(dataDir, 'e2e-authentication-record.json'), JSON.stringify({
      authority: 'login.microsoftonline.com', clientId: 'e2e-client', homeAccountId: 'e2e-object.e2e-tenant',
      tenantId: 'e2e-tenant', username: 'operator@contoso.onmicrosoft.com', version: '1.0',
    }));
    if (foundryRevisitReview) {
      // File boundary only: reset the review fixture on launch, then use the real
      // Go read/change/save operations and the production frontend unchanged.
      const foundries = [
        { id: '/subscriptions/saved-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast', name: 'contoso-foundry-production-japaneast', subscriptionName: 'Contoso AI Production Subscription', resourceGroupName: 'rg-ai-production-japaneast' },
        { id: '/subscriptions/saved-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development', name: 'contoso-foundry-development', subscriptionName: 'Contoso Development', resourceGroupName: 'rg-ai-development' },
      ];
      const models = [
        [{ id: `${foundries[0].id}/deployments/saved-production-chat`, deploymentName: 'saved-production-chat', modelName: 'gpt-4.1', version: '2025-04-14' }],
        [
          { id: `${foundries[1].id}/deployments/saved-development-chat`, deploymentName: 'saved-development-chat', modelName: 'gpt-4.1-mini', version: '2025-04-14' },
          { id: `${foundries[1].id}/deployments/saved-development-embedding`, deploymentName: 'saved-development-embedding', modelName: 'text-embedding-3-large', version: '1' },
        ],
      ];
      const modelsDir = join(dataDir, 'foundry-models');
      mkdirSync(modelsDir, { recursive: true });
      for (const [index, foundry] of foundries.entries()) {
        const hash = createHash('sha256').update(foundry.id).digest('hex');
        writeFileSync(join(modelsDir, `${hash}.json`), JSON.stringify({ fetchedAt: '2026-09-01T09:00:00+09:00', deployments: models[index] }, null, 2));
      }
      writeFileSync(join(dataDir, 'foundry-state.json'), JSON.stringify({
        foundries, selectedFoundryId: foundries[0].id, deployments: models[0],
        foundriesFetchedAt: '2026-09-01T09:00:00+09:00', deploymentsFetchedAt: '2026-09-01T09:00:00+09:00',
      }, null, 2));
    }
    if (foundryRefreshReview) {
      // File boundary only: a saved state with past fetch times and a Legacy Foundry
      // that the e2e fixed source no longer returns. Other IDs match the fixed source.
      const foundries = [
        { id: '/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast', name: 'contoso-foundry-production-japaneast', subscriptionName: 'Contoso AI Production Subscription', resourceGroupName: 'rg-ai-production-japaneast' },
        { id: '/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development', name: 'contoso-foundry-development', subscriptionName: 'Contoso Development', resourceGroupName: 'rg-ai-development' },
        { id: '/subscriptions/review-legacy/resourceGroups/rg-ai-legacy/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-legacy', name: 'contoso-foundry-legacy', subscriptionName: 'Contoso Legacy', resourceGroupName: 'rg-ai-legacy' },
      ];
      // Saved model files exist for Production and Legacy.
      const owners = [foundries[0], foundries[2]];
      const models = [
        [['chat-production', 'gpt-4.1', '2025-04-14'], ['chat-mini', 'gpt-4.1-mini', '2025-04-14'], ['embeddings', 'text-embedding-3-large', '1']],
        [['legacy-chat', 'gpt-4o', '2024-11-20']],
      ].map((rows, index) => rows.map(([deploymentName, modelName, version]) => ({ id: `${owners[index].id}/deployments/${deploymentName}`, deploymentName, modelName, version })));
      const fetchedAt = '2026-09-01T09:00:00+09:00';
      const modelsDir = join(dataDir, 'foundry-models');
      mkdirSync(modelsDir, { recursive: true });
      for (const [index, deployments] of models.entries()) {
        const hash = createHash('sha256').update(owners[index].id).digest('hex');
        writeFileSync(join(modelsDir, `${hash}.json`), JSON.stringify({ fetchedAt, deployments }, null, 2));
      }
      writeFileSync(join(dataDir, 'foundry-state.json'), JSON.stringify({
        foundries, selectedFoundryId: foundries[0].id, deployments: models[0], foundriesFetchedAt: fetchedAt, deploymentsFetchedAt: fetchedAt,
      }, null, 2));
    }
    console.log(`review data directory: ${dataDir}`);
    run(serverE2E, [], { env: { ...process.env, WAILS_DATA_DIR: dataDir, ...(foundryChangeReview || foundryRevisitReview || foundryRefreshReview ? { WAILS_SERVER_PORT: '34116' } : {}), ...(foundryRevisitReview ? { AZFOUNDRYDECK_E2E_HOLD_FOUNDRY: '1' } : {}) } });
  } else if (command === 'run' || command === 'run-server') {
    run({ run: target, 'run-server': server }[command], []);
  } else if (command === 'package') {
    if (!windows) throw new Error('NSIS packaging must be run on Windows.');
    const include = [
      ['APP_ID', app.id], ['APP_NAME', app.name], ['APP_EXE', app.executable],
      ['APP_VERSION', app.version], ['APP_ARCH', arch],
      ['INSTALLER_NAME', `${app.executable.slice(0, -4)}-${app.version}-${arch}-setup.exe`],
    ].map(([key, value]) => `!define ${key} "${safeNSIS(value)}"`).join('\n') + '\n';
    writeFileSync('build/windows/nsis/app.nsh', '\ufeff' + include);
    const nsisCandidates = [process.env.NSIS_EXE,
      process.env['ProgramFiles(x86)'] && resolve(process.env['ProgramFiles(x86)'], 'NSIS/makensis.exe'),
      process.env.ProgramFiles && resolve(process.env.ProgramFiles, 'NSIS/makensis.exe')];
    const nsis = nsisCandidates.find(p => p && existsSync(p)) || 'makensis';
    run(nsis, ['/V3', 'build/windows/nsis/project.nsi']);
  } else throw new Error(`Unknown internal build action: ${command}`);
} catch (error) { console.error(error instanceof Error ? error.message : error); process.exitCode = 1; }
