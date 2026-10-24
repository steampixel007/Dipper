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
copies in `cmd/dipper/assets/` (`dipper-icon.png`, `dipper-tray.png`), and the
sidebar chip and the cover fallback render `assets/dipper.svg` (a copy of
`dipper-small.svg`). Refresh the copies with the same generator:

```sh
go run build/icon/gen.go app 256 cmd/dipper/assets/dipper-icon.png
go run build/icon/gen.go tray 32 cmd/dipper/assets/dipper-tray.png
```

## Design principles

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

Color token names below are the Go variables at the top of `view.go`; sizes are
DIP. Every color the UI uses is one of those variables — a grep for `ui.Hex(`
or `ui.RGBA(` outside that block means a color bypassed the token layer. When a
value changes, change the variable and this block together. Never introduce a
second palette.

```yaml
colors:
  # Accent: primary actions, selected navigation, focus, include conditions
  blue: "#3b63fb"
  blueHover: "#274dea"
  bluePressed: "#1d3ecf"
  blueWash: "#f5f9ff"          # selected states, include chips, info panels
  # Exclude direction: always with a minus sign and wording
  avoidInk: "#b1262b"
  avoidWash: "#fff5f5"
  # Status inks and washes
  green: "#12805c"             # successful matches, seeding, saved notice
  greenWash: "#e9f7ef"
  amber: "#9c5700"             # states needing explanation, duplicates
  amberWash: "#fff4db"
  errorInk: "#c9252d"          # save-failure banner
  errorWash: "#fff0f0"
  # Text
  ink: "#292929"               # main content
  muted: "#505050"             # supporting information
  # Surfaces
  chrome: "#f3f3f3"            # title bar, sidebar, hover surface
  paper: "#ffffff"             # main reading area, cards, popovers
  canvas: "#f8f8f8"            # recessed hints and low-emphasis sections
  # Strokes
  line: "#e9e9e9"              # dividers, hairline card borders
  outline: "#c6c6c6"           # input and secondary-button borders
  hoverLine: "#b1b1b1"         # hovered card border
  # Alpha overlays
  scrim: "rgba(0,0,0,.24)"     # dialog backdrop
  popoverShadow: "rgba(0,0,0,.10)"  # the only shadow, word-choice popover

typography:
  family: "Segoe UI, Microsoft YaHei UI"
  page-headline: { size: 28, weight: 700 }  # following / downloads / settings openings
  browse-heading: { size: 26, weight: 600 } # year-and-season heading
  detail-title: { size: 23, weight: 700 }   # anime title, dialog titles
  brand: { size: 22, weight: 600 }          # sidebar wordmark
  stat: { size: 20, weight: 600 }           # aggregate speeds
  section-heading: { size: 18, weight: 600 }# heading() helper
  card-title: { size: 17, weight: 600 }     # follow-card title
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
  panel: 10        # follow/settings cards, word popover
  dialog: 12       # dialog panels
  icon-button: 16  # 32×32 icon buttons (circle)
  button: 20       # all text buttons

spacing:
  base: 4
  common-gaps: [4, 6, 8, 12, 14, 16, 18, 24]

motion:
  ease: "cubic-bezier(0, 0, .4, 1)"
  feedback: 130ms  # press scale .97, hover color
  popover: 160ms   # popovers and dialog fades
  content: 190ms   # pages, rows, poster lift
  progress: 400ms  # progress-fill value changes
```

### Color roles

Treat colors as semantic roles; reuse the variables rather than picking values.

| Role | Token(s) | Use |
| --- | --- | --- |
| Window chrome | `chrome` | Title bar, sidebar, hover surface. |
| Primary surface | `paper` | Main reading area, cards, popovers. |
| Recessed surface | `canvas` | Hints and low-emphasis sections. |
| Divider / border | `line` / `outline` / `hoverLine` | Section separation / control borders / hovered card border. |
| Text / muted text | `ink` / `muted` | Main content / supporting information. |
| Accent | `blue` (+ hover, pressed, wash) | Primary actions, selected navigation, include conditions. |
| Exclude | `avoidInk` / `avoidWash` | Exclude conditions, always with a minus sign and text. |
| Success | `green` / `greenWash` | Successful matches and status. |
| Caution | `amber` / `amberWash` | States needing explanation, duplicate releases. |
| Failure | `errorInk` / `errorWash` | Save-failure banner. |
| Overlay | `scrim` / `popoverShadow` | Dialog backdrop / the single popover shadow. |

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

Depth comes from the surface ladder and borders, not from stacked shadows.

| Layer | Treatment | Examples |
| --- | --- | --- |
| Recessed | `canvas` on `paper` | Hint strips, confirm-summary block. |
| Base | `paper` inside the `chrome` frame | Content column, cards. |
| Raised | `paper` + 1px `line`/`outline` border | Cards, inputs, chips, group cards. |
| Overlay | `paper` + border, over `scrim` | Dialogs — no panel shadow. |
| Popover | `paper` + border + the single shadow | Word-choice popover: `Shadow(0, 6, 20, 0, popoverShadow)`. |

The word-choice popover is the only element that casts a shadow; dialogs rely
on the scrim alone. No gradients, and no additional shadows — the poster hover
lift is motion, not elevation.

### Motion

The easing curve is `cubic-bezier(0, 0, .4, 1)`. Button feedback uses 130 ms,
local popovers and dialog fades 160 ms, and page and content changes 190 ms;
the progress fill animates its value over 400 ms. Buttons press to 0.97 scale;
poster hover lifts 3 DIP and scales to 1.008. Motion explains a change without
delaying the next action. When the in-app reduce-effects option or system
reduce-motion preference is active, the UI disables animations. These values
are defined in `appearance.go` and `view.go`.

### Icons and imagery

Line icons are supplementary to labels and tooltips. Posters lead the browsing
layout; each card pairs its image with a title, weekday, and version entry.
The fallback cover keeps the same dimensions and shape as a loaded image.
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
| My follows | Saved rules, latest check result, folder, pause/resume, edit and check-now actions. |
| Downloads | Separate aggregate upload/download speeds; per-task progress and speed, with pause/resume/retry or seeding actions appropriate to state. |
| Settings | Appearance, source line, download location, shared upload limit, and window/background behavior. |

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
| `cover` | Radius 8 | Same box for every non-image state: while its cover loads, a blue `Ripple` indicator (`mygo/plugins/loading`); with no cover, the bird mark at 0.62 of the short side. |

Word chips are a `quiet` variant: radius 5, padding 5/8, 1px `line` border,
12 DIP. Direction gives the pair — include is `blue` on `blueWash` with `＋`,
exclude is `avoidInk` on `avoidWash` with `−`, unrestricted is `ink` on `paper`.

Sidebar nav items are 44 DIP tall, radius 8; the active item lifts to `paper`
with weight 600 and a 3 × 18 DIP `blue` marker (radius 2); hover is `line`.
Cards are `paper` with a 1px `line` border — radius 10 for follow and settings
cards, 8 for download, group, and poster cards; a selected group card switches
its border to `blue` and its fill to `blueWash`. Dialogs are 500–510 DIP wide,
radius 12, padding 28, over `scrim`. The word-choice popover is 320 DIP wide,
radius 10, over the single `popoverShadow`.

Every interactive control goes through `tactile`: pressed scales to 0.97 with
the 130 ms feedback motion, hover colors transition in 130 ms. Disabled states
keep the label and drop the action; they never hide what is disabled.

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
resolution, subtitle, or codec labels helps reading and suggesting words but
never adds a restriction without the user's choice. The
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
unchecking one changes no rule. Choosing 照这条选版本 (Choose version from this
release) switches the left column to candidate words from that title while the
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

- Don't introduce a second palette, raw hex values, or gradients.
- Don't add shadows; the word popover's single shadow is the only exception.
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

The seven [UI reference renders](docs/assets/ui/) cover browse, filtering,
following, downloads, settings, source failure, and backfill. They come from
`cmd/dipper/ui_render.go` fixtures; `--render ./docs/assets/ui` regenerates
them. These images show the native view layout with sample data, not a live
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
