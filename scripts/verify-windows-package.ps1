param([Parameter(Mandatory)][string]$Version)
$ErrorActionPreference = 'Stop'
if ($env:GITHUB_ACTIONS -ne 'true' -or !$env:RUNNER_TEMP) { throw 'Run installer smoke tests only on a disposable GitHub runner' }
$stem = "dist/dipper_${Version}_windows_amd64"
$archive = Join-Path $env:RUNNER_TEMP 'archive'
$install = Join-Path $env:RUNNER_TEMP 'Dipper install'
Expand-Archive -LiteralPath "$stem.zip" -DestinationPath $archive
$expected = (Get-FileHash "$archive/Dipper.exe").Hash
$setup = (Resolve-Path -LiteralPath "${stem}_setup.exe").Path
$process = Start-Process -FilePath $setup -ArgumentList "/S /D=$install" -Wait -PassThru -WindowStyle Hidden
if ($process.ExitCode -ne 0) { throw "Installer exited $($process.ExitCode)" }
try {
    if ((Get-FileHash "$install/Dipper.exe").Hash -ne $expected) { throw 'Installed executable differs from ZIP' }
    $key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\io.github.steampixel007.dipper'
    if ((Get-ItemProperty -LiteralPath $key).DisplayVersion -ne $Version) { throw 'Wrong installed version' }
} finally {
    $process = Start-Process -FilePath "$install/Uninstall.exe" -ArgumentList "/S _?=$install" -Wait -PassThru -WindowStyle Hidden
    if ($process.ExitCode -ne 0) { throw "Uninstaller exited $($process.ExitCode)" }
}
if (Test-Path -LiteralPath "$install/Dipper.exe") { throw 'Uninstaller left the application behind' }
if (Test-Path -LiteralPath $key) { throw 'Uninstaller left its registry entry behind' }
