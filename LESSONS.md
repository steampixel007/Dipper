<!-- reviewed-through: 3604636 (2026-11-08 02:28 +0800) -->
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

**A destructive action coordinates with work in flight before forgetting state.**
- b26ce9b's DeleteTask dropped the outbox record while an admission could still be inside `Ensure` outside every lock: the transfer then admitted into the engine after the delete reported success, invisible to the UI because core no longer listed it. Reproduced by holding the torrent HTTP response during a retry and deleting mid-fetch.
- Fixed in the night-audit worktree: `RemoveTask` cancels per-hash admissions, waits on the delivery channel, and only then finalizes records; the GUI calls it instead of sequencing engine and core calls itself.
- Seen 1× (2026-10-29).

**A promised retry keeps its own durable intent, not a subscription's.**
- Deleting and re-downloading a task whose subscription was cancelled (f49ed62 kept accepted records on purpose) deleted the files and the record, returned success, and nothing could resubmit: ResetTask only turned surviving subscription items pending. The user's explicit intent was lost with no error.
- Fixed in the night-audit worktree: the saved task becomes an independent intent that preserves source and folder; flush submits it without a subscription, across restarts. `TestResetAfterUnsubscribeRedownloadsIndependently` and `TestUIRedownloadAfterUnsubscribeKeepsOriginalDestination` fail on the old code (REAL RED: "independent intent was not resubmitted: calls=1").
- Seen 1× (2026-10-29).

**A committed removal reports cleanup failure without undoing the commit.**
- Engine.Remove deleted the durable checkpoint first, so a later file-deletion failure returned an error after the task was already forgotten. The GUI treated any error as "nothing happened" and kept core's record, so the next restart replayed a transfer whose folder was already gone.
- Fixed in the night-audit worktree: post-commit cleanup failures return `ErrRemoveCleanup` (deterministic regressions in `internal/downloader/removal_error_test.go`), and the removal caller finalizes core's records while surfacing the warning.
- Seen 1× (2026-10-29).

**A control's rendered label is not its behavior; click it in a test.**
- 36dde46 kept a 继续 label on stopped tasks, but pauseDownload only checked `Paused`, so the first click paused the stopped task and a second was needed. `TestUIDownloadsOfferOneTransferActionPerState` asserted visibility only and stayed green through the bug.
- Fixed in the night-audit worktree: the condition resumes paused-or-stopped, and `TestUIDownloadTransferActionChangesIntentOnce` renders the card, clicks once against a scripted engine, and asserts the command and projected flags.
- Seen 1× (2026-10-29).

**A cleanup collects the directories a claim names, not the files that survived.**
- 48389c8's `ownership.remove` walked a file's parent directories only after the file opened, so a payload the user had already deleted skipped its directories; deleting a non-shared task then left `season/Subs` behind, the folder removal failed with ENOTEMPTY after the claim was released, and the journal entry could never finish — every later `Ensure` answered `finish earlier removal`, across restarts.
- Fixed in the night-audit worktree: the directory walk runs before the existence check; `TestNonSharedRemovalAfterDeletedPayloadLeavesNoJournal` fails on the old code (REAL RED: "removal after a deleted payload file = download task removed but cleanup failed ... The directory is not empty.").
- Seen 1× (2026-11-08).

**A batch action is offered only when the predicate that schedules its work matches.**
- a12744f rendered 重试检查 for any `sub.CheckFailed` while `retryFailedChecks` schedules only failed follows that are not paused: with every failed follow paused the button was enabled and did nothing — no busy state, no notice, no error, and `core`'s `Check` returns nil for a paused follow anyway.
- Fixed in the night-audit worktree: one `retryableChecks` helper feeds both the button and the batch; `TestUIFollowingHidesRetryWhenOnlyPausedFollowsFailed` fails on the old render (REAL RED: "batch retry offered for follows it would skip").
- Done right: the per-follow inline action in `library_hero.go` already consulted `Paused` (`Disabled(sub.Paused || s.Busy)`).
- Seen 1× (2026-11-08).

**A removal's finalize and an intent serialize on one slot, and an intent is never dropped silently.**
- 29f1207 coordinated removal with admissions, but a re-download recorded while `RemoveTask` was between its tombstone and its finalize was still swept: `enqueue` returned nil for any hash with `removing[hash] > 0`, so the intent vanished without an error, and the finalize then marked the item skipped with 已删除下载任务. A failed removal had the same window: recovery finished the tombstone after an explicit re-download had already been accepted.
- Fixed in the night-audit worktree: every path that records or forgets a task holds the delivery slot across its transaction (`acquireDelivery`), the finalize re-reads its record under that slot (`recoverRemoval`), `supersedeTombstone` clears a stale failed tombstone when an explicit intent is recorded, and `enqueue`'s removing guard is gone. `TestRedownloadDuringRemovalKeepsIntent` fails on the old code (REAL RED: "the removal swept up the re-download: status = \"skipped\" reason = \"已删除下载任务\""); `TestRedownloadAfterFailedRemovalSupersedesTombstone` fails too (REAL RED: "the re-download was not submitted: ensures = 1").
- Seen 1× (2026-11-08).

**A retry that rewrites its own deadline at the base delay never escalates.**
- f05547b's `recoverRemovals` stored `NextRetry = now + backoff(Attempts+1)` and never incremented `Attempts`, so a removal that kept failing (a locked folder) retried at the base interval forever and hammered the downloader instead of backing off.
- Fixed in the night-audit worktree: the attempt count increments before the delay is computed. `TestRecoveryEscalatesRemovalBackoff` fails on the old code (REAL RED: "removal retry did not back off: first delay = 2m0s, second delay = 2m0s").
- Seen 1× (2026-11-08).

**A control that writes its bound value before OnChange must compare against live state, not the bound field.**
- 720fd8a's settings selector wrote the picked label into `SourceRoute` before calling `switchSourceRoute`, whose first check was `label == s.SourceRoute`; the check was therefore always true and picking a route never switched anything. `TestSourceRouteSwitchTestsThenSaves` calls the handler directly with an older bound value, so it stayed green.
- Fixed in the night-audit worktree: the no-op check compares the target origin with `s.Source.Origin()`, and the rollback label is derived from that origin. `TestUISettingsSelectSwitchesRoute` clicks both labels and fails on the old code (REAL RED: "choosing \"线路二 · 测试\" left the origin at ...").
- Seen 1× (2026-11-08).

**An alternative after an ASCII `\b` cannot match CJK; write the boundary the alphabet actually has.**
- bb24820 put 简/繁/外挂/内嵌/内封 into `flagsPattern` under `\b`, but Go's `\b` is an ASCII word boundary, so those alternatives were dead; a flag standing alone ("[1080P][内封][MKV]", "[中日外挂]") lost its label even though the captured testdata contains both forms.
- Fixed in the night-audit worktree: a separate pattern bounds CJK flags with `(?:^|[^一-鿿])`. `TestReleaseTagsRecognizeCommonLabels` fails on the old code (REAL RED: "\"[组] 剧名 [1080P][内封][MKV]\": got [1080p mkv], want [1080p mkv 内封]").
- Seen 1× (2026-11-08).

## Moved to the code standards

(None yet. Add a line here when a lesson is promoted.)
