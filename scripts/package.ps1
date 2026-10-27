param(
    [Parameter(Mandatory)][string]$Version,
    [Parameter(Mandatory)][ValidateSet('windows/amd64', 'linux/amd64', 'darwin/amd64', 'darwin/arm64')][string]$Platform
)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path $PSScriptRoot -Parent
Push-Location $repoRoot
try {
    $number = '(0|[1-9][0-9]*)'
    $identifier = '(0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*)'
    $pattern = "\A$number\.$number\.$number(-$identifier(\.$identifier)*)?(\+[0-9a-zA-Z-]+(\.[0-9a-zA-Z-]+)*)?\z"
    if ($Version -notmatch $pattern) { throw 'Expected a SemVer version without v' }
    $goos, $goarch = $Platform.Split('/')
    if (($goos -eq 'windows' -and !$IsWindows) -or
        ($goos -eq 'linux' -and !$IsLinux) -or
        ($goos -eq 'darwin' -and !$IsMacOS)) { throw 'Package on the target operating system' }
    if ($env:GOOS -or $env:GOARCH) { throw 'Unset GOOS and GOARCH before running the host packaging tool' }

    $configPath = Join-Path $repoRoot 'mygo.json'
    $versionPath = Join-Path $repoRoot 'cmd/dipper/release_version_generated.go'
    if (Test-Path -LiteralPath $versionPath) { throw "Remove the leftover generated file before packaging: $versionPath" }
    $original = [IO.File]::ReadAllBytes($configPath)
    $originalCgo = $env:CGO_ENABLED
    try {
        $config = Get-Content $configPath -Raw | ConvertFrom-Json -AsHashtable
        $config.version = $Version
        # Apple's bundle version fields accept numeric components, not SemVer suffixes.
        $core = ($Version -split '[-+]')[0]
        $config.macos.infoPlist.CFBundleVersion = $core
        $config.macos.infoPlist.CFBundleShortVersionString = $core
        [IO.File]::WriteAllText($configPath, ($config | ConvertTo-Json -Depth 10))
        # MyGo supplies its own linker flags; keep the full tag independently of Apple's numeric bundle version.
        [IO.File]::WriteAllText($versionPath, "package main`n`nfunc init() { version = `"$Version`" }`n")
        $env:CGO_ENABLED = '0'
        go run github.com/egoist/mygo/cmd/mygo build -platform $Platform -skip-notarize
        if ($LASTEXITCODE) { throw 'MyGo packaging failed' }
    } finally {
        $env:CGO_ENABLED = $originalCgo
        [IO.File]::WriteAllBytes($configPath, $original)
        Remove-Item -LiteralPath $versionPath -ErrorAction SilentlyContinue
    }

    $package = Join-Path $repoRoot ".cache/packages/$goos-$goarch"
    $dist = Join-Path $repoRoot 'dist'
    New-Item -ItemType Directory -Force $dist | Out-Null
    $stem = "dipper_${Version}_${goos}_${goarch}"
    switch ($goos) {
        windows {
            Compress-Archive -LiteralPath "$package/Dipper.exe" -DestinationPath "$dist/$stem.zip" -Force
            Copy-Item -LiteralPath "$package/Dipper Setup $Version $goarch.exe" -Destination "$dist/${stem}_setup.exe"
        }
        linux {
            tar -czf "$dist/$stem.tar.gz" -C $package dipper
            if ($LASTEXITCODE) { throw 'Linux archive failed' }
            $deb = @(Get-ChildItem -LiteralPath $package -Filter '*.deb')
            if ($deb.Count -ne 1) { throw 'Expected exactly one Debian package' }
            Copy-Item -LiteralPath $deb[0].FullName -Destination "$dist/$stem.deb"
        }
        darwin {
            tar -czf "$dist/$stem.tar.gz" -C $package Dipper.app
            if ($LASTEXITCODE) { throw 'macOS archive failed' }
            Copy-Item -LiteralPath "$package/Dipper $Version $goarch.dmg" -Destination "$dist/$stem.dmg"
        }
    }
    Get-ChildItem -LiteralPath $dist | Where-Object Name -Like "$stem*" | Select-Object Name, Length
} finally {
    Pop-Location
}
