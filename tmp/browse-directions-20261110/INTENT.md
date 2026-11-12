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
`继续逛逛` and `追这部`, without group or version controls. The latter opens a
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
