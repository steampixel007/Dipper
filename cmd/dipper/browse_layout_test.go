package main

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIBrowseCardWeekdayOnlyInAllDays(t *testing.T) {
	for day := -1; day <= 6; day++ {
		t.Run(fmt.Sprint(day), func(t *testing.T) {
			state := uiFixture(t, "browse")
			state.Day = day
			state.Subscriptions = nil
			state.Downloads = nil
			anime := state.Catalog[0]
			if day >= 0 {
				anime.Day = day
			}
			tester := ui.NewTester(func(c *ui.Context) {
				state.animeCard(c, anime, 180)
			}, 300, 500)
			if got, want := tester.HasText(dayName(anime.Day)), day == -1; got != want {
				t.Fatalf("card weekday visible = %v, want %v for day filter %d", got, want, day)
			}
			if got, want := tester.HasText("·"), day == -1; got != want {
				t.Fatalf("weekday separator visible = %v, want %v for day filter %d", got, want, day)
			}
			uiRequireText(t, tester, anime.Title, "选择版本")
		})
	}
}

func TestUIBrowseFiltersShareOneCompactBand(t *testing.T) {
	state := uiFixture(t, "browse")
	tester := ui.NewTester(state.view, 1040, 720)
	tester.SetPreferences(ui.Preferences{TextScale: 1, ReduceMotion: true})

	search := requireLayoutBound(t, tester, "搜索番剧", 1040, 720)
	season := requireLayoutBound(t, tester, "春季", 1040, 720)
	if delta := (search.Y + search.H/2) - (season.Y + season.H/2); delta > 8 || delta < -8 {
		t.Fatalf("search and season controls sit on different bands: search=%+v season=%+v", search, season)
	}

	days := requireLayoutBound(t, tester, "全部", 1040, 720)
	if gap := days.Y - (season.Y + season.H); gap > 26 {
		t.Fatalf("weekday filter floats %.0f DIP below the season band: season=%+v days=%+v", gap, season, days)
	}
	firstCard := requireLayoutBound(t, tester, "查看 星海旅记", 1040, 720)
	if stack := firstCard.Y - search.Y; stack > 180 {
		t.Fatalf("browse chrome pushes the first poster %.0f DIP down: search=%+v card=%+v", stack, search, firstCard)
	}
}

func TestUIBrowseControlsStayInsideAtDoubleTextScale(t *testing.T) {
	state := uiFixture(t, "browse")
	tester := ui.NewTester(state.view, 1040, 720)
	tester.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
	for _, label := range []string{"搜索番剧", "冬季", "春季", "夏季", "秋季", "全部", "周日", "来源：蜜柑计划", "回到当季"} {
		requireLayoutBound(t, tester, label, 1040, 720)
	}
}
