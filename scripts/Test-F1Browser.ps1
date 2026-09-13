$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $repoRoot
$cli = Join-Path $repoRoot 'frontend/node_modules/@playwright/cli/playwright-cli.js'
$session = "f1-ui-$PID"
$serverProcess = $null
$previousEnvironment = @{}
foreach ($name in @('AZFOUNDRY_MOCK', 'WAILS_SERVER_HOST', 'WAILS_SERVER_PORT')) {
    $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}

try {
    if (Get-NetTCPConnection -LocalPort 9245 -State Listen -ErrorAction SilentlyContinue) {
        throw 'Port 9245 is in use. Stop the browser mock before running test:ui.'
    }
    $env:AZFOUNDRY_MOCK = '1'
    $env:WAILS_SERVER_HOST = '127.0.0.1'
    $env:WAILS_SERVER_PORT = '9245'
    $serverProcess = Start-Process -FilePath (Join-Path $repoRoot 'bin/azfoundry-deck-browser.exe') -WorkingDirectory $repoRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput bin/f1-test-server-stdout.log -RedirectStandardError bin/f1-test-server-stderr.log
    $ready = $false
    for ($attempt = 0; $attempt -lt 100; $attempt++) {
        if ($serverProcess.HasExited) {
            throw "Wails server exited: $(Get-Content -Raw bin/f1-test-server-stderr.log)"
        }
        try {
            $ready = (Invoke-WebRequest -Uri 'http://127.0.0.1:9245/' -NoProxy -TimeoutSec 1).StatusCode -eq 200
        } catch { $ready = $false }
        if ($ready) { break }
        Start-Sleep -Milliseconds 100
    }
    if (!$ready -or $serverProcess.HasExited) { throw 'Wails server did not start on 127.0.0.1:9245.' }

    & node $cli "-s=$session" open http://127.0.0.1:9245 --config=frontend/tests/browser.config.json
    if ($LASTEXITCODE -ne 0) { throw 'Headless browser launch failed.' }
    $output = & node $cli "-s=$session" --json run-code --filename=frontend/tests/f1.browser.js
    $output | Set-Content -LiteralPath docs/verification/f1-browser-result.json -Encoding utf8NoBOM
    if ($LASTEXITCODE -ne 0) { throw ($output -join "`n") }
    $envelope = ($output -join "`n") | ConvertFrom-Json
    $result = $envelope.result | ConvertFrom-Json
    if ($result.passed -ne $true) { throw ($output -join "`n") }
    $result | ConvertTo-Json -Depth 10
} finally {
    & node $cli "-s=$session" close
    if ($null -ne $serverProcess -and !$serverProcess.HasExited) {
        Stop-Process -Id $serverProcess.Id
    }
    foreach ($name in $previousEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], 'Process')
    }
}
