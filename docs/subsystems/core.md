# core

`internal/core` owns the model a subscription is made of and the state it lives
in. It has no UI dependency: the window calls it, it never calls the window.

## What it owns

- The subscription model: an anime, a fansub group, a matching rule, and the
  items seen so far.
- Identity: how a resource is told apart from another resource.
- The scheduler: when each subscription is checked next.
- The delivery outbox: what has been handed to the downloader, and what has not.

## Files

| File | Holds |
| --- | --- |
| `types.go` | `Anime`, `Group`, `Resource`, `Rule`, `Item`, `Subscription`, `TaskRecord`, `Options`, and the sentinel errors |
| `store.go` | `Core`, SQLite open and schema, the in-memory `diskState`, transactions, directory naming |
| `engine.go` | `CreateSubscription`, `Check`, `DownloadExisting`, `UpdateRule`, `SetPaused`, `RetryPending`, the scheduler and `flush` |
| `rules.go` | `Normalize`, `NormalizeRule`, `Match`, `ParseEpisode`, `SuggestWords` |
| `hash.go` | `ResourceHash`, `resolveHash`, the bencode reader that derives a torrent's v1 info hash |
| `download_root.go`, `video_*.go` | The default download root per platform |

## State

One SQLite file holds everything. Two tables:

- `core_state` — a single row, `id=1`, whose `body` is the whole `diskState`
  marshalled as JSON: settings, anime, subscriptions.
- `core_tasks` — one row per BT info hash, the delivery outbox.

The database opens with `journal_mode=WAL`, `synchronous=FULL`, a 5000 ms
`busy_timeout`, and `SetMaxOpenConns(1)`. **The single connection is load-bearing**:
every mutation goes through `c.transaction`, which takes `c.mu`, reads the row,
applies the callback, and writes it back. Two connections would let two
read-modify-write cycles interleave and lose one.

`diskState` keeps subscriptions in a map keyed by subscription ID and anime in a
map keyed by anime ID. A `Subscription.Items` map is keyed by the resource's
identity (`resourceKey`), not by position.

## Identity

A resource is identified by the first of these that is available:

1. its `ID` from the source,
2. the v1 BT info hash,
3. its torrent URL.

`ResourceHash` reads the hash out of a magnet link's `xt=urn:btih:` parameter,
accepting both the 40-character hex and the 32-character base32 form, and
rejecting a magnet that carries two different info hashes. A resource whose hash
is only knowable by downloading a `.torrent` gets `resolveHash`, which fetches it
under an 8 MiB cap and hashes the `info` dictionary from the bencode.

A resource with no hash yet is still accepted: `flush` resolves it later and
records the failure in the item if it cannot. This is why `Item.Hash` may be
empty while `Item.Status` is `pending`.

## Matching

`Match` normalizes the title and the rule, then applies three groups in order:

- `All` — every word must appear.
- `None` — no word may appear.
- `Any` — for each group, at least one word must appear.

`Normalize` folds ASCII case and full-width ASCII into ASCII, and converts the
ideographic space to a normal space. It deliberately does not fold anything else:
script and punctuation differences outside that range keep their literal
meaning. This matters for a Chinese catalog where `简` and `繁` are different
words, not spelling variants.

`ParseEpisode` and `SuggestWords` are for the window, not for matching. An
episode number is never part of identity.

## Revision and cancellation

Every subscription carries a `Revision`. `UpdateRule` and `SetPaused` increment
it and cancel any operation watching the old one. A check captures the revision
before fetching and refuses to commit if it changed (`ErrStale`) — otherwise a
slow check would write items under a rule the user has already replaced.

## The scheduler

`Start` runs one goroutine: an initial `flush` with `replay`, then `schedule`
unless `DisableScheduler` is set. `schedule` sleeps until the earliest of the
next subscription check, the next pending-item retry, the next failed-task
retry, or `Interval`, and is woken early by `signal` after any mutation.

`backoff` doubles from `RetryBase` up to `RetryMax`. Both a failed check and a
failed submission use it.

## The outbox

`flush` is the only path that hands a task to the downloader. It holds a
single-slot `delivery` channel for its whole run, so two flushes never run at
once. For each pending item it:

1. resolves the hash if it has none, recording a failure on the item if that
   fails;
2. writes the hash and clears the error;
3. copies the status from an earlier item with the same hash, unless the item
   was selected explicitly;
4. enqueues a `TaskRecord` if the item is still pending.

Then, for each task record not yet accepted, it calls `Downloader.Ensure` and
writes the result back to the task and to every pending item sharing its hash.
The downloader contract in `types.go` is the reason this can be retried:
`Ensure` must be idempotent by hash and must durably accept the task before
returning nil.

## Tests

`internal/core` tests open a real SQLite file in `t.TempDir()` through
`openTest`, inject a `Feed` and a `Downloader`, and drive the clock through
`Options.Now`. A test that needs to observe a submission uses a fake downloader
that records what it was handed.

Points a change is expected to prove:

- A rule change mid-check discards the check's result (`ErrStale`), and the
  items it would have added are not written.
- Two subscriptions pointing at the same hash submit one task, and both items
  end up downloaded.
- An item selected explicitly is downloaded even when an earlier item with the
  same hash was excluded.
- A subscription's directory is fixed when the subscription is created and does
  not move when the global root changes.

## Invariants a change must preserve

- Every state mutation goes through `transaction`. No direct `db.Exec` outside
  `store.go`'s open path.
- A write that a reader could observe half-done is not allowed: the whole
  `diskState` is rewritten in one statement.
- `SetMaxOpenConns(1)` stays 1 unless the read-modify-write cycle is redesigned.
- A failed read, parse or hash is recorded as unknown, never as empty.
- A directory name is decided once per anime ID and never recomputed.
