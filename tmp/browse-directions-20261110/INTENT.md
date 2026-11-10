# Browse direction study

Medium: three interactive desktop UI studies in a local comparison page. These
are design simulations using Dipper's bundled catalog and posters; they never
change subscriptions or start downloads. The native Go application is outside
this study.

The focal object is the anime, followed by the decision to choose a version.
Existing sidebar navigation stays visually quiet. Source management, download
rules, settings, fabricated ratings and invented streaming availability are
omitted.

1. Shelf: scan a generous poster library, filter it, then inspect a title in a
   side sheet without losing the scroll position.
2. Daily: choose a weekday, scan a compact list, and compare the selected title
   in a persistent detail column.
3. Cinema: choose one title at a time from an artwork rail, then choose a
   version in a focused dialog.

All directions complete the same simulated version-selection flow. Selection
must precede following. Search, weekday changes, dismissal and undo remain
interactive. Narrow layouts recompose the content rather than shrink text.

Starting gold: stageshow `examples/static-card`, copied unchanged except its
relative kit import. Its Window / Stage / Row composition is the framing
reference. Final controls reuse the kit, restyled for Dipper and Windows.

Delivery sequence: film the gold, build three directions, exercise their flows
and narrow layouts, capture the outcomes, then record verification boundaries.
