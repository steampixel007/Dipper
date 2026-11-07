# Using Dipper

[README](../README.md) · [Development](development.md)

## Install

Open the repository's [Releases page](https://github.com/steampixel007/Dipper/releases/latest)
and expand **Assets**. Download an application package for your computer;
GitHub's **Source code** archives require development tools and are not the app.
The filenames below were checked against release `v0.1.2` on November 6, 2026.
For a later release, use that release's version number.

| Platform | Recommended package | Archive alternative |
| --- | --- | --- |
| Windows x64 (Intel / AMD) | `dipper_0.1.2_windows_amd64_setup.exe` | `dipper_0.1.2_windows_amd64.zip` |
| macOS, Apple silicon (M series) | `dipper_0.1.2_darwin_arm64.dmg` | `dipper_0.1.2_darwin_arm64.tar.gz` |
| macOS, Intel | `dipper_0.1.2_darwin_amd64.dmg` | `dipper_0.1.2_darwin_amd64.tar.gz` |
| Debian / Ubuntu Linux x64 | `dipper_0.1.2_linux_amd64.deb` | `dipper_0.1.2_linux_amd64.tar.gz` |

**Windows:** run the setup EXE, then open Dipper from the Start menu. The
installer is for the current user. Alternatively, extract the ZIP and open
`Dipper.exe` from the extracted folder.

**macOS:** check **Apple menu → About This Mac** for the chip or processor
([Apple's guide](https://support.apple.com/en-us/116943)).
Open the matching DMG, drag `Dipper.app` to Applications, then open it there.
The archive alternative also contains `Dipper.app`.

**Debian / Ubuntu Linux x64:** from the folder containing the downloaded DEB,
install it with the package manager, then open Dipper from the applications menu:

```sh
sudo apt install ./dipper_0.1.2_linux_amd64.deb
```

The DEB declares its GTK3 and Ayatana AppIndicator dependencies. For other
distributions, the Linux tarball contains the executable `dipper`; extract it,
install the corresponding GTK3 and Ayatana AppIndicator runtime packages for
your distribution, and run `./dipper`. A tarball does not install those
dependencies or establish compatibility with every distribution.

`SHA256SUMS` on the release page lists the application downloads' hashes.
See [release downloads](releasing.md#downloads) for verification commands.

Windows packages have no publisher certificate and may show a SmartScreen
warning. macOS packages use ad-hoc signing, without a Developer ID certificate
or Apple notarization. If macOS blocks an app you have verified and trust,
follow [Apple's guidance for opening apps](https://support.apple.com/en-us/102445).
These instructions do not establish that the packages pass operating-system
trust checks. Clean-machine installation, native macOS/Linux first launch,
and the full live download-to-playback journey were not validated in this
documentation update.

To compile Dipper or use diagnostic flags, see [Development](development.md).

## Follow a series and watch

The interface is currently in Chinese. Dipper needs no browser, Node.js, or
separate downloader to run. Playback uses your system's associated media player.

1. **Browse:** on first launch, **浏览番剧** loads the current season. Browse by
   season and weekday, or search the selected season by title.
2. **Choose a group:** open a series and select a fansub group to see its releases.
3. **Filter:** click title words and choose **只要** (include), **不要** (exclude),
   or **不限** (any). Preview the matching releases and leave the ones you want
   to download now selected. Existing matches are selected by default; clearing
   a release skips that release, without changing the future matching rule.
4. **Download and follow:** keep the releases you want selected, then choose
   **下载并追番** (Download and follow). While Dipper is running, it checks for
   updates every 15 minutes, with backoff retries on failure. You can also
   check for updates manually.
5. **Open the episode library:** in **我的追番** (My Follows), click the series
   poster. The library lists its downloads and their states.
6. **Play:** choose **播放** beside a completed download. Dipper opens a media
   file in your associated player; install or associate a player if needed.
   A release may contain multiple files, which can also be opened from its folder.

## Manage follows and downloads

Use the episode library's **···** menu to view resources, edit conditions,
pause or resume following, check for updates, or open the series folder.
Viewing resources lets you download older releases without changing the saved
rule. Editing conditions affects future unread entries. Pausing a follow and
pausing a download are separate operations.

The Downloads page shows progress plus total and per-task transfer speeds,
and supports pause, resume, retry, open folder, and stop sharing. It lists
waiting, downloading, paused, failed, incomplete stopped, and sharing tasks.
Once a completed task stops sharing, automatically or manually, it disappears
from this page. Its files and download record remain available in the episode
library under My Follows. An episode's More menu opens its file list for
individual playback, or the confirmation dialog for deletion and redownload.
Failed tasks stay visible for retry.

All tasks share one upload limit. The default is 1,048,576 bytes/s (1 MiB/s).
The current interface labels it `1 MiB/s`. Transfer-speed displays use binary
`KiB/s`, `MiB/s`, and `GiB/s`; file and resource sizes use decimal `KB`, `MB`,
and `GB`. Release `v0.1.2` displayed `1 MB/s` for the same binary rate, so older
screenshots do not use the current unit labels.

After a download completes, it shares data with other peers. By default,
sharing stops when either the cumulative uploaded amount reaches the downloaded
amount (1:1), or effective sharing time reaches one hour, whichever comes first.
Effective time counts while the completed task can share; pauses, sleep, detected
offline periods, and errors do not count. It is not time spent actively uploading.
For imported files with no recorded downloaded bytes, the engine uses the
content size for the ratio.

Settings can change the time and ratio limits. Disabling one limit leaves the
other in effect; disabling both allows sharing until you stop it.

## Background operation

On Windows and Linux, closing the window asks whether to keep following and downloading in the tray or quit. The optional "记住我的选择" (Remember my choice) checkbox saves the action; Settings can change it or reset it to ask each time. If the tray is unavailable, the window stays open and offers an explicit exit. On macOS, closing keeps the app running and clicking its Dock icon reopens the window; Quit or Command-Q stops background activity. Settings and the tray menu also provide "退出 Dipper" (Quit Dipper).

Only one main process runs per data directory; launching the program again brings back the existing window and restores it when minimized. The single-instance check runs before database and download engine initialization, and a failed wake-up shows an error instead of continuing to contend for the state files.

## Appearance

In Settings, under "外观与动效", choose "浅色" (Light), "夜间" (Night), or
"跟随系统" (Follow system). The theme changes immediately and is remembered
across restarts. Follow system switches with the desktop's application theme.
Existing installations keep the light theme until you choose another mode.
"减少动态效果" remains a separate option. The tray icon follows the taskbar's
appearance so it stays legible even when the app uses a different theme.

## Files and state

Default application directories (where `$XDG_*_HOME` is set to an absolute path on Linux, it replaces the corresponding fallback):

| Platform | Persistent data (`dipper.db`, `engine/`, `instance.lock`, `instance.json`) | UI settings (`preferences.json`) | Cover cache (`covers/`) | Logs (`app.log`) |
| --- | --- | --- | --- | --- |
| Windows | `%LOCALAPPDATA%/Dipper/` | Same data directory | `%LOCALAPPDATA%/Dipper/Cache/covers/` | `%LOCALAPPDATA%/Dipper/Logs/` |
| macOS | `~/Library/Application Support/Dipper/` | Same data directory | `~/Library/Caches/Dipper/covers/` | `~/Library/Logs/Dipper/` |
| Linux | `$XDG_DATA_HOME/dipper/` (default `~/.local/share/dipper/`) | `$XDG_CONFIG_HOME/dipper/` (default `~/.config/dipper/`) | `$XDG_CACHE_HOME/dipper/covers/` (default `~/.cache/dipper/covers/`) | `$XDG_STATE_HOME/dipper/` (default `~/.local/state/dipper/`) |

`dipper.db` holds follow rules, read/skipped entries, fixed anime directories and download submissions. `engine/` holds BT metadata, piece verification records, pause/stop intents and transfer checkpoints. The instance files prevent concurrent access and forward a later launch to the existing window. An abnormal exit releases the system lock. Explicit `--data PATH` keeps all these files in one directory: `dipper.db`, `engine/`, `preferences.json`, `covers/` and `app.log`. The previous executable-adjacent `data/` directory is not read automatically; use `--data ./data` to run with that directory.

- Default video directory: the `Dipper` folder under the system user's "视频" (Videos) directory (Windows uses the video path actually configured in the system, macOS uses `~/Movies/Dipper`, Linux follows `XDG_VIDEOS_DIR`), changeable in Settings. The old default `Downloads/Dipper` is updated to the new default after a restart; custom paths and the fixed directories of subscribed anime are kept.

The same anime keeps the directory created on its first fansub group across groups; changing the global root affects only new anime. Different anime with the same name use different directories.

New downloads use `download-root/anime-title/fansub-group/`. Episodes from the same group share that folder. A single-file torrent puts its original file directly there; a multi-file torrent keeps its original release folder and subtitle tree. Names that would collide use readable suffixes such as `Group [2]`. The saved group directory stays fixed across restarts and later name changes.

Existing tasks keep their saved destinations, including older hash subdirectories. Retrying or deleting and downloading those tasks again uses the same saved path; this change does not move existing files. Deleting one download removes only its recorded files and leaves other episodes and unrelated files in the shared folder. A new release that conflicts with an existing file is reported as an error rather than overwriting that file.
