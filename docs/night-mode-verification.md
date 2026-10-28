# Night mode verification

Verified on Windows on 2026-10-28, from local `main` at
`51cec66986e8220109b2a9186e84fafaa2aad198`, using Go 1.27.1.

## Behavior

Settings offers 浅色, 夜间, and 跟随系统. The selected ASCII `Appearance`
preference is saved before the native theme changes. It is restored before
window creation. Missing, empty, and unknown values retain the light default.
A failed save restores the previous choice and leaves the native theme alone.
All pages, dialogs, status washes, inputs, and selected controls share the
context's palette. Fixed bird artwork retains its pale tile.

## Automated checks

- `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build.ps1 -Test -CrossCompile`
  passed: `go test -count=1 -timeout 5m ./...`, `go vet ./...`, the Windows
  executable, and Linux amd64 / macOS arm64 / macOS amd64 builds.
- `gofmt -l cmd internal` printed nothing; `git diff --check` passed.
- `go run ./cmd/redcheck -pkg ./cmd/dipper -test '^TestAppearance' -expect pass`
  passed before mutations and again after all mutations were restored.
- `go run ./cmd/dipper --render ./docs/assets/ui` generated all eight fixture
  states in both appearances. Settings, filtering, and the close dialog were
  visually reviewed in night mode.

The eight appearance tests cover actual settings clicks, save-before-apply,
restart restoration and preservation of other preferences, absent/legacy/unknown
preferences, failed writes, disabled controls, all five pages, system appearance
changes, isolated contexts, a word popover, a confirmation dialog, and dark
text/status/action contrast of at least 4.5:1.

## Regression evidence

Each mutation below was made separately and judged with `cmd/redcheck` using
`-expect fail`. Each produced `REAL RED`, with assertion failures rather than
compile errors, skips, timeouts, or panics. All mutations were restored.

| Temporary mutation | Test | Assertion |
| --- | --- | --- |
| Restore the old forced-light render path | `TestAppearanceRenderedPagesUseDarkSurfacesAndReadableText` | `sidebar surface luminance=0.896, want dark=true` |
| Omit `Appearance` from saved preferences | `TestAppearanceSettingsClickPersistsAndAppliesAfterSave` | `restart after "夜间": appearance="light", want "dark"` |
| Omit the native theme callback | `TestAppearanceSettingsClickPersistsAndAppliesAfterSave` | `click "夜间" did not apply native appearance exactly once: []` |

## Native Windows check

The built executable was launched once for each saved mode using separate,
new directories under `bin/native-appearance-smoke/`, an unreachable loopback
source, and `--quit-after 5s`. No user's application data or OS theme settings
were changed. Each run created a visible native window, wrote readiness,
exited with code 0, logged orderly shutdown, and preserved the saved mode.

`DwmGetWindowAttribute(DWMWA_USE_IMMERSIVE_DARK_MODE)` reported true for saved
night mode, false for saved light mode, and true for system mode on the current
dark desktop. This checks native startup wiring as well as headless rendering.

Not run: interactive native switching while the OS theme changes, native
file-dialog appearance, tray hide/restore during a theme change, or native
Linux/macOS execution. Cross-compilation establishes build compatibility;
headless system-mode tests establish UI behavior without those native checks.
