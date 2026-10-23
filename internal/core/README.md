# Core integration

`core` has no UI dependency. `Open(path, Options)` initializes SQLite and returns
`*Core`; call `Start()` after the source and downloader adapters are ready.
`Start` is idempotent and returns immediately. `Close` cancels active work, waits,
then closes SQLite. Shut down the downloader after closing core.

```go
kernel, err := core.Open(databasePath, core.Options{
    DownloadRoot: root,
    Feed: func(ctx context.Context, animeID, groupID string) ([]core.Resource, error) {
        items, err := sourceClient.Feed(ctx, animeID, groupID)
        if err != nil { return nil, err }
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

The adapter implements `Ensure(core.DownloadTask) error`. Map `Hash` to downloader
task `ID`, `Directory` to `Folder`, preserve `Context`, and pass `Resource.Magnet`
and `Resource.TorrentURL`. `Hash` is a lowercase 40-character BT v1 info hash.
Source resource IDs and torrent filenames are never treated as BT identities.
Opaque torrents are fetched with cancellation, a 30-second timeout and an 8 MiB
limit; the exact bencoded `info` bytes determine the hash. Core persists a magnet
for that confirmed hash; the downloader must verify any torrent it fetches against
the expected hash. Admission context only controls acceptance, not the lifetime of
an admitted transfer. Replaying `Ensure` must retain pause and manual stop intent.

Public actions:

```go
CreateSubscription(ctx, anime, group, rule, completeHistory, selectedIDs) (Subscription, error)
DownloadExisting(ctx, subscriptionID, history, selectedIDs) error
UpdateRule(ctx, subscriptionID, rule) error
SetPaused(ctx, subscriptionID, paused) error
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

`Snapshot` is an independent read transaction and safe to project from a UI worker.
Settings, anime directories, subscriptions and outbox task records are durable.
The default upload limit is 1 MiB/s. A zero saved limit means unlimited; the UI
adapter must translate this to the downloader's unlimited value. Core persists
settings; the UI applies limits to the running downloader.

Subscriptions are unique by anime/group pair. Pass the full fetched history and
an explicit selected-ID slice; nil selects nothing. Creation commits skipped,
excluded and pending items atomically before submitting any download. An error
after commit returns a populated subscription, so the UI should refresh rather
than assume creation was rolled back. Pending items remain unread on failure.
Task acceptance and read transitions commit together. The unique hash table and
idempotent downloader close the crash window between acceptance and commit.
`Start` replays every accepted task, including tasks of paused subscriptions, to
restore already running transfers. It also retries durable pending intents.

Paused subscriptions do not fetch or mark items read. Pausing/changing rules
cancels active checks and pending admission contexts; revision checks reject old
responses even if a source ignores cancellation. The downloader must cooperate
with cancellation. Already accepted transfers keep running. Manual backfills on
a paused subscription are saved pending until resume. Rule edits preserve all
existing item decisions; explicit `DownloadExisting` overrides only selected
items and does not edit rules or unselected items.

The default check interval is 15 minutes. Source/admission failures use exponential
backoff, from 1 minute up to 1 hour. Manual checks and `RetryPending` can retry
immediately. The scheduler never retrieves complete history to backfill RSS gaps.

`PreviewDirectory` uses the same allocator as creation without persisting anything.
Same-anime subscriptions share the first saved directory across group, title and
root changes. Different anime with colliding names receive distinct directories.
Each BT task uses a hash subdirectory to isolate original torrent filenames.
A settings change or another creation between preview and commit can change a
new anime's allocation; use the returned subscription directory after commit.

Matching is literal substring matching with ASCII case and full-width ASCII
folding only. `All` requires every word; `None` rejects any word; each `Any` group
requires at least one word. Empty words/groups are ignored. `Match` supplies the
same explanatory result for previews and RSS. `ParseEpisode` and `SuggestWords`
are advisory pure functions and do not introduce hidden download constraints.
