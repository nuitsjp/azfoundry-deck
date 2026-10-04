$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$previewRoot = Join-Path ([IO.Path]::GetTempPath()) ('AzFoundryDeck-installer-preview-' + [Guid]::NewGuid().ToString('N'))
$nsisRoot = Join-Path $previewRoot 'build/windows/nsis'
$previewBin = Join-Path $previewRoot 'bin'
New-Item -ItemType Directory -Path $nsisRoot, $previewBin -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $projectRoot 'build/windows/nsis/project.nsi') -Destination $nsisRoot
$utf8 = [Text.UTF8Encoding]::new($false)
$metadata = @'
!define APP_ID "AzFoundryDeckInstallerReview"
!define APP_NAME "AzFoundryDeck (画面確認)"
!define APP_EXE "azfoundrydeck-review.exe"
!define APP_VERSION "0.1.1"
!define APP_ARCH "amd64"
!define INSTALLER_NAME "azfoundrydeck-review-setup.exe"
'@
$previewId = 'AzFoundryDeckInstallerReview' + (Split-Path $previewRoot -Leaf).Split('-')[-1]
[IO.File]::WriteAllText((Join-Path $nsisRoot 'app.nsh'), $metadata.Replace('AzFoundryDeckInstallerReview', $previewId), $utf8)
$previewSource = @'
using System;
using System.IO;
using System.Windows.Forms;
class PreviewApplication {
    [STAThread]
    static void Main() {
        File.WriteAllText(Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "launched.txt"), "launched");
        Application.EnableVisualStyles();
        var window = new Form { Text = "AzFoundryDeck (画面確認)", Width = 420, Height = 160 };
        window.Controls.Add(new Label { Text = "アプリを起動しました（画面確認用）。", AutoSize = true, Left = 30, Top = 40 });
        Application.Run(window);
    }
}
'@
$sourcePath = Join-Path $previewRoot 'PreviewApplication.cs'
[IO.File]::WriteAllText($sourcePath, $previewSource, $utf8)
$compiler = Join-Path $env:WINDIR 'Microsoft.NET/Framework64/v4.0.30319/csc.exe'
& $compiler /nologo /target:winexe /reference:System.Windows.Forms.dll "/out:$(Join-Path $previewBin 'azfoundrydeck-review.exe')" $sourcePath
if ($LASTEXITCODE -ne 0) { throw 'Preview application compilation failed' }
$nsisCandidates = @(
    $env:NSIS_EXE
    $(if (${env:ProgramFiles(x86)}) { Join-Path ${env:ProgramFiles(x86)} 'NSIS/makensis.exe' })
    $(if ($env:ProgramFiles) { Join-Path $env:ProgramFiles 'NSIS/makensis.exe' })
    (Get-Command makensis -ErrorAction SilentlyContinue).Source
)
$nsis = $nsisCandidates | Where-Object { $_ -and (Test-Path -LiteralPath $_) } | Select-Object -First 1
if (-not $nsis) { throw 'NSIS is required. Set NSIS_EXE to makensis.exe.' }
& $nsis /V2 (Join-Path $nsisRoot 'project.nsi')
if ($LASTEXITCODE -ne 0) { throw 'Preview installer compilation failed' }
$installPath = Join-Path $previewRoot 'install'
Write-Output "Preview folder: $previewRoot"
Write-Output "Uninstall after closing the preview app: $installPath\uninstall.exe"
Start-Process -FilePath (Join-Path $previewBin 'azfoundrydeck-review-setup.exe') -ArgumentList "/D=$installPath" -Wait
