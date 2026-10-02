# Requires PowerShell 7 and an elevated terminal. config.cmd prompts for the
# registration token and service account; neither is stored by this script.
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateRange(1, [int]::MaxValue)]
    [int]$RunnerNumber = 1
)

$ErrorActionPreference = 'Stop'

if (-not $IsWindows -or [Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'X64') {
    throw 'This task requires Windows x64.'
}
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Run mise run setup:runner from an administrator PowerShell terminal.'
}

$runnerInstance = if ($RunnerNumber -eq 1) { 'azfoundry-deck' } else { "azfoundry-deck-$RunnerNumber" }
$runnerDirectory = Join-Path 'D:\actions-runner' $runnerInstance
$runnerName = "$env:COMPUTERNAME-$runnerInstance"
if ((Test-Path -LiteralPath $runnerDirectory) -and
    (-not (Test-Path -LiteralPath $runnerDirectory -PathType Container) -or
    (Get-ChildItem -LiteralPath $runnerDirectory -Force | Select-Object -First 1))) {
    throw "Runner directory must be empty: $runnerDirectory"
}

$release = Invoke-RestMethod -Uri 'https://api.github.com/repos/actions/runner/releases/latest'
$version = $release.tag_name -replace '^v', ''
$asset = $release.assets | Where-Object name -EQ "actions-runner-win-x64-$version.zip"
if (-not $asset -or $asset.digest -notmatch '^sha256:[0-9a-fA-F]{64}$') {
    throw 'The official Windows x64 release or its SHA256 digest is missing.'
}

$archive = [IO.Path]::GetTempFileName()
try {
    Write-Host "Downloading GitHub Actions runner $version..."
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $archive
    $digest = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash
    if ("sha256:$digest" -ne $asset.digest) {
        throw 'Runner archive SHA256 verification failed.'
    }
    New-Item -ItemType Directory -Path $runnerDirectory -Force | Out-Null
    Expand-Archive -LiteralPath $archive -DestinationPath $runnerDirectory
} finally {
    Remove-Item -LiteralPath $archive
}

Push-Location -LiteralPath $runnerDirectory
try {
    # Automatic updates are enabled by default; do not pass --disableupdate.
    # Let the official installer prompt for the service account and password.
    & .\config.cmd --url 'https://github.com/nuitsjp/azfoundry-deck' `
        --name $runnerName --labels 'azfoundry-deck' `
        --work '_work' --runasservice
    if ($LASTEXITCODE -ne 0) {
        throw "Runner configuration failed with exit code $LASTEXITCODE."
    }
    $serviceName = (Get-Content -LiteralPath '.service' -Raw).Trim()
    $service = Get-Service -Name $serviceName
    $service.WaitForStatus([ServiceProcess.ServiceControllerStatus]::Running, [TimeSpan]::FromSeconds(30))
    Write-Host "Runner service is running: $serviceName (automatic updates enabled)."
} finally {
    Pop-Location
}
