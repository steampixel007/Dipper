# Dipper design system

This is the design system for Dipper's desktop interface. Use it when changing
pages, controls, visual tokens, copy, or interaction states. The implementation
is the native MyGo UI in `cmd/dipper/view.go` and `appearance.go`; keep this
specification and those files in sync. [Architecture](docs/architecture.md)
explains the UI stack, [README](README.md) describes the current user workflow,
and the [GUI subsystem](docs/subsystems/gui.md) records implementation rules.
The [core subsystem](docs/subsystems/core.md) defines matching, persistence,
and download decisions; this page specifies how people see and control them.

## Name and brand

Dipper is the white-throated dipper (河乌), the songbird that dives into rushing
mountain torrents to feed. The name is the product: the app dives into torrents
of the BitTorrent kind and comes back with anime.

The logo is the bird itself: a chubby dipper crouched over rushing water —
white throat-and-breast bib flush against the front edge, tail cocked, eye
with a pupil, bill tucked deep into the head. The species rules hold in every
drawing: the bib stays clearly visible (it is how the bird is recognized), and
the water reads as a rushing torrent with uneven strokes, never a calm line.
The palette is tokens only: ink bird, paper bib, `blue` water, `blueWash` tile
behind the app icon.

Masters live in `build/icon/`: `dipper.svg` (symbol), `dipper-small.svg`
(tray sizes), `dipper-tile.svg` (app icon), `lockup-horizontal.svg` (wordmark
lockup; the wordmark stays Segoe UI Semibold, the shipped brand type). The
generator `build/icon/gen.go` embeds the same geometry with zero dependencies
and regenerates every platform asset — tray cuts light and dark, the tile
icon, and `dipper.ico`:

```sh
go run build/icon/gen.go all build/icon/dist
```

The app consumes the mark: the window icon and the tray come from the PNG
copies in `cmd/dipper/assets/` (`dipper-icon.png`, `dipper-tray.png`,
`dipper-tray-dark.png`), and the sidebar chip and the cover fallback render
`assets/dipper.svg` (a copy of `dipper-small.svg`). The sidebar chip is a
40 DIP `blueWash` tile carrying the bird at 32 DIP, so the mark reads at
least as large as the 18 DIP nav icons beside the 22 DIP wordmark. The tray
follows the taskbar's appearance: the ink cut on a light bar, the dark cut
on a dark one.
Refresh the copies with the same generator:

```sh
go run build/icon/gen.go app 256 cmd/dipper/assets/dipper-icon.png
go run build/icon/gen.go tray 32 cmd/dipper/assets/dipper-tray.png
go run build/icon/gen.go tray -dark 32 cmd/dipper/assets/dipper-tray-dark.png
go run build/icon/gen.go syso amd64 cmd/dipper/dipper_windows_amd64.syso
go run build/icon/gen.go syso arm64 cmd/dipper/dipper_windows_arm64.syso
```

The `.syso` files embed the tile icon in the Windows executable's resources,
which the taskbar and Explorer read before any window exists.

## Design principles

Anime details keep a short synopsis above the subtitle groups. Full text opens
in a reading dialog so the release preview and its selection keep their place.
Group choices include a local name search, result counts and known release
counts, retaining the source order. Names wrap to two lines within each row.

- **Browse before configuring.** Open on the anime catalog. Finding an anime,
  choosing a fansub group, and reading real release titles precede any rule.
  First use must not demand an RSS URL, BT configuration, or regular expressions.
- **Show the choice beside its effect.** Keep editable conditions and the release
  preview visible together. A change updates counts, reasons, and the current
  selection immediately. The original release title remains readable.
- **Make intent explicit.** The directions are 只要 (Only), 不要 (Don't), and 不限
  (Any). Text and symbols carry the meaning; color reinforces it. Opening a
  choice panel alone never applies a rule.
- **Make experimentation recoverable.** A draft supports repeated undo and redo.
  Saving an edit gives inline feedback and a session-only undo for that save.
  Reserve confirmation for starting a new follow with its selected downloads.
- **Explain exceptions in place.** A failed source is a failure with retry, not
  an empty catalog. Zero matches, repeated episode releases, unknown labels,
  and failed saves need specific text near the affected content.

## Tokens

Color token names below are fields of `colorPalette` in `appearance.go`; sizes
are DIP. `colors(c)` chooses the palette from the current UI context, so separate
windows and offline renders do not share mutable appearance state. Every UI color
comes from this token layer. Keep the values and this block in sync; do not add
page-specific colors.

```yaml
colors:
  # Accent: primary actions, selected navigation, focus, include conditions
  blue: "#4661e8"
  blueHover: "#3a52d0"
  bluePressed: "#2f44b4"
  blueWash: "#eef1ff"          # selected states, include chips, info panels
  # Exclude direction: always with a minus sign and wording
  avoidInk: "#6b6257"
  avoidWash: "#f4f1ec"
  # Status inks and washes
  green: "#287958"             # successful matches, seeding, saved notice
  greenWash: "#e8f6f0"
  amber: "#9c5a0a"             # states needing explanation, duplicates
  amberWash: "#fdf2e3"
  errorInk: "#c33a34"          # save-failure banner
  errorWash: "#fdeeec"
  # Text
  ink: "#171a22"               # main content
  muted: "rgba(35,40,56,.62)"  # supporting information
  # Surfaces
  chrome: "#fcfcff"            # hover surface over the canvas
  paper: "#ffffff"             # cards, popovers, raised controls
  canvas: "#f7f8fc"            # window canvas
  glass: "rgba(255,255,255,.58)"  # selective frosted panes
  sunken: "rgba(23,26,34,.045)"   # inset fields and control wells
  glowBlue: "rgba(122,156,255,.40)"   # ambient canvas light
  glowViolet: "rgba(196,168,255,.32)" # ambient canvas light
  # Strokes
  line: "rgba(23,26,34,.07)"   # dividers, hairline card borders
  outline: "rgba(23,26,34,.16)" # input and secondary-button borders
  hoverLine: "rgba(23,26,34,.30)" # hovered card border
  accentText: "#ffffff"        # text and checked marks on the accent
  brandWash: "#eef2ff"         # fixed black-and-white bird artwork
  # Alpha overlays
  scrim: "rgba(16,18,26,.32)"  # dialog backdrop
  popoverShadow: "rgba(20,24,40,.12)" # floating popover
  cardShadow: "rgba(20,24,40,.10)"    # poster cards and raised chips

night-colors:
  blue: "#9db4ff"
  blueHover: "#b3c6ff"
  bluePressed: "#8098e8"
  blueWash: "#1d2540"
  accentText: "#101526"
  muted: "rgba(232,236,248,.78)"
  ink: "#f2f4fa"
  line: "rgba(255,255,255,.08)"
  outline: "rgba(255,255,255,.18)"
  hoverLine: "rgba(255,255,255,.32)"
  chrome: "rgba(19,22,30,.72)"
  paper: "#161a24"
  canvas: "#0f1117"
  glass: "rgba(10,13,20,.42)"
  sunken: "rgba(255,255,255,.04)"
  glowBlue: "rgba(78,107,255,.22)"
  glowViolet: "rgba(150,110,255,.16)"
  brandWash: "#edf2ff"
  green: "#6fd3ab"
  greenWash: "#143229"
  amber: "#f0bf7a"
  amberWash: "#3a2c17"
  avoidInk: "#b9b3a7"
  avoidWash: "#242219"
  errorInk: "#ff9d95"
  errorWash: "#3a2020"
  scrim: "rgba(0,0,0,.60)"
  popoverShadow: "rgba(0,0,0,.45)"
  cardShadow: "rgba(0,0,0,.45)"

typography:
  family: "Segoe UI, Microsoft YaHei UI"
  page-headline: { size: 28, weight: 700 }  # following / downloads / settings openings
  browse-heading: { size: 26, weight: 700 } # year-and-season heading
  detail-title: { size: 23, weight: 700 }   # anime title, dialog titles
  brand: { size: 22, weight: 600 }          # sidebar wordmark
  stat: { size: 20, weight: 600 }           # aggregate speeds
  section-heading: { size: 18, weight: 600 }# heading() helper
  card-title: { size: 14, weight: 600 }     # poster-grid titles: browse and my follows
  row-title: { size: 15, weight: 600 }      # download rows, guide steps
  body: { size: 14 }                        # default; weight 600 when emphasized
  control: { size: 13 }                     # compact controls and labels
  caption: { size: 12, lineHeight: 1.5 }    # caption() helper; release titles use 1.6
  badge: { size: 11, weight: 600 }          # badge() helper, match reasons

rounded:
  marker: 2        # sidebar active bar
  dot: 3           # status dot, progress fill
  badge: 4         # badges
  chip: 5          # word chips
  field: 6         # inputs, inline notes; theme default
  card: 8          # cards, covers, nav items
  panel: 10        # settings cards, word popover
  dialog: 12       # dialog panels
  icon-button: 16  # 32×32 icon buttons (circle)
  button: 20       # all text buttons

spacing:
  base: 4
  common-gaps: [4, 6, 8, 12, 14, 16, 18, 24]

motion:
  ease: "cubic-bezier(0, 0, .4, 1)"
  page-ease: "cubic-bezier(.22, 1, .36, 1)"
  feedback: 220ms spring, bounce .18  # press scale .97, hover color
  popover: 160ms   # popovers and dialog fades
  content: 320ms spring, bounce .22  # pages, rows, poster lift
  page: 240ms      # enter +10 DIP, exit -6 DIP
  progress: 400ms  # progress-fill value changes
```

### Appearance

The appearance card offers 浅色, 夜间, and 跟随系统. Light remains the default
for existing preferences and missing or unknown values. A choice takes effect
immediately after a successful preferences write and survives restart. A write
failure keeps the previous selection and reports the error. Follow system tracks
the desktop application appearance. The native frame and clear background follow
the same choice; the tray follows its taskbar independently.

Night mode keeps the same typography, spacing, content, and hierarchy, using blue
slate surfaces and lighter semantic inks. `accentText` is distinct from `paper`:
a button's text and the followed badge need contrast on the accent, not a surface
color. The fixed black-and-white bird stays on `brandWash` in either appearance,
including a small tile inside the no-cover frame in night mode.

### Color roles

Treat colors as semantic roles; reuse the variables rather than picking values.

| Role | Token(s) | Use |
| --- | --- | --- |
| Window canvas | `canvas` | The window floor behind every page. |
| Selective glass | `glass` | Sidebar, browse control band, beginner hint. |
| Top bar | `canvas` + `glass.ScrollEdge` | Fixed 64 DIP bar; no divider, no brighter slab. |
| Primary surface | `paper` | Cards, popovers, raised controls. |
| Recessed surface | `sunken` | Inputs, segmented-control wells, inset fields. |
| Hover surface | `chrome` | Control and navigation hover fill. |
| Ambient light | `glowBlue` / `glowViolet` | Decorative radial gradients only; never a data cue. |
| Divider / border | `line` / `outline` / `hoverLine` | Section separation / control borders / hovered card border. |
| Text / muted text | `ink` / `muted` | Main content / supporting information. |
| Accent | `blue` (+ hover, pressed, wash) | Primary actions, selected navigation, include conditions. |
| Exclude | `avoidInk` / `avoidWash` | Exclude conditions, always with a minus sign and text. |
| Success | `green` / `greenWash` | Successful matches and status. |
| Caution | `amber` / `amberWash` | States needing explanation, duplicate releases. |
| Failure | `errorInk` / `errorWash` | Save-failure banner. |
| Overlay | `scrim` / `popoverShadow` | Dialog backdrop / floating popover. |
| Elevation | `cardShadow` | Poster cards and the selected segment chip. |

A status is never conveyed by color alone. The selected sidebar entry also has
text weight and an accent marker; word chips use `＋` or `−` plus wording.

### Typography

The Windows font stack is Segoe UI, Microsoft YaHei UI. Size and weight
establish the hierarchy: 28/26/23 DIP titles for pages and detail, 18 DIP
section headings, 14 DIP body, 12 DIP supporting text at 1.5 line height, and
11 DIP badges. Compact controls and labels step down to 13 DIP. Keep long
original titles selectable and accessible even if a surrounding card truncates
its own label; release titles render at 12 DIP with 1.6 line height.

### Layout and spacing

Spacing uses a 4 DIP base with gaps stepping by 2 (4, 6, 8 … 24); page padding
is 26–28 DIP and card padding 10–22 DIP. Compact 12–13 DIP text may pair with
odd paddings (5, 7, 9) for optical alignment within a row.

The initial window is 1320 × 900 DIP, the minimum is 1040 × 720 DIP; the top
bar is 64 DIP and sidebar 204 DIP. Detail conditions occupy a 306 DIP column
beside the release preview, with separate scrolling. The primary action stays
visible below both. The poster grid adapts from three to six columns —
`(width − 260) / 180` clamped to that range — with 18 DIP horizontal and 24 DIP
vertical gaps; posters keep a 1 : 1.36 ratio and card titles wrap to at most
two lines.

### Depth and elevation

Depth comes from the surface ladder, borders, and restrained shadows.

| Layer | Treatment | Examples |
| --- | --- | --- |
| Recessed | `sunken` on the canvas or a glass band | Inputs, segmented-control wells. |
| Base | `canvas` with ambient light | Page background. |
| Glass | `glass` + `glass.Glass` material | Sidebar, browse control band. |
| Top edge | `canvas` + hard `glass.ScrollEdge` | The 64 DIP top bar. |
| Raised | `paper` + 1px `line`/`outline` border | Cards, popovers, group cards. |
| Poster | cover art + two `cardShadow` layers | `0, 2, 8, -2` for contact and `0, 16, 36, -18` for lift; hover deepens the second layer. |
| Selected chip | `cardShadow` | `0, 2, 6, -2`. |
| Overlay | `paper` + border, over `scrim` | Dialogs — no panel shadow. |
| Popover | `paper` + border + `popoverShadow` | Word-choice popover: `Shadow(0, 6, 20, 0, popoverShadow)`. |

Only three things cast: the word-choice popover, poster art in the catalog and
follow grids, and the selected segment chip. Dialogs rely on the scrim alone.
The only gradients are the two ambient radial glows behind the canvas; no
component paints a decorative gradient. A poster's hover lift is motion on top
of that resting shadow, not a second elevation.

### Motion

Press and cancel use a 220 ms spring with 0.18 bounce; hover colors use a 130 ms
color transition; page and content changes use a 320 ms spring with 0.22 bounce.
Local popovers and dialog fades use the 160 ms `cubic-bezier(0, 0, .4, 1)`
curve, and page transitions use the 240 ms `cubic-bezier(.22, 1, .36, 1)` curve
with a 10 DIP enter and 6 DIP exit. The progress fill animates its value over
400 ms. Buttons press to 0.97 scale; posters hover 5 DIP up at 1.01 scale.
Motion explains a change without delaying the next action. When the in-app
reduce-effects option or system reduce-motion preference is active, the UI
disables animations; static depth, borders, and contrast remain. MyGo's glass
falls back to an opaque surface when the system asks for reduced transparency.
These values are defined in `appearance.go` and `view.go`.

### Icons and imagery

Line icons are supplementary to labels and tooltips. Posters lead the browsing
layout; each card pairs its image with a title, weekday, and version entry.
Loaded covers fill their frame — scaled uniformly, centered, and cropped to
the frame — so no filler shows beside a poster. The fallback
cover keeps the same dimensions and shape as a loaded image.
Followed anime have a visible text marker. The poster grid and title hierarchy
are illustrated in the [browse screen](docs/assets/ui/01-browse.png).

## Navigation and page patterns

The persistent sidebar contains 浏览番剧 (Browse anime), 我的追番 (My follows), and
下载 (Downloads); Settings and the follow guide sit at its bottom. Anime detail
is part of browsing, not another top-level destination. Returning from detail
preserves catalog filters and scroll position.

| Surface | Layout and action hierarchy |
| --- | --- |
| Browse | Year and season heading, search, season and weekday controls, then poster grid. A short hint can be dismissed; the guide opens on request. |
| Anime detail | Fansub group and editable conditions on the left; quick words, match count, reasons and release rows on the right; current-run summary and primary action below. |
| My follows | Follow search with a matching count, then a poster grid: cover, title, subtitle group, and an 进入剧集库 hint. Status stays minimal — a 检查失败/已暂停/下载中 badge on the poster, a 新资源 N 条 button, and a completed-download count. The whole tile opens the episode library; pause, edit, check-now, and folder actions live in the library's ··· menu. |
| Downloads | Separate aggregate upload/download speeds; per-task progress and speed, with pause/resume/retry or seeding actions appropriate to state. |
| Settings | Appearance, source line, download location, shared upload limit, seeding-duration target, and window/background behavior. |

Each download offers one transfer action for its current state: 暂停 while
downloading, 继续 while paused or stopped before completion, 重试 on failure,
and 停止做种 after completion while seeding. A completed, stopped task has no
transfer action. 暂停 preserves progress for a later 继续; there is no separate
停止下载 action. 删除 and 打开文件夹 remain separate task and file actions.

Seeding stops when the transfer reaches the ratio target or the chosen
seeding-duration target, whichever comes first. The transfer settings offer
30 minutes, one hour, two hours, and seeding without a time limit (ratio-only).

A page must provide distinct loading, empty, failure, and content states. Source
failure offers retry without clearing saved follows or downloads. Empty search
offers a way to clear filters. Failure to save must never look like success.

## Components

Pages are built from the helpers in `view.go`; extend a helper instead of
restyling a page separately.

| Helper | Shape | Notes |
| --- | --- | --- |
| `primary` | Radius 20, padding 9/20, weight 600 | One per view (start, confirm); disabled keeps its label. |
| `secondary` | Radius 20, padding 7/16, 1px `outline` border | Cancels, retries, secondary actions. |
| `quiet` | Radius 20, padding 7/12; hover `chrome` | Inline actions drop to 12 DIP; may take `blue` or `muted` text. |
| `iconButton` | 32 × 32, radius 16 (circle), hover `line`; 18 DIP icon | Label doubles as tooltip. |
| `heading` | 18 DIP, weight 600 | Section titles. |
| `caption` | 12 DIP `muted`, line-height 1.5 | Supporting text. |
| `badge` | 11 DIP weight 600, padding 3/7, radius 4 | Ink + wash pair: `blue/blueWash`, `green/greenWash`, `amber/amberWash`, `muted/canvas`. |
| `cover` | Radius 8 | The image fills the frame, centered and cropped without stretching. Same `blueWash` box for every non-image state: when a cover URL exists but its bitmap is unavailable, a blue `Blocks` indicator (`mygo/plugins/loading`), including after a failed load; without a cover URL, the bird mark at 0.62 of the short side. |

Word chips are a `quiet` variant: radius 5, padding 5/8, 1px `line` border,
12 DIP. Direction gives the pair — include is `blue` on `blueWash` with `＋`,
exclude is `avoidInk` on `avoidWash` with `−`, unrestricted is `ink` on `paper`.

Sidebar nav items are 44 DIP tall, radius 8; the active item lifts to `paper`
with weight 600 and a 3 × 18 DIP `blue` marker (radius 2); hover is `line`.
Cards are `paper` with a 1px `line` border — radius 10 for settings cards,
8 for download and group cards; catalog and follow posters use a 14 DIP cover
radius, with cover, title, and metadata sitting directly on the canvas. A
selected group card switches its border to `blue` and its fill to `blueWash`.
Both poster grids carry `cardShadow`. Dialogs are 500–510 DIP wide,
radius 12, padding 28, over `scrim`. The word-choice popover is 320 DIP wide,
radius 10, over `popoverShadow`.

Every interactive control goes through `tactile`: pressed scales to 0.97 with
the 220 ms feedback spring, and hover colors use the same feedback motion.
Disabled states keep the label and drop the action; they never hide what is
disabled.

## Controls and interaction patterns

### Rule chips and word choice

A candidate word is a chip, not an immediate filter. Clicking it opens a
popover next to the word with 只要含这个词的 (Only with this word) and
不要含这个词的 (Without this word). Show the remaining release count for **each**
choice before applying it. The current restriction can be removed with
不再限制这个词 (No longer restrict this word). Clicking outside or pressing Esc
closes the popover without changing the draft.

Include uses blue and `＋`; exclude uses red and `−`. A word cannot be required
and excluded at once. Custom text needs an explicit direction; alternate
spellings form one any-of group instead of a silent synonym rule. Parsing
resolution, codec, source, container, audio, subtitle and version labels into
recognized words helps reading and suggesting words but never adds a
restriction without the user's choice. Recognized labels render as quiet tags
on each release row, and the quick-filter row lists words by how often they
appear across the current releases. The
[filter screen](docs/assets/ui/02-filter.png) illustrates the two-column
arrangement.

### Preview and release selection

Keep unmatched releases visible with reasons by default; allow 只看匹配 (Only
matching). Show original titles, recognized episode information, size, and
match reason. If an episode has several releases, display the count and allow
people to choose each release separately. Unknown episode or subtitle labels
remain unknown; the original title still works for textual filtering.

Title rules decide which *future* releases match. Checkboxes decide which
*existing* matching releases to submit now. Existing matches start checked;
unchecking one changes no rule. Choosing 按此资源筛选 (Use this release as a
filter) switches the left column to candidate words from that title while the
right-hand preview stays usable. Editing saved rules affects future unread
items; existing releases require an explicit backfill selection.

Starting a new follow summarizes the fansub group, rules, selected release
count, estimated size, and folder before confirmation. Cancelling keeps the
draft. Editing an existing follow saves directly with inline feedback and
never silently resubmits earlier releases.

### Feedback, undo, and errors

Show the match count and reason near the preview as a rule changes. Zero
matches is an adjustable state with a suggestion to remove or check a word;
do not block the user with a dialog. Draft changes support repeated undo/redo;
a new change after undo ends the previous redo branch. On Windows Ctrl+Z
undoes, and Ctrl+Y or Ctrl+Shift+Z redoes. Text-input undo takes precedence
while the input is active. Switching groups starts fresh draft history.

An undo for a saved edit restores only the prior group and rules; it does not
reverse later pause changes, checks, or downloads. It is available in the
current session, not as a persistent version history. Pausing a follow and
pausing a download are separate actions and must have distinct labels.

| Situation | Visible response |
| --- | --- |
| Source unavailable | Name the failure and offer retry; do not show an empty catalog. |
| No search results or no matches | Explain the current scope and offer a way to adjust filters. |
| Several versions of an episode | Show the count; keep separate releases. |
| Download failure | Show the reason and retry action without suggesting progress was lost. |
| State write failure | Show the save error; do not show a success notice. |

### Window close and background work

On Windows and Linux, the first close asks `关闭 Dipper？`, explaining that
`收起到托盘` keeps following and downloading, while `退出 Dipper` stops
background work. Include `取消` and an initially unchecked `记住我的选择`.
Cancel, Escape, and clicking outside dismiss without taking an action or
saving a preference. Remembering is optional; settings always allows
`每次询问`, `收起到托盘`, or `退出应用` to replace the choice. A separate
`退出 Dipper` button is an action, not a preference.
The [close-window preview](docs/assets/ui/08-close-window.png) shows the dialog
with deterministic local sample state.

If the tray is unavailable, keep the window open, explain why, and disable
the hide action while retaining cancel and explicit quit. Check availability
again immediately before hiding. On Windows explain the hidden-icons overflow
and allow a single click on the tray icon to restore the window. Never change
the user's Windows taskbar visibility preference.

On macOS, closing the window keeps the application running, Dock activation
reopens it, and the application menu's Quit or Command-Q stops it. Explain this
native behavior in settings rather than presenting the Windows close choices.
The menu bar icon is supplementary to the Dock.

Preserve the window and report a failed save before hiding or quitting.

## Accessibility

Every action needs a text label or accessibility name; icon-only actions also
have tooltips. Keep keyboard focus visible and preserve native control
behavior. Esc closes the local choice before leaving the current workflow.
Maintain access to the primary action at the minimum window size; do not move
a page's scroll position as a side effect of clicking a control.

## Do's and Don'ts

### Do

- Take every color from the token variables and start every control from a
  helper (`primary`, `secondary`, `quiet`, `iconButton`, `heading`, `caption`,
  `badge`, `cover`).
- Pair color with wording and symbols: `＋`/`−` on chips, text weight plus the
  marker on selected navigation.
- Keep original release titles readable and selectable.
- Give every page distinct loading, empty, failure, and content states, with
  the failure named and retryable.
- Put counts and reasons next to the content they describe, and show the
  remaining count for each choice before it is applied.
- Keep user-visible strings Chinese; they are product copy.
- Let Esc close the innermost layer first, and keep the primary action
  reachable at 1040 × 720.

### Don't

- Don't add page-specific palettes or raw hex values outside the token layer.
- Don't add gradients beyond the two ambient canvas glows.
- Don't add shadows beyond `popoverShadow` on the word popover and
  `cardShadow` on catalog/follow posters and the selected segment chip.
- Don't mark state with a colored left-border stripe; use a dot or a swatch.
- Don't render a source failure as an empty catalog, or a failed save as
  success.
- Don't open a dialog for zero matches; keep it an inline adjustable state.
- Don't turn opening a panel into applying a rule; a restriction needs an
  explicit choice, and a word cannot be both required and excluded.
- Don't translate Chinese product copy or literals read from the source site.
- Don't move a page's scroll position as a side effect of a click, and don't
  restyle one page around the shared helpers.

## Reference screens and maintenance

The eight [UI reference states](docs/assets/ui/) cover browse, filtering,
following, downloads, settings, source failure, backfill, and window closing.
They come from `cmd/dipper/ui_render.go` fixtures; `--render ./docs/assets/ui`
regenerates both light screens and matching `-dark.png` screens. These images show the native view layout with sample data, not a live
source, running transfer, or operating-system window. When a design change
alters a reference state, update the render and review the image along with
the code. [README verification boundaries](README.md#verification-and-boundaries)
describes the remaining real-platform checks.

The visual direction draws on [Adobe Spectrum color tokens](https://opensource.adobe.com/spectrum-design-data/tokens/color-palette/)
and [motion guidance](https://spectrum.adobe.com/page/motion/) and on the
[Apple HIG interaction guidance](https://developer.apple.com/design/human-interface-guidelines/).
These are reference principles; the shipped interface is a native Go theme,
not Adobe components or Apple platform components. Where this system has a
gap — dense data lists such as the download page, or Chinese typography
guidance — supplement it with the relevant Ant Design chapter rather than
switching systems.
