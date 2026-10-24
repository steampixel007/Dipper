<!-- reviewed-through: 97682a3 (2026-10-25 01:04 +0800) -->
# Lessons from merged work

Each day the commits since the `reviewed-through` mark above are reviewed:
what had to be fixed again, what should not have merged, and what was
verified too thinly. What we learn is kept here. Read this before changing
code. A review ends by moving the mark to the last commit it read, lessons
or not.

A lesson is a rule, then why, then the evidence (a commit or an issue). The
`Seen` count records the days a review found it.

**A lesson seen on three or more separate days moves into
[docs/code-standards.md](docs/code-standards.md)**, from experience into a hard
rule that every review checks. Leave a line under "Moved to the code standards"
once it moves.

## The shape of one lesson

```
**<one-line rule>**
- <which commit or issue it came from, and what went wrong>
- <the commit that fixed it>
- Done right: <a good example, optional>
- Seen N× (YYYY-MM-DD, YYYY-MM-DD).
```

## Current lessons

**Moving a block of statements reviews every line in it.**
- 36047a9 moved the `newState` wiring in `cmd/dipper/main.go` below the engine setup and dropped `s.Engine = engine` in the move; loading saved settings then dereferenced a nil engine applying the upload limit and the desktop exited before creating its window. The commit body listed native GUI runs under Not run.
- Fixed in cfd4a45: the wiring lives in `newAppState`, guarded by `TestStartupStateHasDownloader`.
- Seen 1× (2026-10-25).

**Path identity compares canonical forms on both sides, under the lock.**
- `TestConcurrentEnsureAndClose` failed only on the windows-latest and macos-latest runners ("task ID already belongs to a different folder"): admission stored the task folder after `EvalSymlinks`, while a replay reaching `Ensure` before the folder exists carried the unresolved alias (`RUNNER~1`, `/var` → `/private/var`). A dev machine's temp dir has no alias, so the suite could not catch it locally.
- Fixed in e961aa1: `Ensure` canonicalizes under the lock before the replay check; `TestEnsureReplayThroughFolderAlias` reproduces the window and skips where the platform offers no alias.
- Seen 1× (2026-10-25).

**An icon change lists all three Windows surfaces: in-app assets, the runtime tray icon, and the exe's embedded resource.**
- ef6aeb6 ("show the bird logo across the app") swapped the window icon and shipped one tray cut — ink on a dark taskbar — while the executables carried no icon resource at all, leaving the taskbar and Explorer blank before a window set its own.
- Fixed in 4748c53: a dark-bar cut that follows the taskbar's mode with a stable tray ID, and `gen.go` writes a `.syso` for amd64 and arm64.
- Seen 1× (2026-10-25).

**A test of a built artifact walks the structure the real consumer walks.**
- The first cut of 4748c53 passed `TestWindowsSysoEmbedsAppIcon` while `saveSyso` wrote every resource directory one level too low: scanning for the PNG bytes passed on a corrupt container, and Windows found neither RT_ICON nor RT_GROUP_ICON (`PrivateExtractIconsW` returned error 1813).
- The follow-up commit in the same PR fixed `saveSyso` and rewrote the test to walk the resource tree (root → type → id → language → data entry), verified with `PrivateExtractIconsW` at 16/32/48/256 px.
- Seen 1× (2026-10-25).

**An exit path enumerates its vetoes: every refusal restores a visible state.**
- 6f53162 preserved the window when shell registration or settings persistence failed, but a preferences save failure during a tray quit silently left the app hidden in the tray — the one veto the first cut did not list.
- Fixed in d2c588f in the same PR: `exitApp` save failures route through `Update` to restore, focus and repaint the window, and the same recovery serves application quit events; `TestWindowTrayQuitSaveFailureRestoresWindowAndRepaints` covers it.
- Seen 1× (2026-10-25).

**What the build depends on at checkout is committed to the repo, not left to the machine.**
- The Formatting step failed on windows-latest: the runner's autocrlf checkout turned every `.go` file CRLF and `gofmt -l cmd internal` listed 44 files, while the Windows dev machine's working tree stayed LF and the same check passed locally.
- Fixed in 51e03a5: `.gitattributes` pins text files to LF and marks the image assets binary.
- Seen 1× (2026-10-25).

**A multi-phase wait gives each phase its own budget.**
- `TestRealMagnetPauseRestoreAndManualStop` failed on windows-latest: the shared 30 s `waitFor` spent itself on the loopback handshake and the magnet metadata exchange (piece-verification warnings landed 4 ms past the deadline), so the payload phase never began; two more waits had the same exposure.
- Fixed in abe4b87: `waitFor` takes its budget — 60 s for metadata, 30 s for payload — so a recurrence names the phase it died in.
- Seen 1× (2026-10-25).

**A library client in a test owns what it holds open, past its own Close.**
- `TestRealUploadRatioStopsAndCloseStopsPayload` failed on windows-latest only (PR #6, 2e68af2, and again inside PR #4): the consuming client's default file storage caches an mmap per written `.part` file and never unmaps it when the torrent drops, so `t.TempDir` cleanup hit a sharing violation on the still-open handle. Linux and macOS unlink mapped files, and the same commit passed a second Windows run — the red was mid-download timing.
- Fixed independently twice: PR #4 (e585b96) moved the consumer to per-torrent `NewMMap` storage whose mappings close with the client, plus a deterministic regression; PR #6 reproduced the leak deterministically on Windows first (close mid-download, then `RemoveAll` fails) per the inject-the-delay rule. The merge keeps PR #4's storage — the rule is the same either way.
- Seen 1× (2026-10-25).

## Moved to the code standards

(None yet. Add a line here when a lesson is promoted.)
