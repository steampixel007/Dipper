package main

import (
	"maps"
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIGroupSearchSelectsOriginalSourceGroup(t *testing.T) {
	for _, appearance := range []string{appearanceLight, appearanceDark} {
		t.Run(appearance, func(t *testing.T) {
			state := uiFixture(t, "detail")
			state.Appearance, state.Group = appearance, -1
			state.Groups = []string{"ANi", "银色子弹字幕组", "LoliHouse", "幻樱字幕组"}
			state.GroupCounts = []int{3, -1, 0, 18}
			tester := ui.NewTester(state.view, 1120, 760)
			uiClick(t, tester, "搜索字幕组")
			tester.Type("幻樱")
			uiRequireText(t, tester, "找到 1 / 4 个字幕组", "18 条资源", "幻樱字幕组")
			if tester.HasText("选择字幕组 ANi") {
				t.Fatal("search did not remove nonmatching choices")
			}
			uiClick(t, tester, "选择字幕组 幻樱字幕组")
			if state.Group != 3 || state.GroupIDs[state.Group] != "render-group-4" || state.ChoosingGroup {
				t.Fatal("filtered group click selected the filtered index instead of the source group")
			}
			state.openAnime(state.Catalog[1].ID)
			if state.GroupSearch != "" {
				t.Fatal("opening another anime retained the previous subtitle-group search")
			}
		})
	}
}

func TestUIGroupSearchPreservesCurrentDraftAndRecovers(t *testing.T) {
	state := uiFixture(t, "detail")
	state.Groups[0] = "ANi"
	state.ChoosingGroup = true
	state.setWord("1080p", wordWant)
	state.ResourceScroll.Y = 80
	tester := ui.NewTester(state.view, 1320, 860)
	checked, rule, scroll := maps.Clone(state.Checked), cloneRule(state.Rule), state.ResourceScroll
	uiClick(t, tester, "搜索字幕组")
	tester.Type("  ａｎＩ  ")
	uiRequireText(t, tester, "找到 1 / 4 个字幕组", "选择字幕组 ANi", "3 条资源")
	if state.Group != 0 || !reflect.DeepEqual(state.Checked, checked) || !reflect.DeepEqual(state.Rule, rule) || !reflect.DeepEqual(state.ResourceScroll, scroll) {
		t.Fatal("searching changed the active group, rule, selection or resource scroll")
	}
	uiClick(t, tester, "选择字幕组 ANi")
	if state.ChoosingGroup || !reflect.DeepEqual(state.Checked, checked) || !reflect.DeepEqual(state.Rule, rule) || !reflect.DeepEqual(state.ResourceScroll, scroll) {
		t.Fatal("reselecting the current group reset the release draft or scroll position")
	}
	uiClick(t, tester, "更换")
	uiClick(t, tester, "清除搜索")
	uiRequireText(t, tester, "4 个字幕组", "选择字幕组 "+state.Groups[3])
	uiClick(t, tester, "搜索字幕组")
	tester.Type("不存在的字幕组")
	uiRequireText(t, tester, "找到 0 / 4 个字幕组", "没有找到这个字幕组，试试简称或清除搜索。")
	uiClick(t, tester, "清除搜索")
	if state.GroupSearch != "" || state.Group != 0 || !reflect.DeepEqual(state.Checked, checked) || !reflect.DeepEqual(state.Rule, rule) {
		t.Fatal("clearing an empty search changed the current version draft")
	}
}

func TestUIGroupCountsDoNotInventMissingTotals(t *testing.T) {
	for _, counts := range [][]int{{0, -1, 23}, nil} {
		state := uiFixture(t, "detail")
		state.Group, state.GroupCounts = -1, counts
		tester := ui.NewTester(state.view, 1320, 860)
		uiRequireText(t, tester, "查看发布记录")
		if counts == nil {
			if tester.HasText("0 条资源") {
				t.Fatal("missing metadata was presented as an empty resource history")
			}
		} else {
			uiRequireText(t, tester, "0 条资源", "23 条资源")
			if tester.HasText("-1 条资源") {
				t.Fatal("truncated history was presented as a negative total")
			}
		}
	}
}

func TestUIGroupSearchRemainsClearableAfterListShrinks(t *testing.T) {
	for _, groups := range [][]string{{"ANi"}, nil} {
		state := uiFixture(t, "detail")
		state.Group, state.GroupSearch = -1, "之前搜索的字幕组"
		state.Groups = groups
		tester := ui.NewTester(state.view, 1120, 760)
		uiRequireText(t, tester, "没有找到这个字幕组，试试简称或清除搜索。")
		uiClick(t, tester, "清除搜索")
		if state.GroupSearch != "" {
			t.Fatal("a reduced group list trapped the previous search")
		}
		if len(groups) == 1 {
			uiRequireText(t, tester, "选择字幕组 ANi")
		} else {
			uiRequireText(t, tester, "来源暂未提供字幕组。")
		}
	}
}
