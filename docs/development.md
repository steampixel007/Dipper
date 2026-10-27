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

The README uses the app icon in `cmd/dipper/assets/dipper-icon.png` and the
native interface renders in `docs/assets/ui/`. Regenerate the screenshots with
the build script's `-Render` (Windows) or `--render` (macOS/Linux) option above.
The sample titles, fansub groups, dates, and transfer figures come from
`cmd/dipper/ui_render.go`; they demonstrate the interface, not live activity.

| Image | View |
| --- | --- |
| [01-browse.png](assets/ui/01-browse.png) | Seasonal catalog; README overview |
| [02-filter.png](assets/ui/02-filter.png) | Title-word selection and release preview |
| [03-following.png](assets/ui/03-following.png) | Saved follows and check results |
| [04-downloads.png](assets/ui/04-downloads.png) | Transfer progress and controls |

Keep the README's sample-data caption when refreshing these images. Review
them at the size readers see on GitHub and keep image alternative text
descriptive when changing a view.

## Verification and boundaries

`go test ./...`, `go vet ./...` and `gofmt -l cmd internal` are the checks; CI runs
them on Windows, macOS and Linux. Acceptance criteria and how to prove a test
catches a bug are in [code standards](code-standards.md). A Linux job also runs
the suite under the race detector with cgo enabled, reaching the code paths the
CGO-free platform checks never build.

What is not verified: native windows, the tray, input methods and install paths
on macOS and Linux need acceptance on real hardware — a cross-compile only shows
the code builds. On non-Windows platforms network availability is inferred from
an active interface and does not prove the internet is reachable. The cover cache
has no eviction policy. Unsigned Windows builds may be blocked by SmartScreen; the
"未知发布者" (Unknown publisher) prompt can only be avoided by signing with a
release certificate.
