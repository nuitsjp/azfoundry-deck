import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import { createHash, generateKeyPairSync, sign, type KeyObject } from 'node:crypto';
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import {
  appProcesses,
  launch,
  powershell,
  pressButton,
  waitFor,
  windowState,
} from '../../support/desktop';

// Installs the real desktop build, so it runs only through `mise run test:desktop`.
// The production build reads the sign-in of this Windows user from the Credential
// Manager; sign in with the installed app before running. Saved views, logs and the
// staged installer go to a temporary data folder. The update source is a local folder
// fixed into both builds in place of GitHub Releases.
const root = resolve(import.meta.dirname, '../../../../..');
const appID = 'AzFoundryDeck';
const installDir = join(process.env.LOCALAPPDATA ?? '', 'Programs', appID);
const installedExe = join(installDir, 'azfoundrydeck.exe');
const installer = (version: string) => `azfoundrydeck-${version}-amd64-setup.exe`;
const installedVersion = () =>
  powershell(
    `(Get-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\${appID}' -ErrorAction SilentlyContinue).DisplayVersion`,
  );
const ready = (version: string) => `バージョン ${version} の準備ができました。`;
const verifyFailed = '更新を検証できませんでした。次回の起動時に確認し直します。';

function build(version: string, source: string, publicKey: string) {
  const result = spawnSync('node', ['scripts/run.mjs', 'package'], {
    cwd: root,
    stdio: 'inherit',
    env: {
      ...process.env,
      BUILD_APP_VERSION: version,
      BUILD_UPDATE_SOURCE: source,
      BUILD_UPDATE_PUBLIC_KEY: publicKey,
    },
  });
  if (result.status !== 0) throw new Error(`package ${version} failed`);
  return join(root, 'bin', installer(version));
}

// The same envelope as cmd/release: the signature covers the exact payload bytes.
function publishRelease(source: string, file: string, version: string, key: KeyObject) {
  const data = readFileSync(file);
  const payload = Buffer.from(
    JSON.stringify({
      appID,
      version,
      os: 'windows',
      arch: 'amd64',
      filename: installer(version),
      size: data.length,
      sha256: createHash('sha256').update(data).digest('hex'),
      notes: '',
    }),
  );
  if (file !== join(source, installer(version)))
    copyFileSync(file, join(source, installer(version)));
  writeFileSync(
    join(source, 'update.json'),
    JSON.stringify({
      payload: payload.toString('base64'),
      signature: sign(null, payload, key).toString('base64'),
    }),
  );
}

const stagedFiles = (dataDir: string) => {
  const updates = join(dataDir, 'updates');
  return existsSync(updates)
    ? readdirSync(updates, { recursive: true })
        .map(String)
        .filter((name) => name.endsWith('.exe'))
    : [];
};
const logEntries = (dataDir: string) => {
  const log = join(dataDir, 'logs', 'app.jsonl');
  return existsSync(log)
    ? readFileSync(log, 'utf8')
        .split('\n')
        .filter(Boolean)
        .map((line) => JSON.parse(line) as Record<string, unknown>)
    : [];
};

async function restart(env: NodeJS.ProcessEnv) {
  for (const pid of appProcesses(installedExe)) process.kill(pid);
  await waitFor('the app to exit', () => appProcesses(installedExe).length === 0);
  launch(installedExe, env);
  return (await waitFor('the app to start', () => appProcesses(installedExe)))[0];
}

function uninstall() {
  for (const pid of appProcesses(installedExe)) process.kill(pid);
  if (!existsSync(join(installDir, 'uninstall.exe'))) return;
  // A copy runs the uninstaller in place, so the call returns after the files are removed.
  const copy = join(tmpdir(), 'azfoundrydeck-e2e-uninstall.exe');
  copyFileSync(join(installDir, 'uninstall.exe'), copy);
  spawnSync(copy, ['/S', `_?=${installDir}`]);
  rmSync(copy, { force: true });
  rmSync(installDir, { recursive: true, force: true });
}

test('起動時に取得した新版で更新して再起動する @desktop', async () => {
  test.skip(process.env.DESKTOP_E2E !== '1', 'installs the desktop app; run mise run test:desktop');
  test.setTimeout(15 * 60_000);
  expect(
    powershell('@(Get-Process azfoundrydeck -ErrorAction SilentlyContinue).Count'),
    'exit Azure Foundry Deck first',
  ).toBe('0');
  expect(existsSync(installedExe), 'uninstall Azure Foundry Deck first').toBe(false);
  const work = mkdtempSync(join(tmpdir(), 'azfoundrydeck-update-e2e-'));
  const source = join(work, 'release');
  const dataDir = join(work, 'data');
  const env = { ...process.env, WAILS_DATA_DIR: dataDir };
  mkdirSync(source);
  const { publicKey, privateKey } = generateKeyPairSync('ed25519');
  let oldPid = 0;
  try {
    await test.step('開始条件', async () => {
      const rawKey = publicKey
        .export({ format: 'der', type: 'spki' })
        .subarray(-32)
        .toString('base64');
      const current = join(work, installer('0.1.0'));
      copyFileSync(build('0.1.0', source, rawKey), current);
      publishRelease(source, build('0.2.0', source, rawKey), '0.2.0', privateKey);
      expect(spawnSync(current, ['/S']).status).toBe(0);
      expect(installedVersion()).toBe('0.1.0');
    });
    await test.step('手順1', async () => {
      launch(installedExe, env);
      oldPid = (await waitFor('the app to start', () => appProcesses(installedExe)))[0];
      // Home shows with the saved sign-in of this Windows user.
      expect(windowState(oldPid).names, 'sign in with the installed app first').toContain(
        'アカウント',
      );
      const expected = createHash('sha256')
        .update(readFileSync(join(source, installer('0.2.0'))))
        .digest('hex');
      await waitFor('the staged installer', () => {
        const file = stagedFiles(dataDir).find((name) => name.endsWith(installer('0.2.0')));
        return (
          file &&
          createHash('sha256')
            .update(readFileSync(join(dataDir, 'updates', file)))
            .digest('hex') === expected
        );
      });
    });
    await test.step('手順2', async () => {
      await waitFor('the update section', () => windowState(oldPid).text.includes(ready('0.2.0')));
      const { names } = windowState(oldPid);
      expect(names).toContain('更新して再起動');
      // The section is at the top of Home, above the Foundry list.
      expect(names.indexOf('更新して再起動')).toBeLessThan(names.indexOf('Foundry'));
    });
    await test.step('手順3', async () => {
      pressButton(oldPid, '更新して再起動');
      // The installer runs without a window while the old version exits.
      expect(
        powershell(
          `@(Get-Process 'azfoundrydeck-*-setup' -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 }).Count`,
        ),
      ).toBe('0');
      await waitFor('the old version to exit', () => !appProcesses(installedExe).includes(oldPid));
    });
    await test.step('手順4', async () => {
      await waitFor(
        'the new version to start',
        () => appProcesses(installedExe).length > 0,
        120_000,
      );
      expect(installedVersion()).toBe('0.2.0');
    });
    await test.step('手順5', async () => {
      const pid = appProcesses(installedExe)[0];
      expect(windowState(pid).names).toContain('アカウント');
      await waitFor('the check of the new version', () =>
        logEntries(dataDir).some(
          (entry) => entry.msg === 'update_checked' && entry.phase === 'checked',
        ),
      );
      expect(
        logEntries(dataDir).some((entry) => entry.msg === 'starting' && entry.version === '0.2.0'),
      ).toBe(true);
      expect(windowState(pid).text).not.toContain('準備ができました');
      expect(stagedFiles(dataDir)).toEqual([]);
    });
    await test.step('受け入れ条件', async () => {
      // A signature that does not match: no section, only the failure class in the log,
      // and nothing staged.
      publishRelease(
        source,
        join(source, installer('0.2.0')),
        '0.3.0',
        generateKeyPairSync('ed25519').privateKey,
      );
      let pid = await restart(env);
      await waitFor('the failed check', () =>
        logEntries(dataDir).some(
          (entry) => entry.msg === 'update_not_staged' && entry.code === 'UPDATE_UNTRUSTED',
        ),
      );
      expect(windowState(pid).text).not.toContain('準備ができました');
      expect(stagedFiles(dataDir)).toEqual([]);

      // An installer changed after staging: the app keeps running and shows the failure.
      publishRelease(source, join(source, installer('0.2.0')), '0.3.0', privateKey);
      pid = await restart(env);
      await waitFor('the update section', () => windowState(pid).text.includes(ready('0.3.0')));
      const staged = stagedFiles(dataDir).find((name) => name.endsWith(installer('0.3.0')));
      expect(staged).toBeDefined();
      writeFileSync(join(dataDir, 'updates', staged ?? ''), 'altered');
      pressButton(pid, '更新して再起動');
      await waitFor('the verification failure', () => windowState(pid).text.includes(verifyFailed));
      expect(appProcesses(installedExe)).toContain(pid);
      expect(installedVersion()).toBe('0.2.0');
    });
  } finally {
    uninstall();
    rmSync(work, { recursive: true, force: true });
  }
  expect(existsSync(installedExe)).toBe(false);
});
