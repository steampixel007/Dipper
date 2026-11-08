param([ValidateSet('pass', 'fail')][string]$Expect = 'pass')
$originalGoCache = $env:GOCACHE
try {
    $env:GOCACHE = Join-Path ([IO.Path]::GetTempPath()) 'dipper-gocache-update-green'
    go run ./cmd/redcheck -pkg ./internal/update -test '^TestCleanupPreservesUnexpectedStageFiles$' -expect $Expect
    if ($LASTEXITCODE) { throw "Cleanup redcheck failed: $LASTEXITCODE" }
} finally {
    $env:GOCACHE = $originalGoCache
}
