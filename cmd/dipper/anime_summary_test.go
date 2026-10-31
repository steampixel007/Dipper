package main

import (
	"fmt"
	"maps"
	"reflect"
	"strings"
	"testing"

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

func TestUIAnimeSummaryReadingPreservesDraft(t *testing.T) {
	for _, appearance := range []string{appearanceLight, appearanceDark} {
		for _, size := range [][2]int{{1120, 760}, {1320, 860}} {
			t.Run(fmt.Sprintf("%s/%dx%d", appearance, size[0], size[1]), func(t *testing.T) {
				state := uiFixture(t, "detail")
				state.Appearance = appearance
				state.setWord("1080p", wordWant)
				anime := state.KnownAnime[state.AnimeID]
				anime.Summary = strings.Repeat(anime.Summary, 30) + "故事仍在继续。"
				state.KnownAnime[state.AnimeID] = anime
				state.ResourceScroll.Y = 80
				tester := ui.NewTester(state.view, size[0], size[1])
				checked := maps.Clone(state.Checked)
				rule := cloneRule(state.Rule)
				scroll := state.ResourceScroll
				if scroll.Y <= 0 {
					t.Fatal("reading test must start with a scrolled release list")
				}
				uiRequireText(t, tester, "番剧简介", anime.Summary)
				uiClick(t, tester, "查看完整简介")
				if !state.SummaryOpen {
					t.Fatal("reading action did not open the synopsis")
				}
				uiRequireText(t, tester, "简介来自蜜柑计划", anime.Summary)
				footer, ok := tester.Find("简介来自蜜柑计划")
				if !ok || footer.Y < 0 || footer.Y+footer.H > float32(size[1]) {
					t.Fatal("long synopsis pushed the reading controls outside the window")
				}
				uiClick(t, tester, "关闭简介")
				if state.SummaryOpen || !reflect.DeepEqual(state.Checked, checked) || !reflect.DeepEqual(state.Rule, rule) || !reflect.DeepEqual(state.ResourceScroll, scroll) {
					t.Fatal("reading the synopsis changed the release selection, filter or scroll position")
				}
				state.SummaryOpen = true
				state.openAnime(state.Catalog[1].ID, "browse")
				if state.SummaryOpen {
					t.Fatal("opening another anime retained the previous reading dialog")
				}
			})
		}
	}
}

func TestUIAnimeSummaryAvailability(t *testing.T) {
	for _, scenario := range []struct {
		name, summary, failure, want string
		loading                      bool
	}{
		{name: "missing", want: "来源暂未提供简介。"},
		{name: "loading", loading: true, want: "正在加载简介…"},
		{name: "failed", failure: "网络中断", want: "简介暂时无法加载，可重试下方的详情。"},
		{name: "cached while loading", summary: "已经加载的简介", loading: true, want: "已经加载的简介"},
		{name: "cached after failure", summary: "已经加载的简介", failure: "网络中断", want: "已经加载的简介"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := uiFixture(t, "detail")
			anime := state.KnownAnime[state.AnimeID]
			anime.Summary = scenario.summary
			state.KnownAnime[state.AnimeID] = anime
			state.DetailLoading, state.DetailError = scenario.loading, scenario.failure
			tester := ui.NewTester(state.view, 1120, 760)
			uiRequireText(t, tester, scenario.want)
			if tester.HasText("查看完整简介") != (scenario.summary != "") {
				t.Fatal("reading action does not match synopsis availability")
			}
		})
	}
}

func TestAnimeSummarySurvivesSourceProjection(t *testing.T) {
	anime := fromAnime(source.Anime{ID: "227", Title: "名侦探柯南", Summary: "主角工藤新一原本是一位高中生侦探。"})
	state := uiFixture(t, "detail")
	state.AnimeID = anime.ID
	state.KnownAnime[anime.ID] = anime
	tester := ui.NewTester(state.view, 1120, 760)
	uiRequireText(t, tester, "主角工藤新一原本是一位高中生侦探。", "查看完整简介")
}

func TestCoreRefreshRetainsLoadedAnimeSummary(t *testing.T) {
	state := uiFixture(t, "detail")
	anime := state.anime(state.AnimeID)
	state.project(core.State{Anime: []core.AnimeRecord{{Anime: core.Anime{ID: anime.ID, Title: "来源更新后的标题", Cover: anime.Cover}}}}, nil)
	tester := ui.NewTester(state.view, 1120, 760)
	uiRequireText(t, tester, "来源更新后的标题", anime.Summary, "查看完整简介")
}
