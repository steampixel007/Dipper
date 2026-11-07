package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUXDetailLargeTextStartsWithGroupChoice(t *testing.T) {
	s := uiFixture(t, "detail")
	s.ChoosingGroup = true
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})

	target := "选择字幕组 " + s.Groups[0]
	group, ok := tt.Find(target)
	if !ok {
		t.Fatalf("missing group choice %q; texts=%q", target, tt.Texts())
	}
	if group.W < 500 {
		t.Fatalf("group choice is squeezed into a narrow column: %+v", group)
	}
	if group.Y < 0 || group.Y+group.H > 720 {
		t.Fatalf("group choice is outside the window before scrolling: %+v", group)
	}

	uiClick(t, tt, target)
	if _, ok := tt.Find("可下载资源"); !ok {
		t.Fatalf("choosing a group did not reveal the resource preview; texts=%q", tt.Texts())
	}
}

func TestUXDetailLongTitleDoesNotCoverGroupChoice(t *testing.T) {
	s := uiFixture(t, "detail")
	s.ChoosingGroup = true
	title := strings.Repeat("关于我们每周一起寻找失落星图 ", 4)
	anime := s.anime(s.AnimeID)
	anime.Title = title
	s.KnownAnime[anime.ID] = anime
	for index := range s.Catalog {
		if s.Catalog[index].ID == anime.ID {
			s.Catalog[index] = anime
		}
	}
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})

	heading, ok := tt.Find(title)
	if !ok {
		t.Fatalf("missing long detail title; texts=%q", tt.Texts())
	}
	if heading.H < 100 {
		t.Fatalf("long detail title did not wrap: %+v", heading)
	}
	target := "选择字幕组 " + s.Groups[0]
	group, ok := tt.Find(target)
	if !ok {
		t.Fatalf("missing group choice %q; texts=%q", target, tt.Texts())
	}
	if group.Y < heading.Y+heading.H {
		t.Fatalf("group choice overlaps the clipped long title: title=%+v group=%+v", heading, group)
	}
	uiScrollTo(t, tt, target)
}

func TestUXDetailLongTitleAtNormalTextScrollsAsOnePage(t *testing.T) {
	s := uiFixture(t, "detail")
	s.ChoosingGroup = true
	title := strings.Repeat("关于我们每周一起寻找失落星图 ", 4)
	anime := s.anime(s.AnimeID)
	anime.Title = title
	s.KnownAnime[anime.ID] = anime
	for index := range s.Catalog {
		if s.Catalog[index].ID == anime.ID {
			s.Catalog[index] = anime
		}
	}
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 1, ReduceMotion: true})

	heading, ok := tt.Find(title)
	if !ok {
		t.Fatalf("missing long detail title; texts=%q", tt.Texts())
	}
	target := "可下载资源"
	if _, ok := tt.Find(target); !ok {
		t.Fatalf("missing resource preview %q; texts=%q", target, tt.Texts())
	}
	before := heading.Y
	uiScrollTo(t, tt, target)
	heading, ok = tt.Find(title)
	if !ok {
		t.Fatalf("long detail title disappeared after scrolling; texts=%q", tt.Texts())
	}
	if heading.Y >= before {
		t.Fatalf("scrolling to the resource preview left the long title pinned in a separate clipped header: before=%+v after=%+v", before, heading)
	}
}

func TestUXDownloadsLargeTextKeepsGroupTitleReadable(t *testing.T) {
	s := uiFixture(t, "downloads")
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})

	title, ok := tt.Find(s.anime(s.Subscriptions[0].AnimeID).Title)
	if !ok {
		t.Fatalf("missing download group title; texts=%q", tt.Texts())
	}
	if title.W < 300 {
		t.Fatalf("download group title has no readable width: %+v", title)
	}
	if title.H > 80 {
		t.Fatalf("download group title wrapped into a narrow column: %+v", title)
	}
	for _, action := range []string{"打开番剧文件夹", "查看详情", "收起"} {
		if _, ok := tt.Find(action); !ok {
			t.Fatalf("missing %q without opening the download group; texts=%q", action, tt.Texts())
		}
	}
}

func TestUXRemoveDialogLargeTextKeepsAllDestructiveActions(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.RemovePrompt, s.RemoveID = true, s.Downloads[0].ID
	title := strings.Repeat("关于我们每周一起寻找失落星图的漫长故事 ", 4) + "Special Edition With A Very Long Latin Subtitle"
	anime := s.anime(s.Downloads[0].AnimeID)
	anime.Title = title
	s.KnownAnime[anime.ID] = anime
	for index := range s.Catalog {
		if s.Catalog[index].ID == anime.ID {
			s.Catalog[index] = anime
		}
	}
	s.Downloads[0].Title = strings.Repeat("超长的番剧标题和版本名称 ", 4) + "Long Latin Release Name 1080p HEVC"
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})

	for _, action := range []string{"保留任务", "确认删除并重新下载", "确认删除任务和文件"} {
		bounds, ok := tt.Find(action)
		if !ok {
			t.Fatalf("missing %q; texts=%q", action, tt.Texts())
		}
		if bounds.W <= 0 || bounds.H <= 0 || bounds.Y < 0 || bounds.Y+bounds.H > 720 {
			t.Fatalf("%q is outside the dialog viewport: %+v", action, bounds)
		}
	}
	uiClick(t, tt, "保留任务")
	if s.RemovePrompt {
		t.Fatal("cancelling the delete dialog did not keep the task")
	}
}

func TestUXSourceErrorLargeTextKeepsRetryVisible(t *testing.T) {
	s := uiFixture(t, "browse")
	s.SourceError = "来源页面的结构发生变化，请稍后再试。"
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})

	bounds, ok := tt.Find("重试加载")
	if !ok {
		t.Fatalf("missing retry action; texts=%q", tt.Texts())
	}
	if bounds.Y < 0 || bounds.Y+bounds.H > 720 {
		t.Fatalf("retry action is outside the window: %+v", bounds)
	}
	for _, stale := range []string{"来源：蜜柑计划", "周一", "全部"} {
		if _, ok := tt.Find(stale); ok {
			t.Fatalf("catalog controls remain visible under the source error: %q", stale)
		}
	}
}

func TestUXDownloadsLargeTextCanReachFirstTransfer(t *testing.T) {
	s := uiFixture(t, "downloads")
	transfer := s.Downloads[2]
	transfer.Bytes = 236_000_000
	transfer.DownloadedBytes = 236_000_000
	transfer.UploadedBytes = 84_000_000
	transfer.SeedSeconds = 432_000
	s.Downloads = []Download{transfer}
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})

	filters, ok := tt.Find("已停止")
	if !ok || filters.H <= 0 {
		t.Fatal("missing download filters")
	}
	footer, ok := tt.Find("全局上传上限 " + s.Upload + "；完成后" + s.seedStopRule() + "。")
	if !ok || footer.H <= 0 {
		t.Fatal("missing transfer policy footer")
	}
	top, bottom := filters.Y+filters.H+18, footer.Y-18
	if bottom <= top {
		t.Fatalf("download list has no viewport: top=%v bottom=%v", top, bottom)
	}
	for _, target := range []string{transfer.Title, "停止分享", "删除", "打开文件夹", "查看文件", "播放"} {
		for attempt := 0; attempt < 8; attempt++ {
			bounds, found := tt.Find(target)
			if !found {
				t.Fatalf("missing transfer content %q; texts=%q", target, tt.Texts())
			}
			if bounds.W > 0 && bounds.H > 0 && bounds.Y >= top && bounds.Y+bounds.H <= bottom {
				break
			}
			tt.Scroll(624, (top+bottom)/2, 0, bounds.Y-(top+bottom-bounds.H)/2)
		}
		bounds, found := tt.Find(target)
		if !found || bounds.W <= 0 || bounds.H <= 0 || bounds.Y < top || bounds.Y+bounds.H > bottom {
			t.Fatalf("transfer content %q remains outside the list viewport: %+v", target, bounds)
		}
	}
}

func TestUXFollowingLongTitleKeepsLibraryEntryReachable(t *testing.T) {
	s := uiFixture(t, "following")
	title := strings.Repeat("星海旅记与失落星图的旅程", 4)
	for index := range s.Subscriptions {
		anime := s.anime(s.Subscriptions[index].AnimeID)
		anime.Title = title
		s.KnownAnime[anime.ID] = anime
		for catalogIndex := range s.Catalog {
			if s.Catalog[catalogIndex].ID == anime.ID {
				s.Catalog[catalogIndex] = anime
			}
		}
	}
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})

	uiScrollTo(t, tt, "进入剧集库")
	uiClick(t, tt, "打开追番 "+s.Subscriptions[0].GroupName)
	if _, ok := tt.Find("返回我的追番"); !ok {
		t.Fatalf("opening the long following card did not enter the episode library; texts=%q", tt.Texts())
	}
}
