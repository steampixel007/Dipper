# Releasing Dipper

Pushing a version tag runs `.github/workflows/release.yml`. It validates the tag,
runs the existing Windows, macOS and Linux checks, builds archives and installers,
and publishes a GitHub Release with generated release notes and `SHA256SUMS`.
No manual publishing step or additional secret is required; only the publishing
job receives `contents: write` through the built-in `GITHUB_TOKEN`.

## Publish a version

The tagged commit must contain the release workflow. Merge the intended changes
first, then select the version yourself. For example, from the repository root:

```powershell
git fetch origin --tags
git log -1 --oneline origin/main
git tag -a v0.1.0 origin/main -m "Release v0.1.0"
git push origin v0.1.0
```

Replace `v0.1.0` with an unused version. Replace `origin/main` with a specific
commit SHA to publish that commit instead. Creating a tag only on your machine
does not start the workflow: push that tag to GitHub.

Tags follow Semantic Versioning with a `v` prefix: `v1.2.3` is a stable release;
`v1.2.3-rc.1` and `v1.2.3-beta.1` become GitHub prereleases and are not marked
Latest. Optional build metadata such as `v1.2.3+build.5` is accepted. Other tags
starting with `v` fail validation; tags without that prefix do not trigger a
release. The executable's version is injected from the tag without the `v`
prefix, including prerelease and build metadata. Local development builds keep
their source default.

Follow progress under **Actions → Release**. The release becomes public only
after all checks, builds and asset uploads succeed. A temporary draft is used
while uploading, and is published automatically after upload completes.

## Downloads

| Platform | Archive | Installer |
| --- | --- | --- |
| Windows x64 | `dipper_VERSION_windows_amd64.zip` | `dipper_VERSION_windows_amd64_setup.exe` |
| Linux x64 | `dipper_VERSION_linux_amd64.tar.gz` | `dipper_VERSION_linux_amd64.deb` |
| macOS Intel | `dipper_VERSION_darwin_amd64.tar.gz` | `dipper_VERSION_darwin_amd64.dmg` |
| macOS Apple Silicon | `dipper_VERSION_darwin_arm64.tar.gz` | `dipper_VERSION_darwin_arm64.dmg` |

There are eight application downloads plus `SHA256SUMS`. GitHub also adds
**Source code (zip)** and **Source code (tar.gz)** automatically: these contain
the tagged repository's source, not a ready-to-run app.

The Windows ZIP contains `Dipper.exe`; extract and run it. The setup EXE installs
for the current user under `%LOCALAPPDATA%\Programs\Dipper`, adds a Start menu
shortcut and an uninstaller, and offers a desktop shortcut. No administrator
privilege is needed. Uninstalling leaves application settings and downloads alone.

The Linux tarball contains the executable `dipper`, with executable permission.
It requires GTK3 and Ayatana AppIndicator on the target system. The Debian package
declares these dependencies and installs the app, icon, applications-menu entry
and `/usr/bin/dipper` command. Install with
`sudo apt install ./dipper_VERSION_linux_amd64.deb`; the DEB supports Debian/Ubuntu
systems, not every Linux distribution.

Both macOS formats contain `Dipper.app`. Extract the tarball, or open the DMG and
drag the app to Applications. Intel and Apple Silicon have separate downloads.

Download `SHA256SUMS` beside the archives. On Linux, verify with
`sha256sum --check SHA256SUMS`; on macOS use `shasum -a 256 -c SHA256SUMS`. On
Windows, use `Get-FileHash .\dipper_VERSION_windows_amd64.zip -Algorithm SHA256`
and compare its hash with that archive's line in `SHA256SUMS`.

Windows builds have no publisher certificate. macOS bundles use free ad-hoc
signing, with no Developer ID certificate or Apple notarization. The operating
system may block an initial launch or show an unknown-publisher warning. Native
graphics and tray dependencies, and the pending
macOS/Linux desktop acceptance, still apply; see
[development boundaries](development.md#verification-and-boundaries).
Passing CI and cross-compilation do not establish native runtime acceptance.

## Packaging and verification

`scripts/package.ps1` uses the MyGo CLI pinned in `go.mod` and `mygo.json` for
application metadata. It builds on each target OS: Windows x64, Linux x64, macOS
Intel and macOS Apple Silicon. On Windows MyGo uses installed NSIS or downloads
its pinned, checksum-verified portable compiler into its own cache. The release
build does not invoke the local build script's installation step.

For a local package, run PowerShell 7 on the target operating system:

```powershell
./scripts/package.ps1 -Version 1.2.3-rc.1 -Platform windows/amd64
```

Use `linux/amd64`, `darwin/amd64` or `darwin/arm64` on those systems. Artifacts go
to `dist/`; MyGo's intermediate packages go to `.cache/packages/`. The script
temporarily supplies the version in `mygo.json` and a generated
`cmd/dipper/release_version_generated.go`, restoring/removing them even on a
build failure. If the process is killed before cleanup, restore `mygo.json` and
remove the generated file before another build. The full tag version stays in
the app's readiness/logs and asset names; Apple's bundle version fields use only
the numeric `major.minor.patch` components.

Before upload, CI installs and uninstalls the Windows package on its disposable
runner and compares its executable with the ZIP. Linux checks the DEB metadata,
dependencies, desktop entry, command link and executable permissions against the
tarball. macOS extracts the tarball, mounts the DMG read-only, checks bundle
metadata, architecture and signatures, and compares the executables. Publishing
requires all eight exact filenames and generates checksums for every application
asset. These package checks do not replace interactive desktop acceptance.

## Failures and retries

No release is created if validation, checks or a build fails. Fix a workflow or
code defect in a new commit and use a new version tag. For a transient runner
failure, rerun the failed jobs from the Actions run.

If uploading or publishing fails after draft creation, inspect the unpublished
draft under Releases. Delete that draft before rerunning the failed publishing
job; keep the original Git tag. The workflow intentionally fails if a release
already exists instead of overwriting its files. Do not move a released tag:
publish fixes under a new version.
