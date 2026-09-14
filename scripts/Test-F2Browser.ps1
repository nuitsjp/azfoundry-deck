param([switch]$Azure, [switch]$AddModel, [switch]$Attach)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $repoRoot
$cli = Join-Path $repoRoot 'frontend/node_modules/@playwright/cli/playwright-cli.js'
$testName = if ($AddModel) { 'add-model' } elseif ($Azure) { 'f2-azure' } else { 'f2' }
$port = if ($Azure) { 9248 } else { 9246 }
$testFile = if ($AddModel) { 'frontend/tests/add-model.browser.js' } elseif ($Azure) { 'frontend/tests/f2.azure.browser.js' } else { 'frontend/tests/f2.browser.js' }
$session = "$testName-ui-$PID"
$serverProcess = $null
$previousEnvironment = @{}
foreach ($name in @('AZFOUNDRY_MOCK', 'WAILS_SERVER_HOST', 'WAILS_SERVER_PORT')) {
    $previousEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}

try {
    if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) {
        throw "Port $port is in use. Close its server before running this test."
    }
    $env:AZFOUNDRY_MOCK = if ($Azure) { '0' } else { '1' }
    $env:WAILS_SERVER_HOST = '127.0.0.1'
    $env:WAILS_SERVER_PORT = "$port"
    $serverProcess = Start-Process -FilePath (Join-Path $repoRoot 'bin/azfoundry-deck-browser.exe') -WorkingDirectory $repoRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput "bin/$testName-test-server-stdout.log" -RedirectStandardError "bin/$testName-test-server-stderr.log"
    $ready = $false
    for ($attempt = 0; $attempt -lt 100; $attempt++) {
        if ($serverProcess.HasExited) {
            throw "Wails server exited: $(Get-Content -Raw "bin/$testName-test-server-stderr.log")"
        }
        try {
            $ready = (Invoke-WebRequest -Uri "http://127.0.0.1:$port/" -NoProxy -TimeoutSec 1).StatusCode -eq 200
        } catch { $ready = $false }
        if ($ready) { break }
        Start-Sleep -Milliseconds 100
    }
    if (!$ready -or $serverProcess.HasExited) { throw "Wails server did not start on 127.0.0.1:$port." }

    if ($Attach) {
        & node $cli "-s=$session" attach --cdp=msedge
        if ($LASTEXITCODE -ne 0) { throw 'Edge attach failed. Enable remote debugging in the running Edge.' }
        & node $cli "-s=$session" tab-new "http://127.0.0.1:$port"
        if ($LASTEXITCODE -ne 0) { throw 'Attached tab did not open.' }
    } else {
        & node $cli "-s=$session" open "http://127.0.0.1:$port" --config=frontend/tests/browser.config.json
        if ($LASTEXITCODE -ne 0) { throw 'Headless browser launch failed.' }
    }
    $output = & node $cli "-s=$session" --json run-code "--filename=$testFile"
    $output | Set-Content -LiteralPath "docs/verification/$testName-browser-result.json" -Encoding utf8NoBOM
    if ($LASTEXITCODE -ne 0) { throw ($output -join "`n") }
    $envelope = ($output -join "`n") | ConvertFrom-Json
    if ($envelope.isError) { throw $envelope.error }
    $result = $envelope.result | ConvertFrom-Json
    if ($result.passed -ne $true) { throw ($output -join "`n") }
    $result | ConvertTo-Json -Depth 10
} finally {
    if ($Attach) {
        & node $cli "-s=$session" tab-close
        & node $cli "-s=$session" detach
    } else {
        & node $cli "-s=$session" close
    }
    if ($null -ne $serverProcess -and !$serverProcess.HasExited) {
        Stop-Process -Id $serverProcess.Id
    }
    foreach ($name in $previousEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], 'Process')
    }
}
