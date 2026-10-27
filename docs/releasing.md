# Releasing Dipper

Pushing a version tag runs `.github/workflows/release.yml`. It validates the tag,
runs the existing Windows, macOS and Linux checks, builds all release archives,
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

| Platform | Archive |
| --- | --- |
| Windows x64 | `dipper_VERSION_windows_amd64.zip` |
| Linux x64 | `dipper_VERSION_linux_amd64.tar.gz` |
| macOS Intel | `dipper_VERSION_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `dipper_VERSION_darwin_arm64.tar.gz` |

Each archive contains a bare executable. Unix archives preserve its executable
permission. These are not installers or macOS application bundles. Windows
builds retain the GUI subsystem and embedded icon; the release build does not
run the local build script's installation step.

Download `SHA256SUMS` beside the archives. On Linux, verify with
`sha256sum --check SHA256SUMS`; on macOS use `shasum -a 256 -c SHA256SUMS`. On
Windows, use `Get-FileHash .\dipper_VERSION_windows_amd64.zip -Algorithm SHA256`
and compare its hash with that archive's line in `SHA256SUMS`.

Releases are unsigned. Native graphics and tray dependencies, and the pending
macOS/Linux desktop acceptance, still apply; see
[development boundaries](development.md#verification-and-boundaries).
Passing CI and cross-compilation do not establish native runtime acceptance.

## Failures and retries

No release is created if validation, checks or a build fails. Fix a workflow or
code defect in a new commit and use a new version tag. For a transient runner
failure, rerun the failed jobs from the Actions run.

If uploading or publishing fails after draft creation, inspect the unpublished
draft under Releases. Delete that draft before rerunning the failed publishing
job; keep the original Git tag. The workflow intentionally fails if a release
already exists instead of overwriting its files. Do not move a released tag:
publish fixes under a new version.
