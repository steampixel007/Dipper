# update

`internal/update` finds official releases, downloads the matching package and
`SHA256SUMS`, and replaces supported installations through a private helper.
It never changes the application database schema or user data.

## What it owns

- Reading the GitHub Releases API for the project repository.
- Selecting a stable release that is newer than this executable.
- Choosing the package for the running platform and architecture.
- Downloading and verifying the package before it is offered for installation.
- Replacement for Windows installed/ZIP copies, macOS app bundles, Linux
  archives, and Debian packages, including readiness and bounded recovery.

## Files

| File | Holds |
| --- | --- |
| `release.go` | SemVer comparison, release selection, asset validation, and the HTTP client |
| `download.go`, `archive.go` | Download, checksums, safe ZIP/tar extraction, and whole-bundle verification |
| `helper.go` | Preparation, commit/abort, validated helper state, replacement, restart, and readiness |
| `platform_unix.go`, `helper_unix_payload.go`, `helper_debian.go` | Unix staging, bundle copying, and authorized package upgrades |
| `process_windows.go`, `process_linux.go`, `process_darwin.go`, `process_other.go` | Exact parent-process identity and waiting |
| `installation*.go` | Updating the matching Windows uninstall record after startup |
| `gate_windows.go`, `gate_other.go` | The per-executable launch gate |
| `paths_windows.go`, `paths_other.go` | Resolved path identity, including Windows junctions |
| `open_windows.go`, `open_linux.go`, `open_darwin.go` | Explicit fallback opening of a verified package when restart replacement is unavailable |

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
`runtime.GOARCH`. Windows uses the ZIP, macOS uses the tarball, and Linux uses
the tarball unless the canonical `/opt/dipper/dipper` belongs to the `dipper`
Debian package, in which case it uses the DEB. Ownership errors are not treated
as an archive installation. A missing or malformed package is a failed check,
never a made-up URL.

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

Linux tarballs contain only `dipper`; macOS tarballs contain only `Dipper.app`
and must include its executable and metadata. Extraction rejects traversal,
duplicate entries, links, special files, and excessive expanded sizes or entry
counts. A macOS bundle digest covers every file, directory, and permission,
including resources and signing data. Debian downloads retain the verified
package rather than extracting a standalone executable.

For self-updating release downloads, the UI passes a persistent
`updates` directory under the preferences directory instead of the OS temporary
root. A successfully verified download records its offer, paths, and hashes in
`preferences.json`. Ordinary shutdown keeps it; cancellation and failed saves
reclaim the incomplete or unpublished download. At the next launch the UI
requires a newer offer for the current installation mode and paths directly
under a managed download directory, then `VerifyUpdateDownload` verifies
the original package and the extracted executable or entire app bundle before showing a
restart action. Missing, modified, or obsolete downloads are not offered.
If installation preparation rejects a download whose bytes no longer verify,
the UI clears the pending record and reclaims only that managed directory, so a
later check can download the release again. A manual check and a new download
request also revalidate the retained download before honoring it, so a file
modified or deleted while Dipper is running cannot block recovery.
Development builds do not download automatically.

## Replacement and restart

Windows installed and extracted copies use the same executable replacement.
After startup, an installed copy updates `DisplayVersion` only when its
existing per-user uninstall record matches the running location and an
uninstaller is present. No new uninstall record is created for extracted copies.

The shared protocol is:

1. `PrepareUpdate` verifies the download and running installation, stages the
   new payload and a copy of the old helper, and starts it in private mode.
2. The helper validates the state file, hashes, paths, local control address,
   token, timestamp, and its own image. It holds the exact parent process handle
   identity, then waits with a bounded timeout. Linux uses `/proc` start time;
   macOS uses the kernel process start time and executable path.
3. The user confirms installation. The UI saves preferences and core state,
   commits the helper, and proceeds through the normal graceful quit path so
   the core and downloader close before the process exits.
4. The helper acquires the per-executable launch gate in a per-user lock
   directory, renames the old
   executable or bundle to a staged backup, moves the staged payload into place, and
   starts it with the original working directory and arguments.
5. The new process validates the one-time restart token, sends `authorize`,
   reaches database and window startup, sends `ready`, and only then removes the
   backup and private state.

For macOS, replacement and recovery move the complete bundle, never only its
executable. The staged bundle and backup retain the same tree digest.
Archive replacements require write permission in the installation's parent
directory. A mounted read-only DMG is not a replacement target.

Debian installations use a fixed system `pkexec` command. A privileged script
copies the verified package into a fresh root-owned directory, rechecks its
SHA-256 and package identity, rejects a non-upgrade, and invokes `dpkg --install`.
The application helper itself is never elevated. Package-manager ownership and
installation records stay consistent. Authorization cancellation or package
failure is reported and the helper attempts to reopen the installed app.
Package recovery belongs to the system package manager; there is no direct
binary rollback of a partially changed Debian installation.

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
cancellation, unsafe ZIP entries and extraction errors. `archive_test.go`
covers Linux/macOS/DEB verified payloads, archive traversal and links, and whole
bundle resource tampering. `helper_test.go` uses
temporary trees and harmless fixture executables; `process_windows_test.go`
exercises the real process identity, locked-target failure, and bounded wait
paths. UI tests in `cmd/dipper/software_update_ui_test.go` click the rendered
controls and assert state and engine effects without contacting the network.
`cmd/dipper/software_update_pending_test.go` adds persisted-switch, background
download, restart restoration, package/executable tampering, cancellation,
managed-path cleanup, and quit-veto coverage using real fixture files and a
stub replacement helper.
Platform compilation and fixture tests do not establish native macOS/Linux
authorization, desktop launch, or package-manager acceptance.

## Invariants

- A failed read or check is unknown, not “up to date”.
- No package is offered until its checksum and archive have been verified.
- The running executable stays usable until an explicit user confirmation.
- Replacement and cleanup are limited to validated paths under the target
  directory.
- A readiness acknowledgement must belong to this update and this API handshake.
- The single-instance lock and launch gate prevent concurrent replacement or
  relaunch.
