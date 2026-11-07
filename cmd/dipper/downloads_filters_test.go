package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDownloadFiltersSeparateStoppedAndPaused(t *testing.T) {
	for _, scenario := range []struct {
		name string
		d    Download
		want DownloadFilter
	}{
		{"running", Download{Progress: .5}, downloadsDownloading},
		{"paused", Download{Progress: .5, Paused: true}, downloadsPaused},
		{"stopped", Download{Progress: .5, Stopped: true}, downloadsStopped},
		{"paused and stopped", Download{Progress: .5, Paused: true, Stopped: true}, downloadsStopped},
		{"sharing", Download{Progress: 1}, downloadsSeeding},
		{"sharing paused", Download{Progress: 1, Paused: true}, downloadsPaused},
		{"completed stopped", Download{Progress: 1, Stopped: true}, downloadsStopped},
		{"failed paused", Download{Progress: .5, Paused: true, Failed: true}, downloadsFailed},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if got := downloadState(scenario.d); got != scenario.want {
				t.Fatalf("download category=%d, want %d", got, scenario.want)
			}
		})
	}
}

func TestUIDownloadEmptyFiltersKeepNavigation(t *testing.T) {
	for _, filter := range []DownloadFilter{downloadsDownloading, downloadsSeeding, downloadsPaused, downloadsFailed, downloadsStopped} {
		t.Run(string(rune('0'+filter)), func(t *testing.T) {
			s := uiFixture(t, "downloads")
			s.Downloads = []Download{{ID: "paused", AnimeID: s.AnimeID, Title: "暂停资源", Paused: true}}
			if filter == downloadsPaused {
				s.Downloads[0].Paused = false
			}
			s.DownloadFilter = filter
			tt := ui.NewTester(s.view, 1040, 720)
			uiRequireText(t, tt, "这个筛选下没有下载", "全部", "下载中", "分享中", "已暂停", "失败", "已停止")
			if _, ok := tt.Find("搜索下载"); !ok {
				t.Fatal("empty filter hides search")
			}
			if filter == downloadsPaused {
				uiRequireText(t, tt, "全部 1", "下载中 1", "分享中 0", "已暂停 0", "失败 0", "已停止 0")
			} else {
				uiRequireText(t, tt, "全部 1", "下载中 0", "分享中 0", "已暂停 1", "失败 0", "已停止 0")
			}
			s.DownloadScroll.Y = 200
			uiClick(t, tt, "全部")
			if s.DownloadFilter != downloadsAll || s.DownloadScroll.Y != 0 {
				t.Fatalf("switch from empty filter left filter=%d scroll=%v", s.DownloadFilter, s.DownloadScroll.Y)
			}
			uiRequireText(t, tt, "暂停资源")
		})
	}
}

func TestUIDownloadEmptySearchRetainsFilterContext(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = []Download{
		{ID: "stopped", AnimeID: s.AnimeID, Title: "停止资源", Progress: .5, Stopped: true},
		{ID: "paused", AnimeID: s.AnimeID, Title: "暂停资源", Progress: .5, Paused: true},
	}
	s.DownloadFilter, s.DownloadSearch = downloadsStopped, "找不到"
	tt := ui.NewTester(s.view, 1040, 720)
	uiRequireText(t, tt, "没有匹配的下载", "全部 2", "下载中 0", "分享中 0", "已暂停 1", "失败 0", "已停止 1")
	uiClick(t, tt, "清除搜索")
	if s.DownloadSearch != "" || s.DownloadFilter != downloadsStopped {
		t.Fatalf("clear search changed context: search=%q filter=%d", s.DownloadSearch, s.DownloadFilter)
	}
	uiRequireText(t, tt, "停止资源", "1 个下载任务，已停止 1")
	if tt.HasText("暂停资源") {
		t.Fatal("paused resource belongs to stopped filter")
	}
	uiClick(t, tt, "已暂停")
	if s.DownloadFilter != downloadsPaused {
		t.Fatalf("paused filter click left %d", s.DownloadFilter)
	}
	uiRequireText(t, tt, "暂停资源", "1 个下载任务，已暂停 1")
	if tt.HasText("停止资源") {
		t.Fatal("stopped resource belongs to paused filter")
	}
}

func TestUIDownloadFiltersWrapAtDoubleTextScale(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = []Download{{ID: "stopped", AnimeID: s.AnimeID, Title: "停止资源", Stopped: true}}
	s.DownloadFilter = downloadsPaused
	tt := ui.NewTester(s.view, 760, 760)
	tt.SetPreferences(ui.Preferences{TextScale: 2})
	tt.Frame()
	first, ok := tt.Find("全部")
	if !ok {
		t.Fatal("all filter is missing")
	}
	last, ok := tt.Find("已停止")
	if !ok || last.X < 0 || last.X+last.W > 760 || last.Y+last.H > 760 {
		t.Fatalf("stopped filter is outside the window: %+v", last)
	}
	if last.Y <= first.Y {
		t.Fatalf("filters did not wrap at double text scale: all=%+v stopped=%+v", first, last)
	}
	uiClick(t, tt, "已停止")
	if s.DownloadFilter != downloadsStopped {
		t.Fatalf("wrapped stopped filter did not select: %d", s.DownloadFilter)
	}
}

func TestUIDownloadFailedPausedOffersOnlyRetry(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = []Download{{ID: "failed-paused", AnimeID: s.AnimeID, Title: "失败资源", Failed: true, Paused: true, Error: "连接超时"}}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "失败")
	if _, ok := tt.Find("重试"); !ok {
		t.Fatalf("failed paused download has no retry: %q", tt.Texts())
	}
	if _, ok := tt.Find("继续"); ok {
		t.Fatal("failed paused download offers resume instead of retry")
	}
}
