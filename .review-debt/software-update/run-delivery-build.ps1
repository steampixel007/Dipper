$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..\..')).Path
$verificationLocal = Join-Path ([IO.Path]::GetTempPath()) ('dipper-localappdata-' + [guid]::NewGuid().ToString('N'))
$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$resolvedLocal = [IO.Path]::GetFullPath($verificationLocal)
if (!$resolvedLocal.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Unsafe verification LOCALAPPDATA: $resolvedLocal"
}

$originalLocalAppData = $env:LOCALAPPDATA
$originalGoCache = $env:GOCACHE
New-Item -ItemType Directory -Path $resolvedLocal -Force | Out-Null
try {
    $env:LOCALAPPDATA = $resolvedLocal
    $env:GOCACHE = Join-Path $tempRoot 'dipper-gocache-update-green'
    Write-Host "LOCALAPPDATA_VERIFY=$resolvedLocal"
    & (Join-Path $repoRoot 'scripts\build.ps1') -Test -CrossCompile
    if ($LASTEXITCODE) {
        throw "Delivery build failed: $LASTEXITCODE"
    }
} finally {
    $env:LOCALAPPDATA = $originalLocalAppData
    $env:GOCACHE = $originalGoCache
    if (Test-Path -LiteralPath $resolvedLocal) {
        Remove-Item -LiteralPath $resolvedLocal -Recurse -Force
    }
}
