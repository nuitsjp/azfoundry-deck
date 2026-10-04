$ErrorActionPreference = 'Stop'
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '../../../..')).Path

# Native window messages work on the runner's own desktop without mouse,
# keyboard focus, or screenshot access to an interactive user's desktop.
Add-Type @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public static class InstallerWindow {
    delegate bool EnumCallback(IntPtr hwnd, IntPtr data);
    [DllImport("user32.dll")] static extern bool EnumWindows(EnumCallback callback, IntPtr data);
    [DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr parent, EnumCallback callback, IntPtr data);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetWindowText(IntPtr hwnd, StringBuilder text, int count);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetClassName(IntPtr hwnd, StringBuilder name, int count);
    [DllImport("user32.dll")] static extern int GetWindowLong(IntPtr hwnd, int index);
    [DllImport("user32.dll")] public static extern IntPtr GetDlgItem(IntPtr hwnd, int id);
    [DllImport("user32.dll")] public static extern bool IsWindowEnabled(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern bool IsWindow(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern bool PostMessage(IntPtr hwnd, uint message, IntPtr wparam, IntPtr lparam);
    [DllImport("user32.dll")] static extern IntPtr SendMessage(IntPtr hwnd, uint message, IntPtr wparam, IntPtr lparam);
    [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint process);
    public static string Text(IntPtr hwnd) {
        var text = new StringBuilder(2048);
        GetWindowText(hwnd, text, text.Capacity);
        return text.ToString();
    }
    public static IntPtr Find(string titlePrefix) {
        IntPtr result = IntPtr.Zero;
        EnumWindows((hwnd, data) => {
            if (Text(hwnd).StartsWith(titlePrefix, StringComparison.Ordinal) && GetDlgItem(hwnd, 1) != IntPtr.Zero) { result = hwnd; return false; }
            return true;
        }, IntPtr.Zero);
        return result;
    }
    public static IntPtr[] Checkboxes(IntPtr parent) {
        var result = new List<IntPtr>();
        EnumChildWindows(parent, (hwnd, data) => {
            var name = new StringBuilder(64);
            GetClassName(hwnd, name, name.Capacity);
            int style = GetWindowLong(hwnd, -16) & 15;
            if (name.ToString() == "Button" && (style == 2 || style == 3)) result.Add(hwnd);
            return true;
        }, IntPtr.Zero);
        return result.ToArray();
    }
    public static bool Checked(IntPtr hwnd) { return SendMessage(hwnd, 0xF0, IntPtr.Zero, IntPtr.Zero).ToInt64() == 1; }
    public static void Check(IntPtr hwnd, bool value) { SendMessage(hwnd, 0xF1, new IntPtr(value ? 1 : 0), IntPtr.Zero); }
    public static uint ProcessId(IntPtr hwnd) { uint process; GetWindowThreadProcessId(hwnd, out process); return process; }
}
'@

function Assert-Installer($condition, [string]$message) {
    if (-not $condition) { throw $message }
}
function Wait-Installer([scriptblock]$condition, [string]$message) {
    $watch = [Diagnostics.Stopwatch]::StartNew()
    while ($watch.Elapsed.TotalSeconds -lt 20) {
        $result = & $condition
        if ($result) { return $result }
        Start-Sleep -Milliseconds 100
    }
    throw "Timeout: $message"
}
function Invoke-Next([IntPtr]$window) {
    $button = Wait-Installer { $control = [InstallerWindow]::GetDlgItem($window, 1); if ([InstallerWindow]::IsWindowEnabled($control)) { $control } } 'Navigation button enabled'
    # Send the same BN_CLICKED notification produced by clicking the button.
    Assert-Installer ([InstallerWindow]::PostMessage($window, 0x111, [IntPtr]1, $button)) 'Could not click installer button'
}

foreach ($choice in @(@($true, $false), @($false, $true), @($true, $true), @($false, $false))) {
    $fixture = & (Join-Path $projectRoot 'scripts/preview-installer.ps1') -BuildOnly
    $installerProcess = $null
    $window = [IntPtr]::Zero
    $passed = $false
    $desktopLink = Join-Path ([Environment]::GetFolderPath('DesktopDirectory')) ($fixture.AppName + '.lnk')
    $startLink = Join-Path ([Environment]::GetFolderPath('Programs')) ($fixture.AppName + '.lnk')
    $registry = 'HKCU:\Software\' + $fixture.AppId
    $uninstallRegistry = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\' + $fixture.AppId
    try {
        # 開始条件: 同じNSIS画面を持つインストーラーを用意し、登録を分離する。
        Assert-Installer (Test-Path -LiteralPath $fixture.Installer) 'Installer must exist'
        Assert-Installer (-not (Test-Path -LiteralPath $registry)) 'Fixture registration must not exist'

        # 手順1: インストーラーを実行し、指定した一時フォルダーへインストールする。
        $installerProcess = Start-Process -FilePath $fixture.Installer -ArgumentList "/D=$($fixture.InstallDir)" -WindowStyle Hidden -PassThru
        $window = Wait-Installer { $foundWindow = [InstallerWindow]::Find($fixture.AppName); if ($foundWindow -ne [IntPtr]::Zero) { $foundWindow } } 'Welcome page'
        Wait-Installer { [InstallerWindow]::Text([InstallerWindow]::GetDlgItem($window, 1)) -match '次へ' } 'Welcome button' | Out-Null
        Invoke-Next $window
        Wait-Installer { [InstallerWindow]::Text([InstallerWindow]::GetDlgItem($window, 1)) -match 'インストール' } 'Directory page' | Out-Null
        Invoke-Next $window
        $checks = Wait-Installer { $found = [InstallerWindow]::Checkboxes($window); if ($found.Count -eq 2) { return ,$found } } 'Finish page'
        $installedExe = Join-Path $fixture.InstallDir $fixture.Executable
        Assert-Installer (Test-Path -LiteralPath $installedExe) 'Application must be installed'
        Assert-Installer (Test-Path -LiteralPath $startLink) 'Start menu must be registered automatically'
        Assert-Installer ((Get-ItemProperty -LiteralPath $registry).InstallDir -eq $fixture.InstallDir) 'Install directory must match selection'

        # 手順2: 完了画面の2項目と、起動あり・デスクトップなしの初期状態を確認する。
        $launch = $checks | Where-Object { [InstallerWindow]::Text($_) -eq 'アプリを起動する' }
        $desktop = $checks | Where-Object { [InstallerWindow]::Text($_) -eq 'デスクトップにショートカットを作成する' }
        Assert-Installer ($launch -and $desktop) 'Finish page must contain exactly the agreed two choices'
        Assert-Installer ([InstallerWindow]::Checked($launch)) 'Launch must be checked by default'
        Assert-Installer (-not [InstallerWindow]::Checked($desktop)) 'Desktop must be unchecked by default'

        # 手順3: チェックを変更して完了し、選択した処理だけが実行されることを確認する。
        [InstallerWindow]::Check($launch, $choice[0])
        [InstallerWindow]::Check($desktop, $choice[1])
        Invoke-Next $window
        Wait-Installer { -not [InstallerWindow]::IsWindow($window) } 'Installer completion' | Out-Null
        $marker = Join-Path $fixture.InstallDir 'launched.txt'
        if ($choice[0]) { Wait-Installer { Test-Path -LiteralPath $marker } 'Application launch' | Out-Null }
        $installerProcess.WaitForExit()
        Assert-Installer ($installerProcess.ExitCode -eq 0) 'Installer must succeed'

        # 受け入れ条件: 起動・デスクトップ作成は選択どおりで、リンクは配置したexeを指す。
        Assert-Installer ((Test-Path -LiteralPath $marker) -eq $choice[0]) 'Launch must match selection'
        Assert-Installer ((Test-Path -LiteralPath $desktopLink) -eq $choice[1]) 'Desktop shortcut must match selection'
        $shell = New-Object -ComObject WScript.Shell
        Assert-Installer ($shell.CreateShortcut($startLink).TargetPath -eq $installedExe) 'Start menu shortcut must target installed app'
        if ($choice[1]) {
            Assert-Installer ($shell.CreateShortcut($desktopLink).TargetPath -eq $installedExe) 'Desktop shortcut must target installed app'
        }
        $passed = $true
    } finally {
        # Always remove this test's install; never stop or uninstall the user's app.
        if ($window -ne [IntPtr]::Zero -and [InstallerWindow]::IsWindow($window)) {
            Stop-Process -Id ([InstallerWindow]::ProcessId($window)) -Force
        }
        if ($installerProcess -and -not $installerProcess.HasExited) { $installerProcess.Kill(); $installerProcess.WaitForExit() }
        $uninstaller = Join-Path $fixture.InstallDir 'uninstall.exe'
        if (Test-Path -LiteralPath $uninstaller) {
            $uninstallProcess = Start-Process -FilePath $uninstaller -ArgumentList '/S' -WindowStyle Hidden -Wait -PassThru
            Assert-Installer ($uninstallProcess.ExitCode -eq 0) 'Uninstall must succeed'
        }
        Assert-Installer (-not (Test-Path -LiteralPath $desktopLink)) 'Uninstall must remove desktop shortcut'
        Assert-Installer (-not (Test-Path -LiteralPath $startLink)) 'Uninstall must remove Start menu shortcut'
        Assert-Installer (-not (Test-Path -LiteralPath $registry)) 'Uninstall must remove install registration'
        Assert-Installer (-not (Test-Path -LiteralPath $uninstallRegistry)) 'Uninstall must remove uninstall registration'
        $resolvedRoot = [IO.Path]::GetFullPath($fixture.Root)
        $allowedPrefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()) + 'AzFoundryDeck-installer-preview-'
        Assert-Installer ($resolvedRoot.StartsWith($allowedPrefix, [StringComparison]::OrdinalIgnoreCase)) 'Cleanup must stay in the fixture temp folder'
        Remove-Item -LiteralPath $resolvedRoot -Recurse -Force
    }
    Assert-Installer $passed 'Installer scenario must complete'
    Write-Output "PASS launch=$($choice[0]), desktop=$($choice[1]); uninstall and cleanup completed"
}
