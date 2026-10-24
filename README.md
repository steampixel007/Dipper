# Dipper

A native anime-following desktop app built on MyGo Native UI + Go. It connects to
the real Mikan catalog, historical releases, RSS, and the built-in BT downloader.
The name is the white-throated dipper (河乌), the songbird that dives into rushing
mountain torrents — a nod to the BitTorrent kind of torrent.

## Launch

On Windows, double-click **`bin/dipper.exe`** after building. With the repository layout (`bin/` beside `data/`), the program uses `data/` by default; it needs no browser, Node, or external downloader.

An unsigned local build may first show the Windows SmartScreen "未知发布者" (Unknown publisher) prompt. The single-instance check runs only after you choose to continue; official distribution should use code signing to remove this prompt.

On first launch the app reads the current season. Choose an anime → fansub group → click title words to set include/exclude conditions → confirm the selection → download and follow. Existing matching releases are all checked by default; unchecked entries persist across restarts. After saving, RSS is checked every 15 minutes, with backoff retries on failure; you can also check immediately.

"我的追番" (My follows) can pause, resume, and edit rules; the edit page can also backfill existing releases on demand. Editing rules affects only future unread entries, and backfill selections do not change the saved rules. Pausing a follow and pausing a download are two separate operations.

The Downloads page shows real progress plus total and per-task upload and download speeds, and supports pause, resume, retry, open folder, and stop seeding. All tasks share the upload limit, 1 MiB/s by default (labeled MB/s in the UI). After completion, seeding stops once cumulative upload/download reaches 1:1 or effective seeding reaches 1 hour.

Closing the window sends it to the tray; choosing "完全退出" (Quit completely) in Settings or the tray stops all background activity. If tray creation fails, closing the window exits directly so the process cannot become unreachable.

Only one main process runs per data directory; launching the program again brings back the existing window and restores it when minimized. The single-instance check runs before database and download engine initialization, and a failed wake-up shows an error instead of continuing to contend for the state files.

## Files and state

- `data/dipper.db`: SQLite — follow rules, read/skipped entries, fixed anime directories, download submission records.
- `data/engine/`: BT metadata, piece verification records, pause/stop intents, and transfer checkpoints.
- `data/preferences.json`: UI preferences.
- `data/covers/`: poster cache; `data/app.log`: local logs.
- `data/instance.lock` / `data/instance.json`: single-instance ownership and the local window wake-up channel; after an abnormal exit the system releases the lock, so the next launch can recover.
- Default video directory: the `Dipper` folder under the system user's "视频" (Videos) directory (Windows uses the video path actually configured in the system, macOS uses `~/Movies/Dipper`, Linux follows `XDG_VIDEOS_DIR`), changeable in Settings. The old default `Downloads/Dipper` is updated to the new default after a restart; custom paths and the fixed directories of subscribed anime are kept.

The same anime keeps the directory created on its first fansub group across groups; changing the global root affects only new anime. Different anime with the same name use different directories. Each BT release gets its own hash subdirectory that preserves the original file names and directory structure inside the torrent.

## Development

The interface design system is [DESIGN.md](DESIGN.md). The Go version and
third-party dependencies are pinned in `go.mod` / `go.sum`. On Windows:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -Test -Render
```

Add `-CrossCompile` to build bare executables for Linux amd64 and macOS arm64/amd64. On macOS and Linux, run the same POSIX shell script with `/bin/sh`:

```sh
sh scripts/build.sh --test --render
```

The native output is `bin/dipper`; run it as `./bin/dipper`. Add `--cross-compile` to also build Linux amd64, macOS arm64/amd64, and Windows amd64 binaries. Cross-compilation verifies that the code builds, while launching or rendering requires MyGo's system graphics and tray runtime dependencies on the host.

Brand icons: `go run build/icon/gen.go all build/icon/dist` regenerates the
tray cuts (light and dark), the tile app icon, and `dipper.ico` from the SVG
masters in `build/icon/`. The generator is dependency-free and embeds the same
geometry as the masters. The app embeds its own copies — refresh them with
`go run build/icon/gen.go app 256 cmd/dipper/assets/dipper-icon.png` and
`go run build/icon/gen.go tray 32 cmd/dipper/assets/dipper-tray.png` and
`go run build/icon/gen.go tray -dark 32 cmd/dipper/assets/dipper-tray-dark.png`;
the in-app vector mark is a copy of `dipper-small.svg` at
`cmd/dipper/assets/dipper.svg`. The Windows executables carry the icon as a
resource: `go run build/icon/gen.go syso amd64 cmd/dipper/dipper_windows_amd64.syso`
(and `arm64`) embeds it, so the taskbar and Explorer show the bird even before
a window sets its own icon.

Development diagnostic flags:

```powershell
./bin/dipper.exe --data ./data
./bin/dipper.exe --probe-source
./bin/dipper.exe --render ./docs/assets/ui
```

`--probe-source` reads only the public catalog, details, history, and RSS, and creates no downloads. `--render` draws the same native views from development test fixtures and is not evidence of public-network downloads. Use `--source URL` to point at a local source test service and `--download-root PATH` to set the initial download directory for new data; `--quit-after 8s` verifies startup and exit.

## Verification and boundaries

`go test ./...`, `go vet ./...` and `gofmt -l cmd internal` are the checks; CI runs
them on Windows, macOS and Linux. Acceptance criteria and how to prove a test
catches a bug are in [docs/code-standards.md](docs/code-standards.md).

What is not verified: native windows, the tray, input methods and install paths
on macOS and Linux need acceptance on real hardware — a cross-compile only shows
the code builds. On non-Windows platforms network availability is inferred from
an active interface and does not prove the internet is reachable. The cover cache
has no eviction policy. Unsigned Windows builds may be blocked by SmartScreen; the
"未知发布者" (Unknown publisher) prompt can only be avoided by signing with a
release certificate.
