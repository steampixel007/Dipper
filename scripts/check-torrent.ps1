param(
    [string]$ModuleVersion,
    [ValidateSet('pass', 'fail')][string]$Expect = 'pass'
)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
$tempRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
$scratch = Join-Path $tempRoot ('dipper-torrent-' + [guid]::NewGuid().ToString('N'))
$savedEnv = @{}
foreach ($name in @('CGO_ENABLED', 'GOWORK', 'GOPROXY', 'GOSUMDB')) {
    $savedEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
Push-Location $repoRoot
try {
    $env:CGO_ENABLED = '0'
    $env:GOWORK = 'off'
    if ($ModuleVersion) {
        $module = go mod download -json ('github.com/anacrolix/torrent@' + $ModuleVersion) | ConvertFrom-Json
    } else {
        $module = go list -m -json github.com/anacrolix/torrent | ConvertFrom-Json
    }
    if ($LASTEXITCODE -or !$module.Dir) { throw 'Cannot locate torrent module' }
    New-Item -ItemType Directory -Path $scratch | Out-Null
    $redcheck = Join-Path $scratch 'redcheck'
    if ($env:OS -eq 'Windows_NT') { $redcheck += '.exe' }
    go build -o $redcheck ./cmd/redcheck
    if ($LASTEXITCODE) { throw 'Cannot build redcheck' }
    $moduleDir = Join-Path $scratch 'module'
    Copy-Item -LiteralPath $module.Dir -Destination $moduleDir -Recurse
    Get-ChildItem -LiteralPath $moduleDir -File -Recurse | ForEach-Object { $_.IsReadOnly = $false }
    Copy-Item -LiteralPath (Join-Path $repoRoot 'internal/downloader/testdata/msg_writer_regression_test.go') -Destination (Join-Path $moduleDir 'dipper_msg_writer_regression_test.go')
    Set-Location $moduleDir
    # Fetch pinned test dependencies before running the offline regression checks.
    go mod download
    if ($LASTEXITCODE) { throw 'Cannot prepare torrent test dependencies' }
    $env:GOPROXY = 'off'
    $env:GOSUMDB = 'off'
    Write-Output ('Checking torrent ' + $module.Version)
    foreach ($test in @('^TestDipperWriterRetainsWakeupDuringFill$', '^TestDipperWriterRearmsIdleKeepalive$')) {
        & $redcheck -pkg . -test $test -expect $Expect
        if ($LASTEXITCODE) { throw 'Torrent regression check failed' }
    }
} finally {
    Pop-Location
    foreach ($name in $savedEnv.Keys) {
        [Environment]::SetEnvironmentVariable($name, $savedEnv[$name], 'Process')
    }
    $scratchFull = [System.IO.Path]::GetFullPath($scratch)
    $tempPrefix = $tempRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    if (!$scratchFull.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Refusing to clean a directory outside the temporary directory'
    }
    if (Test-Path -LiteralPath $scratchFull) {
        Remove-Item -LiteralPath $scratchFull -Recurse -Force
    }
}
