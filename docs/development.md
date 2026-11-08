# Developing Dipper

[README](../README.md) · [User guide](usage.md)

Version tags publish GitHub Releases automatically; see [releasing](releasing.md)
for tag naming, downloads and failure recovery.

## Build and run

Run these commands from the repository root.

The interface design system is [DESIGN.md](../DESIGN.md). The Go version and
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

## README screenshots

The README uses the app icon in `cmd/dipper/assets/dipper-icon.png` and actual
Windows window captures in `docs/assets/screenshots/`. The screenshots were
taken on November 8, 2026, from a locally built Dipper with the public Mikan
source and a separate application data directory.

| Image | Captured state |
| --- | --- |
| [01-browse.png](assets/screenshots/01-browse.png) | The real autumn 2025 catalog, with loaded covers and source titles |
| [02-filter.png](assets/screenshots/02-filter.png) | 桜都字幕组 releases for Spy x Family season 3, filtered by 1080p and simplified Chinese; existing releases unchecked |
| [03-following.png](assets/screenshots/03-following.png) | Saved Ranma 1/2 season 2 and Spy x Family season 3 follows, both paused |

These are captures of a running native window, not `--render` output. No
historical downloads were submitted, and the isolated database contained zero
download tasks. The images do not demonstrate completed downloads or playback.

To refresh the screenshots:

1. In PowerShell, set `$env:CGO_ENABLED = "0"` and build a capture-only executable with `go build -trimpath -ldflags "-H windowsgui" -o ./bin/dipper-readme.exe ./cmd/dipper`. This keeps the installed application separate from the capture build.
2. Launch `./bin/dipper-readme.exe --data ./data/readme-capture/state --download-root ./data/readme-capture/downloads` using a fresh, isolated data directory.
3. Browse the autumn 2025 catalog and wait for covers to load. Open a title, choose a real fansub group, and select the title words to show matching releases.
4. Clear the existing release selection before saving a follow. Pause saved follows before leaving the capture session unattended.
5. Capture only the Dipper window, keeping its native window controls visible. The recorded captures used Windows `PrintWindow` with `PW_RENDERFULLCONTENT`; invisible resize borders were excluded from the captured bounds.
6. Review the images at the README's displayed width, update both README languages and this provenance note, and verify every local image reference.

The separate `docs/assets/ui/` directory remains the offline UI regression
reference. The build script's `-Render` (Windows) or `--render` (macOS/Linux)
option regenerates those fixture renders from `cmd/dipper/ui_render.go` without
overwriting the real README screenshots. Fixture titles, groups, dates and
transfer figures are illustrative. They do not establish live source access or
public-network downloads.

| Fixture render | Offline reference |
| --- | --- |
| [01-browse.png](assets/ui/01-browse.png) | Seasonal catalog |
| [02-filter.png](assets/ui/02-filter.png) | Title-word selection and release preview |
| [03-following.png](assets/ui/03-following.png) | Saved follows and the episode-library entry |
| [04-downloads.png](assets/ui/04-downloads.png) | Transfer progress and controls |
| [09-anime-summary.png](assets/ui/09-anime-summary.png) | Full synopsis in a reading dialog |
| [10-group-picker.png](assets/ui/10-group-picker.png) | Subtitle-group search and release counts |
| [11-view-resources.png](assets/ui/11-view-resources.png) | Resource entry and manual backfill |
| [12-seed-settings.png](assets/ui/12-seed-settings.png) | Seeding-duration choice |
| [14-following-check-failure.png](assets/ui/14-following-check-failure.png) | Failed follow check and retry |

## Verification and boundaries

`go test ./...`, `go vet ./...` and `gofmt -l cmd internal` are the checks; CI runs
them on Windows, macOS and Linux. Acceptance criteria and how to prove a test
catches a bug are in [code standards](code-standards.md). A Linux job also runs
the suite under the race detector with cgo enabled, reaching the code paths the
CGO-free platform checks never build.

UI pixel tests render Chinese product copy using the host's text engine. Linux
needs Chinese fonts as well as Pango/Cairo; on Debian/Ubuntu install
`fonts-noto-cjk` before running tests or generating screenshots. Both Linux CI
workflows install this package. Without a font covering a character, Pango's
unknown glyph has no bitmap in MyGo, so text-presence checks can pass while
pixel-contrast checks correctly fail.

What is not verified: native windows, the tray, input methods and install paths
on macOS and Linux need acceptance on real hardware — a cross-compile only shows
the code builds. On non-Windows platforms network availability is inferred from
an active interface and does not prove the internet is reachable. Cover cache
eviction is covered by unit tests but not by long browsing sessions on real
hardware. Unsigned Windows builds may be blocked by SmartScreen; the
"未知发布者" (Unknown publisher) prompt can only be avoided by signing with a
release certificate.
