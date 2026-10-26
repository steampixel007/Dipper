<div align="center">

<img src="cmd/dipper/assets/dipper-icon.png" width="104" height="104" alt="Dipper's white-throated dipper over blue rushing water">

# Dipper

### Pick your anime. Leave the updates to Dipper.

Browse the season, choose the version you like, and bring new episodes home.<br>
A native desktop app with automatic following and a built-in downloader.

[![Checks](https://github.com/steampixel007/Dipper/actions/workflows/check.yml/badge.svg)](https://github.com/steampixel007/Dipper/actions/workflows/check.yml)
[![Go](https://img.shields.io/badge/Go-1.27.1-3b63fb?logo=go&logoColor=white)](go.mod)
[![Native UI](https://img.shields.io/badge/UI-MyGo_Native-3b63fb)](https://github.com/egoist/mygo)

**[Get started](#get-started)** &nbsp;·&nbsp; **[User guide](docs/usage.md)** &nbsp;·&nbsp; **[Development](docs/development.md)** &nbsp;·&nbsp; **[Feedback](https://github.com/steampixel007/Dipper/issues)**

<br>

<a href="docs/assets/ui/01-browse.png">
  <img src="docs/assets/ui/01-browse.png" width="960" alt="Dipper's seasonal anime catalog, with cover artwork, season selection, search, and weekday filters">
</a>

<sub>Native interface rendered with sample data. Titles, groups, and transfer figures are illustrative.</sub>

</div>

## From finding a show to following it

Dipper keeps the whole journey in one window: the Mikan catalog, fansub
releases, version selection, and downloads to your computer. Start with a show;
there is no feed URL to paste or regular expression to write.

- **Find something to follow.** Browse by season and weekday, or search the
  current season by title.
- **Choose your version in plain words.** Pick a fansub group, then click words
  such as `1080p` or `HEVC` to include or exclude them. See matching releases
  before confirming.
- **Let new episodes arrive.** While Dipper is running, it checks your follows
  every 15 minutes and downloads new matching releases. Check immediately
  whenever you like.
- **Keep control of your files.** Pause or resume downloads, retry a failure,
  open the destination folder, and stop seeding from the same app.

Built with Go and [MyGo Native UI](https://github.com/egoist/mygo). No browser,
Node.js, or separate download client is needed to run it. The interface is
currently in Chinese.

## Pick the release you actually want

Click a title word and choose **只要** (include), **不要** (exclude), or
**不限** (any). Matching counts and release selections update beside your
choices, and undo lets you try another combination.

<a href="docs/assets/ui/02-filter.png">
  <img src="docs/assets/ui/02-filter.png" width="960" alt="Version selection with a 1080p include rule, matching release previews, and selected downloads before confirmation">
</a>

Existing matches are selected by default. Uncheck anything you do not want
this time; your future matching rule stays separate from that selection.
Later, you can fetch older releases without changing the rule.

## Follow along, without losing track

Your follows show what was checked and what needs attention. Downloads show
progress and transfer speeds, with the next action beside each task.

<table>
  <tr>
    <th width="50%">Your follows</th>
    <th width="50%">Your downloads</th>
  </tr>
  <tr>
    <td><a href="docs/assets/ui/03-following.png"><img src="docs/assets/ui/03-following.png" alt="Followed shows with fansub groups, saved rules, and last-check results"></a></td>
    <td><a href="docs/assets/ui/04-downloads.png"><img src="docs/assets/ui/04-downloads.png" alt="Download progress, upload and download speeds, retry, and seeding controls"></a></td>
  </tr>
</table>

On Windows and Linux, you can keep following and downloading in the tray when
you close the window. On macOS, closing the window keeps Dipper running in the
Dock. Quitting the app stops background activity.

## Get started

Build from source with the Go version pinned in [go.mod](go.mod).

**Windows · PowerShell**

```powershell
git clone https://github.com/steampixel007/Dipper.git
cd Dipper
powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -Test
.\bin\dipper.exe
```

**macOS / Linux · Shell**

```sh
git clone https://github.com/steampixel007/Dipper.git
cd Dipper
sh scripts/build.sh --test
./bin/dipper
```

Windows builds are unsigned and may show a SmartScreen prompt. Native graphics
and tray runtime dependencies are required on macOS and Linux; their native
desktop acceptance is still pending. See the [development guide](docs/development.md)
for build options and verification boundaries.

Once open: **choose a show → pick a fansub group → select your version →
confirm downloads and follow.** Files go to `Dipper` under your system's Videos
folder by default; change the destination in Settings.

## A little bird, a rushing torrent

Dipper is named after the white-throated dipper (河乌), a small songbird that
dives into rushing mountain streams and returns with what it came for.
Here, the torrents bring back anime.

## More about Dipper

| | |
| :-- | :-- |
| [User guide](docs/usage.md) | Following, downloads, background behavior, and where your files live |
| [Development](docs/development.md) | Build scripts, diagnostic flags, icons, and verification boundaries |
| [Design system](DESIGN.md) | The bird, colors, controls, and interaction principles |
| [Architecture](docs/architecture.md) | How the native app, source, core, and downloader fit together |
| [Subsystems](docs/subsystems/README.md) | Implementation contracts and what their tests prove |
| [Report an issue](https://github.com/steampixel007/Dipper/issues) | Bugs, questions, and feature ideas |
