<div align="center">

<img src="cmd/dipper/assets/dipper-icon.png" width="88" height="88" alt="Dipper app icon">

# Dipper

**English** &nbsp;·&nbsp; [简体中文](README.zh-CN.md)

Dipper is a desktop app for following anime. Browse Mikan by season, pick a fansub group and version, and Dipper's built-in downloader fetches new matches while the app is running.

[![Checks](https://github.com/steampixel007/Dipper/actions/workflows/check.yml/badge.svg)](https://github.com/steampixel007/Dipper/actions/workflows/check.yml)
[![Go](https://img.shields.io/badge/Go-1.27.3-3b63fb?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-3b63fb.svg)](LICENSE)
[![Native UI](https://img.shields.io/badge/UI-MyGo_Native-3b63fb)](https://github.com/egoist/mygo)

**[Download Dipper](https://github.com/steampixel007/Dipper/releases/latest)** &nbsp;·&nbsp; [Get started](#get-started) &nbsp;·&nbsp; [User guide](docs/usage.md) &nbsp;·&nbsp; [Report an issue](https://github.com/steampixel007/Dipper/issues)

<a href="docs/assets/screenshots/01-browse.png">
  <img src="docs/assets/screenshots/01-browse.png" width="960" alt="Dipper's browse page with anime covers, season selection, title search, and weekday filters">
</a>

<sub>Windows screenshot of the fall 2025 Mikan catalog.</sub>

</div>

## Get started

Open the [download page](https://github.com/steampixel007/Dipper/releases/latest), expand **Assets**, and look for a filename ending in the suffix for your computer.

| Your computer | Filename suffix | How to open Dipper |
| --- | --- | --- |
| Windows x64 (Intel or AMD) | `_windows_amd64_setup.exe` | Run the installer, then open Dipper from the Start menu. |
| Mac with Apple silicon | `_darwin_arm64.tar.gz` | Extract the archive, move Dipper to Applications, and open it there. |
| Mac with an Intel processor | `_darwin_amd64.dmg` | Open the DMG, drag Dipper to Applications, and open it there. |
| Debian or Ubuntu x64 | `_linux_amd64.deb` | Install the package, then open Dipper from the applications menu. |

Dipper's interface is in Chinese. On a Mac, check your chip under **Apple menu → About This Mac**. For install-free packages, first-launch warnings, or in-app updates, see [Installation](docs/usage.md#install) and [Software updates](docs/usage.md#software-updates).

## Follow a series

1. Open **浏览番剧** (Browse) to find a series by season or weekday, or search the selected season by title.
2. Choose a fansub group. Click a word in a release title, such as `1080p`, and choose **只要** (include), **不要** (exclude), or **不限** (any). The matching list updates as you go.
3. Leave the releases you want now checked and click **下载并追番** (Download and follow). Unchecking a release skips that download and keeps your rule for future releases.

<a href="docs/assets/screenshots/02-filter.png">
  <img src="docs/assets/screenshots/02-filter.png" width="960" alt="Filtering Spy x Family season 3 releases to 1080p and simplified Chinese">
</a>

Dipper checks for new releases every 15 minutes while it runs and downloads the ones that match your rule. You can also check manually or download older releases later.

## Watch downloaded episodes

In **我的追番** (My Follows), click a series poster to open its episode library. Click **播放** (Play) on a completed download to open it in your system's media player. Downloads go to the `Dipper` folder under Videos by default. You can change that location in Settings.

<a href="docs/assets/screenshots/03-following.png">
  <img src="docs/assets/screenshots/03-following.png" width="960" alt="My Follows on Windows, showing posters for Ranma 1/2 and Spy x Family">
</a>

On Windows and Linux, you can close the window and leave Dipper running in the tray. On macOS, closing the window leaves it running in the Dock. Quitting Dipper stops checks and downloads. [Manage follows and downloads](docs/usage.md#manage-follows-and-downloads)

## Development

Dipper uses Go and [MyGo Native UI](https://github.com/egoist/mygo). Clone the repository and run these commands from its root with the Go version in [go.mod](go.mod).

| Platform | Build and test | Run |
| --- | --- | --- |
| Windows (PowerShell) | `powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -Test` | `.\bin\dipper.exe` |
| macOS or Linux | `sh scripts/build.sh --test` | `./bin/dipper` |

[Development guide](docs/development.md) · [Architecture](docs/architecture.md) · [Subsystems](docs/subsystems/README.md) · [Design](DESIGN.md) · [Contributing](CONTRIBUTING.md) · [Build checks](https://github.com/steampixel007/Dipper/actions/workflows/check.yml)

## About the name

Dipper is named after the white-throated dipper (河乌), a small songbird that dives into rushing mountain streams. Here, the torrents bring back anime.

## License

Dipper's original code and documentation are under the [MIT License](LICENSE). Keep the copyright and license notices when redistributing. The license does not grant trademark rights to the Dipper name or logo. See the [third-party notices](THIRD-PARTY.md) for third-party components and Mikan sample data and artwork.

Contributions require a Developer Certificate of Origin sign-off. See [Contributing](CONTRIBUTING.md).
