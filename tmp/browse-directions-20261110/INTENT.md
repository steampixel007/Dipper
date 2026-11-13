# Browse direction study

Medium: three interactive desktop UI studies in a local comparison page. These
are design simulations using Dipper's bundled catalog and posters; they never
change subscriptions or start downloads. The native Go application is outside
this study.

The focal object is the anime, followed by a deliberate decision to follow it.
Existing sidebar navigation stays visually quiet. Source management, download
rules, settings, fabricated ratings and invented streaming availability are
omitted.

1. Shelf: scan a generous poster library, filter it, then inspect a title in a
   wide, centered detail panel without losing the scroll position.
2. Daily: choose a weekday, scan a compact list, and compare the selected title
   in a persistent detail column.
3. Cinema: choose one title at a time from an artwork rail, then enter setup.

All directions use a separate setup page: choose a subtitle group, inspect its
resources, then confirm. Groups and resources are mock data. Search, weekday changes, dismissal and undo remain
interactive. Narrow layouts recompose the content rather than shrink text.

Starting gold: stageshow `examples/static-card`, copied unchanged except its
relative kit import. Its Window / Stage / Row composition is the framing
reference. Final controls reuse the kit, restyled for Dipper and Windows.

Delivery sequence: film the gold, build three directions, exercise their flows
and narrow layouts, capture the outcomes, then record verification boundaries.

## Shelf detail revision, November 12, 2026

The poster and title remain visible while the synopsis scrolls independently.
The detail view answers what the work is about. Its fixed footer offers
`返回选片` and `选择字幕组`, without group or version controls. The latter opens a
separate setup page; returning preserves the reading position. Confirmation
returns to detail with the result and undo. Closing restores the shelf position and poster focus. Compact
layouts pair a small cover with the title above the scrollable synopsis,
retaining the footer.

Keep Dipper's neutral paper surface, Chinese UI font stack, 24px desktop titles,
14px synopsis text, and single blue action. Use an opaque poster without
blurred artwork behind it. No invented ratings, episode availability, genres,
or cast metadata. Two local synopsis samples demonstrate content; other works
explicitly report unavailable descriptions. Weekdays remain mock data.

## Shelf discovery and motion revision, November 12, 2026

Medium and placement: the same interactive Shelf product in the comparison
page, with an opt-in stageshow rehearsal route. The existing three directions
remain available. This is a local prototype, not a native application change.

The focal object is a work and its synopsis. The payoff is an explicitly
configured follow in the demonstration list. Detail remains a reading surface;
subtitle-group selection and attributed resource previews belong on a separate
setup page. New works have no selected group. Existing follows show their saved
group. Browsing, hovering, group selection, and returning without confirmation
must never create a follow. Updating an existing group must not duplicate counts.

Reduce discovery effort as well as clicks. Search the whole local season,
regardless of weekday and unfollowed filters; clearing search restores those
filters. Enter opens a sole result. Ctrl+K focuses search. Previous/next detail
controls compare adjacent filtered works without dismissing the panel.
Dismissal preserves the originating shelf scroll and focus.
The comparison list stays stable when following removes a work from the
unfollowed filter. If the originating poster disappears, focus returns to
the nearest remaining poster, or search when no posters remain.

Motion reference: stageshow phone-touch, particularly the causal open, held
result, explicit close and legible reduced-motion poster. Retain the outgoing
detail layer through its exit. Keep the panel, header and footer anchored when
changing works; crossfade only the reading area. Give filters immediate feedback
and a short result entrance without per-card delays. No text scaling, decorative
loops, fabricated recommendations or playback claims.
The manual Stage has an idle script to retain interactive animation; a Stage
without a script intentionally freezes its descendants. Reduced motion still
settles the complete scene and keeps manual controls usable.

Rehearsal: show the Shelf; open Tian Shi Hong He An; read; enter setup; choose a
subtitle group and inspect its resources; confirm and hold the returned detail
result; undo and close visibly. About 17 seconds,
using the same interaction handlers and components as the live prototype.
Use once playback with an explicit replay, keeping the final context visible.
Film the actual motion at desktop and 390px, plus reduced motion and interruption.

## Footer color revision, November 13, 2026

Keep the existing detail and separate follow-setup composition. Pair a white
secondary action with a quiet gray-green edge and text with a muted slate-blue
primary action. The primary fill is #46647C; hover deepens the same hue.
Apply this local action palette to both footers so entering follow setup keeps
the visual relationship. Retain the shared button geometry and pointer feedback.
This is a prototype color exploration; the production design tokens are unchanged.

## Work-first detail study, November 13, 2026

The user's next direction is to understand the work first, then either follow
it or return comfortably to discovery. Explore that direction in the existing
Shelf prototype before deriving production design rules.

Give the real poster a 250px desktop column and let a 28px title lead the
adjacent reading column. The owner rejected the blue-teal button color.
Try a neutral charcoal action (#333333), a white outlined return action, white
paper, and slate ink. The artwork supplies the visual character. Keep the toolbar
quiet; label it `番剧详情` and retain a visible close control with an Escape
tooltip. Return to the shelf to choose another work; omit adjacent-work
navigation and its counter. Keep short descriptions compact rather than
stretching the panel.

Align the footer with the reading columns: an outlined `返回选片` action below the
poster, and a primary `选择字幕组` action at the end of the reading path. Explain
that the next step chooses a subtitle group. Keep follow confirmation on its
existing separate setup page. Preserve real pointer feedback, keyboard focus,
reduced motion, synopsis scroll, and shelf position. At narrow widths, pair a
96px cover with the title, then use the full width for reading and retain both
actions. This remains an exploratory proposal, not an approved DESIGN.md update.

## Detail typography trial, November 14, 2026

Use the same Segoe UI / Microsoft YaHei UI font stack throughout the detail
panel. Four desktop type sizes express content roles: 28px for the work title,
16px with 28px leading for reading, 14px for interface headings and actions,
and 12px with 18px leading for supporting information. Use only regular and
semibold weights. Compact titles use 20px; reading text retains its desktop size.

Use three neutral text roles: strong #242424, body #414141 and muted #6B6B6B.
White on charcoal is the inverse action role; green still denotes a followed
work. Keep both footer actions at the same type size and weight. Give the
empty synopsis the supporting role and simplify the new-follow helper to
`选好字幕组后，再确认追番`. The confirmed-follow result still identifies the saved
group and offers undo. This is a local typography trial, pending owner review.

The close control uses body ink at rest and strong ink on hover. Its fill
uses neutral #F5F5F5 and #EBEBEB, replacing the remaining gray-green surface.
Its keyboard focus ring uses the muted text role.

## Shelf typography and color trial, November 14, 2026

Shelf and detail now reference the same font, type sizes and neutral ink roles.
Use 28px / 600 for the desktop page heading, 14px for navigation, search,
filters and poster titles, and 12px for schedule metadata, counts and hints.
Compact page headings use 20px; controls and poster titles retain 14px.
Reading prose remains 16px in detail. Use only 400 and 600 weights.

Use #242424 for titles and selection, #414141 for interface text, and #6B6B6B
for supporting text. Neutral white and gray surfaces replace the shelf's
blue-gray navigation and filters. Hover and keyboard focus stay visible;
#287250 identifies followed works, matching detail. Artwork supplies color.
This trial covers the Shelf homepage, not the alternate Daily/Cinema concepts,
the subtitle-group setup page or the production native application.

`node scripts/export-detail.mjs --shelf` exports the actual interactive Shelf
without opening a detail panel. The existing detail export remains available.

## Followed status color revision, November 14, 2026

Replace the green followed state with warm amber. Green reads as a system
success confirmation, while amber reads as saved or collected and remains
distinct from the neutral interface. Use #7A4E10 for status text, an ivory
#FFF8ECF2 pill on posters, and a subtle #EAD9BC inset edge. The detail metadata
uses the same amber text without a pill. Keep the existing type scale and
poster badge dimensions. This is a local prototype color revision, not a
production design-token change.

## Follow-status filter revision, November 14, 2026

Keep the unfollowed filter, but separate it from weekday navigation. Weekdays
choose the content range; follow status is a secondary view of the resulting
works. Move the control into the result row, rename the action to `只看未追番`,
and express it as a checkbox with a checked state instead of ambient text.

Compose follow status with weekday filters and search. Searching must not clear
the filter or silently bypass it. Keep the control available in compact layouts
rather than hiding the feature. If the combination produces no works, name the
filter as the cause and offer a direct way to show all works. Do not add an
`已追番` shelf mode: followed works already have a home in `我的追番`.
Do not repeat a per-card `查看详情` instruction: the poster is the entry point,
its hover treatment confirms that affordance, and the footer carries only the
broadcast day. The followed badge already communicates saved status.
Keep the functional season selector as the single source of season context.
Remove the decorative English season eyebrow and do not repeat the season in the
result row. Show the count as a complete phrase in the same toolbar row as the
weekday navigation, aligned with `只看未追番` rather than sitting alone under the
tabs: `共 N 部番剧` in the unfiltered and weekday-filtered states, and
`找到 N 部番剧` while a query is active. Do not repeat a selected weekday in
the result row or on every poster; keep broadcast days on cards only when all
days are shown or search crosses the week. Use `没有找到相关番剧` when a query
has no results.
This remains a local interaction prototype, not a production design-token or
native-application change.

## Shelf action and count revision, November 14, 2026

The owner found `19 部` incomplete and visually stranded at the left edge of a
separate result row. Combine weekday navigation, the result count and
`只看未追番` into one filter toolbar. Keep the count right-aligned with the
follow filter and phrase it as `共 19 部番剧`, `共 1 部番剧` or
`找到 N 部番剧`. In the compact layout, keep the days on their own line and
place the count and follow filter on the line below, aligned right.

The owner found `了解这部` indirect and the full-width charcoal hover strip out
of step with the neutral Shelf. Replace it with a compact white chip,
`查看番剧详情` followed by a chevron. The chip sits near the bottom of the
artwork, while the cover dims slightly and scales 1.5% on pointer hover or
keyboard focus. This confirms the poster as the entry point without covering
the artwork with a heavy plate. Keep the chip hidden from the accessible name;
the poster title already names the destination.
