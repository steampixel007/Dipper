package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"
)

// uiRenderFixture is used only by --render and UI tests; it never starts core or a source client.
func uiRenderFixture() *State {
	s := newState("")
	s.Year, s.Season = "2025", "春"
	s.Root = `C:\Downloads\Dipper`
	s.ProxyAuto = "跟随系统代理：http://127.0.0.1:7897"
	s.Catalog = []Anime{
		{ID: "render-1", Title: "星海旅记", Day: 1, Cover: "3887.jpg", Summary: "为了寻找失落的星图，少女与伙伴们踏上穿越群岛的旅途。在陌生的港口与夜空下，一段关于相遇、离别与成长的故事缓缓展开。"},
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
		{ID: "render-sub-1", AnimeID: s.AnimeID, GroupID: s.GroupIDs[0], GroupName: s.Groups[0], Folder: folder, Rule: Rule{All: []string{"1080p"}}, LastCheck: "2025-04-14 22:01 · 新增 1 条"},
		{ID: "render-sub-2", AnimeID: s.AnimeID, GroupID: s.GroupIDs[1], GroupName: s.Groups[1], Folder: folder, CheckFailed: true, LastCheck: "2025-04-14 22:02 · RSS 请求超时"},
	}
	s.Downloads = []Download{
		{ID: "render-download-1", AnimeID: s.AnimeID, Title: s.ResourceList[0].Title, Folder: folder, MB: 486, Progress: .43, DownSpeed: 2.7, UpSpeed: .08},
		{ID: "render-download-2", AnimeID: s.AnimeID, Title: s.ResourceList[1].Title, Folder: folder, Progress: .12, Failed: true, Error: "连接超时，请重试"},
		{ID: "render-download-3", AnimeID: s.AnimeID, Title: s.ResourceList[2].Title, Folder: folder, MB: 236, Progress: 1, UploadedMB: 84, UpSpeed: .12, SeedSeconds: 720},
	}
	s.OpenFolder = func(string) {}
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
	}{
		{name: "01-browse", page: "browse"},
		{name: "02-filter", page: "detail", configure: func(s *State) { s.setWord("1080p", wordWant) }},
		{name: "03-following", page: "following"},
		{name: "04-downloads", page: "downloads"},
		{name: "05-settings", page: "settings"},
		{name: "06-source-error", page: "browse", configure: func(s *State) { s.SourceError = "无法连接番剧来源，请检查网络后重试。" }},
		{name: "07-backfill", page: "detail", configure: func(s *State) {
			s.Editing, s.EditID = true, s.Subscriptions[0].ID
			s.Rule = cloneRule(s.Subscriptions[0].Rule)
			s.beginBackfill()
			s.Checked[s.ResourceList[0].ID] = true
		}},
		{name: "08-close-window", page: "browse", configure: func(s *State) {
			s.Platform = "windows"
			s.TrayAvailable = true
			s.ClosePrompt = true
		}},
		{name: "09-anime-summary", page: "detail", configure: func(s *State) { s.SummaryOpen = true }},
		{name: "10-group-picker", page: "detail", configure: func(s *State) { s.ChoosingGroup = true }},
	} {
		for _, appearance := range []string{appearanceLight, appearanceDark} {
			s := uiRenderFixture()
			s.Page, s.Appearance = screen.page, appearance
			if screen.configure != nil {
				screen.configure(s)
			}
			tt := ui.NewTester(s.view, 1320, 860)
			tt.Advance(500 * time.Millisecond)
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
