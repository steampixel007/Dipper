# Dipper V1 Product Requirements Document

- Version: V1 draft
- Date: 2026-10-19
- Status: Core product behavior confirmed; pending implementation details are listed at the end of this document

## 1. Product Positioning

Dipper is a cross-platform desktop anime-following app for users who are not familiar with RSS, BT downloaders, or regular expressions. Users can browse anime in the app, choose a fansub group, visually filter the releases that group publishes, and let the app automatically download future updates with the same rules.

The core experience of V1 is: **browse anime → view the fansub group's actual releases → click include/exclude terms in the title and preview → matching existing releases are selected by default → download and enable ongoing automatic following.** Users do not need to find RSS URLs themselves, write regular expressions, or install and configure an external downloader.

## 2. V1 Scope

### 2.1 Launch Platforms and Release Sources

- Desktop builds for Windows, macOS, and Linux; all three platforms share the same core workflow.
- The launch source is [Mikan Project](https://mikanani.me/): read its anime catalog, anime detail, fansub groups, and release lists; later updates use the RSS for the selected anime and fansub group.
- Mikan RSS provides the release title, publish time, size, and torrent link; the release detail page may provide a magnet link. Users never have to handle these addresses directly.
- When the source site is unreachable, show an explicit load failure with a retry entry; do not display it as "暂无番剧" (no anime) or "暂无更新" (no updates).

### 2.2 User Workflow

1. Launch the app; it opens by default on the anime list for the current year and season.
2. Switch year and the spring, summer, fall, or winter season to browse other seasons; open an anime's detail page.
3. View the anime's fansub groups and the releases each group actually published; choose the group to follow.
4. In that group's release list, click words to add them to "必须包含" (must include) or "不得包含" (must not include), or type text manually; the list shows the filter results and reasons immediately.
5. Review the existing releases that match. Releases are **selected by default**; users can uncheck any entry and see the release count and estimated total size before starting.
6. Start the download and save the filter conditions. The app then checks that fansub group's RSS on a schedule and automatically downloads new releases that match.
7. Closing the window leaves the app in the system tray, still checking for updates and downloading; only "完全退出" (quit completely) stops it.

### 2.3 Main Pages

- **Browse Anime**: Browse by year and season; in anime detail, view fansub groups, filter releases, and start following. Anime detail and filtering are pages within the browse flow and do not take separate main navigation slots.
- **My Follows**: View, adjust, pause, or resume saved follow tasks, and view the result of the most recent RSS check; after resuming, continue processing unread entries in the RSS.
- **Downloads**: View downloading, completed, and seeding states; the page shows live total download speed and total upload speed, each release task shows its own download and upload speed, and the page provides pause, retry, and open folder actions.
- **Settings**: Configure the download root directory, the global upload rate limit, and other options; the upload rate limit defaults to **1 MB/s** and is user-editable.

## 3. Functional Requirements

### F01 Anime Browsing and Detail

- The home page shows the current season by default; it provides a year selector, spring/summer/fall/winter switching, and previous/next season navigation. Switching across a year boundary updates the year at the same time.
- Users can browse past seasons that Mikan has indexed, rather than being limited to the current season. When a given year and season has no available data, show an explicit empty state.
- An anime card opens the detail page; the detail page shows the available anime information and its fansub group list. Returning to the list preserves the previous year, season, and browse position.
- The fansub group list shows the number of releases actually available or recent publishing activity. Metrics that cannot be obtained from the data source are not shown as definite values.

### F02 Fansub Group Release List

- After entering a fansub group, show its published releases, including the **original title**, publish time, size, and auxiliary labels such as the episode number that can be recognized from the title.
- Organize the list by episode number where possible; releases whose episode number cannot be recognized show their original title on their own and still allow filtering and downloading by title.
- Multiple distinct releases for the same episode are shown separately; they are not collapsed or merged in download eligibility because the episode number is the same.
- For anime and fansub group associations obtained from source pages, prefer the source information; do not rely on guessing the fansub group from the title.

### F03 Visual Title Filtering

- Each filter condition is a text match against the **release's original title**:
  - "必须包含" (must include): the title must contain the specified term; multiple required terms must **all** match.
  - "不得包含" (must not include): the title must not contain the specified term; matching any excluded term fails the condition.
  - "包含其中任意一个" (include any of): add multiple spellings for the same preference, for example `简日内嵌`, `CHS`; the title matching any one of them satisfies that condition group; if there are multiple groups, each group must still be satisfied.
  - When no condition is set, every release matches; when only exclusion terms are set, releases in which no excluded term appears match.
- Users can click words in a release title or suggested labels to add conditions, and can also type or delete keywords themselves; no regular expression input is required. When the user selects "按这条生成建议条件" (generate suggested conditions from this release) on a release, the app offers only candidate terms observed in that title and leaves the user to check the match preview on other releases and confirm; the episode number is not used as a default suggested term.
- Filter results update immediately as conditions change, with a per-release explanation of "符合条件" (matches) or "缺少某个必需词／包含某个排除词" (missing a required term / contains an excluded term). If the rules set required terms and the title does not contain one, that release does not match, is not downloaded automatically, and does not need to enter a pending-confirmation queue.
- If the rules **set only exclusion terms**, releases whose titles do not mention those terms also match. The UI preview must show this result clearly, so users do not mistakenly believe the app can judge what the video actually contains.
- Filtering targets the title text, not the actual content of the downloaded video. Labels recognized from the title, such as resolution and subtitles, serve reading and condition suggestions and do not silently add download restrictions the user did not save.
- Regular expressions are not a required feature for completing filtering in V1.

### F04 Filter Preview and Downloading Existing Releases

- Show live the number of matching releases, the episodes covered (when recognizable), the estimated total size, and the match reason for each release.
- When multiple releases for the same episode match, label it "本集匹配 N 条" (N releases match this episode) and help the user compare title differences; **do not block the download, and do not arbitrarily select only one**. Users can narrow the range with include/exclude terms.
- All existing matching releases are selected by default; users can uncheck them one by one. The selection state only decides **which existing releases are downloaded this time** and never silently changes the saved future filter conditions.
- The start-download action clearly shows the number of releases actually submitted and the total size; unmatched releases are not selected automatically.

### F05 RSS Automatic Following and Deduplication

- For each follow task, save the selected anime, fansub group, and filter conditions, and read the corresponding fansub group RSS on a schedule.
- Process RSS entries one by one: **entries that are unread and whose original title matches the saved rules are submitted for download**; entries that do not match the rules are also marked read and are not downloaded retroactively after the rules are adjusted. Read state survives an app restart; if submitting a download task fails, the entry stays unread for the next retry. Different BT releases for the same episode can all be downloaded.
- The same BT release is submitted only once even if it appears repeatedly in the RSS; "same episode number" is not a valid basis for deduplication.
- Show the last check time, success/failure state, and match and exclusion records of each follow task; retry is available after a network or source failure.
- When a follow task is created, the existing releases the user selected are submitted for download as selected; existing releases the user unchecked are recorded as skipped and are not downloaded automatically even if they appear in the RSS later, although the user can still choose to download them.
- Pausing a follow neither reads nor marks that subscription's RSS entries; downloads already submitted and seeding tasks keep running. After resuming, only **the unread entries still obtainable in the RSS at that moment** are read, and they are processed with the currently saved title rules; read entries and BT releases already submitted are not downloaded again.

### F06 Built-in Magnet Downloads and Anime Folders

- Built-in BT downloading handles magnet links directly; users do not need to install or configure BitComet or qBittorrent. All tasks share one upload rate limit, **1 MB/s** by default, editable in settings; both downloading and seeding after download completion are bound by this cap.
- First use provides a default download root directory that works out of the box, while also letting the user choose another root. Each anime has a folder named after the **anime name** under the root directory selected at subscription time, for example `DownloadRoot/AnimeName/`; this anime's past releases and later RSS downloads all go into this folder and are not scattered into other anime directories because of a different fansub group. When the user later changes the global download root, already-subscribed anime keep their original folders and newly subscribed anime use the new root; the settings page explains this rule and existing files are not moved automatically.
- Anime folder names are based on the source anime name, with characters the operating system does not allow sanitized. When anime share a name but are genuinely different, their directories must be distinguished to avoid an incorrect merge; later downloads for the same anime keep using the directory already created.
- Preserve the **original file names and relative directory structure** of the files inside the torrent; do not automatically rename videos or subtitles by episode number. When different releases contain files with the same name, isolate them in a separate per-release subdirectory to avoid overwriting existing content; all content still sits under the corresponding anime folder.
- A BT download may also upload while it runs. After a single release finishes downloading it keeps seeding until **its cumulative uploaded amount reaches its downloaded amount (1:1)** or **cumulative seeding reaches 1 hour after download completion**, whichever comes first; stopping seeding does not delete downloaded files. Upload volume accumulates from when that release starts downloading; seeding time counts only while the app is running, the network is available, and the task is seeding, and it counts even when no other peer is downloading at that moment. Time while a task is paused, the network is down, the system is asleep, or the app has fully quit does not count.
- Persist each release's uploaded amount and cumulative seeding time; fully quitting stops uploading immediately, and on restart seeding tasks that have not met a stop condition continue until one of the two is satisfied. Users can manually stop seeding a single release, and after a manual stop it does not resume automatically on restart.
- The download page shows **current total download speed and current total upload speed**, and each release task shows its **current download speed and current upload speed**; upload speed is still shown while seeding after download completion, and shows zero when there is no transfer. The page also shows task names, save locations, download progress, and seeding state; it supports pause, resume, and retry, and restores unfinished tasks after an app restart.
- A download failure should show an understandable reason, such as insufficient disk space, an unreachable release, or an inaccessible source.

### F07 System Tray and Cross-Platform Operation

- Clicking the window close button hides the app to the system tray, where RSS checks, downloads, and seeding continue; the tray entry can reopen the window or quit completely.
- Quitting completely stops RSS checks, downloads, and seeding; after an app restart, follow tasks, unfinished downloads, and seeding tasks that have not met a stop condition and were not manually stopped are restored.
- Windows, macOS, and Linux must each complete the main flow "install → browse → filter → download → run in tray".

### F08 Plugin Capabilities

- The desktop kernel provides a unified UI, plugin loading and lifecycle, persistent configuration, task scheduling, and data contracts between plugins.
- V1 preinstalls and enables by default: **the Mikan release source plugin, the title recognition plugin, the title filtering plugin, and the built-in download plugin**. The plugin architecture adds no first-time configuration steps for ordinary users.
- The title recognition plugin receives the original title and anime and fansub group information known from the source, and outputs auxiliary fields such as episode number, subtitle description, resolution, codec, and format where possible; when it cannot recognize something, it keeps the value unknown and keeps the original title.
- Title recognition serves display and clickable term suggestions; **automatic downloading follows the original-title matching rules the user saved**. Recognition plugins for specific fansub groups may be added later without changing the meaning of the user's filter rules.
- When a single plugin or release source errors, the UI shows the corresponding error and the user's follow rules and download records are preserved.

## 4. Key Acceptance Scenarios

1. A user switches from the current season to a past season, opens an anime's detail page, can view that anime's fansub groups and each group's releases, and returns to the original list position.
2. A user selects a fansub group, sets `1080p` and `简日内嵌` as "必须包含" (must include) and `合集` as "不得包含" (must not include); titles missing either required term or containing `合集` are immediately marked as not matching in the preview.
3. When two different magnet releases for the same episode match, both are selected by default and both can be downloaded; the preview clearly indicates that the episode has two matches. After the user adds another keyword, the list and counts update immediately.
4. All existing releases are selected by default; after the user unchecks one, that release is not downloaded this time, and later RSS checks and app restarts do not download it automatically when it reappears. Entries published after that are still judged by the saved title conditions.
5. The same magnet release appearing repeatedly in the RSS or appearing again after an app restart does not produce a second download task; releases with the same episode number but different magnets are still downloaded individually.
6. After the window is closed it can be reopened from the tray, and RSS checks and downloads keep running; quitting completely stops them, and restarting restores the tasks.
7. When the title recognition plugin cannot recognize the subtitles or episode number of a title, the user can still see the original title and filter with include/exclude terms; the actual download result is determined only by the saved rules.
8. After the user selects a download root, releases downloaded for the same anime from different fansub groups all sit under the directory named after that anime; downloaded videos and subtitles keep the names they had inside the torrent. When different anime share a name or different releases contain files with the same name, nothing is merged incorrectly and no existing file is overwritten; releases with the same name are distinguished by subdirectories.
9. After a release finishes downloading, the app quits completely at 25 minutes of cumulative seeding; nothing uploads and no time accumulates while it is closed. The next start resumes seeding and stops once the remaining 35 minutes are reached or 1:1 is hit first; downloaded files are still kept. If the user manually stopped seeding, it does not resume automatically after restart.
10. The download page shows live total download speed and total upload speed at the same time, and the task list shows each task's download and upload speed; upload speed is still visible while a release keeps seeding after its download completes, and shows zero once transfer stops.
11. When the user pauses a follow, downloads already started keep running, and two different releases are added to the RSS during the pause; on resume, if both are still in the RSS, the app downloads only the unread entries that match the current rules and marks the other as read. Further refreshes or restarts do not submit them again; if an entry is no longer in the readable RSS, V1 does not back-fill it from the fansub group's release list.
12. After the user changes the download root from A to B, new releases for already-subscribed anime still go into their existing anime folders under A; newly subscribed anime get anime folders under B, and existing files are not moved automatically.

## 5. Not Required in V1

- Requiring users to register a Mikan account, copy RSS links manually, or connect an external downloader.
- Making regular expressions a mandatory step for ordinary users configuring a follow.
- Treating "download only one version per episode" as a default rule, or forcing users to resolve conflicts between multiple releases of the same episode.
- A built-in player, watch progress sync, cloud accounts, and cross-device sync.
- Multi-source integration at launch, and a third-party plugin marketplace for ordinary users.
- Automatically re-filtering and downloading already-read RSS entries after keywords change.
- Automatically renaming videos or subtitles by episode number, or generating another set of normalized file names for a media library.

## 6. Items Pending Design or Technical Validation

The following items do not change the core behavior above and are settled during interaction design and technical planning:

- Mikan's data access method, the site's terms of use, accessibility, and a reasonable RSS check frequency; how to adapt when site pages or the RSS format change.
- Normalization strategy for case, full-width/half-width, and punctuation in title text matching, and how keyword suggestions appear under different fansub group spellings. Normalization must not silently change filter meaning such as simplified versus traditional Chinese.
- Task recovery details when space runs out, for example how to retry after the user frees disk space.
- Plugin interface versioning, cross-platform packaging, and the security and update mechanism for installing third-party plugins in the future.

## 7. Reference Resources

- [Mikan Project](https://mikanani.me/)
- [Mikan anime detail example](https://mikanime.tv/Home/Bangumi/4063): shows fansub groups, releases, and the fansub group RSS entry.
- [Fansub group RSS example](https://mikanime.tv/RSS/Bangumi?bangumiId=4063&subgroupid=370): shows titles, times, sizes, and torrent links.
- [Release detail example](https://mikanime.tv/Home/Episode/9dd7eb5c920bddd3cd46c9256decda56d3d36008): shows magnet links.
- [Jellyfin show file organization documentation](https://jellyfin.org/docs/general/server/media/shows/) and [Plex TV show file naming documentation](https://support.plex.tv/articles/naming-and-organizing-your-tv-show-files/): background on media library naming conventions; Dipper V1 keeps the original file names inside the torrent.
