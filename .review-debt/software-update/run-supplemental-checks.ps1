$ErrorActionPreference = 'Stop'
$originalGoCache = $env:GOCACHE
$originalCgo = $env:CGO_ENABLED
$originalGoos = $env:GOOS
$originalGoarch = $env:GOARCH
try {
    $env:GOCACHE = Join-Path ([IO.Path]::GetTempPath()) 'dipper-gocache-update-green'
    $env:CGO_ENABLED = '0'
    & ./scripts/check-torrent.ps1
    if ($LASTEXITCODE) { throw 'Torrent regression failed' }
    foreach ($target in @(@('windows', 'amd64'), @('linux', 'amd64'), @('darwin', 'arm64'), @('darwin', 'amd64'))) {
        $env:GOOS = $target[0]
        $env:GOARCH = $target[1]
        $extension = ''
        if ($env:GOOS -eq 'windows') { $extension = '.exe' }
        $output = 'bin/redcheck-' + $env:GOOS + '-' + $env:GOARCH + $extension
        go build -trimpath -o $output ./cmd/redcheck
        if ($LASTEXITCODE) { throw "Cross-build failed: $output" }
        Write-Output "BUILT $output"
    }
} finally {
    $env:GOCACHE = $originalGoCache
    $env:CGO_ENABLED = $originalCgo
    $env:GOOS = $originalGoos
    $env:GOARCH = $originalGoarch
}
