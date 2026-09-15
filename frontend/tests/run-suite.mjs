// Runs one browser scenario file against an already running server.
//
//   node frontend/tests/run-suite.mjs <url> <test file> <result file>
//
// The Playwright CLI is not used here: on this machine its Edge channel exits
// with 3221225477 and its session daemon does not return when the bundled
// Chromium is selected. This driver launches the bundled Chromium directly, so
// it uses a temporary browser and never touches a signed-in profile.
import { chromium } from 'playwright-core';
import { readFileSync, writeFileSync } from 'node:fs';

const [, , url, file, out] = process.argv;
if (!url || !file || !out) {
  console.error('usage: node frontend/tests/run-suite.mjs <url> <test file> <result file>');
  process.exit(2);
}

const browser = await chromium.launch({ headless: true });
const page = await (await browser.newContext({ viewport: { width: 1440, height: 920 } })).newPage();
let result;
try {
  await page.goto(url, { waitUntil: 'networkidle' });
  await page.waitForTimeout(1000);
  result = await eval('(' + readFileSync(file, 'utf8') + ')')(page);
} catch (error) {
  result = { passed: false, error: String(error).split('\n').slice(0, 6).join(' | ') };
}
await browser.close();

writeFileSync(out, JSON.stringify(result, null, 2));
console.log(JSON.stringify({
  file,
  passed: result.passed === true,
  checks: result.checks?.length ?? 0,
  pageErrors: result.pageErrors?.length ?? null,
  error: result.error ?? null,
}));
process.exit(result.passed === true ? 0 : 1);
