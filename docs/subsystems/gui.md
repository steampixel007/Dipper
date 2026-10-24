# gui

`cmd/dipper` is the window. It renders state, forwards user actions to `core`,
`downloader` and `source`, and owns no domain logic of its own. Visual tokens,
components, and interaction conventions are specified in
[DESIGN.md](../../DESIGN.md).

## What it owns

- The window, its pages, and the navigation between them.
- The `State` the pages render.
- Which async load is in flight, and which one it replaced.
- Startup: single-instance acquisition, then opening the database.

## Files

| File | Holds |
| --- | --- |
| `main.go` | Process start: single instance, data directory, engine and core construction, the window |
| `model.go` | `State`, `Saved`, the update loop, and the helpers that turn core types into UI types |
| `view.go` | The pages: browse, detail, following, downloads, settings |
| `core_ui.go` | The bridge to `core`: subscriptions, tasks, settings |
| `source_ui.go` | The bridge to `source`: catalog, detail, history, covers |
| `covers.go` | Cover image loading and its cache |
| `appearance.go` | Fonts, sizing, theme values |
| `proxy_ui.go` | The settings-page proxy card and the auto-mode description |
| `ui_render.go` | Deterministic sample state for layout rendering |
| `startup_error_windows.go`, `startup_error_other.go` | Reporting a failure before the window exists |

## State and the update loop

`State` embeds `Saved`, whose window preferences are persisted to
`data/preferences.json`; the rest is per-run. UI code reads `State` and calls
`Update` to schedule a repaint, and never mutates it from another goroutine
without going through `Update`.

The Mikan origin is a saved preference, not core state: the window tests a new
origin with one catalog probe before committing it, rolls the client back on
failure, and applies the saved origin at startup. The preference stores the
ASCII origin URL, never the Chinese label, so rewording the label does not
reset a user's choice.

Each async load has a revision and a cancel pair:

```
catalogRevision, detailRevision, historyRevision   uint64
catalogCancel,  detailCancel,  historyCancel       context.CancelFunc
```

Starting a load increments its revision and cancels the previous one. A result
that arrives with a stale revision is dropped rather than applied. This is the
same pattern `core` uses for subscriptions, and for the same reason: without it,
a slow catalog fetch for a season the user already navigated away from would
overwrite the newer page.

## Pages

`Page` is one of `browse`, `detail`, `following`, `downloads`, `settings`. Each
page is a function over `State` in `view.go`; a page never calls `core` directly,
it calls the bridge in `core_ui.go` or `source_ui.go`.

The detail page runs in one of two modes, distinguished by `Backfill`:
- **Subscribe** — choose a group and a rule, preview which existing releases
  match, and confirm. Existing matches are selected by default.
- **Backfill** — pick releases from an existing subscription's history to
  download now, without changing the rule.

The two modes share the resource list but not the confirm action. Keeping them
distinct is what makes "I cancelled this one release" not change what future
releases are matched against.

The settings page has a proxy card: a three-way choice (自动 / 关闭 / 自定义),
the address auto mode currently follows, and a 测试 button that saves the
current values and then probes the source origin. Test feedback and the
in-flight flag are per-run state, not settings.

## Startup

`main` acquires the single instance first, before opening the database or
starting the engine. If activation of an already-running window fails, the
process exits with an error instead of continuing — two processes on one data
directory would corrupt the SQLite file and the task folder.

## Rendering

`ui_render.go` holds a fixed sample state used to render each page's layout
without a live source or database. It is how a layout change is checked in
isolation, and it is the reason `view_test.go` can assert on rendering without
network access.

## Covers

`covers.go` loads a cover on a background goroutine and decodes it on the UI
thread, because the bitmap cache belongs to that thread. The file is named by
the SHA-256 of its URL and cached under `data/covers/`. A cover whose load is
in flight is marked in `loadingCovers`, set when the load is scheduled and
cleared on decode, on failure, and for the rest when the load's context is
cancelled; the card shows a ripple indicator while the flag is set, and the
bird mark only once the cover truly has none. The flag also keeps a second
`loadCovers` call from fetching the same URL again.

**Known limit:** the cache has no size cap and no eviction. A cover is fetched
once and then kept forever, so the folder grows with every anime ever browsed.
Adding eviction is a real change, not a tidy-up: it must decide what a browsing
session can afford to re-fetch, and it must not delete a cover the current page
is still showing.

## Brand marks

The bird logo shows in four places: the window icon and the tray icon (PNG
copies embedded from `cmd/dipper/assets/`, set in `main.go`), and the sidebar
chip and the cover fallback (`assets/dipper.svg` parsed in `loadImages`,
rendered through `ui.Image` in its own colors). The assets are copies of the
masters in `build/icon/`; regenerate them with `build/icon/gen.go` and keep
the copies in step, as [DESIGN.md](../../DESIGN.md) records.

## UI text

Every string the user sees is Chinese and lives in `view.go` or the bridge
files. This is product copy, not code documentation: it stays Chinese even
though comments and docs are English. See the language rule in
[code-standards.md](../code-standards.md).

Source-side Chinese literals — season names, weekday names, the `放送日期：`
label read from the page — are in `internal/source`, not here, and are data from
the upstream site.

## Tests

`view_test.go` renders pages from `ui_render.go`'s sample state and asserts on
what the page produces. Tests do not open a window, do not touch the network, and
do not read the user's real data directory.

Points a change is expected to prove:

- A stale catalog result does not overwrite a newer one.
- Switching from subscribe to backfill does not change the subscription's rule.
- Cancelling a selected release skips only that release.
- A page with an empty list shows its empty state, not a blank area.

## Invariants a change must preserve

- Every async load increments a revision and drops a stale result.
- The single instance is acquired before any state is opened.
- A page never calls a package directly; it goes through the bridge.
- A click never moves the page's scroll position.
- User-visible strings stay Chinese; comments and docs are English.
