package main

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
)

func TestUIDownloadsHideFinishedSharing(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		download Download
		visible  bool
	}{
		{name: "waiting", visible: true},
		{name: "downloading", download: Download{Progress: .5}, visible: true},
		{name: "paused", download: Download{Progress: .5, Paused: true}, visible: true},
		{name: "stopped incomplete", download: Download{Progress: .5, Stopped: true}, visible: true},
		{name: "sharing without traffic", download: Download{Progress: 1}, visible: true},
		{name: "sharing paused", download: Download{Progress: 1, Paused: true}, visible: true},
		{name: "sharing ended", download: Download{Progress: 1, Stopped: true}},
		{name: "sharing ended while paused", download: Download{Progress: 1, Paused: true, Stopped: true}},
		{name: "failed incomplete", download: Download{Progress: .5, Failed: true}, visible: true},
		{name: "failed completed", download: Download{Progress: 1, Stopped: true, Failed: true}, visible: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := uiFixture(t, "downloads")
			download := scenario.download
			download.ID, download.AnimeID, download.Title = "active-task", s.AnimeID, "状态测试资源"
			s.Downloads = []Download{download}
			tt := ui.NewTester(s.view, 1320, 860)
			if got := tt.HasText(download.Title); got != scenario.visible {
				t.Fatalf("finished-sharing visibility=%v, want %v; texts=%q", got, scenario.visible, tt.Texts())
			}
			if len(s.Downloads) != 1 || s.Downloads[0] != download {
				t.Fatal("rendering the downloads page changed the retained task")
			}
			if !scenario.visible {
				uiRequireText(t, tt, "当前没有进行中的任务", "我的追番")
				uiClick(t, tt, "我的追番")
				if s.Page != "following" {
					t.Fatalf("finished downloads did not lead to follows: page=%q", s.Page)
				}
			}
		})
	}
}

func TestUIDownloadsFinishedSharingDoesNotLeakIntoSearchOrCounts(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = []Download{
		{ID: "running", AnimeID: s.AnimeID, Title: "正在下载的资源", Progress: .5},
		{ID: "sharing", AnimeID: s.AnimeID, Title: "正在分享的资源", Progress: 1},
		{ID: "finished", AnimeID: "render-2", Title: "已经结束分享的资源", Progress: 1, Stopped: true, MB: 900},
	}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "2", "全部 2", "下载中 1", "分享中 1", "2 个下载任务，下载中 1，分享中 1")
	if !slices.Contains(tt.Texts(), "2") || slices.Contains(tt.Texts(), "3") {
		t.Fatal("navigation count does not match the queue")
	}
	if tt.HasText("雨后的星期天") || tt.HasText("已经结束分享的资源") || tt.HasText("900 MB") {
		t.Fatalf("finished sharing leaked into groups or totals: %q", tt.Texts())
	}
	uiClick(t, tt, "搜索下载")
	tt.Type("已经结束分享")
	tt.Frame()
	uiRequireText(t, tt, "没有匹配的下载", "全部 2", "分享中 1")
	if tt.HasText("已经结束分享的资源") {
		t.Fatal("search revealed a finished sharing task")
	}
	uiClick(t, tt, "清除搜索")
	uiClick(t, tt, "分享中")
	uiRequireText(t, tt, "正在分享的资源", "1 个下载任务，分享中 1")
	if tt.HasText("正在下载的资源") {
		t.Fatal("sharing filter includes an incomplete transfer")
	}
}

func TestUIDownloadsFinishedSharingRemainsPlayableInLibrary(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.LibraryID = s.Subscriptions[0].ID
	s.Downloads = s.Downloads[2:3]
	download := s.Downloads[0]
	played := make(chan string, 1)
	path := filepath.Join(s.Root, "episode.mkv")
	s.Engine = &playTestEngine{files: []string{path}}
	s.PlayFile = func(file string) error { played <- file; return nil }
	updates := make(chan func(), 4)
	s.Update = func(fn func()) { updates <- fn }
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, download.Title, "停止分享")
	s.Downloads[0].Stopped = true
	tt.Frame()
	if tt.HasText(download.Title) {
		t.Fatal("task remained visible after sharing stopped")
	}
	uiRequireText(t, tt, "当前没有进行中的任务")
	s.Page = "episodes"
	tt.Frame()
	uiRequireText(t, tt, "已完成")
	uiClick(t, tt, "播放 "+download.ID)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case file := <-played:
			if file != path {
				t.Fatalf("played %q, want %q", file, path)
			}
			return
		case update := <-updates:
			update()
		case <-deadline.C:
			t.Fatal("finished sharing task could not be played from the library")
		}
	}
}

func TestUIDownloadsSharingOnlyHasNoBatchDownloadAction(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[2:3]
	tt := ui.NewTester(s.view, 1320, 860)
	for _, label := range []string{"暂停全部下载", "继续全部下载"} {
		if _, ok := tt.Find(label); ok {
			t.Fatalf("sharing-only page offers an inapplicable %q action", label)
		}
	}
}

func TestUIDownloadsFinishedSharingCanBeManagedFromLibrary(t *testing.T) {
	for _, action := range []string{"cancel", "delete", "redownload"} {
		t.Run(action, func(t *testing.T) {
			engine := &removalTestEngine{}
			state, tester, sub, hash, updates := removalFixture(t, engine)
			snapshot, err := state.Core.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			transfers := engine.Snapshot()
			transfers[0].Progress, transfers[0].AutoStopped = 1, true
			state.project(snapshot, transfers)
			state.Page, state.LibraryID = "episodes", sub.ID
			tester.Frame()
			uiClick(t, tester, "剧集操作 "+hash)
			if err := tester.ChooseMenuItem("删除或重新下载"); err != nil {
				t.Fatal(err)
			}
			tester.Frame()
			uiRequireText(t, tester, "删除任务和文件？", "已下载文件会一并删除，无法恢复。")
			tasks, err := state.Core.ListTasks()
			if err != nil || len(tasks) != 1 || engine.calls != 1 {
				t.Fatalf("opening management changed tasks: tasks=%+v calls=%d error=%v", tasks, engine.calls, err)
			}
			if action == "cancel" {
				uiClick(t, tester, "保留任务")
				if state.RemovePrompt || state.Busy {
					t.Fatal("cancel kept the removal dialog or started work")
				}
				if tasks, err := state.Core.ListTasks(); err != nil || len(tasks) != 1 || len(engine.Snapshot()) != 1 {
					t.Fatalf("cancel changed stored tasks: tasks=%+v error=%v", tasks, err)
				}
				return
			}
			button := "确认删除任务和文件"
			if action == "redownload" {
				button = "确认删除并重新下载"
			}
			uiClick(t, tester, button)
			awaitRemovalAction(t, state, updates)
			if state.SaveError != "" {
				t.Fatal(state.SaveError)
			}
			if err := state.Core.RetryPending(t.Context()); err != nil {
				t.Fatal(err)
			}
			tasks, err = state.Core.ListTasks()
			if err != nil {
				t.Fatal(err)
			}
			if action == "delete" {
				if len(tasks) != 0 || len(engine.Snapshot()) != 0 {
					t.Fatalf("library deletion retained task: tasks=%+v", tasks)
				}
			} else if len(tasks) != 1 || tasks[0].Task.Hash != hash || !tasks[0].Accepted || engine.calls != 2 {
				t.Fatalf("library redownload did not recreate the task: tasks=%+v calls=%d", tasks, engine.calls)
			}
		})
	}
}

func TestUIDownloadsFinishedSharingLibraryPlaysSelectedFile(t *testing.T) {
	state := uiFixture(t, "downloads")
	state.Downloads = state.Downloads[2:3]
	download := &state.Downloads[0]
	download.Stopped = true
	state.LibraryID = state.Subscriptions[0].ID
	first := filepath.Join(t.TempDir(), "01.mkv")
	second := filepath.Join(filepath.Dir(first), "02.mkv")
	state.Engine = &filesTestEngine{files: []downloader.DownloadedFile{{Path: first, Size: 10}, {Path: second, Size: 20}}}
	played := ""
	state.PlayFile = func(path string) error { played = path; return nil }
	updates := make(chan func(), 2)
	state.Update = func(update func()) { updates <- update }
	tester := ui.NewTester(state.view, 1320, 860)
	uiRequireText(t, tester, "当前没有进行中的任务")
	state.Page = "episodes"
	tester.Frame()
	uiClick(t, tester, "剧集操作 "+download.ID)
	if err := tester.ChooseMenuItem("查看文件"); err != nil {
		t.Fatal(err)
	}
	select {
	case update := <-updates:
		update()
	case <-time.After(5 * time.Second):
		t.Fatal("library file list did not load")
	}
	tester.Frame()
	uiRequireText(t, tester, "01.mkv", "02.mkv")
	uiClick(t, tester, "播放此文件 "+second)
	if played != second {
		t.Fatalf("played %q, want selected second episode %q", played, second)
	}
}

type sharingQueueTestEngine struct {
	resumeTestEngine
}

func (e *sharingQueueTestEngine) Stop(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.commands = append(e.commands, "stop "+id)
	e.snapshot.ManuallyStopped = true
	return nil
}

func TestUIDownloadsSharingCompletionRefreshKeepsStoredTask(t *testing.T) {
	for _, mode := range []string{"manual", "automatic"} {
		t.Run(mode, func(t *testing.T) {
			hash := strings.Repeat("bc", 20)
			engine := &sharingQueueTestEngine{resumeTestEngine: resumeTestEngine{
				snapshot: downloader.Snapshot{ID: hash, Progress: 1},
			}}
			root := t.TempDir()
			kernel, err := core.Open(filepath.Join(root, "dipper.db"), core.Options{
				DownloadRoot: filepath.Join(root, "downloads"), Downloader: engine, DisableScheduler: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := kernel.Close(); err != nil {
					t.Error(err)
				}
			})
			s := uiFixture(t, "downloads")
			resource := core.Resource{ID: "sharing-episode", Title: "分享测试 第 1 集", Magnet: "magnet:?xt=urn:btih:" + hash}
			sub, err := kernel.CreateSubscription(context.Background(), core.Anime{ID: s.AnimeID, Title: s.anime(s.AnimeID).Title}, core.Group{ID: "sharing-group"}, core.Rule{}, []core.Resource{resource}, []string{resource.ID})
			if err != nil {
				t.Fatal(err)
			}
			s.Core, s.Engine, s.Ctx = kernel, engine, context.Background()
			updates := make(chan func(), 4)
			s.Update = func(fn func()) { updates <- fn }
			s.refreshState()
			(<-updates)()
			tt := ui.NewTester(s.view, 1320, 860)
			uiRequireText(t, tt, resource.Title, "停止分享", "分享中 1", "1")
			if mode == "manual" {
				uiClick(t, tt, "停止分享")
				deadline := time.NewTimer(5 * time.Second)
				defer deadline.Stop()
				for s.Busy {
					select {
					case update := <-updates:
						update()
					case <-deadline.C:
						t.Fatal("stop sharing did not complete")
					}
				}
				if s.SaveError != "" {
					t.Fatalf("stop sharing failed: %s", s.SaveError)
				}
			} else {
				engine.mu.Lock()
				engine.snapshot.AutoStopped = true
				engine.mu.Unlock()
				s.refreshState()
				(<-updates)()
			}
			engine.mu.Lock()
			commands := append([]string(nil), engine.commands...)
			engine.mu.Unlock()
			var want []string
			if mode == "manual" {
				want = []string{"stop " + hash}
			}
			if !slices.Equal(commands, want) {
				t.Fatalf("%s completion issued %v, want %v", mode, commands, want)
			}
			tt.Frame()
			uiRequireText(t, tt, "当前没有进行中的任务")
			if tt.HasText(resource.Title) {
				t.Fatalf("finished sharing remains in the downloads queue: %q", tt.Texts())
			}
			tasks, err := kernel.ListTasks()
			if err != nil || len(tasks) != 1 || tasks[0].Task.Hash != hash {
				t.Fatalf("completion removed the stored task: tasks=%+v error=%v", tasks, err)
			}
			s.Page, s.LibraryID = "episodes", sub.ID
			tt.Frame()
			uiRequireText(t, tt, resource.Title, "已完成", "播放 "+hash)
		})
	}
}
