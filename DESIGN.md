# Dipper design system

This is the design system for Dipper's desktop interface. Use it when changing
pages, controls, visual tokens, copy, or interaction states. The implementation
is the native MyGo UI in `cmd/dipper/view.go` and `appearance.go`; keep this
specification and those files in sync. [Architecture](docs/architecture.md)
explains the UI stack, [README](README.md) describes the current user workflow,
and the [GUI subsystem](docs/subsystems/gui.md) records implementation rules.
The [core subsystem](docs/subsystems/core.md) defines matching, persistence,
and download decisions; this page specifies how people see and control them.

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

## Foundations

### Color and surface

The current interface uses a light theme. Treat colors as semantic roles;
reuse the values in `view.go` and `spectrumTheme()` rather than introducing a
second palette.

| Role | Value | Use |
| --- | --- | --- |
| Window chrome | `#f3f3f3` | Title bar, sidebar, hover surface. |
| Primary surface | `#ffffff` | Main reading area, cards, popovers. |
| Secondary surface | `#f8f8f8` | Hints and low-emphasis sections. |
| Divider / control outline | `#e9e9e9` / `#c6c6c6` | Section separation / input and secondary-button borders. |
| Text / muted text | `#292929` / `#505050` | Main content / supporting information. |
| Accent / accent wash | `#3b63fb` / `#f5f9ff` | Primary actions, selected navigation, include conditions. |
| Accent hover / pressed | `#274dea` / `#1d3ecf` | Interactive feedback. |
| Exclude / exclude wash | `#b1262b` / `#fff5f5` | Exclude conditions, always with a minus sign and text. |
| Success / warning | `#12805c` / `#9c5700` | Successful matches and status / caution needing explanation. |

A status is never conveyed by color alone. The selected sidebar entry also has
text weight and an accent marker; word chips use `＋` or `−` plus wording.

### Type, spacing, and geometry

The current Windows font stack is Segoe UI, Microsoft YaHei UI. Body text is
14 DIP; supporting text 12 DIP; badges 11 DIP; section headings 18 DIP; the
browse heading 26 DIP; the anime detail title 23 DIP. Weight and size establish
hierarchy. Keep long original titles selectable and accessible even if a
surrounding card truncates its own label.

Use a 4 DIP spacing base, commonly 8, 12, 16, 24, and 28 DIP. The initial
window is 1320 × 900 DIP, the minimum is 1040 × 720 DIP; the top bar is 64 DIP
and sidebar 204 DIP. Detail conditions occupy a 306 DIP column beside the
release preview, with separate scrolling. The primary action stays visible
below both. The poster grid adapts from three to six columns with 18 DIP
horizontal and 24 DIP vertical gaps.

Buttons have a 20 DIP radius; inputs and cards generally use 6–8 DIP. Icon
buttons are 32 × 32 DIP. Posters use an 8 DIP radius. Use the existing `quiet`,
`secondary`, `primary`, `iconButton`, `badge`, and `cover` helpers in `view.go`
for these roles rather than restyling every page separately.

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
| Settings | Appearance, download location, shared upload limit, and window/background behavior. |

A page must provide distinct loading, empty, failure, and content states. Source
failure offers retry without clearing saved follows or downloads. Empty search
offers a way to clear filters. Failure to save must never look like success.

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

## Motion and accessibility

The easing curve is `cubic-bezier(0, 0, .4, 1)`. Button feedback uses 130 ms,
local popovers 160 ms, and page and content changes 190 ms. Buttons press to
0.97 scale; poster hover lifts 3 DIP and scales to 1.008. Motion explains a
change without delaying the next action. When the in-app reduce-effects option
or system reduce-motion preference is active, the UI disables animations.
These values are defined in `appearance.go` and `view.go`.

Every action needs a text label or accessibility name; icon-only actions also
have tooltips. Keep keyboard focus visible and preserve native control
behavior. Esc closes the local choice before leaving the current workflow.
Maintain access to the primary action at the minimum window size; do not move
a page's scroll position as a side effect of clicking a control.

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
not Adobe components or Apple platform components.
