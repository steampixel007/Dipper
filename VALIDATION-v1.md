# Dipper Technical Validation Overview

- Validation dates: 2026-10-20 to 2026-10-21.
- Purpose: for the confirmed **MyGo Native UI + Go** approach, verify whether the three key risks hold.
- Status: **all three validations were actually run and produced results** (not desk research).
- Environment: Windows amd64, Go 1.27.1, MyGo v0.5.5, anacrolix/torrent v1.61.0, CGO_ENABLED=0.

| Validation item | Conclusion |
|---|---|
| Windows native interface and desktop E2E | **Pass**: external UIA can read and operate controls; closing to tray and restoring after restart both work correctly |
| BT engine rate limiting, stop, resume and seeding policy | **Pass**: shared rate limiting, upload stop, sha256-identical resume and correct policy persistence |
| Mikan history list traversal and termination condition | **Algorithm proven**: `take` is an absolute row count, and termination is returned rows ≠ take; long sample has 1522 rows |

## 1. Native Interface and Desktop E2E (Windows)

Passed:

- Windowless `ui.NewTester` tests: 6 groups all pass (clicking a term to create a rule, checkboxes, dark rendering, episode tags, directory safety, selection persistence).
- The real window is discovered by external UI Automation after launch: `ControlType.Window`, `class=MyGoWindow`.
- External reading of the full control tree: **21 Text, 33 Button, 6 CheckBox** controls, with correct Chinese term names.
- External `Invoke` on a term button → the rule row changes in real time; external `Toggle` on a checkbox → the summary goes from 6 items to 5 and is persisted to `settings.json`.
- After the window closes, **the process stays alive**, the window disappears and the log records `window hidden`.
- After restart, the saved state reads back: the deselected release is still unselected and the summary is restored to 5 items.

Not verified: tray menu interaction, full exit, macOS/Linux, animation regression, real IME.

## 2. BT Engine (anacrolix/torrent v1.61.0)

Passed (`go test -count=1 ./...` all ok):

| Item | Measured result |
|---|---|
| 3 tasks sharing a 200 KiB/s global rate limit | 73.0/73.0/73.8 KiB/s per peer, about 219.8 KiB/s total; instantaneous 159.7~240.6 KiB/s (burst and sampling fluctuation) |
| `DisallowDataDownload` | Payload growth 0 B in a 3-second window; +1,032,192 B after resuming |
| `DisallowDataUpload` (seeder) | Seeder payload growth 0 B in a 3-second window |
| `Client.Close` stops transfers | After the leecher closes, seeder growth 0 B in 3 seconds and peer connections 2→0; after the seeder closes, leecher2 growth 0 B |
| New client resumes after a partial download | 11/32 pieces → resumed to 24/32 pieces, on-disk sha256 matches the original |
| Seeding policy persistence (virtual clock) | Accumulation across restarts is neither lost nor double-counted; reaching 3600 seconds triggers automatic stop; the manual-stop flag survives restart; ratio 1.5 triggers correctly |

Two important corrections made along the way (to avoid misjudgement):

1. `torrent.TestingConfig` sets `MaxAllocPeerRequestDataPerConn` to 5 bytes and it must be restored to 1 MiB, otherwise transfers never start; the first-round timeout came from this, not from an engine defect.
2. The policy test's expected value was originally wrong (it should be 15000, not 11000); the implementation itself had no double-counting problem.

Not verified: real public-network BT, multiple seeders, a real 1-hour wait, and the effect of OS sleep/network loss on timing.

## 3. Mikan History Traversal

Proven (obtained from the site's JS + 31 rate-limited requests):

- Endpoint `GET /Home/ExpandEpisodeTable`, with only the `bangumiId`, `subtitleGroupId` and `take` parameters, and **no offset/page/cursor**.
- `take` is an **absolute row count** (not a page size); the initial value comes from the page's `data-take` (65 for 227), stepping by +50.
- Termination condition: **returned rows ≠ requested take** (including 0 rows), with HTTP always 200.
- Long sample 227/161: actually **1522 rows / 1522 unique episode ids / 0 duplicates**; `take=1522` returns 1522 (continue), `take=1523` returns 1522 (terminate).
- Short sample 4063/370: 3 rows, terminating on the first request; when the page has no expansion control, that is the full set.
- **RSS has a hard cap of 100 entries**: the long anime history has 1522 entries and RSS covers only the latest 100 ⇒ **RSS cannot be used for full historical initialization**.
- Each row's episode id is the BT infohash and can resolve torrent and magnet links.

Not proven: whether hidden/deleted entries are returned, the server-side hard cap on take, mobile endpoints, and the sorting contract.

## 4. Implications for the Tech Choices

- **`anacrolix/torrent` is usable as the primary BT engine**, but the product must implement effective seeding timing (wired to system sleep/network events) and stop-intent persistence itself.
- **Mikan integration must go through the history list** (a single large take request), with RSS used only for subsequent increments; RSS cannot replace full history.
- **Black-box desktop E2E for MyGo Native UI is feasible**: on Windows, real-window automation can be retained rather than relying only on in-process tests.
- Still to be verified: tray menu interaction, full exit, three-platform differences, animation and IME regression.
