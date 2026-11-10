# Browse Anime direction study

An interactive checkpoint of three alternatives for Dipper's Browse Anime
page. This is a standalone React prototype, separate from the native Go UI.
It does not change real follows, preferences, or downloads.

## Resume the study

From this directory:

```powershell
npm ci
npm run dev
```

Open `http://127.0.0.1:4397`. Use the direction controls at the top, or open:

- `/#shelf`: light poster library with a detail sheet.
- `/#daily`: weekday list with a persistent selected preview.
- `/#cinema`: dark spotlight with a poster rail.

Search, weekday filtering, version selection, follow confirmation, and undo
are simulated and interactive. Follow state is temporary. Bundled covers
and titles are real catalog assets; weekdays and version choices are mock
data and do not establish current availability.

## Validate

With the preview server running and Google Chrome installed:

```powershell
npm run check
npm run build
```

The last browser run passed 39 checks, and the web build passed. The browser
check writes state screenshots into `evidence/`. Dependencies and generated
`dist/` are ignored; the lockfile is included.

## Continue the discussion

The current recommendation is Shelf as the default browsing direction.
Daily explores a weekday routine; Cinema explores artwork-led discovery.
These are alternatives to compare, not an approved production design.

- `INTENT.md`: original scope and interaction goals.
- `REHEARSAL.md`: latest verification results and explicit remaining limits.
- `evidence/reviewer.md`: independent review of an earlier version, before
  the final fixes recorded in `REHEARSAL.md`.
- `evidence/*-wide.png` and `evidence/*-small.png`: final scene captures.

Some scene JSON reports record earlier film runs, before the final PNGs
were refreshed. The final gate verdicts are recorded in `REHEARSAL.md`;
those older JSON files are not current approval evidence.

Remaining discussion includes catalog density, useful show-specific metadata,
and whether either alternative should become a separate entry point. Minor
scroll-edge and spacing findings remain documented. No native GUI changes
have been made.
