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
| `window_ui.go` | Close/quit choices, remembered preferences, and the window/background settings card |
| `tray.go` | Tray registration checks, recreation after a lost registration, and cleanup |
| `ui_render.go` | Deterministic sample state for layout rendering |
| `startup_error_windows.go`, `startup_error_other.go` | Reporting a failure before the window exists |

## State and the update loop

`State` embeds `Saved`, whose window preferences are persisted to
`State.ConfigDir/preferences.json` (the data directory on Windows/macOS and
with explicit `--data`, the XDG config directory on Linux); the rest is per-run.
UI code reads `State` and calls `Update` to schedule a repaint, and never mutates it from another goroutine
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

The downloads page offers one transfer action per task state: pause during a
download, resume while paused or stopped before completion, retry on failure,
or stop seeding after completion. It does not also offer a stop-download action
beside pause: both preserve progress and require an explicit resume. Deletion
and opening the folder remain available separately. A single resume click clears
both pause and manual-stop intent, including tasks stopped before completion.
`State.Engine` depends on the download controls and snapshots it consumes; the
production downloader implements that boundary.
`TestUIDownloadsOfferOneTransferActionPerState` checks these controls, including
older stopped tasks and completed tasks. `TestUIDownloadTransferActionChangesIntentOnce`
clicks the rendered controls and checks the selected command and refreshed state
for stopped, paused, and running downloads without starting network activity.

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

Release packaging generates a temporary Go source file that sets `main.version`
from the Git tag, including prerelease and build metadata. The packaging script
removes it after the build; direct Go builds keep the source default or a linker
override. Startup readiness and logs report that version.

`main` resolves application directories through `internal/appdata` and acquires
the single instance lock in the data directory before opening the database or
starting the engine. Explicit `--data` keeps settings, covers and logs in that
same directory. If activation of an already-running window fails, the process
exits with an error instead of continuing — two processes on one data
directory would corrupt the SQLite file and the task folder.

`newAppState` attaches the core, source and running downloader before saved
settings are loaded. Loading settings applies the upload limit and projects
download snapshots, so the downloader must already be present.
`TestStartupStateHasDownloader` guards this startup wiring without starting
network activity.

## Closing, background work, and quitting

On Windows and Linux, closing the window asks whether to keep working in the
notification area or quit. The remember checkbox starts unchecked. Only an
explicit remembered choice changes `preferences.json`'s ASCII `CloseBehavior`
(`ask`, `background`, or `quit`); missing and unknown values ask. Settings can
change the choice or reset it to ask without taking a close action.

On macOS, closing hides the window and keeps the app running. Dock activation
reopens the existing window; the application quit menu and Command-Q stop the
app. The settings card explains this convention rather than offering the
Windows close choices. The menu bar icon uses a template image.

Before hiding on Windows, `trayController.ensure` queries the shell through
the tray's bounds, rather than treating a successful `NewTray` as proof of
registration. It recreates an icon whose registration has disappeared. If
registration still cannot be confirmed, the window stays open and the dialog
offers cancellation or explicit exit. Availability is checked again when the
user chooses to hide. Linux's AppIndicator exposes no bounds, so its creation
result is the available check; this is not proof that a particular desktop's
panel displays it. macOS can reopen from the Dock without a menu bar icon.

Single-clicking the Windows tray icon opens, restores, and focuses the window.
Its context menu offers open and quit. The quit sequence removes the icon.
An icon in Windows' overflow remains a registered icon; the app does not
override the user's taskbar visibility preference.

Closing or quitting while an operation is busy leaves the window open. A
failed settings save also leaves it open, reports the error, and restores the
previous close preference. If saving vetoes a quit requested from the tray or
the application menu, restore and focus the existing window through `Update`
so the save error is repainted even when the window was hidden. Cancel, Escape,
and backdrop dismissal never hide, quit, or remember a choice.

`window_ui_test.go` exercises the rendered controls, cancellation, one-time
and remembered choices, startup preference loading, changing/resetting the
choice in settings, failed writes, busy operations, and both platform policies.
`tray_test.go` checks failed registration, recreation, reuse, and cleanup with
a fake shell boundary. These tests do not establish native tray visibility or
Dock behavior; those require running the app on the respective platform.
The [window lifecycle verification](../window-lifecycle-verification.md)
records the checks that were run and the remaining platform boundaries.

## Appearance

Settings offers 浅色, 夜间, and 跟随系统. `Saved.Appearance` and the UI
preferences persist the ASCII values `light`, `dark`, and `system`; missing,
empty, and unknown values keep the original light appearance. Selecting a mode
saves it before applying the native theme; a failed write restores the previous
selection and reports an inline error. It preserves the other UI preferences.

`appearance.go` owns both palettes, the control theme, and the native window
background expression. `view` reads the host's effective appearance before
setting its custom theme. Every page and dialog resolves colors from its own
context, including accent foregrounds and status washes. The fixed black-and-white
bird stays on a pale brand tile. Startup applies the saved native theme before
creating the window; system mode uses MyGo's normal appearance notifications.
Tray icons continue to follow the taskbar appearance independently.

`appearance_test.go` covers rendered switching, system changes, persistence,
defaults, write failures, and context isolation. Offline `--render` produces both
light reference screens and matching `-dark.png` screens. These tests do not
establish native desktop theme notifications or native dialog appearance. The
[night-mode verification](../night-mode-verification.md) records the executed
checks, regression evidence, and remaining native boundaries.

## Rendering

`ui_render.go` holds a fixed sample state used to render each page's layout
without a live source or database. It is how a layout change is checked in
isolation, and it is the reason `view_test.go` can assert on rendering without
network access.

## Covers

`covers.go` loads a cover on a background goroutine and decodes it on the UI
thread, because the bitmap cache belongs to that thread. The file is named by
the SHA-256 of its URL and cached under `State.CoverDir` (the system cache
directory by default, or `--data/covers` when explicitly configured). A cover whose load is
in flight is marked in `loadingCovers`, set when the load is scheduled and
cleared on decode, on failure, and for the rest when the load's context is
cancelled. The flag keeps a second `loadCovers` call from fetching the same URL
again. A card with a cover URL shows a Blocks indicator until its bitmap is
available, including after a failed load; only a card without a cover URL shows
the bird mark. Starting or refreshing the catalog also loads saved subscription
covers from disk or the source, using the application context so a season change
or unavailable catalog cannot cancel their restoration. A later catalog refresh
retries missing covers. Loaded covers fill the
fixed frame in browsing, detail and following views: scaled uniformly,
centered, and cropped to the frame, so a poster never shows filler beside it.
`blueWash` is the box for the loading and no-cover states.

**Known limit:** the cache has no size cap and no eviction. A cover is fetched
once and then kept forever, so the folder grows with every anime ever browsed.
Adding eviction is a real change, not a tidy-up: it must decide what a browsing
session can afford to re-fetch, and it must not delete a cover the current page
is still showing.

## Brand marks

The bird logo shows in four places: the window icon and the tray icon (PNG
copies embedded from `cmd/dipper/assets/`, set in `main.go`), and the sidebar
chip and the cover fallback (`assets/dipper.svg` parsed in `loadImages`,
rendered through `ui.Image` in its own colors). The tray carries two cuts —
ink on a light taskbar, paper on a dark one — and `main.go` follows the
taskbar's appearance between them. The Windows executable also carries the
tile icon as a resource (`dipper_windows_*.syso`, made by `build/icon/gen.go`),
which the taskbar and Explorer read before any window sets its own. The assets
are copies of the masters in `build/icon/`; regenerate them with
`build/icon/gen.go` and keep the copies in step, as [DESIGN.md](../../DESIGN.md)
records.

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
