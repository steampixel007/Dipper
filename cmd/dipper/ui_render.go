package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/proxy"
	"github.com/steampixel007/Dipper/internal/source"
	"github.com/steampixel007/Dipper/internal/trackers"
)

// uiRenderFixture is used only by --render and UI tests; it never starts core or a source client.
func uiRenderFixture() *State {
	s := newState("")
	s.Year, s.Season = "2025", "春"
	s.Root = `C:\Downloads\Dipper`
	s.ProxyAuto = "跟随系统代理：http://127.0.0.1:7897"
	s.Catalog = []Anime{
		{
			ID:             "render-1",
			Title:          "星海旅记",
			Day:            1,
			Cover:          "3887.jpg",
			Summary:        "为了寻找失落的星图，少女与伙伴们踏上穿越群岛的旅途。在陌生的港口与夜空下，一段关于相遇、离别与成长的故事缓缓展开。",
			BroadcastStart: "4/7/2025",
			OfficialURL:    "https://example.invalid/star-voyage",
			BangumiURL:     "https://bgm.tv/subject/400001",
		},
		{ID: "render-2", Title: "雨后的星期天", Day: 0, Cover: "3984.jpg"},
		{ID: "render-3", Title: "向着下一站", Day: 3, Cover: "4031.jpg"},
		{ID: "render-4", Title: "花与远方", Day: 5, Cover: "4056.jpg"},
		{ID: "render-5", Title: "夏日来信", Day: 6, Cover: "4057.jpg"},
		{ID: "render-6", Title: "月下的约定", Day: -1, Cover: "4058.jpg"},
	}
	for _, a := range s.Catalog {
		s.KnownAnime[a.ID] = a
	}
	s.AnimeID, s.Group = s.Catalog[0].ID, 0
	s.Groups = []string{"回归测试组甲", "回归测试组乙", "回归测试组丙", "回归测试组丁"}
	s.GroupIDs = []string{"render-group-1", "render-group-2", "render-group-3", "render-group-4"}
	s.GroupCounts = []int{3, -1, 0, 12}
	s.ResourceList = []Resource{
		{ID: "render-resource-1", Title: "[回归测试组甲] 星海旅记 - 01 [1080p][简日内嵌][HEVC].mkv", Episode: "01", Published: "2025-04-07 21:30", MB: 486, Tokens: []string{"1080p", "简日内嵌", "HEVC"}},
		{ID: "render-resource-2", Title: "[回归测试组甲] 星海旅记 - 02 [1080p][简日内嵌][HEVC].mkv", Episode: "02", Published: "2025-04-14 21:30", Tokens: []string{"1080p", "简日内嵌", "HEVC"}},
		{ID: "render-resource-3", Title: "[回归测试组甲] 星海旅记 - 02 [720p][CHS][AVC].mp4", Episode: "02", Published: "2025-04-14 22:00", MB: 236, Tokens: []string{"720p", "CHS", "AVC"}},
	}
	s.initSelection()
	folder := filepath.Join(s.Root, s.Catalog[0].Title)
	s.Subscriptions = []Subscription{
		{ID: "render-sub-1", AnimeID: s.AnimeID, GroupID: s.GroupIDs[0], GroupName: s.Groups[0], Folder: folder, Rule: Rule{All: []string{"1080p"}}, LastCheck: "2025-04-14 22:01，新增 1 条"},
		{ID: "render-sub-2", AnimeID: s.AnimeID, GroupID: s.GroupIDs[1], GroupName: s.Groups[1], Folder: folder, LastCheck: "2025-04-14 22:02"},
	}
	s.Downloads = []Download{
		{ID: "render-download-1", AnimeID: s.AnimeID, ResourceID: s.ResourceList[0].ID, Title: s.ResourceList[0].Title, Episode: s.ResourceList[0].Episode, Folder: folder, MB: 486, Progress: .43, DownSpeed: 2.7, UpSpeed: .08},
		{ID: "render-download-2", AnimeID: s.AnimeID, ResourceID: s.ResourceList[1].ID, Title: s.ResourceList[1].Title, Episode: s.ResourceList[1].Episode, Folder: folder, MB: 486, Progress: .68, DownSpeed: 2.4, UpSpeed: .06},
		{ID: "render-download-3", AnimeID: s.AnimeID, ResourceID: s.ResourceList[2].ID, Title: s.ResourceList[2].Title, Episode: s.ResourceList[2].Episode, Folder: folder, MB: 236, Progress: 1, UploadedMB: 84, UpSpeed: .12, SeedSeconds: 720},
	}
	s.OpenFolder = func(string) {}
	s.OpenURL = func(string) {}
	s.PlayFile = func(string) error { return nil }
	s.CopyText = func(string) {}
	return s
}

func renderScreens(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	loadImages()
	for _, screen := range []struct {
		name, page string
		configure  func(*State)
		narrow     bool
		hover      bool
		scroll     float32
		textScale  float32
	}{
		{name: "01-browse", page: "browse"},
		{name: "09-anime-summary", page: "browse", configure: func(s *State) {
			s.Subscriptions = nil
			s.IntroHidden = true
			s.cacheDetail(s.Catalog[0].ID, "", source.Detail{
				Anime: source.Anime{
					ID: s.Catalog[0].ID, Title: s.Catalog[0].Title, Cover: s.Catalog[0].Cover,
					Summary: s.Catalog[0].Summary, BroadcastStart: s.Catalog[0].BroadcastStart,
					OfficialURL: s.Catalog[0].OfficialURL, BangumiURL: s.Catalog[0].BangumiURL,
				},
				Groups: []source.Group{
					{ID: s.GroupIDs[0], Name: s.Groups[0], Count: s.GroupCounts[0]},
					{ID: s.GroupIDs[1], Name: s.Groups[1], Count: s.GroupCounts[1]},
					{ID: s.GroupIDs[2], Name: s.Groups[2], Count: s.GroupCounts[2]},
					{ID: s.GroupIDs[3], Name: s.Groups[3], Count: s.GroupCounts[3]},
				},
			})
			s.openBrowseDetail(s.Catalog[0])
		}},
		{name: "16-browse-detail-narrow", page: "browse", narrow: true, textScale: 2, configure: func(s *State) {
			s.IntroHidden = true
			s.cacheDetail(s.Catalog[0].ID, "", source.Detail{
				Anime: source.Anime{
					ID: s.Catalog[0].ID, Title: s.Catalog[0].Title, Cover: s.Catalog[0].Cover,
					Summary: s.Catalog[0].Summary, BroadcastStart: s.Catalog[0].BroadcastStart,
					OfficialURL: s.Catalog[0].OfficialURL, BangumiURL: s.Catalog[0].BangumiURL,
				},
				Groups: []source.Group{
					{ID: s.GroupIDs[0], Name: s.Groups[0], Count: s.GroupCounts[0]},
					{ID: s.GroupIDs[1], Name: s.Groups[1], Count: s.GroupCounts[1]},
					{ID: s.GroupIDs[2], Name: s.Groups[2], Count: s.GroupCounts[2]},
					{ID: s.GroupIDs[3], Name: s.Groups[3], Count: s.GroupCounts[3]},
				},
			})
			s.openBrowseDetail(s.Catalog[0])
		}},
		{name: "17-browse-hover", page: "browse", hover: true, configure: func(s *State) { s.IntroHidden = true }},
		{name: "02-filter", page: "detail", configure: func(s *State) {
			s.Subscriptions, s.Downloads = nil, nil
			s.ResourceList = append(s.ResourceList, Resource{
				ID: "render-resource-1v2", Title: "[回归测试组甲] 星海旅记 - 01v2 [1080p][简日内嵌][HEVC].mkv",
				Episode: "01", Published: "2025-04-08 12:00", MB: 492, Tokens: []string{"1080p", "简日内嵌", "HEVC"},
			})
			s.setupVersions()
			s.Platform, s.CustomWindowControls = "windows", true
			s.WindowIsMaximized = func() bool { return false }
		}},
		{name: "22-original-title", page: "detail", configure: func(s *State) {
			s.Subscriptions, s.Downloads = nil, nil
			s.setupVersions()
			s.switchVersionMode(true)
			s.ActiveTitleRow, s.ActiveTitleWord = s.ResourceList[0].ID, "简日内嵌"
		}},
		{name: "03-following", page: "following"},
		{name: "14-following-check-failure", page: "following", configure: func(s *State) {
			s.Subscriptions[1].CheckFailed = true
			s.Subscriptions[1].LastError = "RSS 请求超时"
		}},
		{name: "04-downloads", page: "downloads"},
		{name: "05-settings", page: "settings"},
		{name: "06-source-error", page: "browse", configure: func(s *State) { s.SourceError = "无法连接番剧来源，请检查网络后重试。" }},
		{name: "07-backfill", page: "detail", configure: func(s *State) {
			s.Editing, s.EditID = true, s.Subscriptions[0].ID
			s.Rule = cloneRule(s.Subscriptions[0].Rule)
			s.beginBackfill()
			s.Checked[s.ResourceList[0].ID] = true
		}},
		{name: "11-view-resources", page: "detail", configure: func(s *State) {
			s.EditID = s.Subscriptions[0].ID
			s.Backfill = true
			s.ViewResources = true
			s.DetailReturn = "following"
			s.Rule = cloneRule(s.Subscriptions[0].Rule)
			s.Checked = map[string]bool{}
		}},
		{name: "13-episode-library", page: "episodes", configure: func(s *State) {
			s.LibraryID = s.Subscriptions[0].ID
			s.Downloads = append(s.Downloads, Download{
				ID: "render-download-4", AnimeID: s.AnimeID, Title: "[回归测试组甲] 星海旅记 [1080p][特典].mkv",
				Folder: s.Downloads[0].Folder, MB: 120, Progress: 1,
			})
		}},
		{name: "08-close-window", page: "browse", configure: func(s *State) {
			s.Platform, s.CustomWindowControls = "windows", true
			s.TrayAvailable = true
			s.ClosePrompt = true
			s.WindowIsMaximized = func() bool { return false }
		}},
		{name: "10-group-picker", page: "detail", configure: func(s *State) {
			s.Subscriptions = nil
			s.openBrowseDetail(s.Catalog[0])
			s.DetailReturn = browseDetailReturn
			s.Group = -1
			s.ResourceList = nil
			s.Checked = nil
		}},
		{name: "12-seed-settings", page: "settings", scroll: 480, configure: func(s *State) { s.Seed, s.TransferLimits.duration.value = "30 分钟", "30" }},
		{name: "19-custom-proxy", page: "settings", scroll: 600, configure: func(s *State) {
			s.ProxyMode = proxyModeIndex(proxy.ModeCustom)
		}},
		{name: "18-tracker-manager", page: "settings", configure: func(s *State) {
			s.ExtraTrackers = "udp://tracker.example.net:1337/announce"
			s.TrackerLists = []core.TrackerList{{
				URL: "https://lists.example.net/trackers.txt", Trackers: []string{"https://tracker.example.net/announce"},
				FetchedAt: time.Date(2026, time.November, 22, 8, 0, 0, 0, time.Local),
			}}
			s.trackerAutomatic = trackers.List{URLs: []string{"udp://recommended.example.net:1337/announce"}}
			s.openTrackerManager()
		}},
		{name: "20-tracker-recommended-addresses", page: "settings", narrow: true, configure: func(s *State) {
			s.trackerAutomatic = trackers.List{URLs: []string{"udp://recommended.example.net:1337/announce"}}
			s.openTrackerManager()
			s.trackerManager.selectList(automaticTrackerList)
		}},
		{name: "21-tracker-deleted-addresses", page: "settings", narrow: true, configure: func(s *State) {
			s.trackerAutomatic = trackers.List{URLs: []string{"udp://recommended.example.net:1337/announce"}}
			s.DisabledTrackers = s.trackerAutomatic.URLs
			s.openTrackerManager()
			s.trackerManager.selectList(automaticTrackerList)
			s.trackerManager.showRemoved = true
		}},
	} {
		for _, appearance := range []string{appearanceLight, appearanceDark} {
			s := uiRenderFixture()
			s.Page, s.Appearance = screen.page, appearance
			if screen.configure != nil {
				screen.configure(s)
			}
			width, height := 1320, 860
			if screen.narrow {
				width, height = 1040, 720
			}
			tt := ui.NewTester(s.view, width, height)
			if screen.textScale != 0 {
				tt.SetPreferences(ui.Preferences{TextScale: screen.textScale, ReduceMotion: true})
			}
			tt.Advance(500 * time.Millisecond)
			if screen.scroll != 0 {
				tt.Scroll(float32(width)*.6, float32(height)/2, 0, screen.scroll)
			}
			if screen.hover {
				poster, found := tt.Find("查看 " + s.Catalog[0].Title)
				if !found || poster.W <= 0 || poster.H <= 0 {
					return fmt.Errorf("render %s: poster has no rendered bounds", screen.name)
				}
				tt.Move(poster.X+poster.W/2, poster.Y+poster.H/2)
				tt.Advance(200 * time.Millisecond)
			}
			name := screen.name
			if appearance == appearanceDark {
				name += "-dark"
			}
			path := filepath.Join(dir, name+".png")
			if err := writeUIScreen(path, tt); err != nil {
				return fmt.Errorf("render %s: %w", name, err)
			}
		}
	}
	return nil
}

func writeUIScreen(path string, tt *ui.Tester) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, tt.Image())
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
