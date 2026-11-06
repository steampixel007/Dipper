package main

import (
	"path/filepath"
	"strings"
	"testing"

	"dipper.local/dipper/internal/downloader"
	"github.com/egoist/mygo/ui"
)

func TestUXFilesDialogKeepsSavedLocationReachable(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[2:3]
	download := &s.Downloads[0]
	download.Folder = filepath.Join(t.TempDir(), strings.Repeat("很长的番剧目录名称"+string(filepath.Separator), 12))
	s.Files = []downloader.DownloadedFile{
		{Path: filepath.Join(download.Folder, "第 01 集.mkv"), Size: 1_250_001},
		{Path: filepath.Join(download.Folder, "第 02 集.mkv"), Size: 2_250_001},
		{Path: filepath.Join(download.Folder, "字幕", "简体中文.ass"), Size: 8_001},
	}
	s.FilesOpen, s.FilesID = true, download.ID

	savedAt := "已保存位置：" + download.Folder
	for _, target := range []string{"第 01 集.mkv", savedAt} {
		tt := ui.NewTester(s.view, 1040, 720)
		tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
		uiScrollTo(t, tt, target)
	}
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
	uiClick(t, tt, "关闭文件列表")
}

func TestUXFilesDialogCloseStaysReachableAtLargeText(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = s.Downloads[2:3]
	download := &s.Downloads[0]
	download.Folder = filepath.Join(t.TempDir(), strings.Repeat("很长的番剧目录名称"+string(filepath.Separator), 12))
	s.Files = []downloader.DownloadedFile{
		{Path: filepath.Join(download.Folder, "第 01 集.mkv"), Size: 1_250_001},
		{Path: filepath.Join(download.Folder, "第 02 集.mkv"), Size: 2_250_001},
		{Path: filepath.Join(download.Folder, "字幕", "简体中文.ass"), Size: 8_001},
	}
	s.FilesOpen, s.FilesID = true, download.ID

	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
	uiScrollTo(t, tt, "已保存位置："+download.Folder)
	closeBounds, found := tt.Find("关闭")
	if !found || closeBounds.W <= 0 || closeBounds.H <= 0 {
		t.Fatalf("footer close has no rendered bounds after scrolling: %+v found=%v", closeBounds, found)
	}
	uiClick(t, tt, "关闭")
	if s.FilesOpen {
		t.Fatal("footer close left the file dialog open")
	}
}

func TestUXSummaryDialogCloseStaysReachableAtLargeText(t *testing.T) {
	s := uiFixture(t, "detail")
	anime := s.anime(s.AnimeID)
	anime.Summary = strings.Repeat("星海旅记的故事从一个遥远的星球开始，旅人沿着失落的星图寻找回家的路。", 30)
	s.KnownAnime[anime.ID] = anime
	s.SummaryOpen = true

	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
	uiScrollTo(t, tt, "简介来自蜜柑计划")
	closeBounds, found := tt.Find("关闭")
	if !found || closeBounds.W <= 0 || closeBounds.H <= 0 {
		t.Fatalf("footer close has no rendered bounds after scrolling: %+v found=%v", closeBounds, found)
	}
	uiClick(t, tt, "关闭")
	if s.SummaryOpen {
		t.Fatal("footer close left the summary dialog open")
	}
}

func TestUXDetailLargeTextKeepsCommitReachable(t *testing.T) {
	s, _, updates, _ := submissionFixture(t, false)
	s.KnownAnime[s.AnimeID] = Anime{ID: s.AnimeID, Title: strings.Repeat("星海旅记与失落星图的旅程", 4)}
	for id := range s.Checked {
		s.Checked[id] = id == "new"
	}
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
	uiScrollTo(t, tt, "下载并追番")
	uiClick(t, tt, "下载并追番")
	if !s.Busy {
		t.Fatalf("commit did not start directly: busy=%v", s.Busy)
	}
	finishSubmission(t, s, updates)
	if s.Page != "following" || s.SaveError != "" {
		t.Fatalf("commit did not finish: page=%s error=%q", s.Page, s.SaveError)
	}
}

func TestUXRemoveDialogExplainsFilesAndKeepsSafeAction(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.RemovePrompt, s.RemoveID = true, s.Downloads[0].ID
	tt := ui.NewTester(s.view, 1040, 720)
	tt.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})

	uiRequireText(t, tt, "删除任务和文件？", "已下载文件会一并删除，无法恢复。")
	for _, action := range []string{"保留任务", "确认删除并重新下载", "确认删除任务和文件"} {
		uiScrollTo(t, tt, action)
	}
	uiClick(t, tt, "保留任务")
	if s.RemovePrompt {
		t.Fatal("keeping the task left the confirmation dialog open")
	}
}

func TestUIEpisodeFixtureKeepsReleaseNumbers(t *testing.T) {
	s := uiFixture(t, "episodes")
	s.LibraryID = s.Subscriptions[0].ID
	s.Downloads = append(s.Downloads, Download{
		ID: "render-download-unknown", AnimeID: s.AnimeID, Title: "[回归测试组甲] 星海旅记 [1080p][特典].mkv",
		Folder: s.Downloads[0].Folder, MB: 120, Progress: 1,
	})
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "第 1 集", "第 2 集", "未识别集数")
}
