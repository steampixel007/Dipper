# Rehearsal: Dipper Browse directions

Date: November 11, 2026.

## Scope and reference

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

Shelf uses a side sheet; Daily keeps a selected preview beside its list on
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
