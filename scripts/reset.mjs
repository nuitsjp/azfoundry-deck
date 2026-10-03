// Windows開発用：アプリの保存データと認証情報をすべて消し、初期状態に戻す。
import { spawnSync } from 'node:child_process';
import { readdirSync, readFileSync, rmSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

if (process.platform !== 'win32') throw new Error('reset は Windows 専用です。');
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const { id } = JSON.parse(readFileSync(join(root, 'build', 'app.json'), 'utf8'));

// 1. 保存データ（閲覧保存・ログ・WebView2データ）。WAILS_DATA_DIR 指定時はそちらを使う。
const dataDir = process.env.WAILS_DATA_DIR || join(process.env.APPDATA, id);
rmSync(dataDir, { recursive: true, force: true });
console.log(`削除: ${dataDir}`);

// 2. トークンキャッシュ。.IdentityService は他ツールと共有のため、このアプリのファイルだけを消す。
const cacheDir = join(process.env.LOCALAPPDATA, '.IdentityService');
try {
  for (const name of readdirSync(cacheDir)) {
    if (name === 'azfoundrydeck' || name.startsWith('azfoundrydeck.')) {
      rmSync(join(cacheDir, name), { force: true });
      console.log(`削除: ${join(cacheDir, name)}`);
    }
  }
} catch (error) {
  if (error.code !== 'ENOENT') throw error;
}

// 3. 資格情報マネージャーのログイン記録。
const target = `${id}:AuthenticationRecord`;
const result = spawnSync('cmdkey', [`/delete:${target}`], { encoding: 'utf8' });
console.log(result.status === 0 ? `削除: 資格情報 ${target}` : `資格情報 ${target} はありません。`);
