import { test as base, expect } from '@playwright/test';
import { spawn, type ChildProcess } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { mkdtemp, rm } from 'node:fs/promises';
import { createServer, type AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

// Each test owns a server-mode Go process, data directory and port.
const root = resolve(import.meta.dirname, '../../..');
const { executable } = JSON.parse(readFileSync(join(root, 'build/app.json'), 'utf8')) as {
  executable: string;
};
// Built with the e2e tag, which fixes the Entra ID / ARM and Credential Manager boundaries.
const server = join(
  root,
  'bin',
  executable.slice(0, -4) + '-server-e2e' + (process.platform === 'win32' ? '.exe' : ''),
);

export interface IsolatedApp {
  /** Changes on restart because each start listens on a new free port. */
  url: string;
  dataDir: string;
  restart: () => Promise<void>;
}

async function freePort() {
  const listener = createServer();
  await new Promise<void>((ready) => listener.listen(0, '127.0.0.1', ready));
  const { port } = listener.address() as AddressInfo;
  await new Promise((closed) => listener.close(closed));
  return port;
}

export const test = base.extend<{ app: IsolatedApp; serverEnv: Record<string, string> }>({
  /** Extra environment for the server, e.g. AZFOUNDRYDECK_E2E_FAIL to inject a failure. */
  serverEnv: [{}, { option: true }],
  app: async ({ serverEnv }, provide) => {
    const dataDir = await mkdtemp(join(tmpdir(), 'wails-e2e-'));
    let child: ChildProcess | undefined;
    let output = '';
    async function start() {
      const port = await freePort();
      const running = spawn(server, [], {
        cwd: root,
        stdio: ['ignore', 'pipe', 'pipe'],
        env: {
          ...process.env,
          ...serverEnv,
          WAILS_DATA_DIR: dataDir,
          WAILS_SERVER_PORT: String(port),
        },
      });
      child = running;
      running.stdout?.on('data', (chunk) => {
        output += chunk;
      });
      running.stderr?.on('data', (chunk) => {
        output += chunk;
      });
      app.url = `http://127.0.0.1:${port}`;
      const deadline = Date.now() + 30_000;
      while (true) {
        if (running.exitCode !== null)
          throw new Error(`E2Eサーバーが起動中に終了しました: ${output}`);
        if (Date.now() > deadline)
          throw new Error(`E2Eサーバーの起動が期限を超えました: ${output}`);
        const healthy = await fetch(`${app.url}/health`).then(
          (response) => response.ok,
          () => false,
        );
        if (healthy) return;
        await new Promise((wait) => setTimeout(wait, 100));
      }
    }
    async function stop() {
      const running = child;
      child = undefined;
      if (!running || running.exitCode !== null || running.signalCode !== null) return;
      // Wait for exit before the data directory is reused or removed.
      await new Promise((exited) => {
        running.once('exit', exited);
        running.kill();
      });
    }
    const app: IsolatedApp = {
      url: '',
      dataDir,
      restart: async () => {
        await stop();
        await start();
      },
    };
    try {
      await start();
      await provide(app);
    } finally {
      await stop();
      await rm(dataDir, { recursive: true, force: true });
    }
  },
  baseURL: async ({ app }, provide) => {
    await provide(app.url);
  },
});

export { expect };
