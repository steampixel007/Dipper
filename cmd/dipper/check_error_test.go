package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/downloader"
)

func TestCheckFailuresKeepTechnicalDetailsInLibrary(t *testing.T) {
	raw := `mikan: GET https://mikanime.tv/RSS/Bangumi?bangumiId=3887&subgroupid=203: Get "https://mikanani.me/rss/bangumi?bangumiId=3887&subgroupid=203": EOF`
	for _, mode := range []string{"all-failed", "partial", "single", "recovered", "unpersisted-error"} {
		t.Run(mode, func(t *testing.T) {
			retrying := false
			kernel, err := core.Open(filepath.Join(t.TempDir(), "checks.db"), core.Options{
				DisableScheduler: true,
				Feed: func(_ context.Context, animeID, _ string) ([]core.Resource, error) {
					if retrying && mode == "unpersisted-error" {
						return []core.Resource{{Title: "没有资源标识"}}, nil
					}
					if retrying && (mode == "recovered" || (mode == "partial" && animeID == "2")) {
						return nil, nil
					}
					return nil, errors.New(raw)
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { kernel.Close() })
			var ids []string
			for _, animeID := range []string{"1", "2", "3"} {
				sub, err := kernel.CreateSubscription(t.Context(),
					core.Anime{ID: animeID, Title: strings.Repeat("很长的番剧标题", 20) + animeID},
					core.Group{ID: "203", Name: "字幕组"}, core.Rule{}, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := kernel.Check(t.Context(), sub.ID); err == nil {
					t.Fatal("initial check should fail")
				}
				ids = append(ids, sub.ID)
			}
			state := uiFixture(t, "following")
			state.Core, state.Engine, state.Ctx = kernel, &downloader.Engine{}, t.Context()
			snapshot, err := kernel.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			state.project(snapshot, nil)
			updates := make(chan func(), 8)
			state.Update = func(update func()) { updates <- update }
			retrying = true
			tester := ui.NewTester(state.view, 1040, 720)
			if mode == "single" {
				state.check(ids[0])
			} else {
				uiClick(t, tester, "重试检查")
			}
			finishSubmission(t, state, updates)
			tester.Frame()
			requireNoSourceDetails(t, tester)
			switch mode {
			case "single":
				if state.SaveError != "网络连接失败，请检查网络后重试。" || state.Notice != "" {
					t.Fatalf("single check result: error=%q notice=%q", state.SaveError, state.Notice)
				}
			case "recovered":
				if state.SaveError != "" || state.Notice != "已检查 3 个追番的更新。" {
					t.Fatalf("recovery result: error=%q notice=%q", state.SaveError, state.Notice)
				}
			default:
				count := 3
				if mode == "partial" {
					count = 2
				}
				want := fmt.Sprintf("%d 个追番检查失败，请打开对应剧集库查看原因并重试。", count)
				if mode == "unpersisted-error" {
					want += "首次错误：feed resource has no identity"
				}
				if state.SaveError != want || strings.Contains(state.SaveError, "很长的番剧标题") {
					t.Fatalf("batch result is not a bounded summary: error=%q want=%q", state.SaveError, want)
				}
			}
			if mode != "recovered" {
				sub, ok := state.subscription(ids[0])
				if !ok || sub.LastError != raw || !sub.CheckFailed {
					t.Fatalf("detailed failure was lost: %+v", sub)
				}
				state.Page, state.LibraryID = "episodes", ids[0]
				tester.Frame()
				uiRequireText(t, tester, "检查更新失败：网络连接失败，请检查网络后重试。", "重试检查 字幕组")
				requireNoSourceDetails(t, tester)
			}
		})
	}
}
