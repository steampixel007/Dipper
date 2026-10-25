param(
    [Parameter(Mandatory)][string]$Exe,
    [Parameter(Mandatory)][string]$Output
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Drawing
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class TrayMouse {
    [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X; public int Y; }
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern bool GetCursorPos(out POINT point);
    [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extraInfo);
}
'@

$root = Split-Path $Output -Parent
New-Item -ItemType Directory -Force $root | Out-Null
$data = Join-Path $root 'auto-data'
$downloads = Join-Path $root 'auto-downloads'
New-Item -ItemType Directory -Force $data, $downloads | Out-Null
$original = New-Object TrayMouse+POINT
[TrayMouse]::GetCursorPos([ref]$original) | Out-Null
$process = $null
try {
    $process = Start-Process -FilePath $Exe -ArgumentList @('--data', $data, '--download-root', $downloads, '--quit-after', '30s') -PassThru
    $desktop = [System.Windows.Automation.AutomationElement]::RootElement
    $window = $null
    $deadline = [DateTime]::UtcNow.AddSeconds(20)
    do {
        Start-Sleep -Milliseconds 250
        $condition = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
        $window = $desktop.FindFirst([System.Windows.Automation.TreeScope]::Children, $condition)
    } until ($null -ne $window -or [DateTime]::UtcNow -ge $deadline -or $process.HasExited)
    if ($null -eq $window) { throw 'Test app window did not appear.' }

    $taskbar = $desktop.FindFirst([System.Windows.Automation.TreeScope]::Children, (New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ClassNameProperty, 'Shell_TrayWnd')))
    if ($null -eq $taskbar) { throw 'Windows taskbar was not found.' }
    $buttons = $taskbar.FindAll([System.Windows.Automation.TreeScope]::Descendants, (New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ClassNameProperty, 'SystemTray.NormalButton')))
    if ($buttons.Count -eq 0) { throw 'Windows notification area was not found.' }
    $tray = $null
    foreach ($icon in $buttons) {
        if ($icon.Current.Name -like 'Dipper *') { $tray = $icon; break }
    }
    if ($null -eq $tray) {
        $overflowCondition = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ClassNameProperty, 'TopLevelWindowForOverflowXamlIsland')
        $iconCondition = New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ClassNameProperty, 'SystemTray.NormalButton')
        $overflowWindow = $desktop.FindFirst([System.Windows.Automation.TreeScope]::Children, $overflowCondition)
        if ($null -eq $overflowWindow) {
            $overflow = $buttons | Sort-Object { $_.Current.BoundingRectangle.X } | Select-Object -First 1
            $overflowRect = $overflow.Current.BoundingRectangle
            [TrayMouse]::SetCursorPos([int]($overflowRect.X + $overflowRect.Width / 2), [int]($overflowRect.Y + $overflowRect.Height / 2)) | Out-Null
            [TrayMouse]::mouse_event(0x0002, 0, 0, 0, [UIntPtr]::Zero)
            [TrayMouse]::mouse_event(0x0004, 0, 0, 0, [UIntPtr]::Zero)
        }
        $deadline = [DateTime]::UtcNow.AddSeconds(6)
        do {
            $overflowWindow = $desktop.FindFirst([System.Windows.Automation.TreeScope]::Children, $overflowCondition)
            if ($null -ne $overflowWindow) {
                foreach ($icon in $overflowWindow.FindAll([System.Windows.Automation.TreeScope]::Descendants, $iconCondition)) {
                    if ($icon.Current.Name -like 'Dipper *') { $tray = $icon; break }
                }
            }
            if ($null -ne $tray) { break }
            Start-Sleep -Milliseconds 250
        } until ([DateTime]::UtcNow -ge $deadline)
    }
    if ($null -eq $tray) { throw 'Dipper notification icon was not found.' }
    $rect = $tray.Current.BoundingRectangle
    if ($rect.IsEmpty) { throw 'Dipper notification icon has no visible bounds.' }
    [TrayMouse]::SetCursorPos([int]($rect.X + $rect.Width / 2), [int]($rect.Y + $rect.Height / 2)) | Out-Null
    [TrayMouse]::mouse_event(0x0008, 0, 0, 0, [UIntPtr]::Zero)
    [TrayMouse]::mouse_event(0x0010, 0, 0, 0, [UIntPtr]::Zero)

    $menu = $null
    $deadline = [DateTime]::UtcNow.AddSeconds(5)
    do {
        Start-Sleep -Milliseconds 150
        foreach ($child in $desktop.FindAll([System.Windows.Automation.TreeScope]::Children, [System.Windows.Automation.Condition]::TrueCondition)) {
            if ($child.Current.ClassName -eq '#32768' -and $child.Current.ProcessId -eq $process.Id) {
                $menu = $child
                break
            }
        }
    } until ($null -ne $menu -or [DateTime]::UtcNow -ge $deadline)
    if ($null -eq $menu) { throw 'Test app tray menu did not open.' }
    $bounds = $menu.Current.BoundingRectangle
    Write-Output "Menu bounds: $bounds"
    $crop = [System.Drawing.Rectangle]::new([int]$bounds.X - 8, [int]$bounds.Y - 8, [int]$bounds.Width + 16, [int]$bounds.Height + 16)
    $bitmap = New-Object System.Drawing.Bitmap($crop.Width, $crop.Height)
    try {
        $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
        try { $graphics.CopyFromScreen($crop.Location, [System.Drawing.Point]::Empty, $crop.Size) }
        finally { $graphics.Dispose() }
        $bitmap.Save($Output, [System.Drawing.Imaging.ImageFormat]::Png)
    } finally { $bitmap.Dispose() }
    Write-Output "Screenshot: $Output"
    $quitX = [int]($bounds.X + $bounds.Width / 2)
    $quitY = [int]($bounds.Y + $bounds.Height * 3 / 4)
    [TrayMouse]::SetCursorPos($quitX, $quitY) | Out-Null
    [TrayMouse]::mouse_event(0x0002, 0, 0, 0, [UIntPtr]::Zero)
    [TrayMouse]::mouse_event(0x0004, 0, 0, 0, [UIntPtr]::Zero)
    if (!$process.WaitForExit(8000)) { throw "Test process $($process.Id) did not exit after selecting the tray menu quit item." }
    Write-Output "Exited process: $($process.Id)"
} finally {
    [TrayMouse]::SetCursorPos($original.X, $original.Y) | Out-Null
    if ($null -ne $process -and !$process.HasExited) {
        Stop-Process -Id $process.Id
        Write-Warning "Stopped lingering test process $($process.Id)."
    }
}
