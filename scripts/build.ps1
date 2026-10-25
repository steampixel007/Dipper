param([switch]$Test, [switch]$Render, [switch]$CrossCompile)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
$env:CGO_ENABLED = '0'
$env:GOTOOLCHAIN = 'auto'
$env:GOTELEMETRY = 'off'
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
    # Executables started inside the repo tree are blocked from writing to user
    # directories by local security software, so keep a runnable copy outside it.
    $outDir = Join-Path $env:LOCALAPPDATA 'Programs\Dipper'
    New-Item -ItemType Directory -Force $outDir | Out-Null
    Copy-Item bin/dipper.exe (Join-Path $outDir 'dipper.exe') -Force
    if (!$?) { throw "Copy to $outDir failed" }
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
