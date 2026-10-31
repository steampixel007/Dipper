package main

import (
	"bytes"
	"context"
	"errors"
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

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
	"github.com/egoist/mygo/ui"
)

var uiTestImages sync.Once

func uiFixture(t *testing.T, page string) *State {
	t.Helper()
	uiTestImages.Do(loadImages)
	s := uiRenderFixture()
	s.Dir = t.TempDir()
	s.ConfigDir = s.Dir
	s.CoverDir = filepath.Join(s.Dir, "covers")
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

func uiScrollTo(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing scroll target %q", label)
	}
	bounds := tt.Image().Bounds()
	height := float32(bounds.Dy())
	if r.Y < 64 || r.Y+r.H > height {
		tt.Scroll(float32(bounds.Dx())*.6, height/2, 0, r.Y-80)
		r, ok = tt.Find(label)
		if !ok || r.Y < 64 || r.Y+r.H > height {
			t.Fatalf("scroll target %q remains outside content viewport: %+v", label, r)
		}
	}
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
	uiClick(t, tt, "搜索字幕组")
	tt.Type(s.Groups[3])
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

func TestUISortedQuickWordsByFrequency(t *testing.T) {
	s := uiFixture(t, "detail")
	tt := ui.NewTester(s.view, 1320, 860)
	words := s.quickWords()
	want := []string{"1080p", "HEVC", "简日内嵌", "720p", "AVC", "CHS"}
	if !slices.Equal(words, want) {
		t.Fatalf("quick words not sorted by frequency: %v", words)
	}
	for _, w := range want {
		if !tt.HasText(w) {
			t.Errorf("missing quick word %q", w)
		}
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

func TestUIFollowingSearchFiltersByTitle(t *testing.T) {
	s := uiFixture(t, "following")
	tt := ui.NewTester(s.view, 1320, 860)
	s.FollowSearch = "组乙"
	tt.Frame()
	uiRequireText(t, tt, "1 部追番", "清除搜索")
	if tt.HasText(s.Subscriptions[0].GroupName) {
		t.Fatal("follow search did not filter out the other subscription")
	}
	uiClick(t, tt, "清除搜索")
	if s.FollowSearch != "" {
		t.Fatal("clearing the follow search did not clear the query")
	}
	s.FollowSearch = "不存在的追番"
	tt.Frame()
	uiRequireText(t, tt, "没有找到这部追番")
	uiClick(t, tt, "清除搜索")
}

func TestUIFollowingShowsPendingAndActiveSummary(t *testing.T) {
	s := uiFixture(t, "following")
	s.Subscriptions[0].NewCount = 2
	s.Subscriptions[0].Recent = []string{"[桥接字幕组] 桥接番剧 - 03 [1080p].mkv", "[桥接字幕组] 桥接番剧 - 04 [1080p].mkv"}
	s.Subscriptions[1].Active = true
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "新资源 2 条", "最近新到：", "下载中")
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

func TestUIFollowingFailedCheckOffersInlineRetry(t *testing.T) {
	kernel, err := core.Open(filepath.Join(t.TempDir(), "dipper.db"), core.Options{
		DisableScheduler: true,
		Feed: func(context.Context, string, string) ([]core.Resource, error) {
			return nil, errors.New("RSS 请求超时")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })
	ctx := context.Background()
	res := core.Resource{ID: "retry-1", Title: "1080p", Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("ab", 20), Size: 100}
	sub, err := kernel.CreateSubscription(ctx, core.Anime{ID: "retry-anime", Title: "追番丙"}, core.Group{ID: "retry-group", Name: "字幕组丙"}, core.Rule{}, []core.Resource{res}, []string{"retry-1"})
	if err != nil && sub.ID == "" {
		t.Fatalf("subscription not committed: %v", err)
	}
	if err := kernel.Check(ctx, sub.ID); err == nil {
		t.Fatal("expected the seeded feed failure to fail the check")
	}
	s := uiFixture(t, "following")
	s.Core = kernel
	s.Engine = &downloader.Engine{}
	s.Ctx = ctx
	updates := make(chan func(), 2)
	s.Update = func(fn func()) { updates <- fn }
	snapshot, err := kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.project(snapshot, nil)
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "RSS 检查失败：RSS 请求超时")
	uiClick(t, tt, "重试检查 字幕组丙")
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for s.SaveError != "RSS 请求超时" {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatalf("save error = %q; texts=%q", s.SaveError, tt.Texts())
		}
	}
}

func TestUIFollowingActionsSurviveSearchFilter(t *testing.T) {
	kernel, err := core.Open(filepath.Join(t.TempDir(), "dipper.db"), core.Options{
		DisableScheduler: true,
		Feed:             func(context.Context, string, string) ([]core.Resource, error) { return nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })
	ctx := context.Background()
	res := core.Resource{ID: "filter-1", Title: "1080p", Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("ab", 20), Size: 100}
	subA, err := kernel.CreateSubscription(ctx, core.Anime{ID: "filter-anime-a", Title: "追番甲"}, core.Group{ID: "filter-group-a", Name: "字幕组甲"}, core.Rule{}, []core.Resource{res}, []string{"filter-1"})
	if err != nil && subA.ID == "" {
		t.Fatalf("first subscription not committed: %v", err)
	}
	subB, err := kernel.CreateSubscription(ctx, core.Anime{ID: "filter-anime-b", Title: "追番乙"}, core.Group{ID: "filter-group-b", Name: "字幕组乙"}, core.Rule{}, []core.Resource{res}, []string{"filter-1"})
	if err != nil && subB.ID == "" {
		t.Fatalf("second subscription not committed: %v", err)
	}
	s := uiFixture(t, "following")
	s.Core = kernel
	s.Engine = &downloader.Engine{}
	s.Ctx = ctx
	// Keep action results off the action goroutine, as the window does.
	updates := make(chan func(), 2)
	s.Update = func(fn func()) { updates <- fn }
	snapshot, err := kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.project(snapshot, nil)
	s.FollowSearch = "字幕组乙"
	tt := ui.NewTester(s.view, 1320, 860)

	uiClick(t, tt, "修改条件 字幕组乙")
	if s.EditID != subB.ID || !s.Editing || s.Page != "detail" {
		t.Fatalf("edit after filter picked id=%q, want %q", s.EditID, subB.ID)
	}

	s.Page = "following"
	tt.Frame()
	uiClick(t, tt, "查看资源 字幕组乙")
	if s.EditID != subB.ID || !s.Backfill || !s.ViewResources || s.Page != "detail" {
		t.Fatalf("view after filter picked id=%q, want %q", s.EditID, subB.ID)
	}

	s.Page = "following"
	tt.Frame()
	uiClick(t, tt, "暂停追番 字幕组乙")
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for s.Notice != "追番状态已更新" {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatalf("notice = %q", s.Notice)
		}
	}
	snapshot, err = kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	paused := map[string]bool{}
	for _, sub := range snapshot.Subscriptions {
		paused[sub.ID] = sub.Paused
	}
	if !paused[subB.ID] || paused[subA.ID] {
		t.Fatalf("pause after filter hit the wrong follow: a=%v b=%v", paused[subA.ID], paused[subB.ID])
	}
}

func TestUIFollowingCardClickOpensResourceList(t *testing.T) {
	s := uiFixture(t, "following")
	tt := ui.NewTester(s.view, 1320, 860)
	wantID := s.Subscriptions[0].ID
	uiClick(t, tt, "打开追番 "+s.Subscriptions[0].GroupName)
	if s.Page != "detail" || !s.ViewResources || !s.Backfill || s.EditID != wantID || s.DetailReturn != "following" {
		t.Fatalf("card click state: page=%q resources=%v backfill=%v edit=%q return=%q", s.Page, s.ViewResources, s.Backfill, s.EditID, s.DetailReturn)
	}
}

func TestUIFollowingCancelSubscriptionDialog(t *testing.T) {
	s := uiFixture(t, "following")
	sub := s.Subscriptions[0]
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "取消追番 "+sub.GroupName)
	if !s.CancelPrompt || s.CancelID != sub.ID {
		t.Fatalf("cancel prompt not armed: prompt=%v id=%q", s.CancelPrompt, s.CancelID)
	}
	uiRequireText(t, tt, "确认取消追番",
		"取消后不再自动检查新资源；已下载的文件保留在 "+sub.Folder,
		"它正在下载的 1 个资源会继续完成，不受取消影响。")
	uiClick(t, tt, "保留追番")
	if s.CancelPrompt {
		t.Fatal("保留追番 did not close the dialog")
	}
	uiClick(t, tt, "取消追番 "+sub.GroupName)
	uiClick(t, tt, "确认取消追番 "+sub.GroupName)
	if s.CancelPrompt || s.CancelID != "" {
		t.Fatalf("confirm left the dialog open: prompt=%v id=%q", s.CancelPrompt, s.CancelID)
	}
}

func TestUIRemoveSubscriptionForwardsToCore(t *testing.T) {
	kernel, err := core.Open(filepath.Join(t.TempDir(), "dipper.db"), core.Options{
		DisableScheduler: true,
		Feed:             func(context.Context, string, string) ([]core.Resource, error) { return nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })
	res := core.Resource{ID: "bridge-1", Title: "1080p", Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("ab", 20), Size: 100}
	sub, err := kernel.CreateSubscription(context.Background(), core.Anime{ID: "bridge-anime", Title: "桥接番剧"}, core.Group{ID: "bridge-group", Name: "桥接字幕组"}, core.Rule{}, []core.Resource{res}, []string{"bridge-1"})
	if err != nil && sub.ID == "" {
		t.Fatalf("subscription not committed: %v", err)
	}
	s := uiFixture(t, "following")
	s.Core = kernel
	s.Engine = &downloader.Engine{}
	s.Ctx = context.Background()
	// Keep action results off the action goroutine, as the window does.
	updates := make(chan func(), 2)
	s.Update = func(fn func()) { updates <- fn }
	snapshot, err := kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.project(snapshot, nil)
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "取消追番 桥接字幕组")
	uiClick(t, tt, "确认取消追番 桥接字幕组")
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for s.Notice != "已取消追番《桥接番剧》" {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatalf("notice = %q", s.Notice)
		}
	}
	snapshot, err = kernel.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Subscriptions) != 0 {
		t.Fatalf("core still tracks %d subscriptions", len(snapshot.Subscriptions))
	}
	if len(s.Subscriptions) != 0 {
		t.Fatalf("ui subscriptions after removal: %d", len(s.Subscriptions))
	}
}

func TestUIDownloadsOfferPauseDeleteAndKeepTaskDismissal(t *testing.T) {
	s := uiFixture(t, "downloads")
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "暂停", "删除")
	uiClick(t, tt, "暂停")
	if s.Downloads[0].Paused || s.Downloads[0].Progress != .43 {
		t.Fatal("UI changed download state without an engine")
	}
	uiClick(t, tt, "删除")
	uiRequireText(t, tt, "确认删除下载", "保留任务", "删除并重新下载", "删除任务")
	uiClick(t, tt, "保留任务")
	if s.RemovePrompt {
		t.Fatal("keep must dismiss the dialog")
	}
	if len(s.Downloads) != 3 {
		t.Fatal("dismissal changed the download list")
	}
}

func TestUIDownloadsOfferOneTransferActionPerState(t *testing.T) {
	for _, tc := range []struct {
		name     string
		progress float64
		paused   bool
		stopped  bool
		failed   bool
		action   string
	}{
		{name: "downloading", progress: .43, action: "暂停"},
		{name: "paused", progress: .43, paused: true, action: "继续"},
		{name: "stopped", progress: .43, stopped: true, action: "继续"},
		{name: "failed", progress: .43, failed: true, action: "重试"},
		{name: "seeding", progress: 1, action: "停止做种"},
		{name: "seeding-paused", progress: 1, paused: true, action: "继续"},
		{name: "completed", progress: 1, stopped: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := uiFixture(t, "downloads")
			s.Downloads = s.Downloads[:1]
			d := &s.Downloads[0]
			d.Progress, d.Paused, d.Stopped, d.Failed = tc.progress, tc.paused, tc.stopped, tc.failed
			tt := ui.NewTester(s.view, 1320, 860)
			for _, action := range []string{"暂停", "继续", "重试", "停止下载", "停止做种"} {
				_, visible := tt.Find(action)
				if visible != (action == tc.action) {
					t.Errorf("transfer action %q visible=%v; want only %q", action, visible, tc.action)
				}
			}
			uiRequireText(t, tt, "删除", "打开文件夹")
		})
	}
}

func TestUIRemoveDialogStopButtonAndLayout(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[:1]
	s.Downloads[0].Stopped = true
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "已停止")
	if tt.HasText("已完成 · 做种中") || tt.HasText("已完成 · 已停止做种") {
		t.Fatal("a stopped mid-download task must not read as completed")
	}
	if _, ok := tt.Find("停止下载"); ok {
		t.Fatal("a stopped task must not offer stopping again")
	}
	if _, ok := tt.Find("继续"); !ok {
		t.Fatal("a stopped task must keep its resume action")
	}
}

func TestUIRemoveDownloadForwardsDeleteAndRedownloadToCore(t *testing.T) {
	for _, tc := range []struct {
		name   string
		button string
		notice string
		want   core.ItemStatus
	}{
		{name: "delete", button: "确认删除下载任务", notice: "已删除该下载任务", want: core.ItemSkipped},
		{name: "redownload", button: "确认删除并重新下载", notice: "已删除任务，将重新下载", want: core.ItemPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kernel, err := core.Open(filepath.Join(t.TempDir(), "dipper.db"), core.Options{
				DisableScheduler: true,
				Feed:             func(context.Context, string, string) ([]core.Resource, error) { return nil, nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { kernel.Close() })
			res := core.Resource{ID: "bridge-1", Title: "1080p", Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("ab", 20), Size: 100}
			sub, err := kernel.CreateSubscription(context.Background(), core.Anime{ID: "bridge-anime", Title: "桥接番剧"}, core.Group{ID: "bridge-group", Name: "桥接字幕组"}, core.Rule{}, []core.Resource{res}, []string{"bridge-1"})
			if err != nil && sub.ID == "" {
				t.Fatalf("subscription not committed: %v", err)
			}
			hash, _ := core.ResourceHash(res)
			s := uiFixture(t, "downloads")
			s.Core = kernel
			s.Engine = &downloader.Engine{}
			s.Ctx = context.Background()
			// Keep action results off the action goroutine, as the window does.
			updates := make(chan func(), 2)
			s.Update = func(fn func()) { updates <- fn }
			snapshot, err := kernel.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			s.project(snapshot, nil)
			if len(s.Downloads) != 1 || s.Downloads[0].ID != hash {
				t.Fatalf("projected downloads: %+v", s.Downloads)
			}
			tt := ui.NewTester(s.view, 1320, 860)
			uiClick(t, tt, "删除")
			uiClick(t, tt, tc.button)
			deadline := time.NewTimer(2 * time.Second)
			defer deadline.Stop()
			for s.Notice != tc.notice {
				select {
				case fn := <-updates:
					fn()
				case <-deadline.C:
					t.Fatalf("notice = %q (error %q)", s.Notice, s.SaveError)
				}
			}
			subs, err := kernel.Subscriptions()
			if err != nil {
				t.Fatal(err)
			}
			item := subs[0].Items["bridge-1"]
			if item.Status != tc.want {
				t.Fatalf("item status = %q, want %q", item.Status, tc.want)
			}
			if tasks, _ := kernel.ListTasks(); len(tasks) != 0 {
				t.Fatalf("outbox kept the task: %+v", tasks)
			}
		})
	}
}

func TestUIActionsDelegateInsteadOfChangingFixtureState(t *testing.T) {
	s := uiFixture(t, "downloads")
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "暂停全部下载")
	uiClick(t, tt, "暂停")
	uiClick(t, tt, "重试")
	uiClick(t, tt, "停止做种")
	if s.Downloads[0].Paused || !s.Downloads[1].Failed || s.Downloads[2].Stopped {
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

func TestUIDownloadsGroupByAnimeWithSummary(t *testing.T) {
	s := uiFixture(t, "downloads")
	tt := ui.NewTester(s.view, 1320, 860)
	headers := 0
	for _, text := range tt.Texts() {
		if text == "星海旅记" {
			headers++
		}
	}
	if headers != 1 {
		t.Errorf("anime header shown %d times; want one per group; texts=%q", headers, tt.Texts())
	}
	uiRequireText(t, tt, "3 个资源 · 1 下载中 · 1 已完成 · 1 失败", "444 MB / 722 MB")
}

func TestUIDownloadsFilterHidesOtherStates(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.DownloadFilter = downloadsCompleted
	tt := ui.NewTester(s.view, 1320, 860)
	uiScrollTo(t, tt, "停止做种")
	uiRequireText(t, tt, "停止做种")
	if tt.HasText("连接超时，请重试") {
		t.Error("failed download visible under completed filter")
	}
	if tt.HasText("208 MB / 486 MB") {
		t.Error("downloading download visible under completed filter")
	}
}

func TestUIDownloadsEmptyFilterOffersReset(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.DownloadFilter = downloadsPaused
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "这个状态下没有下载", "查看全部下载")
	uiClick(t, tt, "查看全部下载")
	if s.DownloadFilter != downloadsAll {
		t.Fatalf("filter is %d; want downloadsAll", s.DownloadFilter)
	}
	uiRequireText(t, tt, "3 个资源 · 1 下载中 · 1 已完成 · 1 失败")
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

type playTestEngine struct{ files []string }

func (*playTestEngine) Snapshot() []downloader.Snapshot {
	return []downloader.Snapshot{{ID: "render-download-3", Progress: 1}}
}
func (*playTestEngine) Pause(string) error          { return fmt.Errorf("unexpected pause") }
func (*playTestEngine) Resume(string) error         { return fmt.Errorf("unexpected resume") }
func (*playTestEngine) Stop(string) error           { return fmt.Errorf("unexpected stop") }
func (*playTestEngine) Remove(string) error         { return fmt.Errorf("unexpected removal") }
func (*playTestEngine) SetUploadLimit(int64) error  { return fmt.Errorf("unexpected upload limit") }
func (*playTestEngine) SetSeedTarget(time.Duration) {}
func (*playTestEngine) SetRatioTarget(float64)      {}
func (*playTestEngine) SetExtraTrackers([]string)   {}

func (e *playTestEngine) PlayableFiles(string) ([]string, error) {
	if e.files == nil {
		return nil, fmt.Errorf("no files")
	}
	return e.files, nil
}
func (e *playTestEngine) DownloadFiles(string) ([]downloader.DownloadedFile, error) {
	if e.files == nil {
		return nil, fmt.Errorf("no files")
	}
	result := make([]downloader.DownloadedFile, 0, len(e.files))
	for _, path := range e.files {
		result = append(result, downloader.DownloadedFile{Path: path})
	}
	return result, nil
}

func TestUIDownloadPlayButtonOpensFirstVideo(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[2:3]
	played := ""
	s.Engine = &playTestEngine{files: []string{"C:\\Downloads\\Dipper\\episode.mkv", "C:\\Downloads\\Dipper\\extra.txt"}}
	s.PlayFile = func(path string) { played = path }
	updates := make(chan func(), 2)
	s.Update = func(fn func()) { updates <- fn }
	tt := ui.NewTester(s.view, 1320, 860)
	if _, ok := tt.Find("播放"); !ok {
		t.Fatal("a finished download must offer 播放")
	}
	uiClick(t, tt, "播放")
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for played == "" && s.SaveError == "" {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatal("play did not open a file")
		}
	}
	if played != "C:\\Downloads\\Dipper\\episode.mkv" {
		t.Fatalf("played %q, want the first video file", played)
	}
	if _, ok := tt.Find("播放"); !ok {
		t.Fatal("播放 must stay visible after playing")
	}
}

func TestUIDownloadPlayReportsMissingFiles(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[2:3]
	s.Engine = &playTestEngine{files: nil}
	s.PlayFile = func(string) {}
	updates := make(chan func(), 2)
	s.Update = func(fn func()) { updates <- fn }
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "播放")
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for s.SaveError == "" {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatal("missing files must surface a save error")
		}
	}
}

type filesTestEngine struct{ files []downloader.DownloadedFile }

func (*filesTestEngine) Snapshot() []downloader.Snapshot {
	return []downloader.Snapshot{{ID: "render-download-3", Progress: 1}}
}
func (*filesTestEngine) Pause(string) error          { return fmt.Errorf("unexpected pause") }
func (*filesTestEngine) Resume(string) error         { return fmt.Errorf("unexpected resume") }
func (*filesTestEngine) Stop(string) error           { return fmt.Errorf("unexpected stop") }
func (*filesTestEngine) Remove(string) error         { return fmt.Errorf("unexpected removal") }
func (*filesTestEngine) SetUploadLimit(int64) error  { return fmt.Errorf("unexpected upload limit") }
func (*filesTestEngine) SetSeedTarget(time.Duration) {}
func (*filesTestEngine) SetRatioTarget(float64)      {}
func (*filesTestEngine) SetExtraTrackers([]string)   {}

func (*filesTestEngine) PlayableFiles(string) ([]string, error) {
	return nil, fmt.Errorf("unexpected play")
}
func (e *filesTestEngine) DownloadFiles(string) ([]downloader.DownloadedFile, error) {
	return e.files, nil
}

func TestUIDownloadFilesDialogShowsEntriesAndActions(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[2:3]
	played, copied := "", ""
	s.Engine = &filesTestEngine{files: []downloader.DownloadedFile{
		{Path: `C:\Downloads\Dipper\release\video.mkv`, Size: 1024},
		{Path: `C:\Downloads\Dipper\release\subs\字幕.ass`, Size: 2048},
	}}
	s.PlayFile = func(path string) { played = path }
	s.CopyText = func(text string) { copied = text }
	updates := make(chan func(), 4)
	s.Update = func(fn func()) { updates <- fn }
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "文件列表")
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for len(s.Files) == 0 {
		select {
		case fn := <-updates:
			fn()
		case <-deadline.C:
			t.Fatal("file list did not load")
		}
	}
	tt.Frame()
	uiRequireText(t, tt, "video.mkv", "字幕.ass", "复制路径", "播放此文件")
	uiClick(t, tt, "播放此文件")
	if played != `C:\Downloads\Dipper\release\video.mkv` {
		t.Fatalf("played %q, want the mkv file", played)
	}
	uiClick(t, tt, "复制路径")
	if copied != `C:\Downloads\Dipper\release\video.mkv` {
		t.Fatalf("copied %q, want the mkv path", copied)
	}
}

func TestUIDownloadFilesEntryHiddenBeforeFinish(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[:1]
	s.Engine = &filesTestEngine{}
	s.CopyText = func(string) {}
	tt := ui.NewTester(s.view, 1320, 860)
	if _, ok := tt.Find("文件列表"); ok {
		t.Fatal("a downloading task must not offer 文件列表")
	}
}
func TestUIDownloadOffersNoPlayButtonBeforeFinish(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[:1]
	s.PlayFile = func(string) {}
	tt := ui.NewTester(s.view, 1320, 860)
	if _, ok := tt.Find("播放"); ok {
		t.Fatal("a downloading task must not offer 播放")
	}
	s.Downloads[0].Progress = 1
	s.Downloads[0].Failed = true
	tt.Frame()
	if _, ok := tt.Find("播放"); ok {
		t.Fatal("a failed task must not offer 播放")
	}
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
	for _, height := range []int{720, 860} {
		t.Run(strconv.Itoa(height), func(t *testing.T) {
			s := uiFixture(t, "settings")
			s.Busy = true
			picked, quit := false, false
			s.PickFolder = func() { picked = true }
			s.Quit = func() { quit = true }
			tt := ui.NewTester(s.view, 1320, height)
			uiClick(t, tt, "选择文件夹")
			before := s.ReduceEffects
			uiClick(t, tt, "减少动态效果")
			uiScrollTo(t, tt, windowText.quit)
			uiClick(t, tt, windowText.quit)
			if picked || quit || before != s.ReduceEffects || s.SaveError != "" {
				t.Fatal("busy settings invoked an operation")
			}
		})
	}
}

func TestUISettingsProxyModesAndCustomAddress(t *testing.T) {
	s := uiFixture(t, "settings")
	tt := ui.NewTester(s.view, 1320, 860)
	uiScrollTo(t, tt, "自定义")
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
	uiScrollTo(t, tt, "代理地址")
	if rect, ok := tt.Find("代理地址"); ok {
		tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	}
	tt.Type("127.0.0.1:7897")
	if s.ProxyURL != "127.0.0.1:7897" {
		t.Fatalf("typed address = %q", s.ProxyURL)
	}
}

func TestUISettingsSeedDurationSavesAndUpdatesCopy(t *testing.T) {
	s := uiFixture(t, "settings")
	s.Seed = "1 小时"
	tt := ui.NewTester(s.view, 1320, 860)
	uiScrollTo(t, tt, "做种时长选择")
	uiRequireText(t, tt, "做种时长选择", "下载完成后按这个时长做种。")
	tt.Click("做种时长选择")
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("30 分钟") {
		t.Fatalf("seed popup did not open; texts=%q", tt.Texts())
	}
	tt.Click("30 分钟")
	tt.Advance(300 * time.Millisecond)
	if s.Seed != "30 分钟" {
		t.Fatalf("seed choice = %q", s.Seed)
	}
	uiRequireText(t, tt, "分享比例 1:1或有效做种 30 分钟，满足任一即停止做种。")
	if s.SaveError != "" {
		t.Fatalf("seed save failed: %q", s.SaveError)
	}
	s.Seed = "不限时长"
	tt.Frame()
	uiRequireText(t, tt, "分享比例 1:1即停止做种。")
	// The download page must reflect the same choice through seedStopRule.
	s.Page = "downloads"
	tt.Frame()
	uiRequireText(t, tt, "分享比例 1:1即停止做种")
}
func TestUISeedStopRuleFollowsDownloadsPage(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[2:3]
	s.Seed = "不限时长"
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "分享比例 1:1即停止做种")
	s.Seed = "30 分钟"
	s.SeedRatio = "2:1"
	tt.Frame()
	uiRequireText(t, tt, "分享比例 2:1或有效做种 30 分钟，满足任一即停止做种")
}

func TestUISeedStopRuleWithoutLimitsSaysSo(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[2:3]
	s.Seed = "不限时长"
	s.SeedRatio = "不限"
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "下载完成后不会自动停止做种。")
}

func TestUISettingsShowSeedAdvancedOptions(t *testing.T) {
	s := uiFixture(t, "settings")
	tt := ui.NewTester(s.view, 1320, 860)
	uiScrollTo(t, tt, "停止分享比例")
	uiRequireText(t, tt, "停止分享比例", "追加 Tracker", "1:1")
	if s.SeedRatio != "1:1" {
		t.Fatalf("default seed ratio = %q, want 1:1", s.SeedRatio)
	}
}

func TestUIRenderScreensOffline(t *testing.T) {
	dir := t.TempDir()
	if err := renderScreens(dir); err != nil {
		t.Fatal(err)
	}
	var previous []byte
	for _, name := range []string{"01-browse", "02-filter", "03-following", "04-downloads", "05-settings", "06-source-error", "07-backfill", "08-close-window", "11-view-resources", "12-seed-settings"} {
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
// letter fallback and of the loading indicator never matches.
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
// square, drawn large enough to read next to the wordmark, and a cover-less
// card shows the bird, not a blue letter.
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
	if chip := countDark(tt, int(word.X)-50, int(word.Y)-10, int(word.X)-10, int(word.Y+word.H)+10, false); chip < 100 || chip > 500 {
		t.Errorf("sidebar chip dark pixels = %d; want the bird at reading scale on a wash, not a solid square (100-500)", chip)
	}
	title, ok := tt.Find("星海旅记")
	if !ok {
		t.Fatal("cover-less card title missing")
	}
	if n := countDark(tt, int(title.X)-20, int(title.Y)-210, int(title.X)+140, int(title.Y)-8, true); n < 30 {
		t.Errorf("cover fallback ink pixels = %d; want the bird mark, not a blue letter (>=30)", n)
	}
}

// A cover whose load is in flight shows the loading indicator: the card says it
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
