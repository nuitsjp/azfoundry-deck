import { spawn, spawnSync } from 'node:child_process';

// Drives the installed desktop app through Windows UI Automation. The production build
// has no remote debugging port, so the window is read as Windows sees it.
export function powershell(script: string): string {
  const result = spawnSync(
    'powershell.exe',
    [
      '-NoProfile',
      '-NonInteractive',
      '-EncodedCommand',
      // The window text is Japanese; the console would otherwise write it in the OEM code page.
      Buffer.from(
        `[Console]::OutputEncoding = [Text.Encoding]::UTF8\n${script}`,
        'utf16le',
      ).toString('base64'),
    ],
    { encoding: 'utf8' },
  );
  if (result.status !== 0) throw new Error(`PowerShell failed: ${result.stderr || result.stdout}`);
  return result.stdout.trim();
}

const quote = (value: string) => `'${value.replaceAll("'", "''")}'`;

export function launch(exe: string, env: NodeJS.ProcessEnv) {
  spawn(exe, [], { env, detached: true, stdio: 'ignore' }).unref();
}

export function appProcesses(exe: string): number[] {
  const out = powershell(
    `@(Get-Process azfoundrydeck -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq ${quote(exe)} } | ForEach-Object { $_.Id }) -join ','`,
  );
  return out ? out.split(',').map(Number) : [];
}

// React may split one sentence into several text nodes; `text` joins them back.
export interface WindowState {
  names: string[];
  text: string;
}

// Reads the window once Home has loaded (the Foundry heading is present when signed in).
// WebView2 builds its accessibility tree on the first request, so it retries.
export function windowState(pid: number): WindowState {
  return JSON.parse(
    powershell(`
Add-Type -AssemblyName UIAutomationClient,UIAutomationTypes
$A = [System.Windows.Automation.AutomationElement]
$win = $null
for ($i = 0; $i -lt 60 -and -not $win; $i++) {
  $win = $A::RootElement.FindFirst('Children', (New-Object System.Windows.Automation.PropertyCondition($A::ProcessIdProperty, ${pid})))
  if (-not $win) { Start-Sleep -Milliseconds 500 }
}
if (-not $win) { throw 'window not found' }
$names = @()
for ($i = 0; $i -lt 60; $i++) {
  $names = @($win.FindAll('Descendants', [System.Windows.Automation.Condition]::TrueCondition) | ForEach-Object { $_.Current.Name } | Where-Object { $_ })
  if ($names -contains 'Foundry') { break }
  Start-Sleep -Milliseconds 500
}
@{ names = $names; text = ($names -join '') } | ConvertTo-Json -Compress
`),
  ) as WindowState;
}

export function pressButton(pid: number, name: string) {
  powershell(`
Add-Type -AssemblyName UIAutomationClient,UIAutomationTypes
$A = [System.Windows.Automation.AutomationElement]
$win = $A::RootElement.FindFirst('Children', (New-Object System.Windows.Automation.PropertyCondition($A::ProcessIdProperty, ${pid})))
$button = $win.FindFirst('Descendants', (New-Object System.Windows.Automation.PropertyCondition($A::NameProperty, ${quote(name)})))
if (-not $button) { throw 'button not found' }
$button.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
`);
}

export async function waitFor<T>(
  what: string,
  probe: () => T | undefined | false,
  timeoutMs = 60_000,
): Promise<T> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const value = probe();
    if (value) return value;
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${what}`);
    await new Promise((done) => setTimeout(done, 500));
  }
}
