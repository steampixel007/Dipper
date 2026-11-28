# Dipper design system

Native desktop software for finding anime, following a release group, and
watching downloaded episodes. It renders with Go and MyGo.

This document is the target design. The screens under `cmd/dipper` migrate to
it; when one does, [docs/subsystems/gui.md](docs/subsystems/gui.md) and the
reference renders under `docs/assets/ui` change in the same commit. Until then
the renders linked at the end are the baseline, not the target.

## Principles

1. **Quiet.** The interface follows Apple's own web style: neutral surfaces, one
   accent, large type for what matters, space for the rest. Poster artwork is
   the only source of color.
2. **Flat.** Content sits on the page. A boundary is made with space and
   alignment, not with a box.
3. **Silent.** The interface does not describe itself. Controls are named by
   their label; nothing explains them.
4. **On intent.** A secondary action appears when the user shows interest in
   its region.

## Product contract

- Browse posters, choose a release group, preview the matching resources, then
  confirm the follow. New matching releases download automatically.
- My Follows is a poster library. The whole tile opens that follow's episode
  library. A completed episode offers playback; an unfinished one shows
  progress.
- Resource selection, rule changes, pause, cancellation and folder access are
  secondary actions. A poster carries no play button.
- A preview writes nothing. Choosing existing resources and setting rules for
  future resources are separate choices.
- RSS, BT, regular expressions and source URLs are implementation details, not
  prerequisites.

## Layout and information

- **No box inside a box.** Pages carry no cards, no bordered groups and no panel
  wrapped around a group of controls. A settings section is a heading and its
  rows on the page. A download row is a row. Only a layer that floats above the
  page (dialog, popover, menu, side drawer) has a surface.
- **No extra dividers.** A hairline separates list rows only when whitespace
  alone cannot. A heading is never underlined, and a section is never framed.
- **No explanatory text.** No slogans, subtitles, secondary tags, hint strips,
  helper captions or descriptions under a control. A screen shows its title, its
  data and its actions. The only prose allowed is a terse state message with its
  action: an empty list, a failed load, a failed check or download, a
  destructive confirmation.
- **Hierarchy comes from type and space.** Weight, brightness and distance say
  what is a title, what is content and what is metadata. Do not add a label or a
  sentence to say it again.
- Titles wrap. Group names, file paths and resource titles wrap or scroll in
  their region; no line limit may hide the part of a release name that tells it
  apart from another.

## Color

**90% of any screen is black, white and gray; at most 10% is the single accent.**
The accent marks the core action (a primary button, the selected item, focus)
and key live data (progress, speed). It is never used for decoration.

No gradients, no saturated or neon colors, no glass or translucent chrome, no
glow, no stacked or soft multi-layer shadows. Posters and rows rest without
shadow.

Values live in `cmd/dipper/appearance.go`. Text meets 4.5:1 against its actual
surface in both themes.

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `canvas` | `#FFFFFF` | `#121212` | Window and page |
| `chrome` | `#F5F5F7` | `#1C1C1E` | Navigation rail, hover fill |
| `surface` | `#FFFFFF` | `#1E1E20` | Dialogs, menus, inputs, selected segment |
| `sunken` | `#EFEFF2` | `#262629` | Segmented well, cover placeholder |
| `ink` | `#181818` | `#F5F5F5` | Titles, body, controls |
| `muted` | `#6A6A6A` | `#9A9A9A` | Metadata, inactive controls |
| `accent` | `#0071E3` | `#2997FF` | Primary action, selection, focus, live data |
| `accentHover` | `#0064CC` | `#47A7FF` | Primary hover |
| `accentPressed` | `#0055B3` | `#1482EB` | Primary press |
| `accentWash` | `#E8F2FC` | `#172A3E` | Selected navigation and filter |
| `accentText` | `#FFFFFF` | `#0B1219` | Text on `accent` |
| `warningInk` / `warningWash` | `#9A6018` / `#FBF4EA` | `#E5B567` / `#30281B` | Failed check or download, attention |
| `errorInk` / `errorWash` | `#B93232` / `#FBF0F0` | `#F28B82` / `#332020` | Blocking error, deletion |

`line`, `outline` and `hoverLine` are neutral alpha hairlines: `ink` at 8%, 16%
and 30% in light; at 8%, 18% and 32% in dark. They outline inputs and secondary
buttons, and edge floating layers. They do not decorate.

Warning and error colors appear only when something needs the user, always with
a text label. A healthy or finished state is plain `ink` or `muted` text, never
a green mark. Weight and brightness carry emphasis: titles are bold and `ink`;
body and metadata are lighter in weight and `muted`.

High contrast strengthens outlines and focus rings. The app icon is the
circular pale-blue badge carrying the ink bird; tray marks stay transparent.
Brand masters under `build/icon` and the embedded copies under
`cmd/dipper/assets` stay synchronized.

## Whitespace

Space is the main material. Padding inside components and margins between them
are 1.5 times what a conventional desktop page uses, and every value is on the
4 DIP grid. When a screen is crowded, remove content or give it another place;
do not tighten spacing.

## Typography

The font is **Geist**, registered with `ui.RegisterFont` in weights 400, 500, 600
and 700 and used for every text role. The family list continues
`Microsoft YaHei UI, PingFang SC, Noto Sans CJK SC` so Chinese text falls back to
a native face per platform. Every role scales with the MyGo text scale.

| Role | Size | Weight | Color | Use |
| --- | --- | --- | --- | --- |
| Title | 18 | 700 | `ink` | Page and anime titles |
| Section | 18 | 600 | `ink` | Section and dialog headings |
| Body | 16 | 500 emphasis, 400 content | `ink` | Poster titles, row titles, content |
| Control | 14 | 500 | `ink` | Buttons, navigation, status labels |
| Meta | 14 | 400 | `muted` | Sizes, times, group names, speeds |

Nothing is smaller than 14. Line height is 1.35 for 18 and 1.5 for 14 and 16.
Large text is never solved by shrinking it.

## Geometry

All spacing, sizes and radii are whole multiples of 4 DIP, without exception.
Only a 1 DIP outline and a 2 DIP underline or progress end cap sit below the
grid; the icon stroke below is a drawing weight, not a spacing value. Type sizes
are a separate scale.

Scale: 4, 8, 12, 16, 20, 24, 32, 40, 48, 64.

| Element | Rule |
| --- | --- |
| Page insets | 40 left and right; 32 at the minimum window width |
| Page header | 36 above the title; 32 between groups; 16 within one |
| Navigation rail | 224 wide; 24 top and bottom, 16 side insets; items 44 high, 12 × 16 padding |
| Top bar | Fixed 64 slot on `canvas`, no divider, clear of native window controls |
| Buttons and inputs | 44 high at 100% text; 12 × 20 padding; 8 between icon and label |
| Compact controls | 8 × 16 padding; inline only |
| Icon buttons | 40 × 40 hit area around the 24 × 24 icon box |
| Poster grid | 24 horizontal gap, 36 vertical gap, 3–6 columns, rows top-aligned |
| Dialog | 40 padding; 24 between groups; footer outside the scroll region |
| Menu and popover | 12 outer padding; items 8 × 12; 4 between items |
| Status pill | 4 × 8 padding; failure and live progress only |

The window starts at 1320 × 900, supports 1040 × 720, and the reference fixture
is 1320 × 860. The grid's outer edges align with the page title.

### Radius

Radius follows component size and level, from 8 to 40.

| Radius | Components |
| --- | --- |
| 8 | Inputs, inline and compact buttons, segment items, menu items, pills |
| 12 | Buttons, navigation items, posters, segmented well |
| 16 | Menus, popovers, select panels |
| 24 | Browse side drawer, confirmation dialogs |
| 32 | Standard large dialogs |
| 40 | The largest sheets |

### Icons

Icons come from Reicon, <https://reicon.dev/docs>, in the outline style. The
glyph is 18 × 18 DIP with a 1.5 DIP stroke (`stroke-width="1.5"`, round caps and
joins, `stroke="currentColor"`, no fill), centered in a transparent 24 × 24 DIP
box. The box, not the glyph, is what layout aligns and spaces. The bird brand
mark is not an icon and is not replaced.

## Components

**Navigation.** A flat `chrome` rail. The selected item is `accentWash` with
`accent` text and a 4 DIP dot, never a colored stripe. A count shares the label
row at 100% text and moves below it when text is enlarged. The bottom holds
Settings and the guide.

**Buttons, inputs and tabs.**

| Kind | Appearance |
| --- | --- |
| Primary | `accent` fill, `accentText` label, weight 500 |
| Secondary | `surface`, 1 DIP `outline`, `ink` label |
| Quiet | No fill; `chrome` on hover |
| Input | `surface`, 1 DIP `outline`, `accent` focus ring |
| Segmented | `sunken` well, 4 padding, `surface` selected segment |
| Text tab | `muted`; `ink` with a 2 DIP `accent` underline when selected |

A page has one primary button. Disabled and busy controls stay recognizable and
cannot start work twice. Focus stays visible without hover.

**Posters.** The whole tile, title included, is the target. The poster is shown
whole at the column width and its original image ratio; while loading or
missing, 1:1.36. A tile carries the title and, only when it needs the user, a
failure or live-progress pill. Metadata is `muted` Meta text. 查看详情 appears on
hover or focus. Wrapped titles may lengthen a row but never move neighboring
posters.

**Lists.** Rows are separated by 16 DIP above and below their content, with a
`chrome` fill on hover. A hairline is allowed between episode rows. A group
(a setting section, a download series) has a Section heading and 40 DIP above
it; it has no frame.

**Dialogs and menus.** A dialog is the one place a surface appears over the
page, with a scrim. Its action footer is outside the scroll region; Escape and
cancel keep their meaning. Reach every action at 1040 × 720 and 200% text.

## Interaction

### Controls on intent

Secondary actions are hidden at rest on posters, episode rows, resource rows,
download rows and group headings. They show when the pointer is over the
region, keyboard focus is inside it, scrolling brings it toward the viewport
edge, or the user starts working in it (selecting, typing, dragging).

- Showing a control never moves layout or scroll position. Its space is
  reserved, and only opacity changes, over 130 ms.
- Keyboard focus reveals the same controls, and hidden controls stay reachable
  by Tab.
- Actions that fix a failure (retry) and the page's primary action are never
  hidden.

### Cascading menus

A desktop menu whose item opens a submenu implements all three of the following
together, for parent menu A and submenu B:

1. **Open delay.** B opens after the pointer rests on its item for 120 ms. A
   click or Right Arrow opens it at once.
2. **Close delay.** B closes 300 ms after the pointer leaves both A's item and
   B. Entering either cancels the close. Escape, Left Arrow and a click outside
   close it at once.
3. **Safe triangle.** While B is open, take the current pointer position and the
   top and bottom edge points of B's side nearest A (extended by 8 DIP). While
   the pointer is inside that triangle and moving toward B, B stays open and the
   other items it crosses on the way do not take over. Leaving the triangle, or
   stopping inside A for longer than the open delay, lets the item under the
   pointer take over.

Up and Down move within a menu, Right enters a submenu and Left or Escape
leaves it. Reduced motion removes the transitions and keeps the timers.

### Motion

Motion explains an interaction and stops. Hover, color and reveal take 130 ms;
a popover 140 ms with at most 4 DIP of travel; a page change 160 ms as a fade.
No spring, overshoot, poster zoom or decorative loop. `State.ReduceEffects` and
the system reduce-motion preference remove motion and keep focus, contrast,
reveals and menu timers.

### Behavior that stays

- Keyboard navigation, visible focus, tooltips and accessible labels.
- Selection, undo and redo, preview isolation, stale-result cancellation and
  persistence.
- No click moves a scroll position, and a state update never replaces a keyed
  scroll container.
- At 200% text, controls wrap and long content scrolls; every primary action is
  reachable at the minimum window size in both themes.

## Pages

Every page obeys the rules above. What is specific to each:

1. **Browse.** The season title, then the year, season, and search controls on
   one wrapping row, then weekday tabs, then the poster grid. No hint strip. The
   synopsis opens beside the grid, or as a dialog at narrow widths, with the
   cover, title, update day and follow state; its body scrolls on its own and
   back restores its reading position.
2. **Choose and follow.** The anime title, the release-group choice, title-word
   filters (只要 includes, 不要 excludes), the resource preview, and one confirm
   action. Matching and selected counts stay distinct. Undo and redo keep their
   behavior.
3. **My Follows.** The title, search and add, then the poster grid. Failed checks
   sort first and keep their retry. The main click opens the episode library.
4. **Episode library.** The anime title with its group and status, a management
   menu, and the episode list in episode order, unknown numbers last. A finished
   episode shows 播放; an unfinished one shows its progress; a failed one shows
   its retry.
5. **Downloads.** Series headings, then rows of filename, progress and speed.
   Failure shows its reason and retry. Stop, remove-task and delete-files stay
   distinct actions. Row actions appear on intent.
6. **Settings.** Section headings with their controls beneath on the page. Saved
   light, dark and system preferences apply at once. No section is framed.

Close-window behavior keeps the existing ask, background or quit preference.
Platform-specific close and activation behavior is not a visual decision.

## Verification

The real rendered picture decides. A computed size, padding or color is not
evidence; an image is.

After each UI change, render with `--render` and look at the actual images, in
both themes, at 1320 × 860, at 1040 × 720 and at 200% text. Check on the image:

- alignment of every edge to the title and grid;
- line breaks, including long titles, group names and file paths;
- whitespace, including that no gap is tighter than the spec;
- clipping and occlusion by the top bar, footers, popovers and dialogs.

Then:

- drive `ui.Tester` through navigation, playback entry, intent reveals and the
  cascading menu (including a pointer path inside and outside the safe
  triangle), and check text contrast;
- prove each behavioral regression with `cmd/redcheck` against the previous
  implementation;
- run `scripts/build.ps1 -Test`, `gofmt -l cmd internal`, and the Linux and macOS
  cross-builds;
- refresh the images under `docs/assets/ui` for each screen the change touches.

Fixture renders prove layout and appearance. They do not prove live source
availability, real downloads, native Windows DPI, screen-reader behavior, or
runtime behavior on macOS and Linux; a report says so.

Baseline renders from the design this document replaces:
[Browse, light](docs/assets/ui/01-browse.png),
[Browse, dark](docs/assets/ui/01-browse-dark.png),
[My Follows](docs/assets/ui/03-following.png),
[Episode library](docs/assets/ui/13-episode-library-dark.png),
[Downloads](docs/assets/ui/04-downloads-dark.png) and
[Settings](docs/assets/ui/05-settings.png).
