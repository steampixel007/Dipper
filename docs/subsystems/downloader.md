# downloader

`internal/downloader` runs the embedded BT client and decides when a task has
done its job. It implements `core.Downloader`, whose contract is in
`internal/core/types.go`.

## What it owns

- The BT client, its per-task lifecycle, and resume across restarts.
- Where a task's files live and how pieces map onto them.
- The transfer policy: when seeding stops.
- The shared upload limit.
- The built-in tracker list and the per-task seeding-time target.

## Files

| File | Holds |
| --- | --- |
| `engine.go` | The engine, task registration, the client's configuration, progress sampling, pause and resume, manual stop, removal |
| `storage.go` | `diskStorage`, `diskTorrent`, `diskPiece`: path safety and piece-to-file mapping |
| `policy.go` | `policyState`: byte counters, seeding time, and the stop decision |
| `trackers.go` | The public tracker list appended to every admitted task |
| `platform_windows.go`, `platform_other.go` | The platform-specific pieces of the engine |

## The `Ensure` contract

`core` calls `Ensure(task)` and expects three things, which the type comment in
`internal/core/types.go` states:

1. **Idempotent by hash.** The same hash may be submitted again after a restart
   or a retry; the second call must not start a second copy.
2. **Durable before returning nil.** Once `Ensure` returns nil the task is
   accepted, and a crash must not lose it. `core` writes `Accepted` into its
   outbox only after the call returns.
3. **Observes the context.** A cancelled context means `core` no longer wants
   this task — the subscription was paused or removed. The engine must stop.

Replaying `Ensure` for the same ID and folder keeps the task's pause and manual
stop intent. The tracker list and URL parameters may differ between calls; a new
magnet is still checked for identity. A repeat submission does not fetch a
changed URL again just to check it.

`Pause`, `Stop` and `Resume` change a task's durable intent. A manual stop is
allowed on a task that has not finished: it detaches the torrent, halts the
payload, and keeps the verified pieces, so a later `Resume` continues instead of
starting over. `Resume` clears both the pause and the manual stop.

`Remove` forgets a task: it drops the torrent, rewrites the checkpoint without
the task, then deletes the content folder and the saved metadata. The checkpoint
goes first, so a crash mid-removal cannot resurrect a task whose folder is
already gone. Files that could not be deleted stay as orphans and the error is
reported as `ErrRemoveCleanup`; the task is already absent from the durable
checkpoint, so callers must still forget their delivery record. Errors before
that commit leave the task registered. Submitting the same hash again re-verifies
what survived. The piece
completion database keeps the removed task's rows — the completion API exposes
no way to delete a torrent's rows, and admission sets
`IgnoreUnverifiedPieceCompletion`, so a re-added task re-verifies those pieces
instead of trusting them.

When both a magnet and a torrent URL are given, metadata is fetched over HTTP
first, the two hashes are compared, and the trackers and explicit peers are
merged. If the fetch fails it falls back to the magnet; an identity mismatch
fails outright, and a cancelled context does not fall back. The metadata request
has a 30-second timeout and a 16 MiB cap.

Every admitted task also gets `trackers.go`'s built-in public trackers appended
as its last announce tier, skipping any URL a previous tier already carries.
The engine applies them on first admission, on restart from persisted metadata,
and to a magnet fetched over HTTP, so a torrent whose announce list is empty,
unreachable or blocked still has redundancy. They are deliberately few and
well-known; peer exchange and DHT stay primary.

The context only governs admission and the metadata request. Once a task is
admitted its lifetime belongs to the engine, so a cancelled admission context
does not stop a transfer that already started.

## Task states

`Snapshot.Status` is one of `metadata`, `downloading`, `seeding`, `paused`,
`stopped`, `completed`, `error`. `completed` means the policy stopped it;
`Complete` is a separate field saying the files have been verified whole. A
snapshot reports speeds in bytes per second and `Progress` in 0–1.

Only v1 and hybrid torrents that carry a v1 hash are accepted. A v2-only
torrent is refused with an error.

## Persistence

`Config.DataDir` holds three things: `tasks.json`, replaced atomically; one
`.torrent` per task, named by the SHA-256 of the task ID; and a piece completion
database under `pieces/`.

The engine samples every 500 ms and persists on a sample, and also on pause,
stop and a clean exit. A crash can lose the payload and time counters of the most
recent sampling window; recovery re-verifies the files' pieces rather than
trusting the checkpoint. Pause, manual stop and auto-stop survive a restart. A
clean `Close` does not change the user's intent, and resuming a running task does
not count the down time or double-count.

The state directory and any content folder must not contain one another, and two
tasks' folders must not overlap. Both are checked when a task is admitted.

## Storage

Every task gets a folder named by its info hash. The engine hands the torrent
library a `diskStorage` rooted there.

Path safety is enforced per component, not per path:

- `safeComponent` rejects an empty component, `.`, `..`, anything containing
  `/ \ : \x00 < > " | ? *`, a name with a trailing dot or space, and the Windows
  reserved device names (`CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`).
- The root is an `os.Root`, opened once per torrent. Every file operation goes
  through it, so a symlink or a crafted path cannot escape the task folder.
- Two files whose paths differ only by case are rejected as colliding, because
  they would be one file on Windows and macOS.

An existing file is accepted only if it is a regular file no larger than the
torrent says. A bigger or non-regular file is an error, not a truncation — the
alternative is silently destroying a file the user put there.

## Piece writes and completion

`diskPiece.transfer` maps a piece-relative offset onto the torrent's file
extents, opening each file for the span it covers. A write that would extend past
the piece's length is `io.ErrShortWrite`; a read that hits the end is `io.EOF`.

`MarkComplete` is ordered deliberately: it `Sync`s every file the piece touches
**before** recording the completion checkpoint. Reversing that order would let a
crash leave a checkpoint claiming a piece is on disk when it is not, and the
resume path would trust it.

Every storage error is stored into an `atomic.Pointer[taskFailure]` on the
torrent, so a failure raised inside the torrent library's goroutines still
reaches the engine's task state instead of being logged and dropped.

## Transfer policy

`policyState` tracks downloaded bytes, uploaded bytes and seeding time, and
`observe` is fed one sample per progress tick.

Seeding time only advances when the previous and current samples are both
eligible and the wall-clock gap between them is at most 3 seconds. A missed
sample means the machine was asleep or the process was busy, and continuity
cannot be assumed — so the seeding clock does not advance across the gap.

`reached` stops the task when either:

- seeding time has reached `seedTarget` (one hour), or
- uploaded bytes have reached the download size.

`UploadedBytes` is compared against the downloaded byte count, and against the
torrent's size when the download count is zero. The zero case is for a task
whose files were imported rather than downloaded: without it the target would be
zero and the task would stop immediately.

`counterDelta` treats a counter that went backwards as a reset and counts from
the current value, which is what the torrent library does after a restart.

## The shared upload limit

All tasks share one upload limit, set from `Settings.UploadLimitBytes` and
defaulted to 1 MiB/s. The UI labels it MB/s. The limit is shared rather than
per-task, so starting a second task does not double the total upload.

HTTP trackers, webseeds and torrent metadata fetches go through the proxy
chosen in settings (`internal/proxy`); peer connections stay direct.

Uploading starts during the download, not after it. `Config.UploadLimit` of zero
means the 1 MiB/s default, and a negative value means no limit. At runtime
`SetUploadLimit(0)` sets the upload budget to zero and a negative value removes
the limit.

The limiter's burst is 32 KiB. A chunk already admitted or reserved before a
budget change can still go out, so lowering the limit is not instantaneous.
Pause, stop and `Close` drop the connection rather than throttling it, which is
what actually stops a task's payload.

## Network and sleep

The torrent dependency is pinned to upstream commit `70072f1a7460` rather than
v1.61.0. It includes the message-writer wakeup fix
([#1078](https://github.com/anacrolix/torrent/pull/1078)) and keepalive timer
rearming fix ([#1071](https://github.com/anacrolix/torrent/pull/1071)). Without
them, a request-update or upload-limiter wakeup can be lost while the writer
fills its buffer, leaving an otherwise connected peer idle. The upstream tests
`TestMsgWriterBroadcastDuringFillWakesWriter` and
`TestMsgWriterKeepAliveTimerRearmed` exercise these windows directly.

`scripts/check-torrent.ps1` also runs Dipper's own writer regressions from
`internal/downloader/testdata/msg_writer_regression_test.go` inside an isolated
copy of the pinned dependency. This is a separate CI step on all three platforms:
`go test ./...` does not run a dependency's private tests. The checks inject a
broadcast during buffer fill and expire an idle keepalive using `synctest`, so
they need no real-time deadline. Dependencies are prepared before the test phase,
which runs offline, and the module cache is never edited. To reproduce acceptance:

```powershell
./scripts/check-torrent.ps1
./scripts/check-torrent.ps1 -ModuleVersion v1.61.0 -Expect fail
```

Both regressions must fail on behavioral assertions against v1.61.0 and pass
against the pin; the script uses `redcheck` to reject compile errors, skips,
panics, timeouts, or a missing regression.

Seeding time must not count time the machine was asleep or offline, so
eligibility is decided per platform:

- **Windows** asks `GetNetworkConnectivityHint` for an internet hint, and uses
  `QueryUnbiasedInterruptTime` for a clock that excludes sleep.
- **Linux and macOS** check for an up, running, non-loopback interface with a
  usable unicast address. That only shows the interface is available; it cannot
  prove the internet is reachable.

The host application passes MyGo Power's suspend and resume events to
`SetSuspended(bool)`, and may pass authoritative system network events to
`SetNetworkAvailable(bool)` to override the automatic detection. Either call
resets eligibility for the timing calculation. Neither changes the user's
persisted intent.

The seeding-time stop target is `policyState.SeedTarget`: a positive duration,
`-1` to seed without a time limit (the ratio target still applies), and zero
or any other negative value for the default one hour. The host passes the
setting at construction (`Config.SeedTarget`) and may change it at runtime with
`SetSeedTarget`; the change applies to running and future tasks and is
persisted in the checkpoint.

## Tests

Tests bind everything to `127.0.0.1` and turn off DHT, trackers, PEX, uTP,
webseed and UPnP. A seed is generated by the test itself and handed to the engine
over a real BT connection from an in-memory seed, which is how multi-file,
subtitle and empty-file SHA-256 checks are exercised. The one-hour policy is
checked against a virtual clock rather than by waiting.

A test client's config starts from `NewDefaultClientConfig`, never from
`torrent.TestingConfig`. `TestingConfig` sets `MaxAllocPeerRequestDataPerConn`
to 5 bytes, below the peer chunk size the buffer must hold, so transfers never
start; the failure surfaces as a timeout that reads like an engine defect.
`localConfig` is the only place the test client's settings are turned off, and
it does not touch that field.

The independent consumer in the upload-ratio test explicitly uses `NewMMap`
storage. Its torrent mappings close with the client, then its piece completion
database closes before the temporary directory is removed. The library's
default `NewFile` storage retains mappings of incomplete `.part` files after
closing, which prevents cleanup on Windows if the ratio stop interrupts the
consumer before it completes a piece. `TestConsumerStorageCloseReleasesIncompleteFiles`
reproduces that partial write directly and requires immediate directory removal
after closing, without depending on network timing or retries.

`pipeline_test.go` additionally wires the real chain together: a real
`core.SQLite` database, the adapter and the engine, covering selecting a release
from history, a real download, RSS deduplication, a core/engine restart, and a
cancelled release staying `skipped`.

Not covered: a real public BT swarm, the machine actually suspending, and any
non-Windows host.

Points a change is expected to prove:

- A path component that is `..`, absolute, or a Windows device name is refused.
- Two paths differing only by case are refused.
- `MarkComplete` syncs before the checkpoint (assert the order, not just the
  outcome).
- A sample gap over 3 seconds does not advance seeding time.
- A task with zero downloaded bytes still reaches its target when the import size
  is known.
- A mid-download manual stop halts the payload and keeps the verified pieces;
  a resume finishes the download.
- Removing a task leaves it out of the checkpoint and deletes its content folder
  and metadata; the same hash can be submitted again and re-verifies.
- The upload limit is shared across two concurrent tasks.
- A v2-only torrent is refused.
- Two tasks whose folders overlap are refused, as is a content folder inside the
  state directory.

## Invariants a change must preserve

- Every filesystem access under a task goes through its `os.Root`.
- Completion is recorded only after the bytes are synced.
- Seeding time never advances across a sampling gap.
- `Ensure` stays idempotent by hash and durable before it returns nil.
- A storage error reaches the task state, never only the log.
