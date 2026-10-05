import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
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
  } else if (['run-server-review-login', 'run-server-review-login-multiple'].includes(command)) {
    const multiple = command === 'run-server-review-login-multiple';
    const dataDir = mkdtempSync(join(tmpdir(), `${app.id}-login-review-`));
    console.log(`review data directory: ${dataDir}`);
    run(serverE2E, [], { env: { ...process.env, WAILS_DATA_DIR: dataDir, WAILS_SERVER_PORT: multiple ? '34118' : '34117', AZFOUNDRYDECK_E2E_TENANTS: multiple ? 'multiple' : '' } });
  } else if (['run-server-review', 'run-server-review-foundry-change', 'run-server-review-foundry-refresh', 'run-server-review-foundry-add', 'run-server-review-foundry-delete', 'run-server-review-deployment-refresh', 'run-server-review-tenant-change', 'run-server-review-no-foundry', 'run-server-review-foundry-empty', 'run-server-review-tenant-revisit', 'run-server-review-deployment-detail', 'run-server-review-deployment-delete', 'run-server-review-deployment-add', 'run-server-review-deployment-update'].includes(command)) {
    // Screen review only, not a production path: the e2e build starts signed in
    // from a fixed record in a fixed temporary data directory.
    const foundryChangeReview = command === 'run-server-review-foundry-change';
    const foundryAddReview = command === 'run-server-review-foundry-add';
    const foundryDeleteReview = command === 'run-server-review-foundry-delete';
    const deploymentRefreshReview = command === 'run-server-review-deployment-refresh';
    // The add and delete reviews start from an empty temporary folder like the detail review.
    const deploymentAddReview = command === 'run-server-review-deployment-add';
    const deploymentDeleteReview = command === 'run-server-review-deployment-delete';
    const deploymentUpdateReview = command === 'run-server-review-deployment-update';
    const deploymentDetailReview = command === 'run-server-review-deployment-detail' || deploymentDeleteReview || deploymentAddReview || deploymentUpdateReview;
    if (command === 'run-server-review-deployment-detail') process.env.AZFOUNDRYDECK_E2E_CAPACITY_REVIEW = '1';
    const tenantRevisitReview = command === 'run-server-review-tenant-revisit';
    // The revisit review is the tenant change fixture plus saved views of another tenant.
    const tenantChangeReview = command === 'run-server-review-tenant-change' || tenantRevisitReview;
    const noFoundryReview = command === 'run-server-review-no-foundry';
    const foundryEmptyReview = command === 'run-server-review-foundry-empty';
    // The deployment refresh review reuses the Foundry refresh fixture.
    const foundryRefreshReview = command === 'run-server-review-foundry-refresh' || deploymentRefreshReview || foundryEmptyReview;
    const dataDir = foundryAddReview
      ? mkdtempSync(join(tmpdir(), `${app.id}-foundry-add-review-`))
      : foundryDeleteReview
      ? mkdtempSync(join(tmpdir(), `${app.id}-foundry-delete-review-`))
      : deploymentDetailReview
      ? mkdtempSync(join(tmpdir(), `${app.id}-deployment-${deploymentUpdateReview ? 'update' : deploymentAddReview ? 'add' : deploymentDeleteReview ? 'delete' : 'detail'}-review-`))
      : join(tmpdir(), `${app.id}-${tenantRevisitReview ? 'tenant-revisit-review' : foundryEmptyReview ? 'foundry-empty-review' : noFoundryReview ? 'no-foundry-review' : tenantChangeReview ? 'tenant-change-review' : deploymentRefreshReview ? 'deployment-refresh-review' : foundryRefreshReview ? 'foundry-refresh-review' : foundryChangeReview ? 'foundry-change-review' : 'review'}`);
    if (deploymentDetailReview || foundryDeleteReview) console.log(`review data directory: ${dataDir}`);
    // The refresh review resets its saved state on each launch.
    if (foundryRefreshReview || tenantChangeReview || noFoundryReview) rmSync(dataDir, { recursive: true, force: true });
    mkdirSync(dataDir, { recursive: true });
    writeFileSync(join(dataDir, 'e2e-authentication-record.json'), JSON.stringify({
      authority: 'login.microsoftonline.com', clientId: 'e2e-client', homeAccountId: 'e2e-object.e2e-tenant',
      tenantId: 'e2e-tenant', username: 'operator@contoso.onmicrosoft.com', version: '1.0',
      tenants: tenantChangeReview
        ? [{ id: 'e2e-azure-tenant', displayName: 'Contoso' }, { id: 'e2e-fabrikam-tenant', displayName: 'Fabrikam' }, { id: 'e2e-northwind-tenant', displayName: 'Northwind' }]
        : [{ id: 'e2e-azure-tenant', displayName: 'Contoso' }],
      selectedTenantId: 'e2e-azure-tenant',
    }));
    // Saved views live under the signed-in account and selected tenant.
    const viewDir = join(dataDir, 'azure-views', createHash('sha256').update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-azure-tenant'])).digest('hex'));
    if (tenantRevisitReview) {
      // File boundary only: Fabrikam already has a saved Foundry list with its second Foundry
      // selected; the deployments are always fetched from the fixed source.
      const fabrikamDir = join(dataDir, 'azure-views', createHash('sha256').update(JSON.stringify(['e2e-object.e2e-tenant', 'e2e-fabrikam-tenant'])).digest('hex'));
      const foundries = [
        { id: '/subscriptions/saved-fabrikam/resourceGroups/rg-fabrikam-production/providers/Microsoft.CognitiveServices/accounts/fabrikam-foundry-production', name: 'fabrikam-foundry-production', subscriptionName: 'Fabrikam Production Subscription', resourceGroupName: 'rg-fabrikam-production' },
        { id: '/subscriptions/saved-fabrikam/resourceGroups/rg-fabrikam-research/providers/Microsoft.CognitiveServices/accounts/fabrikam-foundry-research', name: 'fabrikam-foundry-research', subscriptionName: 'Fabrikam Research Subscription', resourceGroupName: 'rg-fabrikam-research' },
      ];
      const fetchedAt = '2026-09-01T09:00:00+09:00';
      mkdirSync(fabrikamDir, { recursive: true });
      writeFileSync(join(fabrikamDir, 'foundry-state.json'), JSON.stringify({
        foundries, selectedFoundryId: foundries[1].id, foundriesFetchedAt: fetchedAt,
      }, null, 2));
    }
    if (foundryRefreshReview) {
      // File boundary only: a saved state with a past fetch time and a Legacy Foundry
      // that the e2e fixed source no longer returns. Other IDs match the fixed source.
      const foundries = [
        { id: '/subscriptions/review-production/resourceGroups/rg-ai-production-japaneast/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-production-japaneast', name: 'contoso-foundry-production-japaneast', subscriptionName: 'Contoso AI Production Subscription', resourceGroupName: 'rg-ai-production-japaneast' },
        { id: '/subscriptions/review-development/resourceGroups/rg-ai-development/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-development', name: 'contoso-foundry-development', subscriptionName: 'Contoso Development', resourceGroupName: 'rg-ai-development' },
        { id: '/subscriptions/review-legacy/resourceGroups/rg-ai-legacy/providers/Microsoft.CognitiveServices/accounts/contoso-foundry-legacy', name: 'contoso-foundry-legacy', subscriptionName: 'Contoso Legacy', resourceGroupName: 'rg-ai-legacy' },
      ];
      const fetchedAt = '2026-09-01T09:00:00+09:00';
      writeFileSync(join(viewDir, 'foundry-state.json'), JSON.stringify({
        foundries, selectedFoundryId: foundries[0].id, foundriesFetchedAt: fetchedAt,
      }, null, 2));
    }
    console.log(`review data directory: ${dataDir}`);
    run(serverE2E, [], { env: { ...process.env, WAILS_DATA_DIR: dataDir, ...(deploymentDetailReview ? { WAILS_SERVER_PORT: deploymentUpdateReview ? '34126' : deploymentAddReview ? '34125' : deploymentDeleteReview ? '34124' : '34123' } : {}), ...(foundryChangeReview || foundryRefreshReview ? { WAILS_SERVER_PORT: '34116' } : {}), ...(foundryAddReview ? { WAILS_SERVER_PORT: '34127', AZFOUNDRYDECK_E2E_FOUNDRY_ADD_REVIEW: '1', AZFOUNDRYDECK_E2E_FOUNDRIES: 'none' } : {}), ...(foundryDeleteReview ? { WAILS_SERVER_PORT: '34128', AZFOUNDRYDECK_E2E_FOUNDRY_DELETE_REVIEW: '1' } : {}), ...(tenantChangeReview ? { WAILS_SERVER_PORT: tenantRevisitReview ? '34122' : '34119' } : {}), ...(noFoundryReview ? { WAILS_SERVER_PORT: '34120', AZFOUNDRYDECK_E2E_FOUNDRIES: 'none' } : {}), ...(foundryEmptyReview ? { WAILS_SERVER_PORT: '34121', AZFOUNDRYDECK_E2E_FOUNDRIES: 'none' } : {}), } });
  } else if (['run-server-review-update', 'run-server-review-update-untrusted'].includes(command)) {
    // Screen review only: signed in from a fixed record, with a signed v0.2.0 in a local
    // folder as the latest release. The installer is never executed.
    const untrusted = command === 'run-server-review-update-untrusted';
    const dataDir = mkdtempSync(join(tmpdir(), `${app.id}-update-review-`));
    writeFileSync(join(dataDir, 'e2e-authentication-record.json'), JSON.stringify({
      authority: 'login.microsoftonline.com', clientId: 'e2e-client', homeAccountId: 'e2e-object.e2e-tenant',
      tenantId: 'e2e-tenant', username: 'operator@contoso.onmicrosoft.com', version: '1.0',
      tenants: [{ id: 'e2e-azure-tenant', displayName: 'Contoso' }], selectedTenantId: 'e2e-azure-tenant',
    }));
    console.log(`review data directory: ${dataDir}`);
    run(serverE2E, [], { env: { ...process.env, WAILS_DATA_DIR: dataDir, WAILS_SERVER_PORT: untrusted ? '34130' : '34129', AZFOUNDRYDECK_E2E_UPDATE: untrusted ? 'untrusted' : 'ready' } });
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
