# Rehearsal: Dipper Browse directions

Latest revision: November 14, 2026. Older sections below are historical.

## Empty-state refinement, November 14, 2026

The three Shelf empty states now share one horizontal status card: the icon and
title name what happened, the supporting line gives only the missing context,
and the action names the next step. Search repeats the query and offers
`清空搜索`. An empty weekday says which day has no updates and suggests trying
another date. An all-followed day explains that `只看未追番` is hiding results
and offers `显示已追番` plus `浏览全部番剧`. At 390px the card keeps its
structure, wraps the explanation, and gives full-width actions their own row.

Inspected `evidence/empty-search-desktop.png`, `empty-day-desktop.png`,
`empty-followed-desktop.png`, `empty-search-compact.png`,
`empty-day-compact.png`, and `empty-followed-compact.png`. The primary action
retains the neutral charcoal fill and white text. `npm run build`,
`node scripts/check.mjs` (84 checks), and `node scripts/discovery.mjs`
(63 discovery checks) passed with no browser errors. The standalone Shelf
export was rebuilt with all 19 covers.

## Follow-status filter revision, November 14, 2026

The shelf now treats the follow-status filter as a secondary result control,
not a weekday. `只看未追番` sits in the result row as a checkbox, keeps a visible
checked state, composes with search and weekdays, and remains available at
390px. Following a work while it is active still removes that work and restores
focus to a remaining poster. Empty filter results identify the filter and offer
`显示全部作品`. The idle result row no longer contains the instructional hint
`点开海报，先了解一下`, and the season-wide search explanation is removed.
Poster cards also omit the repeated `查看详情` and `已在片单` labels: the footer
carries the broadcast day only when all days are shown or search crosses the
week, while the followed badge and poster hover treatment carry the remaining
state. Inspected
`evidence/poster-card-footer-1144.png` at 1144px; the grid footer is materially
quieter without losing schedule information.

Season context now appears once, in the functional top-right selector. The shelf
heading omits the decorative English season eyebrow, and the idle result row
shows only the count. A selected weekday is already named by the active tab, so
it is omitted from the result row and poster footers; search reports `找到 N 部`
without explaining its scope, and an empty query says `没有找到相关番剧`.
Inspected `evidence/shelf-copy-all-1144.png`,
`evidence/shelf-copy-weekday-1144.png`,
`evidence/shelf-copy-search-1144.png`,
`evidence/shelf-copy-empty-1144.png`, and
`evidence/shelf-copy-restored-1144.png` at 1144px; the result row stays quiet
without losing the count or the cross-week broadcast day.

Inspected the desktop hover and checked states plus the checked compact state in
`evidence/filter-hover-1144.png`, `filter-active-1144.png`, and
`filter-active-390.png`. The filter moved from 19 to 17 works with the two
initial follows. `node scripts/discovery.mjs` passed 55 checks, including search
composition, preserved filter state after clearing search, weekday scope in the
Daily concept, the removed-origin focus return, and compact visibility.
`npm run build`, the standalone shelf export, and `git diff --check` passed. The
broader browser regression run was attempted but still stops at the existing
reduced-motion press-scaling assertion; this copy revision does not touch that
motion code. No independent visual review was run.

## Detail header simplification, November 13, 2026

The header now reads `番剧详情`. Adjacent-work arrows, the counter, and visible
Escape text are removed. The close button remains, with `关闭详情（Esc）` as
its tooltip. Choose another work by returning to the shelf. The standalone
export passed header, close, Escape, and follow-flow checks at 728px and 390px.
All 39 current discovery checks and the Vite production build passed.
The broader regression run reached its reduced-motion press check and failed:
the existing detail action override still scales the button under reduced
motion. That separate interaction issue remains unresolved.

## Work-first detail and explicit next step, November 13, 2026

The detail study gives the poster a 250px column, enlarges the title and
synopsis, and aligns actions with the reading columns. The owner rejected the
blue-teal action color. Both detail actions now use neutral charcoal: an outlined
`返回选片` action and a filled `选择字幕组` action, with matching height, corner
radius, minimum width, and padding. The helper says
`选好字幕组后，再确认追番`. Choosing a subtitle group remains a separate page,
and only its confirmation creates the simulated follow.

Inspected the refreshed desktop and 390px detail crops, compact setup, and
compact missing-description state. Both viewport sizes have a visible close
control and no horizontal document overflow. Actual pointer checks confirm
background feedback on both actions. The full suite passed 81 regression and
41 discovery checks before the later button refinements. Targeted checks of
the earlier ghost variant verified hover feedback, setup entry, Escape back
to detail, and return to the shelf at both widths. Current export verification
is recorded below.

The outlined-return revision passed focused checks at 728px and 390px: both
actions have a 44px height, 8px corner radius, and matching total horizontal
padding. The return action has a white idle fill, visible inset outline,
hover feedback, press feedback, and dismisses back to the shelf. Inspected
settled screenshots at both widths. The standalone export passed its setup,
confirmation, undo, and reset flow with no console errors or external
requests; the Vite production build passed.

The earlier 390px reduced-motion still gate passed. The desktop still gate
reported two padding heuristics on the panel and footer. Their toolbar, reading
columns, and action rows intentionally use different insets; this is a recorded
gate failure, not a full visual-gate approval. No fresh animation audit or
independent visual review was completed. An optional capture helper timed out
when searching a filtered shelf; fresh direct-link captures succeeded.

Current frames use `evidence/detail-redesign-*`. `detail-review.html` presents
the desktop and narrow crops with a link to the running interactive prototype.
This is an exploratory React prototype; DESIGN.md and the native app are
unchanged.

## Footer color correction, November 13, 2026

The owner's feedback concerns the two detail actions. The secondary action
now has a white surface, a gray-green edge, and matching dark text. The primary
uses muted slate blue (#46647C) instead of saturated blue. Follow setup shares
the footer tokens. The existing Button hover, press, disabled, and reduced-motion
behavior remains in place.

Inspected `evidence/button-colors-1320.png`, `button-colors-390.png`,
`button-colors-setup-390.png`, and both desktop hover close-ups in `evidence/`.
Actual pointer checks at both widths verified a background change on each
action and restoration after leaving. `npm run check` passed 81 regression
and 41 discovery checks, including press and reduced-motion checks.
`npm run build` passed.

The wide and reduced-motion 390px still gates both ended:
`rehearsal passed (necessary, not sufficient: references/verify.md and the fresh eye in review.md still stand)`.
Reports and frames use the `evidence/button-colors-gate-` prefix. The narrow
report repeats the historical 14px label-offset observation. In the inspected
reduced-motion frame, that offset is the secondary button's internal padding;
the button edge aligns with the footer copy. Nothing is clipped there.
These are still gates; motion was not judged. The complete walkthrough and a
new independent review were not repeated for this color-only correction.
No native application or binary changed.

## Current correction: separate follow setup, November 12, 2026

The owner's correction supersedes the inline-version flow below. Detail now
introduces the work only. `追这部` opens a separate `追番设置` page inside
the product window. An explicit subtitle-group choice reveals only that
group's attributed mock resources. `返回详情` and Escape preserve the mounted
synopsis and its scroll. Confirmation returns to detail with the result and
undo. Previously followed works can change groups without increasing the count.

Final `npm run check` (t18x): 69 detail and 41 discovery checks passed.
Final `npm run build` (t190): passed. No native application or binary changed.
`scripts/group-flow-red.mjs` (t184) injected a group radio into detail, triggered
the exact synopsis-boundary assertion, and restored the source in `finally`.
This is bounded mutation evidence, not a rebuild of the historical implementation.

Desktop detail and desktop/390px setup screenshots were inspected. They show
no group controls inside detail, a distinct setup page, and compact fixed
actions. The group search input's native border and the oversized return action
were corrected before the final build and captures.

The first compact walkthrough exposed simultaneous opacity fades when setup
content changed: the resource prompt and rows were briefly legible together.
The resource transition now clears the prompt before showing rows, and group
selection is screenshotted only after its rows reach full opacity. Leaving
setup fades its contents first, then its white surface, while the detail panel
is already mounted underneath. The later 390px capture shows no double
exposure in the tested frames.

Current stills: `evidence/shelf-detail.png`,
`evidence/shelf-group-resources.png`, and
`evidence/shelf-group-resources-390.png`.
Current walkthroughs use `evidence/shelf-group-flow-wide.png` and
`evidence/shelf-group-flow-390.png`. Historical inline-version captures below
do not establish acceptance of the corrected navigation.

Latest walkthrough gates (both exit 2 because sampling coverage is
inconclusive, with `rehearsal passed`):

- `t18y`, width 390: 38 shots requested at 450ms; 1405 sampled frames over
  17.82s; largest gap 125.2ms; 109 gaps over 16ms. Remaining observations are
  the footer's 14px label offset, cursor-on-label presses, the hidden browse
  page's direct removal, and two same-box text changes.
- `t18z`, width 1320: 38 shots requested at 450ms; 1307 sampled frames over
  19.37s; largest gap 270.8ms; 122 gaps over 17ms. Remaining observations are
  the hidden browse page's direct removal, a status badge's direct removal,
  and three same-box text changes.

These reports do not establish frame-complete motion approval. The sampled
gaps and manual observations remain limits, and the latest separate flow has
not received a new independent fresh-eye review.

## Historical inline-version discovery revision (superseded)

Gold: stageshow `examples/phone-touch`, copied into `motion-gold/` and filmed
before this revision as `evidence/motion-gold.png` with its JSON report. Kept
the causal open, explicit result hold and close, and a complete reduced-motion
poster. Applied those relationships to the existing Shelf, rather than copying
the phone's identity. The earlier static-card reference remains historical.

Beats: shelf, open a work, read, choose an inline version, confirm, hold the
followed result, undo, close back to the same shelf. Playback is opt-in at
`/?capture=shelf&rehearse`, once with explicit replay. The manual route is
`/#shelf`. Reduced motion shows the completed followed result with usable
manual controls.

Changes:

- Removed the separate Shelf version page: poster, version, confirm takes
  three clicks instead of four. A new work starts without a selected version.
  Existing follows retain their saved version; unchanged confirmation is
  disabled and version updates do not duplicate counts or expose destructive
  undo.
- Search covers the whole local season while typing; clearing restores filters.
  Enter opens a sole result, Ctrl+K focuses search, and Escape clears it.
- Previous/next detail controls retain the original comparison list, including
  when following removes a work from the unfollowed filter. Dismissal preserves
  shelf scroll and restores a connected poster, or search for an empty list.
- The cover and title stay fixed while the keyboard-focusable synopsis scrolls.
  Outgoing reading layers retain their previous scroll offset; new works start
  at the top. Panel, toolbar and footer stay anchored.
- Retained badges, undo, status text and toast have transitions. The confirmation
  button retains the kit's press easing as well as its opacity transition.
  A harmless idle Stage script enables actual manual motion; a Stage without
  a script intentionally makes descendants still.
- Weekdays are a pressed-button filter group with arrow/Home/End navigation.
  Compact framing reduces the introduction and improves essential helper text.

Functional validation:

- Before implementation, t16q failed the new assertion `opening a work must
  expose version choices without another page or click`. This was a real
  assertion failure, not a timeout, missing import or compile failure.
- Latest `npm run check`, t17w: 67 regression checks and 30 discovery checks
  passed. Coverage includes the three-click flow, search/filter restoration,
  existing-version updates, keyboard scrolling with retained identity, outgoing
  scroll offset, stable comparison, fallback focus and reduced-motion controls.
- Latest `npm run build`, t17x: passed after the final CSS correction.
  No Go code or native binary changed.

Current captures and film results:

| Capture | Latest observation |
| --- | --- |
| `evidence/shelf-motion-wide.png` / `.json`, t17s | 38 shots requested at 450ms, width 1320; 1404 sampled frames over 22.71s; largest gap 240.0ms; 154 gaps over 20ms. |
| `evidence/shelf-motion-390.png` / `.json`, t17t | 38 shots requested at 450ms, width 390; 1183 sampled frames over 17.99s; largest gap 132.7ms; 121 gaps over 19ms. |
| `evidence/shelf-open-beat.png`, `shelf-confirm-beat.png`, `shelf-close-beat.png` | Eight shots each requested at 80ms around reading, followed and returned marks, width 390. Actual screenshot intervals are longer, approximately 350–500ms; marks follow the actions, so these show state continuity rather than every press or exit frame. |
| `evidence/shelf-motion-reduced.png` | Reduced-motion still; complete followed result. Static capture does not judge motion. |
| `evidence/shelf-home-390.png` | Compact homepage still; search, weekdays and two poster columns remain legible. |

Both current full-walkthrough gates end:

```text
film: inconclusive; watch the motion and inspect every image before approval
rehearsal passed (necessary, not sufficient: references/verify.md and the fresh eye in review.md still stand)
```

The exit code is 2 because coverage is inconclusive. Gate passage is necessary
evidence, not an overall motion pass. Opened both full contact sheets and all
three smaller mark sequences. The smaller sheets show readable, stable content
in the new, followed and returned states. The large desktop sheet is difficult
to inspect at readable scale; it is not evidence for fine text or every frame.
The target-following confirmation close-up attempt t17v failed to find a visible
target and is not approval evidence.

Remaining film observations:

- Desktop: sidebar count `2` and poster metadata `查看详情` change in the same
  box at 7.58s. Compact: the poster metadata changes at 7.41s. These peripheral
  values still update directly. The primary detail status crossfades; the
  peripheral text should receive further motion review before production.
- Compact: cursor overlap on `撤销` at 9.5s. This is a walkthrough choreography
  issue; the control remains visible in settled frames. It remains unresolved.
- Previous confirmation-button pop/jump reports did not recur after restoring
  the kit press transition and removing forwards animation fill. Sampling gaps
  prevent treating that absence as proof that no intermediate defect exists.

Fresh-eye review: `evidence/fresh-eye.md`, t16y, inspected source and earlier
contact sheets, not a live browser. Addressed its seven findings: focusable
synopsis scrolling, connected fallback focus, retained outgoing scroll, fixed
identity, filter semantics, compact header space, and helper readability.
The latest 30 discovery checks exercise the behavioral fixes. That reviewer did
not independently verify the final motion or final frames.

Compared with phone-touch, the anime cover/title remains the focal object and
the footer carries the result. This study has more content because comparing
catalog works is its purpose; the phone's sparse composition is not the target.
The full desktop composition, compact layout and transitions still need user
evaluation at their actual display size.

Assumptions and limits: local temporary follow state, mock versions and weekdays,
two synopsis samples. This prototype ends at following, not playback. No real
subscription/download, metadata completeness, time-to-find improvement, physical
touch, screen reader, zoom, cross-browser performance, native GUI or production
integration was verified. Remaining sampling gaps and choreography observations
prevent claiming complete animation acceptance.

## Historical Shelf detail revision, November 12, 2026

Replaced Shelf's narrow right sheet with an 800px centered panel. A persistent
poster anchors both the synopsis and version-selection steps. The footer
stays visible while the body scrolls. Two sourced synopsis samples replace
generic subscription instructions; missing descriptions are explicit. Existing
follows retain their selected version, and saving a version does not offer an
undo action that would remove the existing follow.

Validation:

- Before implementation, the new geometry assertion failed against the old
  drawer with `shelf detail should be a wide centered panel, with equal space
  on both sides` (terminal t16f). This was an assertion failure, not a missing
  element or compilation failure.
- Latest `npm run check`: 65 browser checks, including centered geometry,
  synopsis content, inactive background, keyboard wrapping and restoration,
  return from version selection, unchanged follow count before confirmation,
  shelf scroll preservation, existing-version updates, backdrop and close
  controls, compact footer visibility, long-title wrapping, and direct links.
- `npm run build`: passed. No Go code changed; native builds, native GUI
  acceptance, and cross-platform checks were not run for this prototype.
- Visually inspected `evidence/shelf-detail.png`, `shelf-version.png`,
  `shelf-detail-390.png`, and `shelf-detail-long-390.png`. The desktop content
  reads poster, title, synopsis, then action. Compact content stacks with a
  separate footer; the long title remains fully visible.

Updated Shelf captures are plain Playwright screenshots. The earlier film
gate results below describe the prior layout; that gate was not rerun and
does not certify this revision. Daily and Cinema were exercised by the
browser regression checks.

## Historical scope and reference

Three interactive design studies, served locally at port 4397. Production Go
code, DESIGN.md, preferences, subscriptions, and downloads are outside this
study. The initial exploration made no commits or branches. At the owner's
request, this prototype is now being saved as a checkpoint on
`design/browse-directions-20261111` for a later design discussion.

## Checkpoint validation on November 11, 2026

The required `scripts/build.ps1 -Test` ran before committing the snapshot.
All Go package tests passed, `go vet ./...` passed, and the Windows
`bin/dipper.exe` build succeeded. The script then exited with an error while
copying the binary to `%LOCALAPPDATA%\Programs\Dipper\dipper.exe`, because
the installed executable was in use. The running application was left open.
This is a partial script result, not a passing invocation.

No Go source files changed. Cross-platform builds, race checks, and redcheck
were not run for this standalone design checkpoint. The earlier frontend
results remain 39 passing browser checks and a successful Vite build.

Gold: Stageshow `examples/static-card`, copied with its relative kit import
adjusted and captured before implementation as `evidence/gold.png`. Retained
Stage, Window, Row, and kit controls. Replaced the customer-list subject with
Dipper's chrome, real bundled covers, Chinese titles, and version selection.
The complete app shell is retained to compare page directions in context.

Beside the gold, Shelf emphasizes scanning a catalog rather than a single
selected result; Daily spreads selection and preview across two columns;
Cinema makes one cover and its action dominant. These are product direction
studies rather than marketing crops.

## Interaction beats

There is no autoplay loop or authored cursor walkthrough. These beats are
driven by the viewer:

1. Switch between Shelf, Daily, and Cinema.
2. Search or choose a weekday; inspect a poster, row, or spotlight.
3. Open version selection and choose an option before confirmation enables.
4. Follow the title; observe the changed action and count; optionally undo.
5. Dismiss the overlay and continue browsing with restored keyboard focus.

Shelf now uses a centered detail panel; Daily keeps a selected preview beside its list on
desktop and above it on compact layouts; Cinema uses a spotlight and rail.

## Captures and functional verification

- Wide captures: 1280px viewport, 1144 by 744 stage.
- Compact captures: 390px viewport, 358 by 810 stage; compact layout below 800px.
- Scenes: `evidence/{shelf,daily,cinema}-{wide,small}.png`.
- Additional states: Shelf detail sheet, version selection, followed state,
  and a reduced-motion still in `evidence/`.
- Host captures and geometry reports:
  `evidence/host-{shelf,daily,cinema}-{1280,390}.{png,json}`.
- Final browser check: `node scripts/check.mjs`, terminal t160,
  **passed 39 checks**.
- Final production web build: `npm run build`, terminal t161,
  **built successfully in 4.43s**.

Browser checks cover cover loading, search and empty results, weekday filters,
selection and confirmation, follow and undo, overlay dismissal, focus trapping
and restoration, Cinema navigation, compact action visibility and clickability,
the compact scrolling boundary above the footer, horizontal overflow, reduced
motion rendering, and browser errors. These establish browser behavior only.

## Final film verdicts

All captures used `--still --gate`; motion was not judged.

| Frame | Terminal | Final verdict |
| --- | --- | --- |
| Shelf wide | t15t | `rehearsal failed: 1 never-intentional line(s) above` |
| Daily wide | t15w | `rehearsal failed: 1 never-intentional line(s) above` |
| Cinema wide | t15y | `rehearsal failed: 1 never-intentional line(s) above` |
| Shelf compact | t15v | `film: no-issues-detected`; `rehearsal passed` |
| Daily compact | t169 | `film: issues`; `rehearsal passed` |
| Cinema compact | t168 | `film: no-issues-detected`; `rehearsal passed` |

The wide gate is **not green**. Each wide frame reports uneven padding on
`main.browse-main`: bottom 0px versus side 26px for Shelf, bottom -8px versus
26px for Daily, and bottom 8px versus 26px for Cinema. Visual inspection of
the three named frames shows the main area is an application pane composed
of an edge-to-edge toolbar, inset scrolling content, and an edge-to-edge
footer. It has no authored card padding. The report compares descendants of
these different regions as though they shared one padded card. This is
recorded as an app-pane classification limitation, not a passing gate.
No extra padding or hidden content was introduced to suppress the report.

Daily compact still reports a 2px crop of `span.quiet-dot` at the
`div.daily-columns` scroll boundary and 60px versus 69px optical insets on
the version-action label. The frame shows the beginning of the schedule
disclaimer at the lower scroll edge; it can scroll into view. This remains
a minor visible polish issue. The action is whole and clickable, but its
heart icon makes the apparent label spacing asymmetric.

All six host layout reports exit 0 with `layout: issues`, reporting
`button.compare-toggle` content 12px from the left and 0px from the right.
The header's comparison control aligns its arrow with the host's right edge
using asymmetric padding. This host-header choice is documented, not a
strict layout pass.

## Independent review and responses

Initial independent review: `evidence/reviewer.md`. The reviewer inspected
the gold, six scene frames, and three Shelf state frames without reading
implementation or rationale. Recommendation: Shelf, then Daily, then Cinema.
Its live switch attempt did not finish; it claims no complete click-through.

| Finding | Response |
| --- | --- |
| Compact Cinema follow action missing | Corrected the compact span selector; action is visible and covered by final browser checks. |
| Compact Cinema footer overlaps rail labels | Gave content its own scroll pane above the footer; verified visually and by browser checks. |
| Shelf introduction delays the catalog | Reduced heading and catalog spacing; further density changes remain a direction choice. |
| Daily list feels sparse | Reduced thumbnail and row pitch; retained the persistent-preview relationship. |
| Cinema alternatives too dark | Increased poster opacity and label contrast. |
| Preview lacks a show-specific premise | Unresolved data limitation; no synopsis was fabricated. |
| Followed helper describes an unfinished step | Changed helper after following to describe continuing from My Follows. |
| Desktop Cinema counter collides with poster | Reduced the tilted poster footprint; latest wide frame shows separation. |
| Daily helper clips under the action | Removed redundant helper content; latest wide frame keeps the primary action whole. |
| Secondary metadata too faint | Increased selected secondary text and control contrast; no numerical compliance claim. |

Parent verification found and fixed a further Cinema arrow click obstruction
by raising navigation above the copy layer. The final 39 checks include this
compact navigation flow. There is no refreshed independent review of the
final fixes; their current verification is by the parent.

## Assumptions and remaining limits

The 19 covers and titles come from the bundled catalog. Weekdays, version
options, and follow state are demonstration data. Friday through Sunday have
honest empty states. Cover availability does not establish a show's current
season, broadcast day, streaming availability, or available release versions.

The prototype needs verified show-specific metadata before a production
decision-support pass. Shelf remains the recommended baseline for browsing;
Daily and Cinema explore different habits and levels of visual emphasis.

Not checked: native GUI integration or Go builds/tests; real source fetching,
subscription persistence, playback, or downloads; cross-browser/platform
behavior; physical touch; measured contrast compliance; 4x glyph inspection;
animation contact sheets, intermediate transition frames, or loop seams.
The reduced-motion still and browser behavior checks do not establish motion
quality. The remaining gate findings are listed above rather than presented
as complete visual approval.

## Interactive detail export, November 13

The earlier inline detail review used screenshots and could not respond to a
pointer. `detail-interactive.html` now embeds the actual React prototype and
its local assets. Its stage fits the chat frame; the underlying product
components and interaction handlers are shared with the Vite version.

The detail primary action changes from `#333` to `#232323` on hover, rises
one pixel, gains a subtle shadow, and moves its trailing arrow two pixels.
Pressing scales the button to `.97` and reduces the shadow. These styles are
scoped to the detail action, retaining the charcoal palette.

`node scripts/check-detail-export.mjs` passed at widths 728 and 390. It
checks actual pointer hover and press, subtitle-group selection,
confirmation, undo, reset, image loading, and absence of external requests
and browser errors. This focused check does not replace the visual and motion
limits recorded above.

## Detail typography trial, November 14

The detail panel now shares one font stack, four desktop type sizes, regular
and semibold weights, and strong/body/muted neutral text roles. Compact titles
use 20px; synopsis text retains its 16px size and 28px leading. Empty synopsis
copy uses the supporting role. The footer helper is one sentence before
following; the saved group and undo remain available after confirmation.

Verification: the standalone detail export passed its pointer and follow-flow
checks at 728px and 390px. All 39 discovery checks, the Vite build, and
`git diff --check` passed. Browser measurements at 1144px and 390px verified
the shared font stack, equal action typography, retained reading space and
footer, and no horizontal panel overflow for works 4031, 3984 and 4063.
Captures in `evidence/detail-type-{id}-{width}.png` include the long-title,
missing-description case. Both desktop-to-compact and compact-to-desktop
resizes retained the primary action.

This reversible prototype styling pass adds no new regression tests.
The broader `scripts/check.mjs` suite was not rerun; its previously recorded
reduced-motion press-scaling failure remains unresolved. Native GUI builds,
Go tests, production design adoption and the motion-quality limits above
remain outside this trial.

## Shelf typography and color trial, November 14

Shelf now uses the detail panel's font stack and neutral text roles. Desktop
page titles are 28px and compact titles 20px. Navigation, search, filters,
poster titles and primary actions are 14px; counts, schedule text, badges and
footer hints are 12px. Titles and selected navigation use strong ink, controls
and descriptions use body ink, and supporting copy uses muted ink. Semibold is
reserved for titles, selected controls and actions.

Blue-gray navigation and filter accents are replaced by neutral gray surfaces.
Green remains only for followed status, matching detail. Poster artwork is the
page's main color source. Pointer hover reveals the poster action and raises
the cover; selected weekdays use weight, ink and an underline; focus rings
remain visible.

Verification: browser measurements at 1144px and 390px found the same font
stack and text colors in Shelf and detail, 14px controls and card titles, 12px
supporting copy, and no horizontal page overflow. Opening a poster retained
the shared detail typography; Esc closed it and restored poster focus with no
browser errors. All 39 discovery checks and the Vite build passed.
`node scripts/check-detail-export.mjs` covers the unchanged detail export.
Captures are `evidence/home-type-{1144,390}.png` and
`evidence/home-type-hover-{1144,390}.png`.

This reversible prototype pass adds no regression test. The unresolved
reduced-motion press-scaling failure in the broader check suite remains out of
scope, as do native builds and platform and motion-quality checks.

## Followed status color revision, November 14

The following status now uses warm amber rather than green. The poster pill is
#7A4E10 text on #FFF8ECF2 with a one-pixel #EAD9BC inset edge; detail metadata
uses the same #7A4E10 ink. The text/background contrast is 6.8:1 against the
opaque ivory equivalent. Preserved dimensions are 12px text and a 20px pill on
desktop, with the existing compact override at 10px.

Verification: browser measurements on the standalone Shelf export confirmed
the amber text, ivory background and inset edge at 1144px and 390px. Following
a work in the standalone detail export produced the same amber status at
1144px. The long and dark covers in `evidence/followed-color-final-shelf-1144.png`
were inspected for legibility. All 39 discovery checks, the standalone detail
export check, the Vite production build and `git diff --check` passed with no
browser errors.

Final captures are `evidence/followed-color-final-shelf-{1144,390}.png` and
`evidence/followed-color-final-detail-1144.png`. Candidate studies are
`evidence/followed-color-{amber,paper,ink}.png`. The broader `scripts/check.mjs`
suite was not rerun; its recorded reduced-motion press-scaling failure remains
outside this color-only pass. Native builds, production adoption and a complete
motion-quality review were not checked.

## Shelf action and count revision, November 14

The isolated `19 部` row is gone. Weekday navigation, the result summary and
`只看未追番` now share one toolbar; the summary reads `共 19 部番剧`,
`共 1 部番剧` or `找到 N 部番剧`. Compact layouts keep the days on one line and
the right-aligned summary and follow filter on the next.

The poster's full-width charcoal strip is replaced by a compact white
`查看番剧详情` chip with a chevron. On pointer hover or keyboard focus the
cover dims to 86% brightness and scales 1.5%, while the chip rises six pixels.
Reduced motion removes those transitions without hiding the affordance.

Verification: all 56 discovery checks passed, including the complete count copy
and the explicit hover destination. The Vite production build, standalone Shelf
export and `git diff --check` passed. Inspected
`evidence/shelf-result-toolbar-{1144,390}.png` and
`evidence/shelf-poster-action-{1144,390}.png`: the count reads as a complete
phrase in the filter toolbar, the compact layout keeps it right-aligned below
the weekdays, and the hover chip remains legible without a heavy full-width
overlay. The pre-existing reduced-motion press-scaling failure in
`scripts/check.mjs` remains outside this revision.
