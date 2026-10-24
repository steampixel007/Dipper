# Window lifecycle verification

Verified on Windows on 2026-10-25. The GUI subsystem's behavior is specified in
[gui.md](subsystems/gui.md#closing-background-work-and-quitting); the dialog's
[offline preview](assets/ui/08-close-window.png) uses deterministic sample data.

## Automated checks

- `scripts/build.ps1 -Test -CrossCompile -Render` passed: the complete
  `go test -count=1 -timeout 5m ./...` suite, `go vet ./...`, the Windows build,
  Linux amd64 and macOS amd64/arm64 cross-compiles, and offline rendering.
- `gofmt -l cmd internal` printed nothing; `git diff --check` passed.
- The sandbox rejected loopback sockets with `connectex: An attempt was made
  to access a socket in a way forbidden by its access permissions`. The full
  suite was therefore run outside that sandbox against its local fixtures.
  The installed Go 1.27.1 toolchain and cached dependencies were reused.

## Regression evidence

Each mutation below was applied separately, judged by `cmd/redcheck` as a
real assertion failure, and restored. A compiler error, skip, timeout or panic
was not used as evidence. The complete window/tray test group passed again
after restoration.

| Mutation | Test | Observed assertion |
| --- | --- | --- |
| Restore the old immediate-hide/exit close handler | `TestWindowCloseAsksAndCancelsWithoutSideEffects` | Missing close dialog |
| Ignore the remember checkbox | `TestWindowCloseChoicesRememberOnlyWhenChecked` | After restart `ask`, wanted `background` |
| Omit the selected action from saved preferences | `TestWindowCloseChoicesRememberOnlyWhenChecked` | After restart `ask`, wanted `background` |
| Ignore the action during startup loading | `TestWindowCloseChoicesRememberOnlyWhenChecked` | After restart `ask`, wanted `background` |
| Trust tray creation without shell registration | `TestTrayRequiresShellRegistrationBeforeHiding` | Unregistered tray was not unavailable/cleaned up |
| Reuse a tray after losing registration | `TestTrayRecoversLostRegistrationAndKeepsExistingIcon` | Lost registration was not replaced |
| Ignore a failed settings write while closing | `TestWindowCloseSaveFailureKeepsWindowAndPreference` | Failed write completed the close |
| Remove the native macOS close policy | `TestWindowCloseMacKeepsAppWithoutRequiringTray` | Close did not keep the app running |
| Ignore the settings-page selection | `TestWindowSettingsCanChangeAndResetRememberedBehavior` | Background choice was not saved |
| Omit the action while saving the other preferences | `TestWindowCloseChoicePreservesOtherPreferences` | Close/appearance/intro/source round trip was not preserved |

Run the final green group with:

```sh
go run ./cmd/redcheck -pkg ./cmd/dipper -test '^(TestWindow|TestTray)' -expect pass
```

## Quit-save failure recovery

`TestWindowTrayQuitSaveFailureRestoresWindowAndRepaints` first hides the window,
then blocks the temporary preferences path with a directory. A tray-style quit
must keep the process running, restore the window through the queued `Update`,
and repaint the save error. After the storage obstruction is removed, retrying
must quit without scheduling failure recovery. Both tray-triggered quit and
the application quit event use the same recovery helper.

The test passed with the fix. `cmd/redcheck` classified each separate mutation
as a real assertion failure:

| Mutation | Observed assertion |
| --- | --- |
| Restore the previous `exitApp` | A vetoed tray quit must schedule a repaint |
| Remove window restoration | A vetoed tray quit must restore the hidden window |
| Restore without scheduling `Update` | A vetoed tray quit must schedule a repaint |

The entire window/tray test group passed after restoring the fix. The full
`scripts/build.ps1 -Test -CrossCompile` checks were rerun for the repair. Native
tray interaction under an actual full-disk condition was not rerun; the
regression uses a deterministic local filesystem write failure and the queued
window-update boundary.

## Native Windows observations

The built executable used a separate data and download directory under
`.cache/` and an HTTP server bound to `127.0.0.1`, serving the captured catalog
fixture. The user's application data was not used.

- The title-bar close button displayed the choice dialog with remember
  unchecked. Escape dismissed it without closing the window.
- Choosing background with remember checked wrote `CloseBehavior: background`,
  hid the window, and left the process running.
- Starting the executable again with the same data directory restored and
  focused the existing window, rather than creating another owner.
- Settings displayed the remembered choice. Selecting ask changed the saved
  value to `ask` without moving the settings scroll position.
- The separate exit button stopped the process and removed `instance.json`.
- After a full exit and restart, the saved background action applied to the
  title-bar close button without another dialog.
- The Windows shell registration lookup succeeded at startup and before
  hiding; the application logged `tray ready`.

## Not run

- Physical mouse interaction with the tray icon and its context menu: the
  desktop tool did not expose the taskbar notification area as a target. The
  click-to-open binding is implemented, but native registration alone does not
  prove that interaction.
- Actual Explorer restart, registration failure, and light/dark taskbar changes.
  Unit tests cover registration loss/failure and appearance selection.
- Native macOS Dock, menu bar, and Command-Q behavior; native Linux panel
  behavior. Cross-compilation and policy tests do not prove these interactions.
- A torrent transferring while hidden; native lifecycle checks used no active
  torrent tasks.
