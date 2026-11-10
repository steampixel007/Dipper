# Dipper Browse Anime: independent visual review

Reviewed on November 11, 2026. Scope: rendered presentation and visible interaction states only. No implementation edits, no subagents. The reference is Stageshow's `static-card`, judged from `gold.png`; the review and verification references were read. INTENT.md, implementation rationale, and REHEARSAL.md were not read.

## Direction recommendation

1. **Shelf: strongest default for Browse Anime.** Large, recognizable posters make the seasonal catalog the subject, titles remain readable, and the supplied drawer states show a comprehensible progression from browsing to choosing a version. Refine the introductory space and make the preview more informative.
2. **Daily: strongest alternative for a weekday routine.** The selected weekday, highlighted row, and persistent preview establish a clear selection relationship. The compact layout deliberately rearranges the preview above the list. Its desktop list needs more visual density, and the preview needs information specific to the selected show.
3. **Cinema: most distinctive mood, least ready across sizes.** The spotlight and thumbnail strip establish a different browsing interaction, rather than merely recoloring Shelf. The compact composition loses the visible follow action and crowds the footer, while the desktop spotlight's counter collides with the poster.

These recommendations concern the supplied presentation. They do not establish that every clickable flow works.

## Ranked findings

Ordered by framing, hierarchy, interaction, then polish. Each finding names a visible frame and a concrete fix.

| Rank | Category | Visible finding and frame | Recommended fix |
| --- | --- | --- | --- |
| 1 | Framing / primary action | **Cinema's compact spotlight spends its height without retaining the action.** In [cinema-small.png](cinema-small.png), the upper-left half of the hero is largely empty, the poster sits high on the right, and the title occupies the bottom; the follow button visible in [cinema-wide.png](cinema-wide.png) is absent from this compact frame. | Recompose the compact hero as a shorter poster-and-title block with the primary action immediately below the title; keep that action visible before the discovery strip. Availability through another route was not checked. |
| 2 | Framing / crop | **Cinema's compact footer overlaps meaningful content.** In [cinema-small.png](cinema-small.png), the provenance footer crosses the thumbnail-title region; the selected title and neighboring labels compete with footer text at the bottom edge. | Give the footer its own space outside the strip's scroll area, and reserve enough height for a complete thumbnail, title, and lower inset. |
| 3 | Framing / content density | **Shelf delays the actual catalog behind a substantial introductory band.** In [shelf-wide.png](shelf-wide.png), the first poster begins around 42% down the frame; [shelf-small.png](shelf-small.png) retains a similarly tall sequence of chrome, English eyebrow, headline, subtitle, search, tabs, and section label. | Compress the introductory stack, move search into a tighter toolbar, and bring the first poster row upward while retaining readable filters. |
| 4 | Hierarchy / density | **Daily's desktop list gives much more space to empty row width than to show recognition.** In [daily-wide.png](daily-wide.png), small cover thumbnails and short two-line labels occupy the left edge of very broad rows; only four entries are visible beneath the weekday bar, while the preview repeats the first cover and title. | Narrow the list or increase the useful cover area, tighten row pitch modestly, and use the recovered space for more entries or show-specific preview information. Preserve the clear selected-row treatment. |
| 5 | Hierarchy / readability | **Cinema's alternatives are visually suppressed too far.** In [cinema-wide.png](cinema-wide.png) and [cinema-small.png](cinema-small.png), unselected covers are darkened and their already truncated title labels are faint; the selected item is clear, but comparing the next choices takes effort. | Keep alternative posters closer to their original brightness and raise label contrast; distinguish the current item with its existing border and preview badge. |
| 6 | Interaction / decision support | **The preview explains saving before helping someone decide whether to watch.** [shelf-detail.png](shelf-detail.png) and [daily-wide.png](daily-wide.png) provide a poster, title, weekday, format, and generic instructions about choosing a version, but no visible show-specific premise. | Add a short, distinct synopsis or another meaningful show-specific detail near the title, and reduce generic workflow explanation to a concise helper. |
| 7 | Interaction / confirmation | **Shelf's followed state retains instructions for the unfinished step.** In [shelf-followed.png](shelf-followed.png), the followed badge, incremented sidebar count, changed button, and undo toast communicate success, but the helper below the button still tells the user to finish choosing a version before following. | Change or remove that helper after success; confirm the saved version or explain the next useful action instead. |
| 8 | Polish / collision | **Cinema's desktop position counter is caught beneath the tilted poster.** In [cinema-wide.png](cinema-wide.png), the `01 / 19` counter touches the poster's lower edge and is partly obscured. | Place counter and navigation together on a clear baseline below or beside the poster, with a deliberate gap from its rotated bounds. |
| 9 | Polish / clipping | **Daily leaves a fragment of helper text beneath its follow button.** In [daily-wide.png](daily-wide.png), faint text at the bottom of the preview is clipped just above the footer region. | Keep the helper wholly inside the preview's available height, or omit it at this height; do not leave a partial line as the panel's final visible element. |
| 10 | Polish / secondary text | **Secondary controls and metadata are very faint.** [shelf-wide.png](shelf-wide.png) shows pale weekday labels and detail links; [daily-small.png](daily-small.png) shows pale weekday metadata; Cinema's navigation labels are particularly subdued against the dark ground in [cinema-wide.png](cinema-wide.png). | Increase text contrast for actionable labels and relevant metadata while keeping provenance and decorative copy quieter. This is a visual readability finding, not a measured contrast-compliance verdict. |

## Comparison with the gold

The gold uses a hard crop and one elevated detail card to make one selected item and its result immediately legible. Its meaningful text stays whole.

| Direction | One principal difference beside `gold.png` |
| --- | --- |
| Shelf | The emphasis is a broad catalog rather than one selected result. That suits browsing, and the posters give the frame substantial real content; the oversized introduction weakens that advantage. |
| Daily | It shares the gold's selected-item-to-detail relationship, but distributes it across a much wider, flatter composition with small list covers and repeated identity in the preview. |
| Cinema | It gives one item clear dominance, but the compact spotlight surrounds the poster with unused space while losing the visible action; the gold keeps its focal result complete. |

The full desktop shell is useful context for comparing desktop page directions, so I do not treat an uncropped window as a defect by itself. The more consequential framing issues are inside the shell. The bird mark, anime covers, seasonal filters, and follow states make these recognizable as Dipper anime browsing concepts rather than generic dashboards.

## Verification coverage and limits

- **Frames inspected with image-viewing tools:** `gold.png`; all three `*-wide.png`; all three `*-small.png`; `shelf-detail.png`, `shelf-version.png`, and `shelf-followed.png`.
- **Framing:** assessed the visible gutters, occupied areas, meaningful crops, and overlap. Shelf's partial second poster row reads as continuation; Cinema's crossed title region and counter do not. No phone-specific island or status-bar judgment applies to these desktop-window scenes.
- **Hierarchy:** actual artwork and readable show titles occupy the subject; no gray placeholder bars replace focal content. Avatar, date-range, calendar, and bento-specific checks are not applicable to these scenes.
- **Interaction evidence:** the supplied Shelf frames show preview, version-choice, and followed states, including a disabled preselection button and an undo affordance. Those frames establish visible states, not successful clicks or persistence. The live root URL loaded and its rendered text exposed the three direction controls; an attempted automated switch did not complete, so no live click-through pass is claimed.
- **Not checked:** reliable direction switching; search, weekday filters, season selector, scrolling, keyboard navigation, focus containment or restoration, Escape dismissal, version selection, follow, undo, and cross-direction state continuity. No claim is made that a control is nonfunctional merely because it was not exercised.
- **Not checked:** animation entrances and exits, 40–80 ms transition frames, loop seams, reduced-motion behavior, `--gate` or `--states` results, 4x glyph inspection, numerical contrast ratios, browser/platform differences, or widths beyond the supplied wide and small frames. The additional `*-390.png`, `*-desktop.png`, and `shelf-reduced.png` files were not visually reviewed.
- No implementation files were inspected. No implementation fixes, builds, tests, or new screenshots were produced. The only authored file is this report.
