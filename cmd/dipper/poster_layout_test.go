package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIPosterRowsKeepTitlesAligned(t *testing.T) {
	for _, page := range []string{"browse", "following"} {
		for _, width := range []int{1040, 1304} {
			for _, scale := range []float32{1, 2} {
				t.Run(fmt.Sprintf("%s/%d/scale%g", page, width, scale), func(t *testing.T) {
					state := uiFixture(t, page)
					state.IntroHidden = true
					state.Subscriptions = nil
					state.Downloads = nil
					cols, _ := gridColumns(float32(width))
					for index := range cols {
						anime := &state.Catalog[index]
						if index == 2 {
							anime.Title = "你好，我是受心上人所托来做恋爱药的魔女。"
						}
						state.KnownAnime[anime.ID] = *anime
						state.Subscriptions = append(state.Subscriptions, Subscription{
							ID: fmt.Sprintf("poster-sub-%d", index), AnimeID: anime.ID,
							GroupName: fmt.Sprintf("字幕组%d", index),
						})
					}
					if page == "following" {
						state.Subscriptions[2].GroupName = strings.Repeat("很长的字幕组名称", 3)
						state.Subscriptions[2].Completed = 12
					}
					tester := ui.NewTester(state.view, width, 1600)
					tester.SetPreferences(ui.Preferences{TextScale: scale, ReduceMotion: true})
					firstTitle := requireLayoutBound(t, tester, state.Catalog[0].Title, float32(width), 1600)
					longTitle := requireLayoutBound(t, tester, state.Catalog[2].Title, float32(width), 1600)
					if longTitle.H <= firstTitle.H {
						t.Fatalf("fixture title did not wrap: short=%+v long=%+v", firstTitle, longTitle)
					}
					for _, anime := range state.Catalog[:cols] {
						title := requireLayoutBound(t, tester, anime.Title, float32(width), 1600)
						if delta := title.Y - firstTitle.Y; delta < -1 || delta > 1 {
							t.Errorf("wrapped content shifts neighboring posters: %q title top=%g, first title top=%g", anime.Title, title.Y, firstTitle.Y)
						}
					}
				})
			}
		}
	}
}
