package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

var uiTestImages sync.Once

func uiFixture(t *testing.T, page string) *State {
	t.Helper()
	uiTestImages.Do(loadImages)
	s := uiRenderFixture()
	s.Page = page
	s.ReduceEffects = true
	if s.Core != nil || s.Engine != nil || s.Source != nil {
		t.Fatal("UI fixtures must not start core, a downloader, or a network source")
	}
	return s
}

func uiClick(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("click %q: %v; texts=%q", label, err, tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
}

func uiRequireText(t *testing.T, tt *ui.Tester, texts ...string) {
	t.Helper()
	for _, text := range texts {
		if !tt.HasText(text) {
			t.Errorf("missing %q; texts=%q", text, tt.Texts())
		}
	}
}

func TestUIViewPagesAndWindowSizes(t *testing.T) {
	for _, size := range [][2]int{{1120, 760}, {1320, 860}, {1560, 980}} {
		for _, page := range []struct{ id, text string }{
			{"browse", "2025 年春季番剧"}, {"detail", "资源预览"}, {"following", "最近 RSS 检查"}, {"downloads", "总下载速度"}, {"settings", "上传与分享"},
		} {
			t.Run(fmt.Sprintf("%s/%dx%d", page.id, size[0], size[1]), func(t *testing.T) {
				s := uiFixture(t, page.id)
				tt := ui.NewTester(s.view, size[0], size[1])
				uiRequireText(t, tt, page.text)
				for _, text := range tt.Texts() {
					if strings.Contains(text, "原型") || strings.Contains(text, "模拟") || strings.Contains(text, "演示") || strings.Contains(text, "样例") || strings.Contains(text, "快照") {
						t.Errorf("product page exposes development UI: %q", text)
					}
				}
				image := tt.Image()
				if image.Bounds().Dx() != size[0] || image.Bounds().Dy() != size[1] {
					t.Fatalf("wrong rendered size: %v", image.Bounds())
				}
			})
		}
	}
}

func TestUIBrowseOlderSeasonSearchAndUnknownDay(t *testing.T) {
	s := uiFixture(t, "browse")
	s.Year = "1999"
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "1999 年春季番剧", s.Catalog[0].Title, "更新时间未知")
	years := catalogYears(s.Year)
	if !slices.Contains(years, "2000") || !slices.Contains(years, "1999") || !slices.Contains(years, strconv.Itoa(time.Now().Year())) {
		t.Fatalf("year selector omitted current or historical year: %v", years)
	}
	uiClick(t, tt, "搜索番剧")
	tt.Type("星海")
	uiRequireText(t, tt, s.Catalog[0].Title)
	if tt.HasText(s.Catalog[1].Title) {
		t.Fatal("search did not filter catalog")
	}
	uiClick(t, tt, "上一季")
	if s.Year != "1999" || s.Season != "冬" {
		t.Fatalf("previous season: %s %s", s.Year, s.Season)
	}
	uiClick(t, tt, "上一季")
	if s.Year != "1998" || s.Season != "秋" {
		t.Fatalf("season did not cross year: %s %s", s.Year, s.Season)
	}
	uiClick(t, tt, "回到当季")
	if s.Year != strconv.Itoa(time.Now().Year()) || s.Season != seasons[(int(time.Now().Month())-1)/3] || s.Search != "" || s.Day != -1 {
		t.Fatal("current season must use the clock and clear filters")
	}
}

func TestUICatalogLoadingAndFailureHideStaleCatalog(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			s := uiFixture(t, "browse")
			if failed {
				s.SourceError = "来源返回 HTTP 503"
			} else {
				s.SourceLoading = true
			}
			tt := ui.NewTester(s.view, 1320, 860)
			if failed {
				uiRequireText(t, tt, "来源返回 HTTP 503", "重试加载")
			} else {
				uiRequireText(t, tt, "正在加载番剧目录")
			}
			if tt.HasText(s.Catalog[0].Title) {
				t.Fatal("stale catalog must not be selectable during load or failure")
			}
		})
	}
}

func TestUIDynamicGroupsAndEmptyGroups(t *testing.T) {
	s := uiFixture(t, "detail")
	s.Group = -1
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, s.Groups[3])
	uiClick(t, tt, "选择字幕组 "+s.Groups[3])
	if s.Group != 3 {
		t.Fatal("fourth source group was not selected")
	}
	s.Groups, s.GroupIDs = nil, nil
	tt.Frame()
	uiRequireText(t, tt, "暂无字幕组")
	uiClick(t, tt, "开启自动追番")
	if s.Confirm {
		t.Fatal("cannot follow with missing groups")
	}
}

func TestUIDetailUnavailableDisablesFollowAndConfirmation(t *testing.T) {
	for _, state := range []struct {
		name, text string
		configure  func(*State)
	}{
		{"detail-loading", "正在加载字幕组", func(s *State) { s.DetailLoading = true }},
		{"detail-error", "字幕组请求失败", func(s *State) { s.DetailError = "字幕组请求失败" }},
		{"history-loading", "正在加载发布记录", func(s *State) { s.HistoryLoading = true }},
		{"history-error", "RSS 发布记录不可用", func(s *State) { s.HistoryError = "RSS 发布记录不可用" }},
		{"stale-group", "先在左侧选择", func(s *State) { s.Group = 99 }},
		{"missing-id", "先在左侧选择", func(s *State) { s.GroupIDs = nil }},
	} {
		t.Run(state.name, func(t *testing.T) {
			s := uiFixture(t, "detail")
			state.configure(s)
			tt := ui.NewTester(s.view, 1320, 860)
			uiRequireText(t, tt, state.text)
			if tt.HasText(s.ResourceList[0].Title) {
				t.Fatal("unavailable detail must hide stale resource choices")
			}
			uiClick(t, tt, "开启自动追番")
			if s.Confirm {
				t.Fatal("unavailable detail opened a follow confirmation")
			}
			s.Confirm = true
			tt.Frame()
			uiRequireText(t, tt, "尚未就绪")
			uiClick(t, tt, "确认开启自动追番")
			if !s.Confirm || s.Busy {
				t.Fatal("unavailable confirmation submitted an operation")
			}
		})
	}
}

func TestUIRawResourcesUnknownSizeAndFilterUndo(t *testing.T) {
	s := uiFixture(t, "detail")
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, s.ResourceList[0].Title, s.ResourceList[0].Published, "已知大小 722 MB，另有 1 条大小未知")
	uiClick(t, tt, "筛选 1080p")
	uiClick(t, tt, "只要含「1080p」的")
	uiRequireText(t, tt, "匹配 2 条", "已知大小 486 MB，另有 1 条大小未知")
	uiClick(t, tt, "撤销上一步")
	uiRequireText(t, tt, "匹配 3 条")
	uiClick(t, tt, "重做筛选")
	uiClick(t, tt, "下载并追番")
	uiRequireText(t, tt, "确认开始追番", "这次下载 2 条 · 已知大小 486 MB，另有 1 条大小未知")
	uiClick(t, tt, "返回调整")
	if s.Confirm {
		t.Fatal("confirmation did not close")
	}
}

func TestUIUnknownResourceSizeNeverInventsTotals(t *testing.T) {
	s := uiFixture(t, "detail")
	s.ResourceList = s.ResourceList[1:2]
	s.ResourceList[0].MB = -1
	s.ResourceList[0].Published = ""
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "大小未知", "发布时间未知", "1 条大小未知")
	for _, text := range tt.Texts() {
		if strings.Contains(text, "0 MB") || strings.Contains(text, "-1 MB") {
			t.Errorf("unknown size rendered as measured bytes: %q", text)
		}
	}
	uiClick(t, tt, "下载并追番")
	uiRequireText(t, tt, "这次下载 1 条 · 1 条大小未知")
}

func TestUIBusyBlocksSaveAndFollow(t *testing.T) {
	for _, editing := range []bool{false, true} {
		t.Run(fmt.Sprint(editing), func(t *testing.T) {
			s := uiFixture(t, "detail")
			s.Busy, s.Editing = true, editing
			tt := ui.NewTester(s.view, 1320, 860)
			label := "下载并追番"
			if editing {
				label = "保存追番条件"
				uiClick(t, tt, "补下已有资源")
				if s.Backfill {
					t.Fatal("busy edit entered backfill")
				}
			}
			uiClick(t, tt, label)
			if s.Confirm || s.SaveError != "" || s.Page != "detail" {
				t.Fatal("busy action changed state")
			}
			s.Confirm = true
			tt.Frame()
			uiClick(t, tt, "下载 3 条并追番")
			if !s.Confirm || s.SaveError != "" {
				t.Fatal("busy confirmation submitted")
			}
		})
	}
}

func TestUIBackfillEntryRequiresExplicitSelectionAndKeepsSavedRule(t *testing.T) {
	s := uiFixture(t, "detail")
	s.Editing = true
	s.EditID = s.Subscriptions[0].ID
	s.Rule = cloneRule(s.Subscriptions[0].Rule)
	savedRule := cloneRule(s.Subscriptions[0].Rule)
	editID := s.EditID
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "补下已有资源")
	if !s.Backfill || s.Editing || s.EditID != editID {
		t.Fatalf("wrong backfill state: backfill=%v editing=%v id=%q", s.Backfill, s.Editing, s.EditID)
	}
	for id, checked := range s.Checked {
		if checked {
			t.Fatalf("backfill automatically selected %s", id)
		}
	}
	uiRequireText(t, tt, "浏览番剧 / 补下已有资源", "请勾选需要补下的已有资源", "补下选中资源", "不会修改已保存的追番条件")
	if tt.HasText("开始追番") || tt.HasText("默认全选") {
		t.Fatal("backfill must not present itself as a new subscription")
	}
	uiClick(t, tt, "补下选中资源")
	if s.Confirm {
		t.Fatal("empty backfill opened confirmation")
	}
	s.Confirm = true
	tt.Frame()
	uiRequireText(t, tt, "确认补下已有资源", "尚未选择需要补下的资源")
	uiClick(t, tt, "确认补下选中资源")
	if !s.Confirm || s.Busy {
		t.Fatal("empty backfill submitted")
	}
	uiClick(t, tt, "返回调整")
	uiClick(t, tt, "选择 "+s.ResourceList[0].ID)
	uiClick(t, tt, "补下选中资源")
	if !s.Confirm {
		t.Fatal("selected backfill did not open confirmation")
	}
	uiRequireText(t, tt, "这次补下 1 条 · 486 MB", "已保存的追番条件保持不变")
	if !reflect.DeepEqual(savedRule, s.Subscriptions[0].Rule) || !reflect.DeepEqual(savedRule, s.Rule) {
		t.Fatal("entering backfill changed the saved rule")
	}
}

func TestUIFollowingUsesSubscriptionIdentityAndRSSResult(t *testing.T) {
	s := uiFixture(t, "following")
	s.Catalog = nil
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, s.KnownAnime[s.AnimeID].Title, s.Subscriptions[0].GroupName, s.Subscriptions[1].GroupName, "新增 1 条", "RSS 请求超时", "检查失败")
	first, ok := tt.Find("修改条件 " + s.Subscriptions[0].GroupName)
	second, ok2 := tt.Find("修改条件 " + s.Subscriptions[1].GroupName)
	if !ok || !ok2 || first.Y == second.Y {
		t.Fatal("same-anime subscriptions did not retain separate controls")
	}
	wantID := s.Subscriptions[1].ID
	uiClick(t, tt, "修改条件 "+s.Subscriptions[1].GroupName)
	if s.EditID != wantID || !s.Editing || s.Page != "detail" {
		t.Fatalf("edited wrong subscription: id=%q, want %q", s.EditID, wantID)
	}
}

func TestUIActionsDelegateInsteadOfChangingFixtureState(t *testing.T) {
	s := uiFixture(t, "downloads")
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "暂停全部下载")
	uiClick(t, tt, "暂停")
	uiClick(t, tt, "重试")
	uiClick(t, tt, "停止做种")
	if s.Downloads[0].Paused || !s.Downloads[1].Failed || s.Downloads[2].SeedStopped {
		t.Fatal("UI changed download state without an engine")
	}
	s.Page = "following"
	tt.Frame()
	uiClick(t, tt, "暂停追番 "+s.Subscriptions[0].GroupName)
	uiClick(t, tt, "立即检查 "+s.Subscriptions[0].GroupName)
	if s.Subscriptions[0].Paused || s.Subscriptions[0].LastCheck != "2025-04-14 22:01 · 新增 1 条" {
		t.Fatal("UI simulated a subscription operation without core")
	}
}

func TestUIDownloadsUseMeasuredSpeedsErrorsAndOpenFolder(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[:1]
	s.Downloads[0].MB = 0
	opened := ""
	s.OpenFolder = func(path string) { opened = path }
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "↓ 2.7 MB/s", "↑ 0.08 MB/s", "大小未知")
	if tt.HasText("4.2 MB/s") || tt.HasText("0 MB / 0 MB") {
		t.Fatal("download shows fabricated speed or size")
	}
	uiClick(t, tt, "打开文件夹")
	if opened != s.Downloads[0].Folder {
		t.Fatalf("opened %q, want %q", opened, s.Downloads[0].Folder)
	}
	s.Downloads[0].Failed = true
	s.Downloads[0].Error = "磁盘空间不足"
	tt.Frame()
	uiRequireText(t, tt, "磁盘空间不足", "重试")
	s.Downloads[0].Failed = false
	s.Downloads[0].Error = ""
	s.Downloads[0].Progress = 1
	tt.Frame()
	uiRequireText(t, tt, "分享比例未知", "停止做种")
}

func TestUICompletedPausedDownloadCanResume(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[2:3]
	s.Downloads[0].Paused = true
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "100%", "已暂停")
	if _, ok := tt.Find("继续"); !ok {
		t.Fatal("completed paused task must retain its resume action")
	}
	if _, ok := tt.Find("停止做种"); ok {
		t.Fatal("stop seeding must not replace resume while paused")
	}
	uiClick(t, tt, "继续")
	if !s.Downloads[0].Paused {
		t.Fatal("resume must delegate to the engine instead of changing fixture state")
	}
	s.Downloads[0].Paused = false
	tt.Frame()
	uiRequireText(t, tt, "已完成 · 做种中", "停止做种")
	if _, ok := tt.Find("继续"); ok {
		t.Fatal("resumed task still shows the paused action")
	}
}

func TestUISettingsBusyDisablesChangesAndQuit(t *testing.T) {
	s := uiFixture(t, "settings")
	s.Busy = true
	picked, quit := false, false
	s.PickFolder = func() { picked = true }
	s.Quit = func() { quit = true }
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "选择文件夹")
	if rect, ok := tt.Find("保存并完全退出"); !ok || rect.Y > 830 {
		for range 4 {
			tt.Wheel(660, 430, 0, 120)
			tt.Advance(100 * time.Millisecond)
		}
	}
	uiClick(t, tt, "保存并完全退出")
	before := s.ReduceEffects
	uiClick(t, tt, "减少动态效果")
	if picked || quit || before != s.ReduceEffects || s.SaveError != "" {
		t.Fatal("busy settings invoked an operation")
	}
}

func TestUISettingsProxyModesAndCustomAddress(t *testing.T) {
	s := uiFixture(t, "settings")
	tt := ui.NewTester(s.view, 1320, 860)
	if rect, ok := tt.Find("网络与代理"); !ok || rect.Y > 780 {
		for range 4 {
			tt.Wheel(660, 430, 0, 120)
			tt.Advance(100 * time.Millisecond)
		}
	}
	uiRequireText(t, tt, "网络与代理", "代理", "跟随系统代理：http://127.0.0.1:7897", "自动", "关闭", "自定义")

	uiClick(t, tt, "关闭")
	if s.ProxyMode != 1 || s.ProxyURL != "" {
		t.Fatalf("direct mode = %d, address = %q", s.ProxyMode, s.ProxyURL)
	}

	uiClick(t, tt, "自定义")
	if s.ProxyMode != 2 {
		t.Fatalf("custom mode = %d", s.ProxyMode)
	}
	uiRequireText(t, tt, "代理地址")
	if rect, ok := tt.Find("代理地址"); ok {
		tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	}
	tt.Type("127.0.0.1:7897")
	if s.ProxyURL != "127.0.0.1:7897" {
		t.Fatalf("typed address = %q", s.ProxyURL)
	}
}

func TestUIRenderScreensOffline(t *testing.T) {
	dir := t.TempDir()
	if err := renderScreens(dir); err != nil {
		t.Fatal(err)
	}
	var previous []byte
	for _, name := range []string{"01-browse", "02-filter", "03-following", "04-downloads", "05-settings", "06-source-error", "07-backfill"} {
		data, err := os.ReadFile(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != 1320 || cfg.Height != 860 {
			t.Fatalf("invalid %s image: %+v %v", name, cfg, err)
		}
		if bytes.Equal(previous, data) {
			t.Fatalf("%s repeated the previous page", name)
		}
		previous = data
	}
}

// countDark counts pixels below a luminance of 120 in a region of the last
// frame; inkOnly keeps only near-neutral dark pixels, which the blue of the
// letter fallback and of the ripple never matches.
func countDark(tt *ui.Tester, x0, y0, x1, y1 int, inkOnly bool) int {
	img := tt.Image()
	b := img.Bounds()
	n := 0
	for y := max(y0, b.Min.Y); y < min(y1, b.Max.Y); y++ {
		for x := max(x0, b.Min.X); x < min(x1, b.Max.X); x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			r, g, bl := int(cr>>8), int(cg>>8), int(cb>>8)
			lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)
			spread := max(r, g, bl) - min(r, g, bl)
			if lum < 120 && (!inkOnly || spread < 40) {
				n++
			}
		}
	}
	return n
}

// The bird mark replaced the letter-D placeholders: no bare "D" text node
// remains, the sidebar chip is the bird on a wash rather than a solid ink
// square, and a cover-less card shows the bird, not a blue letter.
func TestUIBrandMarkReplacesLetterPlaceholders(t *testing.T) {
	s := uiFixture(t, "browse")
	s.Catalog[0].Cover = ""
	tt := ui.NewTester(s.view, 1320, 860)
	if slices.Contains(tt.Texts(), "D") {
		t.Errorf("bare D placeholder still rendered; texts=%q", tt.Texts())
	}
	word, ok := tt.Find("Dipper")
	if !ok {
		t.Fatal("wordmark missing")
	}
	if chip := countDark(tt, int(word.X)-42, int(word.Y)-6, int(word.X)-10, int(word.Y+word.H)+6, false); chip < 30 || chip > 400 {
		t.Errorf("sidebar chip dark pixels = %d; want the bird on a wash, not a solid square (30-400)", chip)
	}
	title, ok := tt.Find("星海旅记")
	if !ok {
		t.Fatal("cover-less card title missing")
	}
	if n := countDark(tt, int(title.X)-20, int(title.Y)-210, int(title.X)+140, int(title.Y)-8, true); n < 30 {
		t.Errorf("cover fallback ink pixels = %d; want the bird mark, not a blue letter (>=30)", n)
	}
}

// A cover whose load is in flight shows the ripple indicator: the card says it
// is loading and the bird fallback waits for a cover that truly has none.
func TestUICoverInFlightShowsLoadingIndicator(t *testing.T) {
	s := uiFixture(t, "browse")
	url := s.Catalog[0].Cover
	saved := covers[url]
	delete(covers, url)
	loadingCovers[url] = true
	t.Cleanup(func() {
		delete(loadingCovers, url)
		if saved != nil {
			covers[url] = saved
		}
	})
	tt := ui.NewTester(s.view, 1320, 860)
	if _, ok := tt.Find("正在加载封面"); !ok {
		t.Fatal("in-flight cover shows no loading indicator")
	}
	title, ok := tt.Find("星海旅记")
	if !ok {
		t.Fatal("card title missing")
	}
	if n := countDark(tt, int(title.X)-20, int(title.Y)-210, int(title.X)+140, int(title.Y)-8, true); n != 0 {
		t.Errorf("bird fallback painted over a loading cover (%d ink pixels)", n)
	}
}
