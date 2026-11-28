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
| `view.go` | The pages: browse, detail, following, episodes, downloads, settings |
| `version_setup.go` | Focused single-column watch-version UI, mainstream-coverage scoring, group preview ranking, and collision warnings |
| `title_setup.go` | Two-step pick-by-example original-title rule extraction and fine-tuning UI |
| `browse_detail.go` | Browse synopsis, its independent source request, and responsive panel |
| `core_ui.go` | The bridge to `core`: subscriptions, tasks, settings |
| `source_ui.go` | The bridge to `source`: catalog, detail, history, covers |
| `software_update_ui.go` | The software update settings card, release dialog, and update lifecycle |
| `covers.go`, `cover_disk.go` | Cover demand, decoding, and memory and disk caches |
| `appearance.go` | Light/dark neutral palettes, accent tokens, control theme, and motion curves |
| `typography.go` | Type scale (`>=14 DIP`, `22 DIP` page title), 4 DIP grid geometry, radius scale (`8–24 DIP`), button/dialog/scroll-edge helpers |
| `fonts.go` | Embedded Geist font weights (`400`, `500`, `600`, `700`) registered with CJK fallback |
| `icons.go` | Embedded Reicon outline icons (`18×18` glyph, `1.5 DIP` stroke, `24×24` alignment box) |
| `intent.go` | Controls-on-intent visibility (`hover`, `FocusWithin`, scroll-near-edge, active work) |
| `cascading_menu.go` | Desktop cascading submenu (`120 ms` open delay, `300 ms` close delay, safe triangle, keyboard navigation) |
| `library_hero.go` | Episode library hero header and its cascading `···` menu |
| `proxy_ui.go` | The settings-page proxy section and connection test controls |
| `window_ui.go` | Close/quit choices, remembered preferences, and the window/background settings section |
| `tray.go` | Tray registration checks, recreation after a lost registration, and cleanup |
| `ui_render.go` | Deterministic sample state for layout rendering |
| `startup_error_windows.go`, `startup_error_other.go` | Reporting a failure before the window exists |

## Visual system

`appearance.go` owns both palettes (warm Xuan Paper Light and Pine-Soot Dark)
and motion tokens; `typography.go` owns type sizes and control geometry;
`fonts.go` registers embedded Geist static weights (`400`, `500`, `600`, `700`)
with platform CJK fallback; `icons.go` renders Reicon outline icons (`18×18`
glyph, `1.5 DIP` stroke, centered in a transparent `24×24` box). `view.go`
composes them without introducing another color system. Warm neutral light
(`#F7F5F0` / `#EFECE5` / `#FCFBF8`) and dark (`#141311` / `#1A1816` / `#22201D`)
surfaces carry >=90% of the screen, paired with a monochrome ink/ivory accent
(Wu-Mo `#23201C` in light, Ya-Bai `#EFECE6` in dark) so anime posters supply
the color, while muted antique gold and restrained crimson appear only for
failures or destructive confirmation. Healthy or completed states use neutral
typography without green badges. Flat layout eliminates nested cards, bordered
settings sections, boxed download rows, decorative dividers, and explanatory
subtitles or helper prose.

Poster status pills and hover actions use `plugins/glass` tinted Liquid Glass
over the artwork, and the dark-mode episode library hero renders a subtle cached
blurred ambient cover backdrop (`.Cache()`) while light paper surfaces and
browse detail panels stay uniform `surface`. A `glass.ScrollEdge`
softly fades scrolled content into the top bar and sticky footers, and floating
dark dialogs carry warm top edge highlights (`InsetShadow`).

The sidebar occupies 224 DIP. The top bar occupies a fixed 64 DIP slot with an
opaque canvas background and no divider, showing a compact title on sub-pages
or once a top-level page scrolls. Page content is clipped below it; `uiScrollTo`
relies on that slot. Browse filters sit directly on the page, with weekdays
below. Poster radii share the 12 DIP cover token and deepen their shadow on
hover. Component radii step across the 8–24 DIP scale (`8, 12, 16, 24`)
by size and hierarchy, and all spacing stays on the 4 DIP grid at 1.5×
spaciousness. Navigation counts share the label row at normal text size and move
below the label when text is enlarged; bottom sidebar utilities left-align with
icons to match the main navigation rail, and the top bar omits a duplicate help
button. Sticky dialog, detail, and drawer footers use a bottom `glass.ScrollEdge`
fade above their divider. Settings appearance uses visual theme
preview cards. `visual_system_test.go`, `design_system_test.go`, and
`ui_polish_test.go` verify registered Geist weights, Reicon icon box geometry,
warm neutral chrome, flat unboxed layout, accent area budget, dark monochrome
ivory primary accent, selected navigation, count alignment, visual theme preview
cards, single-row split browse toolbar, clickable group chips and collapsible
download group titles, shared-folder deduplication, keyboard shortcuts, and
clickable navigation at normal and doubled text size.

Browse and follow tiles use the column width and each loaded image's original
ratio to determine poster height. This keeps the complete artwork visible
without letterboxing or stretching. Placeholders keep a 1:1.36 ratio until the
bitmap loads; bounded detail thumbnails retain their existing contain behavior.
`poster_layout_test.go` checks all four artwork edges and the title's position
for portrait, landscape and square covers in both grids and themes.
`poster_source_test.go` decodes real captured source artwork through the loader,
page grid and disk cache so a cropped thumbnail cannot hide behind synthetic
bitmaps.

Interactions combine MyGo v0.9.0's velocity-preserving springs and Oklab color
interpolation with controls-on-intent (`intent.go`) and cascading menus
(`cascading_menu.go`): critically-damped 220 ms springs for tactile
press/hover/shadow feedback, 280 ms lightly-damped springs for shared-element
`Match` indicators (sidebar selection pill, season thumb, weekday underline,
download filter pill, and version/proxy segmented thumbs) and follow-to-library
Hero poster flights, directional `ChildExit` season
switches with title `Morph`, rotating download-group disclosure chevrons, and
`ContentTransition{Numeric: true}` for live speeds and counts.
Secondary controls follow controls-on-intent (`intent.go`): hidden at rest
while reserving layout space (`Opacity` 0->1 over 130 ms, remaining keyboard
Tab-reachable), revealed on pointer hover, `FocusWithin`, scrolling near the
container's viewport edge, or active work (such as an open menu). Primary
actions (`播放`) and failure recovery (`重试`) stay visible at rest. Desktop
cascading menus (`cascading_menu.go`, used in the episode library hero's `···`
menu) implement a 120 ms open delay, 300 ms close delay, and an 8 DIP extended
safe triangle from the pointer to the open submenu's nearest edges, plus full
keyboard navigation (`Up`/`Down`/`Right`/`Left`/`Escape`/`Enter`/`Space`).
`State.ReduceEffects` or the system reduce-motion preference removes spatial
motion and transitions while preserving focus, borders, contrast, and menu
open/close timers. `intent_reveal_test.go` and `cascading_menu_test.go` verify
controls-on-intent and cascading submenu behavior.

## State and the update loop

`State` embeds `Saved`, whose window preferences are persisted to
`State.ConfigDir/preferences.json` (the data directory on Windows/macOS and
with explicit `--data`, the XDG config directory on Linux); the rest is per-run.
UI code reads `State` and calls `Update` to schedule a repaint, and never mutates it from another goroutine
without going through `Update`.

Software update state is separate from anime-feed checks. `SoftwareAuto`,
`SoftwareAutoDownload`, `SoftwarePending`, and `SoftwareCheckedAt` are saved in
`preferences.json`; the timestamp limits automatic release checks to once per
day. Automatic downloads are opt-in and apply only to release Windows portable
offers. The release client uses the same proxy transport as the
rest of the app. A check or download carries a revision and cancellation pair,
and a stale result is dropped. Automatic failures are quiet; manual failures
show the error and a retry action. A discovered update is a nonblocking top-bar
notice until the user opens and confirms it. With automatic downloads enabled,
an automatic check starts a verified background download without opening a
dialog. Its completed notice calls `installSoftware` directly when clicked;
preparation and graceful-quit vetoes retain a retryable download.

`software_update_pending.go` restores completed portable downloads at startup,
before the automatic-check and daily-throttle gates. It checks the version,
current installation type, managed paths, and both package and executable
hashes off the UI thread. Only the UI callback publishes the verified result.
An obsolete, missing, or modified download is cleared from preferences and
reclaimed only inside the managed update directory. Other settings saves keep
pending metadata. Software fixture tests in `software_update_pending_test.go`
cover these states, notice clicks through the full view, cancellation, and
quiet failures without using the network or a real replacement helper.

The Mikan origin is a saved preference, not core state: the window tests a new
origin with one catalog probe before committing it, rolls the client back on
failure, and applies the saved origin at startup. The preference stores the
ASCII origin URL, never the Chinese label, so rewording the label does not
reset a user's choice. Line two (`mikanani.me`) is the default when there is no
saved origin. An explicit `--source` takes precedence at startup.

Each async load has a revision and a cancel pair:

```
catalogRevision, detailRevision, historyRevision   uint64
catalogCancel,  detailCancel,  historyCancel       context.CancelFunc
filesRevision                                       int
```

Starting a load increments its revision and cancels the previous one. A result
that arrives with a stale revision is dropped rather than applied. This is the
same pattern `core` uses for subscriptions, and for the same reason: without it,
a slow catalog fetch for a season the user already navigated away from would
overwrite the newer page.

The download file dialog loads its list through the same revision guard
(`filesRevision`) without a cancel pair: opening another task clears the
previous task's entries and drops a stale result instead of applying it. While
the load runs the dialog shows a loading line, and an empty result is a
terminal state with its own copy — an empty list must never leave the dialog
reading forever. `TestUIFilesDialogScopesResultsToTask`,
`TestUIFilesDialogClearsStaleEntriesOnOpen` and
`TestUIFilesDialogEmptyResultIsTerminal` check the three states.

## Pages

`Page` is one of `browse`, `detail`, `following`, `episodes`, `downloads`, `settings`. Each
page is a function over `State` in `view.go`; a page never calls `core` directly,
it calls the bridge in `core_ui.go` or `source_ui.go`.

The downloads page offers one transfer action per task state: pause while
downloading or queued, resume while paused or stopped before completion, retry on failure,
or stop seeding after completion. A row that is actively
downloading also shows its estimated remaining time, derived from the
remaining size and the current speed, and hides the estimate whenever the
transfer pauses, stops, fails or the speed is unknown. It does not also offer a stop-download action
beside pause: both preserve progress and require an explicit resume. Deletion
and opening the folder remain available separately. A single resume click clears
both pause and manual-stop intent, including tasks stopped before completion.
`State.Engine` depends on the download controls and snapshots it consumes; the
production downloader implements that boundary.
`TestUIDownloadsOfferOneTransferActionPerState` checks these controls, including
older stopped tasks and completed tasks. `TestUIDownloadTransferActionChangesIntentOnce`
clicks the rendered controls and checks the selected command and refreshed state
for stopped, paused, and running downloads without starting network activity.

The downloads page is a transfer queue: waiting, downloading, paused, failed,
incomplete stopped, and sharing tasks remain visible. Completed tasks disappear
when sharing stops, whether stopped manually or by the sharing policy. A failed
task stays visible even if complete and stopped so that recovery remains
available. Queue filtering never removes the stored task or its files; the
episode library retains playback and management through its episode overflow
menu: file listing and individual file playback, deletion, and redownload use
the existing handlers and confirmation dialog. Navigation counts, status
counts, group totals, and search all use the same queue membership. The sharing
filter excludes paused completed tasks, which belong under paused. A sharing
task with zero current upload speed remains in the queue. When only finished
tasks remain, the empty state points to 下载记录, a secondary history view on
the downloads page. It lists every hidden finished task regardless of whether
its subscription still exists and keeps its own scroll position; management
uses the same file, playback, deletion, and redownload handlers as the queue and
episode library. The back action restores the current queue, while entering the
downloads page from navigation always returns to it. Batch download controls
appear only when there is a download to pause or a task to resume. Batch pause
also pauses queued tasks, which have their own 排队中 filter and badge.
Settings → Download accepts a whole number from 1 to 10 for simultaneous
tasks and defaults to 3. Enter or Save applies a valid count independently
of the transfer limits and Tracker settings.
Settings → Network and Proxy explains what download-source addresses do and
opens a manager for online lists and manual entries. The XIU2 recommended
list is the sole default supplemental source.
Each online list has a detail view with its link, last update, and addresses.
Search and pagination keep the overview and address details usable with many
entries. Deleting an address from a list removes it from the current view.
It can be added back from the deleted-address view. The recommended list can
also be deleted and added back as a whole. Edits take effect on Save.
`downloads_active_test.go` covers visibility, counts, search, live completion,
and retained library playback, including a later file in a batch and confirmed
deletion or redownload through the library. `download_history_test.go` covers
management after unsubscribing, cancellation, retained destinations on
redownload, and selecting a later file without a follow.

Download sizes are projected from the stored raw byte count; legacy MB fields
are used only when that count is absent. A group total is called known only
when every included task has a size, and a mixed group qualifies its total and
reports the unknown count separately. Transfer speed uses binary units while
file and resource sizes use decimal units. Stopped transfers have their own
filter instead of sharing 已暂停, a failed transfer offers 重试 before any
recovery action, and an empty search or filter keeps the toolbar, counts, and
reset controls visible. `exact_sizes_test.go` and `downloads_filters_test.go`
cover these distinctions.

The detail page runs in one of two modes, distinguished by `Backfill`:
- **Subscribe** — choose a group and a watch version in a focused single-column
  layout, confirm whether to download aired episodes or follow future updates
  only, and optionally expand the per-episode detail list (`自选集数 / 查看明细`).
  The synopsis request caches the full `Detail` response (including each group's
  detail-page `Preview`) per origin and anime ID so opening watch-version setup
  reuses it without a second `Detail` fetch. A new setup defaults to the best
  subtitle group using those previews — preferring Chinese-subtitled groups with
  recognized versions and stronger episode coverage, breaking ties by the user's
  saved language and priority — and skips the `History` request completely when
  the selected group is not truncated. A truncated group renders its preview
  recommendation immediately while blocking submission until its full history
  loads. Language and priority choices persist in `preferences.json` using ASCII
  keys. Recommendation ranks mainstream episode-coverage versions ahead of
  sparse one-off releases, grades compatibility without rejecting untagged MP4
  or HEVC/MKV-only groups, falls back gracefully when sizes are unknown, and
  warns when non-revision same-episode releases collide under one signature.
  The initial download selects one matching release per recognized episode,
  preferring explicit revisions then the later publish date, with repetitive
  original filenames hidden by default behind a `显示原标题` toggle while
  unknown-episode rows and episode comparisons show titles automatically.
  The pick-by-example original-title flow (`按原标题自选`) lets users pick a
  representative sample release to extract distinguishing keywords automatically
  (with optional fine-tuning, clickable words with explanations, and manual
  keyword/alias entry), supports downloading matched existing episodes together
  with following future updates, and still permits saving a zero-match future
  rule.
- **Backfill** — pick releases from an existing subscription's history to
  download now, without changing the rule.
Browse cards mark followed anime with a 已追番 badge. Clicking a card opens
its detail panel. The catalog reflows
beside the right panel on wide windows and stays full width behind a centered
dialog on narrow windows. Switching cards cancels the previous detail request.
The panel shows the cover, title, update day, optional premiere date, synopsis,
available subtitle groups (distinguishing an unloaded response from an empty
group list), and validated external links (Bangumi and official site). Followed
anime also show their followed subtitle group(s) and offer 进入剧集库 beside
查看资源; the episode library's back button returns to browse details when
entered from there. The panel retains its reading position when returning from
group selection, resources, or the episode library. Closing it restores the
full-width catalog's scroll position.
The downloads page can open a followed anime's resource list directly. Its
back button returns to downloads.

The synopsis is shown in browse details. The group and resource pages keep the
compact anime header and start their options with the subtitle group.
Loading, unavailable and missing descriptions have distinct copy in browse
details. A detail response loaded from the active browse request is cached per
source origin and reused when switching posters, reopening the panel, or
entering group selection and resources — avoiding a duplicate detail fetch;
explicit reload buttons bypass the cache, and switching source routes invalidates
it. Catalog and core state refreshes retain loaded anime metadata, while a
successful group-page detail response is authoritative. `anime_summary_test.go`
checks the rendering boundary and `browse_detail_test.go` checks the return
path, library entry, metadata display, and detail reuse.


The sidebar uses MyGo's Sidebar tree and badges. Detail selects Browse and
the episode library selects Following. Arrow keys, Home, and End navigate
the tree when it has focus.
The browse page uses MyGo's SearchField: typing or clearing resets the
catalog scroll to the top, and the field has a built-in clear button.
Year, season, and search share one band above a text-only weekday row.
Season and weekday choices use MyGo segmented controls, so the
poster grid keeps the visual weight; `browse_layout_test.go` pins that
hierarchy and the 200% text-scale bounds.
Browse navigation runs from autumn 2013 through autumn of the current year.
Both source routes return the autumn 2013 catalog and an empty response for
the advertised spring 2012 entry. Selecting 2013 opens autumn. The toolbar
and source-error recovery actions disable arrows that would leave this range.
The following page searches saved follows by anime title, subtitle group, or a
word the saved rule requires. The count line reports how many follow tiles
match, the SearchField has a built-in clear button, and an empty result
offers a separate clear action. Cards whose last RSS check failed
float above the saved order so a silent failure cannot hide at the bottom.
Card actions resolve the subscription by its stable ID, never by its position in
the filtered list, so a narrowed search cannot pause, edit, or open the
resources of a follow the card does not show.
Each tile is a poster: cover, title, and subtitle group.
Rules, RSS timestamps, skipped-resource counts, folder paths, and recent
release titles stay off the tile. Status is minimal: a 检查失败, 已暂停, 排队中, or
下载中 badge on the poster — the progress mark carries the follow's overall
download progress, weighted by the known size of each transfer, so several
running episodes read as one 下载中 75% badge instead of a bare mark, while
healthy idle follows stay clean with no green status badge. A pending-release count keeps its
新资源 N 条 button that opens that follow's resource view with 只看新资源
already on, and a finished follow shows a quiet 已下载 N 个资源 count;
failed transfers never count.
Paused and stopped transfers are never running marks: resource rows and
download groups label them 已暂停 / 已停止, the follow card skips them when
deciding 下载中, and the pause-all toggle counts only transfers that are
actually running. The download filter resets its scroll on every filter
change, including filters whose list renders empty. A SearchField beside the speed summary
filters groups by anime title or release title, case/width-insensitive;
typing resets the download scroll, and a search with no matches offers its own
empty state with 清除搜索. Each anime group has a MyGo disclosure with
session-only state, so a fresh start lists every group open. A closed group
removes its download rows from keyboard navigation. Rows inside a group sort by episode
number like the resource preview — numeric ascending, releases without an
episode last, ties keeping task order — so a season reads top to bottom. A row whose episode is known
shows a 第 N 集 badge beside its transfer status.
The group summary ends with a 剩余约 N estimate when the group's active
transfers have known sizes and measured speeds: the leftover bytes of every
running transfer divided by their combined speed, reusing the row-level
estimate's wording.
Each group header also offers 打开番剧文件夹 for the anime's download directory, so a
collapsed group still reaches its files. New transfers share a fansub folder
under the anime directory; legacy transfers keep their saved hash directory.
In both layouts the header opens the parent and is hidden when no transfer
folder is known. The core adapter carries the task's shared-directory flag
unchanged into the downloader.
Resource rows match downloads by identity, not by title: the core-resolved
info hash first, then the resource ID recorded on the task, so two releases
with the same title never show each other's progress or playback. A completed row offers 打开文件夹 for that
download's folder beside 播放, disabled while a batch action is running.
A failed RSS check shows a 检查失败 badge on the poster so the failure floats
visibly; the stored error text and an inline 重试检查 action live in the
episode library header, where the action re-checks that one follow and stays
disabled while the follow is paused or the window is busy.
The following and downloads lists remember their scroll position across page
switches; searching follows or switching the download filter jumps back to the
top.
When a follow failed its last check and is not paused, the header offers
重试检查, which re-checks every failed follow that is not paused in one click. The
action follows the rule its per-follow counterpart already uses, so it is hidden
rather than offered as a no-op while every failed follow is paused. Its result
banner reports the failed count and points to each episode library, so long
titles and raw request URLs cannot expand the banner.
It also preserves the first error that could not be saved in core state,
so a transaction failure is not hidden behind an older library error.
Single-follow checks and the library header map source failures, including EOF,
to Chinese guidance; actionable local download errors remain visible, including
paths containing network-related words. The raw
last-check error stays in core state for diagnosis. A finished single-follow
check clears its checking notice even when it fails.
The 我的追番 navigation entry counts failed checks while any follow is failing
and falls back to the follow count when all are healthy. The 下载 navigation entry does the same for failed downloads, counting
entries whose last transfer failed and falling back to the item count when
none did.
The whole tile is a button that opens the follow's episode library, so the
poster is the large target and the tile carries no separate play shortcut:
watching goes through the library's episode list.

The episode library (`episodes` page) is a follow's watch view with a compact header
(poster, title, follow status, completed-download count, and the failure row
when the last check failed) above the episode list, so the follow is
recognizable before the rows. It lists that anime's downloads sorted by
episode number. A divider separates each row, and the episode number, transfer
state, and release title wrap beside the actions. Completed transfers show a
primary 播放 button, a folder icon with a tooltip, and a 更多 menu for file
selection and removal. Failed, paused, and stopped transfers offer recovery
in the same action area. Releases without a parsed episode number sort last,
labeled 未识别集数. The library is reached
from the follow tile and stays per-run state: it follows one subscription ID,
and rule editing or release picking stay on the resource view reached through
查看资源.

Inside the followed resource view, rows whose title matches one of the
subscription's latest pending RSS releases (the source of the card's
新资源 N 条 badge) carry a 新 badge, so the newest check's releases are
identifiable without rereading titles. The mark follows the subscription's
current pending list and disappears with it.

Following management actions — 查看资源, 修改条件, 暂停/恢复追番, 检查更新,
the 文件与记录 cascading submenu (打开番剧文件夹, 查看下载任务, 查看下载记录),
and 取消追番 — collapse into the library hero's ··· cascading menu,
so the watch view stays the primary surface. 查看资源 opens the detail page. It
locks the saved subscription's group and rule, opens as a view-only backfill
where nothing is selected while history loads and a later history result must
not auto-select matches, and returns to 我的追番.
Group switching is hidden in this mode, and 重新选版本 resets the group
selection so rechoosing the same group re-identifies its subscription instead
of creating a duplicate. The saved rule is never
modified by entry or by the manual selection. The two modes share the resource list but not the confirm action. Keeping them
distinct is what makes "I cancelled this one release" not change what future
releases are matched against.

The resource preview renders rows in episode order: numeric ascending (so 第 10 集 follows 第 2 集), releases without an episode last, and ties keep their publish order. The stored resource list keeps publish order; sorting is a render concern only.

In the initial version setup with a version rule, the preview collapses
releases to one per parsed episode. The chosen release is the explicit
revision (`01v2` over `01`) and then the newest publish time; unexplained
rows stay independent. A 本集匹配 N 个版本 · 换一个 control opens every matching
release for that episode, and choosing one replaces the episode's current
choice instead of adding a second download. Backfill and the followed
resource view keep one row per release because every download there is an
explicit manual choice.

When several releases share an episode, each row also shows a badge that is
a button: clicking it narrows the preview to that episode, clicking the
active episode's badge or the 清除 chip above the list clears the filter.
The filter is per session — opening another anime or choosing another
subtitle group resets it — and it only hides rows in the preview, never the
stored list or what a subscription matches.

Beside 只看匹配, a 只看已有任务 checkbox hides preview rows that have
no download in the library, whatever the download state — including failed
transfers. Opening another anime resets it, like the episode filter, and it
only affects the preview rows, never the stored list. In the followed resource view, a 只看新资源
checkbox beside it hides releases outside the subscription's pending set. It
stays rendered while active even if the pending set empties, so the history
remains one toggle away; leaving the followed-resource view resets it with
the other filters. When every row is hidden by these
filters, the preview offers a 没有符合筛选的资源 empty state whose 清除筛选
action resets all of them without touching the saved rule.

Subtitle-group selection supports a local name search with a result count and
one-click reset. Search is case/width insensitive, retains source order and
never changes the active group, filter draft or release selection. Results keep
their original source indices so a filtered click selects the correct group.
Known complete counts are shown as resource counts; a truncated or absent
count offers the release history without claiming a total. A new anime clears
the search. Picker cards mark a group the
anime already follows with a 正在追 badge. `group_picker_test.go` covers typing, selection and recovery.
Re-selecting the current group closes the picker without resetting the release
scroll position; selecting another group resets it for the new history.
Search remains clearable if a later source refresh leaves zero or one group.

The proxy card shows the active connection mode. Selecting 自定义 opens an
address and optional credentials editor. Save applies the draft and stores
it in the local database. 测试 probes the source origin with a separate
transport configured for the current selection. Test feedback and the
in-flight flag are per-run state.

The upload and sharing card accepts numeric limits for upload bandwidth, seeding
minutes, and share ratio. Each limit has its own unlimited toggle and keeps its
last finite value. Explicit save applies the complete transfer draft.

The Network and Proxy card explains how supplemental addresses help find people
sharing files and opens the address manager. The manager lets users delete
and add back the recommended list, and add or delete their own addresses and
list subscriptions. Individual exclusions apply across supplemental sources.
A new subscription previews its parsed addresses before it enters the draft.
Saving persists the draft and reconciles supplemental tiers on active tasks.
Closing the dialog discards the draft.

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

Browse cards show their weekday only under the all-days filter, with no
explanatory 查看详情 subtitle.

`ui_render.go` holds a fixed sample state used to render each page's layout
without a live source or database. It is how a layout change is checked in
isolation, and it is the reason `view_test.go` can assert on rendering without
network access.

## Covers

The browse page restores the last successful catalog from `catalog.json`
under the data directory only when the source origin, year, and season match.
It refreshes in the background. If the refresh fails, cached rows remain
available with a retry action. Older requests cannot replace a newer view.

The view registers detail posters first, then visible grid posters, then grid
posters up to 900 DIPs above or below the scroll viewport. MyGo
reevaluates the viewport after layout and scrolling. The scheduler deduplicates demand by original
URL and withdraws queued requests that leave the demand range or yield to a
higher priority. A started download can finish into disk cache after scrolling
away. It is canceled when no covers are wanted or when a newly visible cover
needs a full download slot and the started download is outside the demand
range. Up to six disk probes are scheduled while at most two covers decode at
once, independently of up to six cover downloads. At most one cover waits for
request admission so a newly visible cover can move ahead of a queued
prefetch. Up to five offscreen downloads use idle slots after visible requests
have been admitted, leaving a slot available for new visible demand. The UI
thread keeps finished bitmaps in the bounded memory cache even if they finish
just after scrolling away.
Saved subscriptions use the application context and can render their covers
while the catalog is unavailable.

The source limiter lets the first visible cache misses fill all six download
slots without waiting for document pacing. After the initial budget is
spent, it restores one start every 750 milliseconds. `403`, `429`, and
challenge responses pause cover downloads for at least one minute. A longer
`Retry-After` takes precedence. Warm hits continue during the pause. Visible
covers with a URL animate while loading and use a static placeholder on
failure. Revisiting the demand range or refreshing the catalog allows another
attempt. Cards without a cover URL use the bird mark.

`source.OriginalCoverURL` strips the server's crop parameters. The encoded
original is stored by normalized URL hash under `State.CoverDir` (the system
cache directory by default, or `--data/covers` when explicitly configured).
That key separates complete artwork from older cropped entries. The disk
cache indexes existing files on the first write and trims the oldest accesses
above 96 MiB. Disk reads update recency in memory and persist it at most
hourly per file.

Large non-JPEG bitmaps are scaled proportionally to a 512-pixel long edge.
JPEGs retain MyGo's EXIF orientation handling. The bitmap LRU targets 128 MiB
of estimated pixel storage and keeps the current demand resident. Grid cards
retain the decoded aspect ratio across bitmap eviction. Detail slots keep a
fixed contain frame.

## Brand marks

The bird logo shows in four places: the window and sidebar use the circular app
badge (`assets/dipper-icon.png`); the tray uses two transparent cuts; and the
cover fallback uses the transparent mark (`assets/dipper.svg`). The PNGs are
embedded from `cmd/dipper/assets/`; `main.go` sets the window icon and follows
the taskbar's appearance between the tray cuts. The Windows executable also
carries the tile icon as a resource (`dipper_windows_*.syso`, made by
`build/icon/gen.go`), which the taskbar and Explorer read before any window
sets its own. The assets
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
