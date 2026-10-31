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
| `engine.go` | `CreateSubscription`, `Check`, `DownloadExisting`, `UpdateRule`, `SetPaused`, `RemoveSubscription`, `RemoveTask`, `DeleteTask`, `ResetTask`, `RetryPending`, the scheduler and `flush` |
| `rules.go` | `Normalize`, `NormalizeRule`, `Match`, `ParseEpisode`, `SuggestWords` |
| `tags.go` | `ReleaseTags`: recognized resolution, codec, container, source, audio, subtitle (compound and standalone) and version labels; every emitted word is a literal substring of the title |
| `hash.go` | `ResourceHash`, `resolveHash`, the bencode reader that derives a torrent's v1 info hash |
| `download_root.go`, `video_*.go` | The default download root per platform |
| `internal/proxy` | The proxy modes settings and HTTP requests share |

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

`ParseEpisode`, `SuggestWords` and `ReleaseTags` are for the window, not for matching. An
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

## Using it

`core` has no UI dependency. `Open(path, Options)` initializes SQLite and returns
`*Core`; call `Start()` after the feed and downloader are ready. `Start` is
idempotent and returns immediately. `Close` cancels active work, waits for it,
then closes SQLite. Shut the downloader down **after** closing core.

```go
kernel, err := core.Open(databasePath, core.Options{
    DownloadRoot: root,
    Feed: func(ctx context.Context, animeID, groupID string) ([]core.Resource, error) {
        items, err := sourceClient.Feed(ctx, animeID, groupID)
        if err != nil {
            return nil, err
        }
        out := make([]core.Resource, len(items))
        for i, item := range items {
            out[i] = core.Resource{
                ID: item.ID, Title: item.Title, Published: item.Published,
                Magnet: item.Magnet, TorrentURL: item.TorrentURL, Size: item.Size,
            }
        }
        return out, nil
    },
    Downloader: adapter,
})
```

The adapter implements `Ensure(core.DownloadTask) error`. It maps `Hash` to the
downloader's task ID and `Directory` to its folder, preserves `Context`, and
passes `Resource.Magnet` and `Resource.TorrentURL` through. `Hash` is a lowercase
40-character BT v1 info hash. A source resource ID and a torrent filename are
never treated as a BT identity.

An opaque torrent is fetched with cancellation, a 30-second timeout and an 8 MiB
limit, and the exact bencoded `info` bytes determine the hash. Core persists a
magnet for that confirmed hash, and the downloader must verify any torrent it
fetches against the expected hash.

Public actions:

```go
CreateSubscription(ctx, anime, group, rule, completeHistory, selectedIDs) (Subscription, error)
DownloadExisting(ctx, subscriptionID, history, selectedIDs) error
UpdateRule(ctx, subscriptionID, rule) error
SetPaused(ctx, subscriptionID, paused) error
RemoveSubscription(ctx, subscriptionID) error
DeleteTask(ctx, hash) error
ResetTask(ctx, hash) error
Check(ctx, subscriptionID) error
RetryPending(ctx) error
PreviewDirectory(anime) (string, error)
Snapshot() (State, error)
GetSettings() (Settings, error)
SetSettings(settings) error
Subscriptions() ([]Subscription, error)
ListTasks() ([]TaskRecord, error)
Anime() ([]AnimeRecord, error)
Start() error
Close() error
```

`Snapshot` is an independent read transaction and is safe to call from a UI
worker. Settings, anime directories, subscriptions and outbox task records are
durable.

The default upload limit is 1 MiB/s. A saved limit of zero means unlimited, and
the UI adapter must translate that to the downloader's unlimited value: core
persists the setting, the UI applies it to the running downloader.

Proxy settings live in `Settings` beside the upload limit: `ProxyMode` is one
of `auto`, `direct` or `custom`, and `ProxyURL` is the address `custom` uses.
`auto` reads `*_PROXY` and then the system proxy (Internet Options on
Windows); `direct` goes without a proxy; `custom` requires an http, https or
socks5 address, which `proxy.Validate` checks before it is saved. Loopback is
never proxied. The setting is durable; the running source client, torrent
fetches and the BT engine's HTTP traffic pick it up through `proxy.Func`
without a restart.

Subscriptions are unique by anime and group. Creation takes the full fetched
history and an explicit selected-ID slice; `nil` selects nothing. Skipped,
excluded and pending items are committed atomically before any download is
submitted. An error after that commit returns a populated subscription, so the UI
refreshes rather than assuming the creation was rolled back. Pending items stay
unread on failure.

Task acceptance and the read transitions commit together, and the unique hash
table plus an idempotent downloader close the crash window between acceptance and
commit.

`Start` replays every accepted task — including those of paused subscriptions —
to restore transfers that were already running, and retries durable pending
intents.

A paused subscription neither fetches nor marks items read. Pausing or changing a
rule cancels active checks and pending admission contexts, and the revision check
rejects an old response even when the source ignores cancellation. The downloader
must cooperate with cancellation. Transfers already accepted keep running. A
manual backfill on a paused subscription is saved pending until resume. A rule
edit preserves every existing item decision; an explicit `DownloadExisting`
overrides only the selected items and changes neither the rule nor the others.

Removing a subscription also cancels its active checks and pending admissions,
and drops the task records the downloader has not accepted yet, so nothing new
starts. Accepted transfers keep running and their records stay: `Start` replays
them to restore the running engine. The anime directory record survives, so a
later re-subscription gets the same folder. Files on disk are never touched.

`DeleteTask` and `ResetTask` forget a single delivery. Both route through
`RemoveTask(ctx, hash, redownload, remove)`, which cancels the hash's pending
admissions, waits on the delivery channel so an in-flight `Ensure` has finished
before anything is forgotten, and only then updates core's records. With a
`remove` callback the caller deletes the transfer between those steps: a true
result finalizes core's records even when deleting the files reported a cleanup
error, and a false result keeps the saved intent so nothing is lost. `Start`'s
replay would otherwise submit a dropped record again.

The two stores cannot share one transaction, so the intent is made durable
first and recovered: `RemoveTask` commits a tombstone (`TaskRecord.Removing`,
with the redownload choice) before invoking the callback. A crash anywhere
after that commit leaves a record that `recoverRemovals` completes at startup
and on every scheduler cycle — it asks the downloader, through the
`Downloader.Remove` the interface now carries, to finish its half (the method
must tolerate a task it no longer holds), then finalizes the tombstone into
the chosen end state. A failing downloader backs off through the record's
`NextRetry`, and `flush` and the scheduler never submit tombstoned records.
Deleted and reset items
return to the state the user chose: `DeleteTask` marks them skipped so neither
the scheduler nor a later check brings the download back, `ResetTask` marks them
pending so the next flush submits it fresh. Neither touches the disk; the
downloader's `Remove` deletes the transfer and its files.

A reset whose subscription is gone — RemoveSubscription deliberately keeps
accepted records — cannot borrow a subscription item, so the saved task becomes
an **independent intent** (`TaskRecord.Independent`): it keeps the original
source, folder and metadata, and `flush` submits it without a subscription.
`RemoveSubscription` preserves independent intents, and the scheduler includes
them in retry timing. This is what makes 删除并重新下载 work after cancelling
the subscription the task came from.

`CreateSubscription` holds a per-creation watcher while its first flush runs.
Removing, pausing or editing the rule during that first delivery cancels the
watcher; the closing transaction then keeps the committed result instead of
writing a zero-valued subscription back under the same ID, and a cancelled
delivery is not recorded as a check failure. Recreating the same subscription
with a fresh revision cannot un-cancel an earlier creation's watcher.

The default check interval is 15 minutes. A source or admission failure backs off
exponentially from 1 minute to 1 hour. A manual check and `RetryPending` retry
immediately. The scheduler never fetches complete history to backfill RSS gaps.

`PreviewDirectory` uses the same allocator as creation without persisting
anything. Subscriptions to the same anime share the first saved directory across
group, title and root changes. Different anime whose names collide get distinct
directories. Each BT task gets a hash subdirectory, so the torrent's own file
names stay isolated. A settings change or another creation between preview and
commit can change a new anime's allocation, so use the returned subscription's
directory after commit.

## Tests

`internal/core` tests open a real SQLite file in `t.TempDir()` through
`openTest`, inject a `Feed` and a `Downloader`, and drive the clock through
`Options.Now`. A test that needs to observe a submission uses a fake downloader
that records what it was handed.

Points a change is expected to prove:

- A rule change mid-check discards the check's result (`ErrStale`), and the
  items it would have added are not written.
- Deleting a task while its admission is blocked cancels the admission: no
  transfer starts after the delete reports success.
- A reset with no surviving subscription resubmits the saved task on its own,
  including after a restart, into the original folder.
- Removing a subscription during creation's first delivery leaves no
  zero-valued subscription behind and re-creating the same ID works.
- A crash after the removal tombstone commits is completed by startup
  recovery: the engine half is re-asked, the record finalizes, and a
  failing downloader backs off without resubmitting the task.
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
