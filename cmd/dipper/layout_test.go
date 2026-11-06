package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func layoutFixture(t *testing.T, page string) *State {
	t.Helper()
	state := uiFixture(t, page)
	state.Appearance = appearanceLight
	return state
}

func requireLayoutBound(t *testing.T, tester *ui.Tester, label string, windowW, windowH float32) ui.Rect {
	t.Helper()
	bounds, found := tester.Find(label)
	if !found || bounds.W <= 0 || bounds.H <= 0 {
		t.Fatalf("%q has no rendered bounds: %+v found=%v", label, bounds, found)
	}
	if bounds.X < 0 || bounds.Y < 0 || bounds.X+bounds.W > windowW || bounds.Y+bounds.H > windowH {
		t.Fatalf("%q is outside the window: %+v", label, bounds)
	}
	return bounds
}

func TestTypographyPagesReflowWithinMinimumWindow(t *testing.T) {
	type pageCase struct {
		name, page, title string
		prepare           func(*State)
	}
	longTitle := strings.Repeat("星海旅记与失落星图的旅程", 4)
	cases := []pageCase{
		{name: "browse", page: "browse", title: "2025 年春季番剧"},
		{name: "detail", page: "detail", title: longTitle},
		{name: "following", page: "following", title: "搜索追番"},
		{name: "downloads", page: "downloads", title: "搜索下载"},
		{name: "settings", page: "settings", title: "外观与动效"},
		{name: "episodes", page: "episodes", title: longTitle, prepare: func(state *State) {
			state.LibraryID = state.Subscriptions[0].ID
		}},
	}
	for _, tc := range cases {
		for _, dark := range []bool{false, true} {
			mode := "light"
			if dark {
				mode = "dark"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				state := layoutFixture(t, tc.page)
				if tc.prepare != nil {
					tc.prepare(state)
				}
				if tc.page == "detail" || tc.page == "episodes" {
					anime := state.anime(state.Subscriptions[0].AnimeID)
					anime.Title = longTitle
					state.KnownAnime[anime.ID] = anime
					for index := range state.Catalog {
						if state.Catalog[index].ID == anime.ID {
							state.Catalog[index] = anime
						}
					}
				}
				tester := ui.NewTester(state.view, 1040, 720)
				tester.SetDark(dark)
				tester.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
				title := requireLayoutBound(t, tester, tc.title, 1040, 720)
				if tc.name != "settings" && title.H <= 40 {
					t.Fatalf("scaled title did not grow: %+v", title)
				}
			})
		}
	}
}

func TestTypographyDialogsRemainScrollableAtMinimumWindow(t *testing.T) {
	longBody := strings.Repeat("A long English sentence keeps the dialog readable without an arbitrary line limit. ", 12)
	cases := []struct {
		name, page, title, action string
		dark                      bool
		prepare                   func(*State)
	}{
		{name: "help/light", page: "detail", title: "怎么追番", action: "关闭指南", prepare: func(state *State) { state.Help = true }},
		{name: "help/dark", page: "detail", title: "怎么追番", action: "关闭指南", dark: true, prepare: func(state *State) { state.Help = true }},
		{name: "summary", page: "detail", title: "星海旅记", action: "关闭", prepare: func(state *State) { state.SummaryOpen = true }},
		{name: "remove", page: "downloads", title: "删除任务和文件？", action: "保留任务", prepare: func(state *State) {
			state.RemovePrompt, state.RemoveID = true, state.Downloads[0].ID
		}},
		{name: "close", page: "downloads", title: "关闭 Dipper？", action: "取消", prepare: func(state *State) {
			state.Platform, state.TrayAvailable, state.ClosePrompt = "windows", true, true
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := layoutFixture(t, tc.page)
			tc.prepare(state)
			state.SourceError = longBody
			if state.SummaryOpen {
				anime := state.anime(state.AnimeID)
				anime.Summary = longBody
				state.KnownAnime[anime.ID] = anime
			}
			tester := ui.NewTester(state.view, 1040, 720)
			tester.SetDark(tc.dark)
			tester.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
			requireLayoutBound(t, tester, tc.title, 1040, 720)
			uiScrollTo(t, tester, tc.action)
		})
	}
}

func TestUXSidebarDropsDecorativeLocalNoteAtLargeText(t *testing.T) {
	state := layoutFixture(t, "browse")

	normal := ui.NewTester(state.view, 1040, 720)
	normal.SetPreferences(ui.Preferences{TextScale: 1, ReduceMotion: true})
	if _, found := normal.Find("保存在本机"); !found {
		t.Fatal("the local-storage note should remain visible at normal text size")
	}

	large := ui.NewTester(state.view, 1040, 720)
	large.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
	if _, found := large.Find("保存在本机"); found {
		t.Fatal("the decorative local-storage note should not fragment the sidebar at 200% text")
	}
}
