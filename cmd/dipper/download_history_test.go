package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/downloader"
)

func TestUIDownloadsFinishedAfterUnfollowCanBeManagedFromHistory(t *testing.T) {
	for _, action := range []string{"cancel", "delete", "redownload"} {
		t.Run(action, func(t *testing.T) {
			engine := &removalTestEngine{}
			state, tester, sub, hash, updates := removalFixture(t, engine)
			if err := state.Core.RemoveSubscription(t.Context(), sub.ID); err != nil {
				t.Fatal(err)
			}
			snapshot, err := state.Core.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			transfers := engine.Snapshot()
			folder := transfers[0].Folder
			transfers[0].Progress, transfers[0].AutoStopped = 1, true
			state.project(snapshot, transfers)
			tester.Frame()
			if len(state.Subscriptions) != 0 {
				t.Fatal("unfollow retained a subscription")
			}
			uiRequireText(t, tester, "当前没有进行中的任务")
			uiClick(t, tester, "下载记录 1")
			uiClick(t, tester, "下载记录操作 "+hash)
			if err := tester.ChooseMenuItem("删除或重新下载"); err != nil {
				t.Fatal(err)
			}
			tester.Frame()
			uiRequireText(t, tester, "删除任务和文件？")
			if action == "cancel" {
				uiClick(t, tester, "保留任务")
				tasks, err := state.Core.ListTasks()
				if err != nil || len(tasks) != 1 || len(engine.Snapshot()) != 1 || state.Busy {
					t.Fatalf("cancel changed retained tasks: tasks=%+v error=%v", tasks, err)
				}
				uiClick(t, tester, "返回当前下载")
				uiRequireText(t, tester, "当前没有进行中的任务")
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
			tasks, err := state.Core.ListTasks()
			if err != nil {
				t.Fatal(err)
			}
			if action == "delete" {
				if len(tasks) != 0 || len(engine.Snapshot()) != 0 {
					t.Fatalf("history deletion retained task: tasks=%+v", tasks)
				}
			} else if len(tasks) != 1 || tasks[0].Task.Hash != hash || tasks[0].Task.Directory != folder || !tasks[0].Accepted || engine.calls != 2 {
				t.Fatalf("unfollowed redownload lost intent or destination: tasks=%+v calls=%d", tasks, engine.calls)
			}
		})
	}
}

func TestUIDownloadsHistoryPlaysSelectedFileWithoutFollow(t *testing.T) {
	state := uiFixture(t, "downloads")
	state.Downloads = state.Downloads[2:3]
	state.Downloads[0].Stopped = true
	state.Subscriptions = nil
	first := filepath.Join(t.TempDir(), "01.mkv")
	second := filepath.Join(filepath.Dir(first), "02.mkv")
	state.Engine = &filesTestEngine{files: []downloader.DownloadedFile{{Path: first, Size: 10}, {Path: second, Size: 20}}}
	played := ""
	state.PlayFile = func(path string) error { played = path; return nil }
	updates := make(chan func(), 2)
	state.Update = func(update func()) { updates <- update }
	tester := ui.NewTester(state.view, 1040, 720)
	uiClick(t, tester, "下载记录 1")
	uiClick(t, tester, "下载记录操作 "+state.Downloads[0].ID)
	if err := tester.ChooseMenuItem("查看文件"); err != nil {
		t.Fatal(err)
	}
	select {
	case update := <-updates:
		update()
	case <-time.After(5 * time.Second):
		t.Fatal("history file list did not load")
	}
	tester.Frame()
	uiRequireText(t, tester, "01.mkv", "02.mkv")
	uiClick(t, tester, "播放此文件 "+second)
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for state.Busy {
		select {
		case update := <-updates:
			update()
		case <-deadline.C:
			t.Fatal("history playback did not settle")
		}
	}
	tester.Frame()
	if played != second {
		t.Fatalf("played %q, want selected file %q", played, second)
	}
}

func TestUIDownloadsNavigationLeavesHistory(t *testing.T) {
	state := uiFixture(t, "downloads")
	state.Downloads[2].Stopped = true
	tester := ui.NewTester(state.view, 1040, 720)
	uiClick(t, tester, "下载记录 1")
	uiRequireText(t, tester, "分享已结束的任务")
	state.enterPage("browse")
	state.enterPage("downloads")
	tester.Frame()
	if state.DownloadHistory {
		t.Fatal("returning through navigation kept the history view")
	}
	if _, ok := tester.Find("返回当前下载"); ok {
		t.Fatal("returning through navigation still rendered history controls")
	}
}
