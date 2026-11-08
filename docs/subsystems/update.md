# update

`internal/update` finds official releases, downloads the matching package and
`SHA256SUMS`, and on Windows replaces a portable executable through a private
helper. It never changes the application database schema or user data.

## What it owns

- Reading the GitHub Releases API for the project repository.
- Selecting a stable release that is newer than this executable.
- Choosing the package for the running platform and architecture.
- Downloading and verifying the package before it is offered for installation.
- The Windows portable replacement helper, its readiness handshake, and its
  bounded recovery path.

## Files

| File | Holds |
| --- | --- |
| `release.go` | SemVer comparison, release selection, asset validation, and the HTTP client |
| `download.go` | Download, `SHA256SUMS` verification, and safe ZIP extraction |
| `helper.go` | Preparation, commit/abort, validated helper state, replacement, restart, and readiness |
| `process_windows.go`, `process_other.go` | Exact parent-process identity and waiting |
| `gate_windows.go`, `gate_other.go` | The per-executable launch gate |
| `paths_windows.go`, `paths_other.go` | Resolved path identity, including Windows junctions |
| `open_windows.go`, `open_linux.go`, `open_darwin.go` | Explicit opening of a verified installer on non-portable paths |

## Release and asset rules

The client reads `https://api.github.com/repos/steampixel007/Dipper/releases`
through the existing proxy configuration. Drafts and prereleases are ignored.
Download URLs must be HTTPS URLs for the exact repository release asset, and
the asset must be present once with a positive size. The client accepts at most
five pages of releases.

SemVer comparison follows the numeric core, then prerelease ordering, with a
normal release newer than a prerelease of the same core. A development build
(`dev`, `(devel)`, or empty) has no meaningful old/new comparison: it is shown
as a development build and may offer the latest stable release for a manual
choice, but automatic checks are disabled.

The package name is built from the release tag and `runtime.GOOS` /
`runtime.GOARCH`. Windows prefers the ZIP, macOS prefers the DMG then tarball,
and Linux prefers the DEB then tarball. A missing or malformed package is a
failed check, never a made-up URL.

## Downloads

Downloads are written to a fresh temporary directory. The client reads
`SHA256SUMS` first, requires exactly one matching entry, and compares the
downloaded byte count and SHA-256 with the release metadata. Cancellation
removes the incomplete directory. Windows ZIPs must contain exactly one regular
`Dipper.exe`; traversal, directory entries, extra entries and oversized entries
are rejected. Path comparisons normalize the OS-selected temporary root, which
can be a Windows short path or a macOS `/var` alias. Links and redirects below
that trusted root remain rejected, including extraction and launch-lock paths.
The running executable is not touched until the user clicks the
install action and the application commits the replacement during graceful
quit.

## Windows portable replacement

Only the Windows ZIP asset is handled as a portable replacement. The installed
copy under `%LOCALAPPDATA%\Programs\Dipper` is recognized by the `Uninstall.exe`
beside the running executable and is offered its official setup executable
instead, without automatic execution. macOS and Linux download a verified
official installer or archive and expose an explicit open action; the app does
not claim to replace itself on those platforms.

For a portable Windows copy:

1. `PreparePortable` verifies the downloaded executable and original executable,
   stages both the new executable and a copy of the old helper beside the
   target, and starts the helper in private mode.
2. The helper validates the state file, hashes, paths, local control address,
   token, timestamp, and its own image. It holds the exact parent process handle
   and creation time, then waits with a bounded timeout.
3. The user confirms installation. The UI saves preferences and core state,
   commits the helper, and proceeds through the normal graceful quit path so
   the core and downloader close before the process exits.
4. The helper acquires the per-executable launch gate in a per-user lock
   directory, renames the old
   executable to a staged backup, moves the staged executable into place, and
   starts it with the original working directory and arguments.
5. The new process validates the one-time restart token, sends `authorize`,
   reaches database and window startup, sends `ready`, and only then removes the
   backup and private state.

A quit veto cancels the pending operation and restores the visible window. A
replacement or startup failure restores the old executable when the target is
still the known staged image and the old copy is available. If restoration is
not safe, the old executable is retained in the private stage and the user is
told its location. The implementation makes no database downgrade promise and
never silently reverts migrated user data.

After acknowledged startup, a completed stage is reclaimed on a later launch.
Cleanup requires the exact completion marker and only the two expected regular
files, `helper.exe` and `complete`, with no redirected paths. It removes those
files individually and then removes the empty directory. Unknown contents and
failed or active stages are preserved.

The distributed ZIP remains a single `Dipper.exe`. Portable data mode is not
part of this subsystem; `--data` and the normal application data directories
continue to determine where user data lives.

## Tests

`release_test.go` uses `httptest` release responses and covers SemVer order,
stable selection, pagination limits, asset matching, proxy use, redirects, and
Windows installed-copy package selection. `download_test.go` covers checksums,
cancellation, unsafe ZIP entries and extraction errors. `helper_test.go` uses
temporary trees and harmless fixture executables; `process_windows_test.go`
exercises the real process identity, locked-target failure, and bounded wait
paths. UI tests in `cmd/dipper/software_update_ui_test.go` click the rendered
controls and assert state and engine effects without contacting the network.

## Invariants

- A failed read or check is unknown, not “up to date”.
- No package is offered until its checksum and archive have been verified.
- The running executable stays usable until an explicit user confirmation.
- Replacement and cleanup are limited to validated paths under the target
  directory.
- A readiness acknowledgement must belong to this update and this API handshake.
- The single-instance lock and launch gate prevent concurrent replacement or
  relaunch.
