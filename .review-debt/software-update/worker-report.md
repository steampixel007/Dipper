# Software update delivery report

Date: 2026-11-08

Workspace: `C:/Users/nathan/.herdr/worktrees/Dipper/v20261108_software_update`

Branch: `feat/software-update`

Implementation base: `7bada3450bf40c6bd3012e6d46358e3190c7ec78`

The initial implementation handoff left all changes uncommitted for review.
No additional agents or worktrees, commits, pushes, PRs, merges, tags, or
publication were performed during that implementation phase. The user
subsequently authorized committing, pushing, opening the pull request, fixing
its checks, and merging it. Tagging and release publication remain
unauthorized. No real installation was updated and no installer was executed.

## Implemented behavior

- Settings contains a separate software-update section with current version,
  manual checking, a persistent automatic-check preference, release notes,
  progress, cancellation, retry, and explicit installation/open actions.
  Anime-feed checking remains separate.
- Startup checks are asynchronous and limited to once per rolling 24 hours.
  Automatic failures are quiet; manual failures provide network/proxy advice.
  A future saved timestamp is treated as due. Development builds require a
  manual choice to move to an official stable build.
- GitHub Releases supplies version and asset metadata without a new backend.
  SemVer handles numeric cores, prerelease ordering, build metadata and large
  numeric components without integer overflow. Draft and prerelease releases
  are excluded. Selection requires an actual matching platform/architecture
  asset and `SHA256SUMS`; failures never report that the app is up to date.
- HTTP uses the existing proxy configuration. Official repository asset URLs
  and GitHub redirect hosts are validated. Release pagination and response
  sizes are bounded. Tests use `httptest`, with no live release API dependency.
- Downloaded bytes must match advertised size and SHA-256. Cancellation or
  verification failure removes incomplete downloads. Portable ZIP extraction
  accepts only one regular root entry named exactly `Dipper.exe`, rejects
  traversal/nested/absolute/alternate-stream paths and extra entries, and never
  overwrites an existing extraction target.
- Windows portable preparation stages beside the destination and copies the
  existing executable into a temporary private helper, preserving the
  single-EXE distribution. It validates helper state, hashes, paths, tokens,
  timestamps, working directory, parent image and process creation identity.
- The helper starts before normal application initialization and single-instance
  acquisition. It holds a handle to the exact parent process and waits with a
  bounded timeout, rather than relying on a fixed sleep or a PID alone.
- User-confirmed installation goes through preference/core persistence and
  normal graceful quit. Core and downloader close before parent process exit.
  A quit veto aborts preparation, restores the visible UI and retains the
  verified download for retry.
- A per-executable gate serializes updates and concurrent launches. Restart
  preserves arguments and working directory, including relative `--data` and
  download-root context, and reacquires the existing data-directory instance
  lock before opening application state.
- Path identity normalizes only the OS-selected temporary root. Windows short
  paths and macOS `/var` aliases therefore name the same trusted root, while
  symbolic links, junctions, and redirected extraction or lock directories
  below it are rejected. Windows resolution uses the path returned by a file
  handle because Go does not treat directory junctions as symbolic links.
- A token-bound authorization and readiness exchange acknowledges this update;
  the existing general `ready` file cannot acknowledge installation.
  Replacement or verified startup failure restores the old executable where
  safe. If the new process remains running without readiness, the helper
  retains the backup and reports recovery information instead of replacing
  the image beneath it.
- Completed-stage cleanup removes only the two expected regular files under
  an exact completion marker, then the empty stage directory. Unknown contents,
  malformed markers, redirected paths, failed stages and active stages are
  preserved.

## Distribution and platform scope

Rechecked `scripts/package.ps1`: the Windows ZIP contains only `Dipper.exe`.
Rechecked `internal/appdata`: Windows defaults to the system LocalAppData
`Dipper` directory. This feature adds no executable-adjacent portable-data mode
and does not relocate preferences, follows, downloads or databases.

Windows portable ZIP replacement is the automatic installation path. An
installed Windows copy is identified by adjacent `Uninstall.exe` and receives
the verified official setup asset with an explicit open action. macOS selects
DMG then archive; Linux selects DEB then archive. Those paths download and
verify the official package and show Chinese manual installation instructions.
They do not claim automatic in-place replacement.

Executable rollback never silently rolls back user data or promises database
downgrade compatibility. An unwritable destination, locked target, missing asset
or failed check is reported; backup recovery remains bounded to validated files.

## Changed files

Tracked existing files:

| File | Change |
| --- | --- |
| `AGENTS.md` | Index the update subsystem documentation |
| `cmd/dipper/core_ui.go` | Persist automatic preference and check timestamp |
| `cmd/dipper/main.go` | Early private helper, launch gate, graceful shutdown and authenticated readiness |
| `cmd/dipper/model.go` | Update state and injectable bridges |
| `cmd/dipper/source_ui.go` | Initialize update dependencies and defaults |
| `cmd/dipper/view.go` | Render update notice and settings section |
| `cmd/dipper/window_ui.go` | Commit on graceful quit; abort and restore on veto |
| `cmd/dipper/window_ui_test.go` | Scroll settings controls into view after the added section |
| `docs/subsystems/README.md` | Index update subsystem |
| `docs/subsystems/gui.md` | Describe software-update state and UI bridge |
| `docs/usage.md` | Explain checks, downloads, confirmation, recovery and platform limits |

New implementation and test files:

- `cmd/dipper/software_update_ui.go`
- `cmd/dipper/software_update_ui_test.go`
- `docs/subsystems/update.md`
- `internal/update/release.go`, `release_test.go`, `testdata/release.json`
- `internal/update/download.go`, `download_test.go`
- `internal/update/helper.go`, `helper_test.go`
- `internal/update/paths_windows.go`, `paths_other.go`, `paths_test.go`,
  `paths_windows_test.go`, `paths_other_test.go`
- `internal/update/process_windows.go`, `process_other.go`, `process_windows_test.go`
- `internal/update/gate_windows.go`, `gate_other.go`
- `internal/update/open_windows.go`, `open_linux.go`, `open_darwin.go`
- `internal/update/text.go`

Review artifacts under `.review-debt/software-update/`:

- This report and `run-delivery-build.ps1`, `run-supplemental-checks.ps1`,
  `run-cleanup-redcheck.ps1`.
- `green-cleanup.txt`, `green-ui-boundaries.txt`.
- `red-archive-entry-redcheck.txt`, `red-cleanup.txt`, `red-stage-marker.txt`,
  `red-future-timestamp.txt`, `red-manual-notice.txt`, `red-release-asset.txt`,
  `red-stale-download.txt`.
- `alias-green.log`, `alias-red.log`, `alias-gate-red.log`,
  `alias-restored-green.log`, `redirect-red.log`,
  `redirect-restored-green.log`.
- Earlier raw diagnostic logs: `red-archive-entry.txt`,
  `red-asset-architecture.txt`, `red-checksum.txt`, `red-parent-identity.txt`,
  `red-prerelease.txt`, `red-quit-download.txt`, `red-quit-veto.txt`,
  `red-rollback.txt`, `red-stale-offer.txt`. These raw logs are not represented
  as independently validated redcheck verdicts. Blank-line whitespace in
  `red-prerelease.txt` was normalized for the repository whitespace check.

## Verification results

Commands were run through background `hc` terminals, with no additional agents.

| Terminal | Command / scope | Result |
| --- | --- | --- |
| `t136` | Isolated delivery build after the path-identity fix | Exit 0; full tests/vet and Windows/Linux/macOS builds |
| `t134` | `go test ./internal/update ./cmd/dipper -run Update -count=1` | Exit 0; update 0.174 s, GUI 2.228 s |
| `t135` | `go test -race ./internal/update -count=1` | Exit 0; 10.390 s |
| `t137` | `go test -race ./internal/source -run ^TestClientInterruptedRetryRespectsCancellationAndRateLimit$ -count=20` | Exit 0; 20 repetitions, 21.834 s |
| `t12f` | Supplemental script: `scripts/check-torrent.ps1` and cross-build `cmd/redcheck` for Windows amd64, Linux amd64, macOS arm64/amd64 | Exit 0; both torrent behavioral checks passed; all four binaries built |
| `t12h` | Cleanup boundary redcheck, restored implementation, `-expect pass` | Exit 0, PASSED |

The delivery wrapper runs `scripts/build.ps1 -Test -CrossCompile`, including
`go test -count=1 -timeout 5m ./...`, `go vet ./...`, Windows amd64 and Linux amd64,
macOS arm64/amd64 application builds. The final full suite passed every package;
GUI took 30.577 s, update 3.179 s and downloader 68.864 s.

The final delivery build used process-scoped
`LOCALAPPDATA=C:/Users/nathan/AppData/Local/Temp/dipper-localappdata-0461eb0baf8c49ad879f69d2e65a5dd0`
and an isolated temporary Go cache. The wrapper restores its environment in
`finally` and removed that dedicated verification directory; its absence was
checked afterwards. The real `LocalAppData/Programs/Dipper` copy was not used.

`gofmt -l cmd internal` and `git diff --check` printed nothing on the final code.

Coverage includes release API/status/pagination errors, SemVer and unavailable
architectures, proxy and redirects, checksum/size/archive failures, cancellation,
stale UI results, preference persistence, quiet automatic failures, manual retry,
real rendered control clicks, minimum window settings scrolling, quit veto and
retained downloads, explicit-only installer opening, helper input validation,
exact parent identity, bounded waits, locked-target errors, replacement/startup
failure and rollback, preserved files/arguments/working directory, and serialized
concurrent launches.

`TestPortableHelperRealProcess` runs copied harmless test executables in
temporary directories for success, startup failure and abort. It exercises the
actual Windows helper process and single-instance handoff without executing
Dipper against the user's installation.

## Behavioral REAL RED and restored green

Each entry below used a deliberate behavioral mutation of the new interface,
retaining a compilable program. Mutations were restored before final testing and
building. Commands use `go run ./cmd/redcheck -pkg <package> -test '^<name>$'
-expect fail`, followed by restored green verification.

| Terminal | Test / deliberately broken behavior | REAL RED assertion |
| --- | --- | --- |
| `t11x` | `TestSoftwareAutomaticCheckTreatsFutureTimestampAsDue`: suppress due check for future timestamp | `software operation did not post its UI result` |
| `t11y` | `TestSoftwareManualFailureDoesNotBecomeGlobalNotice`: leak manual failure into global notice | `manual release failure leaked into the global software notice` |
| `t120` | `TestCleanupCompletedUpdateStages`: disable completed-stage reclamation | `acknowledged stage was not removed` |
| `t121` | `TestReplaceRestartPreservesContextAndData`: omit completion marker | `successful update retained unexpected stage files: [- helper.exe]` |
| `t122` | `TestChecksumAndArchiveErrors`: relax exact ZIP executable path | `accepted unsafe archive` for traversal, nested, absolute, ADS and lowercase paths |
| `t123` | `TestReleaseSelection`: accept wrong-architecture asset | `invented an unavailable Linux ARM64 asset` |
| `t126` | `TestSoftwareCancelDropsStaleCheckAndDownload`: remove canceled/stale-result cleanup guards | `stale canceled download retained files` |
| `t12g` | `TestCleanupPreservesUnexpectedStageFiles`: restore recursive stage deletion | `cleanup removed unexpected stage content` |
| `t12z` | `TestSystemTempAliasKeepsUpdatePathsSafe`: remove temporary-root normalization from path equality | `the OS temporary alias does not identify the same directory` |
| `t131` | `TestSystemTempAliasKeepsUpdatePathsSafe`: resolve the target before normalizing an absent launch-gate path | `aliases selected different launch gates while target absent` |
| `t130` | `TestUpdateRejectsDirectoryRedirects`: restore `filepath.EvalSymlinks`, which does not resolve Windows junctions | `did not reject redirected extraction directory before writing` |

Redcheck returned exit 0 and `REAL RED` for each entry. Restored green evidence
includes `t11v`/`t11w`, the full restored source runs `t127`/`t128`/`t12j`, and
`t12h` explicitly rerunning the last cleanup boundary test with `-expect pass`.
The latter covers unexpected files, nested directories and a malformed marker.
The path-identity fix was restored and separately verified with `-expect pass`
in `t132` (temporary alias, nested redirect, and launch gate) and `t133`
(redirected extraction and lock directories).

The earlier CI failure was reproduced as a legitimate OS alias: the Windows
runner uses `C:\Users\RUNNER~1`, and macOS maps `/var` to `/private/var`. Raw
path comparison rejected those safe temporary roots. A separate source race
failure passed 20 focused repetitions (`t137`) and is outside this feature.

Earlier compile errors, shell quoting failures, and `NOT RED` attempts
(`t11z`, `t124`, `t125`) do not count as regression proof. They were resolved
before the successful evidence above.

## Built artifacts

All paths below are relative to this worktree. The binaries are ignored build
outputs, not committed release assets. Runtime version is `dev` because this is
an untagged local build.

| Output | Bytes | Modified UTC | SHA-256 |
| --- | ---: | --- | --- |
| `bin/dipper.exe` | 50,946,048 | 2026-11-08T08:12:55Z | `45D47A1FED763A484B0626ECA2BFCE8B12C0445C1DDD68B54A13FF14E41BD91C` |
| `bin/dipper-linux-amd64` | 50,032,657 | 2026-11-08T08:13:01Z | `98D4540DAFB84AB4E6E4DB5DFE89CF030FA0F04760071EDB856F52ADB12D19D1` |
| `bin/dipper-darwin-arm64` | 48,150,914 | 2026-11-08T08:13:05Z | `ED0732FD120C02DA18B6DB7518AABB600FDF616C2855F848EB2A0A4798743D29` |
| `bin/dipper-darwin-amd64` | 51,043,968 | 2026-11-08T08:13:09Z | `6EDC1B6FE2C8F5E8C34BFCB118C1FBB3E26041D8D426309E77CAA81A05FEE70F` |

## Limits and outstanding validation

- No outstanding implementation or permission blocker was observed.
- Merge is authorized. This report was updated before the final commit; the
  pull request head and merged `main` are verified separately after CI passes.
- A full native Dipper GUI startup/update smoke was not run. Rendered-control
  tests and harmless Windows process fixtures passed; they do not establish a
  real installed-app upgrade or migrated-database downgrade compatibility.
- macOS/Linux binaries were cross-built, not executed on native hosts. Their
  platform launch gates and package-open commands have no native runtime proof
  from this worker.
- No live GitHub download smoke, real release ZIP installation, setup execution,
  or package publication was performed.
- No Windows ARM64 or Linux ARM64 build/runtime claim is made.
- Failed/unacknowledged stages and backups are intentionally retained for
  recovery. A running new process without authenticated readiness is not
  forcibly terminated or silently rolled back.

The worker remains available in its existing Herdr terminal for review and
follow-up. Pull request creation is now authorized; merging and release
publication remain unauthorized.
