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

- `/#shelf`: light poster library with a centered detail panel.
- `/?anime=4031#shelf`: open the revised detail panel directly.
- `/?capture=shelf&rehearse`: one-shot discovery walkthrough with replay.
- `/#daily`: weekday list with a persistent selected preview.
- `/#cinema`: dark spotlight with a poster rail.

Search, weekday filtering, subtitle-group selection, follow confirmation, and undo
are simulated and interactive. Follow state is temporary. Bundled covers
and titles are real catalog assets; weekdays, groups, and resource previews are mock
data and do not establish current availability.

Shelf detail introduces the work without group or version controls. `追这部`
opens a separate setup page: choose a subtitle group, inspect its attributed
resource previews, then confirm. Returning without confirmation preserves the
synopsis scroll and creates no follow. Confirmation returns to the detail with
the result and undo. Existing follows use `追番设置`; group updates do not add
another follow. Search covers the whole local season while text is
entered; clearing it restores the previous filters. Enter opens a sole result,
Ctrl+K focuses search, and Escape clears it. Previous/next controls compare works
without closing details. Closing preserves the shelf position and restores focus,
including when a newly followed work disappears from the unfollowed filter.

The cover and title stay visible while the synopsis scrolls. Compact layouts
shorten the header and retain the same footer. Manual interactions animate;
reduced motion settles the scene without disabling the controls.

## Validate

With the preview server running and Google Chrome installed:

```powershell
npm run check
npm run build
```

The latest browser run passed 110 checks (69 regression and 41 discovery checks),
and the web build passed. The browser
check writes state screenshots into `evidence/`. Dependencies and generated
`dist/` are ignored; the lockfile is included.

`node scripts/group-flow-red.mjs` temporarily injects a group selector into
detail, verifies the exact boundary assertion fails, and restores the source
in `finally`. Run separately from builds and captures against the live preview.

## Continue the discussion

The current recommendation is Shelf as the default browsing direction.
Daily explores a weekday routine; Cinema explores artwork-led discovery.
These are alternatives to compare, not an approved production design.

- `INTENT.md`: original scope and interaction goals.
- `REHEARSAL.md`: latest verification results and explicit remaining limits.
- `evidence/reviewer.md`: independent review of an earlier version, before
  the final fixes recorded in `REHEARSAL.md`.
- `evidence/fresh-eye.md`: Shelf review of source and earlier frames; the current
  fixes and reviewer boundaries are recorded in `REHEARSAL.md`.
- `evidence/shelf-group-flow-wide.png` and `evidence/shelf-group-flow-390.png`:
  current separate-setup walkthroughs with matching coverage reports.
- `evidence/shelf-motion-wide.png` and `evidence/shelf-motion-390.png`: historical
  inline-version walkthroughs, superseded by the separate setup flow.
- `evidence/shelf-open-beat.png`, `shelf-confirm-beat.png`, and
  `shelf-close-beat.png`: compact state sequences around the named story marks.

Motion sampling has coverage gaps and cannot establish complete animation
approval. Reports and remaining observations are in `REHEARSAL.md`; historical
inline-version captures do not certify the corrected navigation.

Remaining discussion includes catalog density, metadata coverage beyond the
two synopsis samples, and whether either alternative should become a separate
entry point. Cursor overlap on undo and instantaneous peripheral count/label
updates remain documented. No native GUI changes
have been made.

## Detail content samples

The Shelf prototype includes two manually summarized, local examples, checked
on November 12, 2026. These are design content samples, not an implemented
metadata fetch or a complete catalog:

- `4031`: synopsis and original title from the Mikan work page:
  `https://mikanani.me/Home/Bangumi/4031`.
- `4063`: opening premise from the official episode-one introduction:
  `https://www.mahoiku.jp/story/1/`.

All other titles show an explicit missing-description message. Weekday labels
and subtitle groups and resources are still simulations. The synopsis samples establish no
current release or download availability.
