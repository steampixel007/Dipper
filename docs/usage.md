# Using Dipper

[README](../README.md) · [Development](development.md)

## Launch

On Windows, double-click **`bin/dipper.exe`** after building. By default the program stores application state under the current user's LocalAppData directory; it needs no browser, Node, or external downloader. Pass `--data PATH` to use a single application-state directory during development.

An unsigned local build may first show the Windows SmartScreen "未知发布者" (Unknown publisher) prompt. The single-instance check runs only after you choose to continue; official distribution should use code signing to remove this prompt.

On first launch the app reads the current season. Choose an anime → fansub group → click title words to set include/exclude conditions → confirm the selection → download and follow. Existing matching releases are all checked by default; unchecked entries persist across restarts. After saving, RSS is checked every 15 minutes, with backoff retries on failure; you can also check immediately.

"我的追番" (My follows) can pause, resume, and edit rules; the edit page can also backfill existing releases on demand. Editing rules affects only future unread entries, and backfill selections do not change the saved rules. Pausing a follow and pausing a download are two separate operations.

The Downloads page shows real progress plus total and per-task upload and download speeds, and supports pause, resume, retry, open folder, and stop seeding. All tasks share the upload limit, 1 MiB/s by default (labeled MB/s in the UI). After completion, seeding stops once cumulative upload/download reaches 1:1 or effective seeding reaches 1 hour.

On Windows and Linux, closing the window asks whether to keep following and downloading in the tray or quit. The optional "记住我的选择" (Remember my choice) checkbox saves the action; Settings can change it or reset it to ask each time. If the tray is unavailable, the window stays open and offers an explicit exit. On macOS, closing keeps the app running and clicking its Dock icon reopens the window; Quit or Command-Q stops background activity. Settings and the tray menu also provide "退出 Dipper" (Quit Dipper).

Only one main process runs per data directory; launching the program again brings back the existing window and restores it when minimized. The single-instance check runs before database and download engine initialization, and a failed wake-up shows an error instead of continuing to contend for the state files.

## Files and state

Default application directories (where `$XDG_*_HOME` is set to an absolute path on Linux, it replaces the corresponding fallback):

| Platform | Persistent data (`dipper.db`, `engine/`, `instance.lock`, `instance.json`) | UI settings (`preferences.json`) | Cover cache (`covers/`) | Logs (`app.log`) |
| --- | --- | --- | --- | --- |
| Windows | `%LOCALAPPDATA%/Dipper/` | Same data directory | `%LOCALAPPDATA%/Dipper/Cache/covers/` | `%LOCALAPPDATA%/Dipper/Logs/` |
| macOS | `~/Library/Application Support/Dipper/` | Same data directory | `~/Library/Caches/Dipper/covers/` | `~/Library/Logs/Dipper/` |
| Linux | `$XDG_DATA_HOME/dipper/` (default `~/.local/share/dipper/`) | `$XDG_CONFIG_HOME/dipper/` (default `~/.config/dipper/`) | `$XDG_CACHE_HOME/dipper/covers/` (default `~/.cache/dipper/covers/`) | `$XDG_STATE_HOME/dipper/` (default `~/.local/state/dipper/`) |

`dipper.db` holds follow rules, read/skipped entries, fixed anime directories and download submissions. `engine/` holds BT metadata, piece verification records, pause/stop intents and transfer checkpoints. The instance files prevent concurrent access and forward a later launch to the existing window. An abnormal exit releases the system lock. Explicit `--data PATH` keeps all these files in one directory: `dipper.db`, `engine/`, `preferences.json`, `covers/` and `app.log`. The previous executable-adjacent `data/` directory is not read automatically; use `--data ./data` to run with that directory.

- Default video directory: the `Dipper` folder under the system user's "视频" (Videos) directory (Windows uses the video path actually configured in the system, macOS uses `~/Movies/Dipper`, Linux follows `XDG_VIDEOS_DIR`), changeable in Settings. The old default `Downloads/Dipper` is updated to the new default after a restart; custom paths and the fixed directories of subscribed anime are kept.

The same anime keeps the directory created on its first fansub group across groups; changing the global root affects only new anime. Different anime with the same name use different directories. Each BT release gets its own hash subdirectory that preserves the original file names and directory structure inside the torrent.
