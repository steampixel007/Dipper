# Dipper design system

Native desktop software for finding anime, following a release group, and
watching downloaded episodes.

## Direction

The artwork supplies the color. The application supplies a calm, legible frame:
neutral surfaces, a single blue accent, compact navigation, consistent controls,
and generous space between groups of information. A screen should read in this
order: its title, its main content, its next useful action.

This specification replaces the glass treatment. Do not add ambient gradients,
violet lighting, translucent navigation, oversized shadows, decorative sparkles,
or bouncing cards. Visual polish comes from alignment, typography and useful
content rather than additional effects.

The design-taste-frontend skill informs the neutral palette, equal edges, plain
copy, coherent components and restrained feedback. Dipper renders with Go/MyGo.
Its working screens are the product, so website hero sections, browser
breakpoints, marketing copy, shader scenes and HTML starter assets do not apply.
Poster titles and release metadata are essential navigation labels.

## Product contract

- Browse posters, choose a release group, preview the matching resources, then
  confirm the follow. New matching releases download automatically.
- My Follows is a poster library. The whole tile opens that follow's episode
  library. A completed episode offers playback; an unfinished one shows progress.
- Resource selection, rule changes, pause, cancellation and folder access belong
  in secondary management actions. Do not place a redundant first-episode play
  button on every poster.
- First use explains the next step in Chinese. RSS, BT, regular expressions and
  source URLs are advanced implementation details, not prerequisites.
- A preview writes nothing. Choosing existing resources and setting rules for
  future resources are separate choices.

## Foundations

All dimensions are DIP. MyGo text scaling applies to every text role. Color
values live in `cmd/dipper/appearance.go`; sizing and type roles live in
`cmd/dipper/typography.go`. Components use these roles rather than introducing
page-specific themes.

### Color

Keep roughly 90–95% of application chrome neutral, excluding poster artwork.
Blue identifies selection, focus and primary actions. Green means success or
active following, amber means attention, and red means error or destructive
action. Include a readable label with every status color.

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `canvas` | `#FAFAF9` | `#141618` | Window and page ground |
| `chrome` | `#F0F1EF` | `#1B1D20` | Opaque navigation rail |
| `paper` | `#FFFFFF` | `#202326` | Inputs, selected segments, panels and dialogs |
| `sunken` | `#EBEDEA` | `#191C1F` | Quiet segmented-control well |
| `ink` | `#222725` | `#ECEFED` | Body text and titles |
| `muted` | `#626963` | `#B4BBB5` | Secondary text |
| `blue` | `#285EA8` | `#9EC3F5` | Primary action, focus and links |
| `blueHover` | `#204F91` | `#B6D2F8` | Primary hover |
| `bluePressed` | `#194177` | `#87ADE1` | Primary press |
| `blueWash` | `#EAF1FA` | `#233247` | Selection and inclusion |
| `accentText` | `#FFFFFF` | `#162331` | Text on primary buttons |
| `green` / `greenWash` | `#287250` / `#EBF5EF` | `#8BCDA8` / `#20372A` | Following and success |
| `amber` / `amberWash` | `#8B590E` / `#FBF2E1` | `#E7BD7C` / `#3B301F` | Retry and attention |
| `avoidInk` / `avoidWash` | `#685E55` / `#F1EEEA` | `#C5BAB0` / `#322D28` | Excluded title words |
| `errorInk` / `errorWash` | `#B73731` / `#FCEDEB` | `#F5A6A0` / `#3D2727` | Failure and deletion |

`line`, `outline` and `hoverLine` are neutral separators, control outlines and
hover outlines, respectively. Use stronger outlines and persistent focus rings
under the high-contrast preference. Text must meet 4.5:1 contrast against its
actual surface in both themes.

The logo keeps its existing assets and a pale neutral `brandWash`. Do not
recolor artwork to match the accent. Missing covers use the existing logo;
loading covers use the existing loading treatment.

### Typography

Use the existing native stack, `Segoe UI, Microsoft YaHei UI`, with platform
fallbacks. Keep Chinese text legible rather than introducing a display face.

| Role | Base size | Weight | Use |
| --- | --- | --- | --- |
| Caption | 12 | 400; 600 for status | Metadata and helper copy |
| Body | 14 | 400; 600 for emphasis | Controls, list rows and poster titles |
| Section | 18 | 600 | Panel and dialog headings |
| Title | 26 | 600 | Page and anime titles |

Titles wrap. Group names, file paths and resource titles wrap or scroll within
their allotted region; no arbitrary line limit may hide the distinguishing
part of a release. Do not solve large text by shrinking it.

### Geometry

Use a 4 DIP spacing base: 4, 8, 12, 16, 20, 24, 28, 32.

| Element | Rule |
| --- | --- |
| Main content | Equal 28 DIP left and right insets |
| Page header | 24 DIP top inset; 20 DIP between major groups |
| Navigation | 204 DIP wide; 14 DIP rail insets |
| Top bar | Fixed 64 DIP slot, opaque canvas, content clipped beneath |
| Controls | 8 DIP radius; 8 × 12 DIP normal padding |
| Icon buttons | 36 × 36 DIP; meaningful accessible label and tooltip |
| Posters | 8 DIP radius; width-to-height ratio 1:1.36 |
| Grid | 18 DIP horizontal gap; 24 DIP vertical gap |
| Dialogs | 12 DIP radius; shared scroll and footer helpers |
| Status badges | 4 DIP radius; 3 × 7 DIP padding |

The window starts at 1320 × 900 and supports 1040 × 720. The reference fixture
uses 1320 × 860. The grid uses 3–6 columns according to available window width;
its outer edges align with the title and toolbar. Preserve poster aspect ratio.

Use borders for boundaries, whitespace for grouping, and small shadows only
for elevation. Resting posters have one subtle shadow. Popovers and dialogs may
use a stronger shadow; toolbar groups do not float.

## Components

### Navigation and top bar

The rail is an opaque neutral surface with one quiet divider. Brand, page links
and bottom utilities share aligned edges. Active navigation has a neutral
selected surface and a small blue dot; never use a colored left stripe.

At normal text size, a navigation label and its count share a row. Failure
counts remain visible and amber. At enlarged text sizes the metadata moves
below the label to keep the target readable. The rail scrolls when necessary.
The bottom utilities contain settings and the guide, without a decorative
local-storage note or status dot.

The top bar is a quiet location cue and window-drag region, with the guide at
the right. Its geometry must not depend on the page below. Respect native
window-control insets.

### Buttons, inputs and tabs

| Kind | Appearance | Use |
| --- | --- | --- |
| Primary | Solid blue, contrasting label | Confirm a follow or another main commit |
| Secondary | Paper surface, neutral outline | Add, open, retry or cancel |
| Quiet | Text with hover feedback | Navigation and secondary management |
| Input | Paper surface, neutral outline | Search and editable values |
| Segmented choice | Neutral well, paper selected segment | Mutually exclusive seasons |
| Text tab | Muted text; ink and blue underline when selected | Weekday filter |

Use the shared button helpers. Disabled and busy actions remain recognizable
but cannot start work twice. Focus stays visible without hover or animation.
Color alone never communicates an action's meaning.

The browse toolbar is one wrapping band directly on the page ground, without
an enclosing glass card. Previous, year, seasons and next form one group.
Search occupies the other end at normal size and wraps at enlarged text sizes.
Weekdays sit directly below. Inputs and buttons share a visual height.

### Posters

The entire poster tile is clickable, including its title and metadata. Browse
opens group selection or existing resources; My Follows opens the episode
library. Keep artwork large and metadata subordinate. Use the same radius,
aspect ratio and hover behavior in both grids.

Browse uses one quiet followed badge. My Follows shows actionable failures,
paused/download status and new-resource counts. Automatic-follow status remains
available on hover. Avoid heavy shadows or multiple ornamental overlays.

### Panels, dialogs and feedback

Settings use opaque paper panels on the neutral canvas. Resource rows use a
single separator and a compact status label. Avoid nested cards for each text
fragment.

Dialogs share the same width constraints, scrolling and padding. Long content
must be reachable at 1040 × 720 and 200% text. Confirmation dialogs keep their
action footer outside the scroll region with `dialogContentWithFooter`.
Keyboard Escape and the dialog's close/cancel action retain their existing
semantics.

Treat loading, empty, error and content as distinct states. Loading describes
what is happening. Empty states explain the next useful step. Source failures
offer retry or source settings. Failed downloads keep their title and retry
action. Avoid a blank content pane or permanent spinner after a failure.

## Motion

Motion explains an interaction and stops promptly. Do not animate decoration.

| Interaction | Behavior |
| --- | --- |
| Color / hover | 130 ms ease |
| Button press | 130 ms ease; scale 0.99 |
| Poster hover | 160 ms ease; lift 2 DIP, no zoom |
| Poster press | Return to rest position; no shrink |
| Page change | 160 ms fade, no page translation |
| Popover | 140 ms; at most 2 DIP translation, no scaling |
| Row changes | 160 ms; at most 2 DIP entry |

No spring bounce or overshoot. Both the system reduce-motion preference and
the saved reduced-effects setting disable motion while retaining focus,
selection, contrast and all actions. Theme changes must not reset text scale.

## Page recipes

1. **Browse:** season title and one-line purpose, compact filters, weekday tabs,
   dismissible first-use guidance, poster grid. Guidance uses a quiet neutral
   strip without a decorative icon. Search and filters remain visible while
   posters scroll; changing a filter follows the existing scroll contract.
2. **Choose and follow:** anime header, release-group choice, understandable
   title-word filters, resource preview, confirmation. Inclusion says “只要”;
   exclusion says “不要”. Undo and redo preserve their existing behavior.
   Matching counts and selected download counts stay distinguishable.
3. **My Follows:** title, search and add action, follow count, poster grid.
   Check failures sort first and remain actionable. The main click watches;
   management remains secondary.
4. **Episode library:** anime identity, group/status, management menu, episode
   list sorted by episode number. Completed files offer playback; incomplete
   downloads show progress. Unknown episode numbers sort last.
5. **Downloads:** stable rows, filename, progress and transfer information,
   failure reason and appropriate actions. Stop, remove-task and delete-files
   have distinct behavior and copy.
6. **Settings:** readable sections for folders, appearance, network and window
   behavior. Saved light/dark/system preferences apply immediately. Advanced
   options explain their effect in plain Chinese.

Close-window behavior keeps the existing ask/background/quit preference.
Background operation depends on a working tray registration; platform-specific
close and activation behavior is not a visual redesign decision.

## Interaction and accessibility

- Preserve keyboard navigation, visible focus, tooltips and accessible labels.
- Preserve selection, undo/redo, preview isolation, stale-result cancellation
  and persistence behavior.
- An action must not unexpectedly move its scroll position. State updates must
  not replace keyed scroll containers or reset their state.
- At 200% text, allow controls to wrap and long content to scroll. Every primary
  action must remain reachable at the minimum window size in both themes.
- Use the existing logo assets. Brand masters under `build/icon` and embedded
  platform assets must stay synchronized if changed separately.

## Acceptance and evidence

Render deterministic fixtures using `--render`, covering browse, filtering,
following, episode library, downloads, settings, errors and dialogs in both
themes. Compare the actual images, not just token values. Also exercise minimum
window size and 200% text with `ui.Tester`, including navigation and playback
entry, and verify text contrast.

New behavioral regressions require an assertion failure against the previous
implementation using `cmd/redcheck`, then a pass against the new one. Run the
repository build/tests/vet and Linux/macOS cross-builds. Refresh reference images
under `docs/assets/ui` when they depict changed screens.

Fixture screenshots prove deterministic layout and rendered appearance. They
do not prove live source availability, real downloads, native Windows DPI,
screen-reader behavior, or runtime behavior on macOS/Linux. Report those
boundaries with the validation results.

Reference renders: [Browse, light](docs/assets/ui/01-browse.png),
[Browse, dark](docs/assets/ui/01-browse-dark.png),
[My Follows](docs/assets/ui/03-following.png),
[Episode library](docs/assets/ui/13-episode-library-dark.png),
[Downloads](docs/assets/ui/04-downloads-dark.png), and
[Settings](docs/assets/ui/05-settings.png).
