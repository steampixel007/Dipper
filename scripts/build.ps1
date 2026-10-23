param([switch]$Test, [switch]$Render, [switch]$CrossCompile)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
$env:GOPATH = Join-Path $repoRoot '.cache/gopath'
$env:GOMODCACHE = Join-Path $repoRoot '.cache/gomod'
$env:GOCACHE = Join-Path $repoRoot '.cache/gobuild'
$env:GOTMPDIR = Join-Path $repoRoot '.cache/gotmp'
$env:CGO_ENABLED = '0'
$env:GOTOOLCHAIN = 'auto'
$env:GOTELEMETRY = 'off'
New-Item -ItemType Directory -Force $env:GOTMPDIR | Out-Null
Push-Location $repoRoot
try {
    if ($Test) {
        go test -count=1 -timeout 5m ./...
        if ($LASTEXITCODE) { throw 'Tests failed' }
        go vet ./...
        if ($LASTEXITCODE) { throw 'Vet failed' }
    }
    go build -trimpath -ldflags '-H windowsgui' -o bin/dipper.exe ./cmd/dipper
    if ($LASTEXITCODE) { throw 'Windows build failed' }
    if ($Render) {
        & ./bin/dipper.exe --render ./docs/assets/ui
        if ($LASTEXITCODE) { throw 'Render failed' }
    }
    if ($CrossCompile) {
        try {
            foreach ($target in @(@('linux','amd64'),@('darwin','arm64'),@('darwin','amd64'))) {
                $env:GOOS = $target[0]
                $env:GOARCH = $target[1]
                $output = 'bin/dipper-' + $env:GOOS + '-' + $env:GOARCH
                go build -trimpath -o $output ./cmd/dipper
                if ($LASTEXITCODE) { throw "Build failed: $output" }
            }
        } finally {
            Remove-Item Env:GOOS,Env:GOARCH -ErrorAction SilentlyContinue
        }
    }
} finally {
    Pop-Location
}
