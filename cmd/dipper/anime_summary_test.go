package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
	"github.com/steampixel007/Dipper/internal/source"
)

func TestUIGroupSetupAndResourcesKeepSynopsisInBrowse(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		textScale float32
		resources bool
	}{
		{name: "group setup", textScale: 1},
		{name: "group setup with large text", textScale: 2},
		{name: "followed resources", textScale: 1, resources: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := uiFixture(t, "detail")
			title := state.anime(state.AnimeID)
			state.ViewResources = scenario.resources
			if !scenario.resources {
				state.Group = -1
			}
			tester := ui.NewTester(state.view, 1320, 860)
			tester.SetPreferences(ui.Preferences{TextScale: scenario.textScale, ReduceMotion: true})
			uiRequireText(t, tester, title.Title, "字幕组", "可下载资源")
			for _, text := range []string{"番剧简介", title.Summary, "查看完整简介"} {
				if tester.HasText(text) {
					t.Errorf("subtitle group or resource page repeats the synopsis: %q", text)
				}
			}
		})
	}
}

func TestUIAnimeSummaryAvailability(t *testing.T) {
	for _, scenario := range []struct {
		name, summary, failure, want string
		loading                      bool
	}{
		{name: "missing", want: "暂无简介"},
		{name: "loading", loading: true, want: "正在加载简介…"},
		{name: "failed", failure: "网络中断", want: "简介加载失败"},
		{name: "cached while loading", summary: "已经加载的简介", loading: true, want: "已经加载的简介"},
		{name: "cached after failure", summary: "已经加载的简介", failure: "网络中断", want: "已经加载的简介"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			state := uiFixture(t, "browse")
			anime := state.Catalog[0]
			delete(state.KnownAnime, anime.ID)
			anime.Summary = scenario.summary
			state.openBrowseDetail(anime)
			state.browseDetail.loading, state.browseDetail.failed = scenario.loading, scenario.failure != ""
			tester := ui.NewTester(state.view, 1120, 760)
			uiRequireText(t, tester, scenario.want)
			if tester.HasText("重试") != (scenario.failure != "" && scenario.summary == "") {
				t.Fatal("retry action does not match synopsis availability")
			}
		})
	}
}

func TestAnimeSummarySurvivesSourceProjection(t *testing.T) {
	anime := fromAnime(source.Anime{ID: "227", Title: "名侦探柯南", Summary: "主角工藤新一原本是一位高中生侦探。"})
	state := uiFixture(t, "browse")
	state.openBrowseDetail(anime)
	tester := ui.NewTester(state.view, 1120, 760)
	uiRequireText(t, tester, "名侦探柯南", anime.Summary)
}

func TestCoreRefreshRetainsLoadedAnimeSummary(t *testing.T) {
	state := uiFixture(t, "browse")
	anime := state.anime(state.AnimeID)
	state.project(core.State{Anime: []core.AnimeRecord{{Anime: core.Anime{ID: anime.ID, Title: "来源更新后的标题", Cover: anime.Cover}}}}, nil)
	state.openBrowseDetail(state.anime(anime.ID))
	tester := ui.NewTester(state.view, 1120, 760)
	uiRequireText(t, tester, "来源更新后的标题", anime.Summary)
}
