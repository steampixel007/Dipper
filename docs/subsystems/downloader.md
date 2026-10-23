# downloader

`internal/downloader` runs the embedded BT client and decides when a task has
done its job. It implements `core.Downloader`, whose contract is in
`internal/core/types.go`.

## What it owns

- The BT client, its per-task lifecycle, and resume across restarts.
- Where a task's files live and how pieces map onto them.
- The transfer policy: when seeding stops.
- The shared upload limit.

## Files

| File | Holds |
| --- | --- |
| `engine.go` | The engine, task registration, the client's configuration, progress sampling, pause and resume |
| `storage.go` | `diskStorage`, `diskTorrent`, `diskPiece`: path safety and piece-to-file mapping |
| `policy.go` | `policyState`: byte counters, seeding time, and the stop decision |
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

## Tests

Tests use a real temp directory and a fake or local-only torrent, and never reach
the network. Points a change is expected to prove:

- A path component that is `..`, absolute, or a Windows device name is refused.
- Two paths differing only by case are refused.
- `MarkComplete` syncs before the checkpoint (assert the order, not just the
  outcome).
- A sample gap over 3 seconds does not advance seeding time.
- A task with zero downloaded bytes still reaches its target when the import size
  is known.
- The upload limit is shared across two concurrent tasks.

## Invariants a change must preserve

- Every filesystem access under a task goes through its `os.Root`.
- Completion is recorded only after the bytes are synced.
- Seeding time never advances across a sampling gap.
- `Ensure` stays idempotent by hash and durable before it returns nil.
- A storage error reaches the task state, never only the log.
