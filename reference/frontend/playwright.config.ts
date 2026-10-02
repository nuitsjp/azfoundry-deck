import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
  // Each test starts its own server through tests/e2e/fixtures.ts, so tests run in parallel.
  testDir: './tests/e2e',
  fullyParallel: true,
  retries: 0,
  forbidOnly: !!process.env.CI,
  use: {
    ...devices['Desktop Chrome'],
    headless: true,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    launchOptions: {
      executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
      ignoreDefaultArgs:
        process.env.PLAYWRIGHT_IGNORE_DISABLE_EXTENSIONS === '1'
          ? ['--disable-extensions']
          : undefined,
    },
  },
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]],
  outputDir: 'test-results',
});
