package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAppearanceThemePreservesTextScaleAcrossModes(t *testing.T) {
	for _, appearance := range []string{appearanceLight, appearanceDark, appearanceSystem} {
		t.Run(appearance, func(t *testing.T) {
			state := uiFixture(t, "following")
			state.Appearance = appearance
			var size float32
			tester := ui.NewTester(func(c *ui.Context) {
				state.view(c)
				size = c.Theme().FontSize
			}, 1040, 720)
			for _, scale := range []float32{1, 1.5, 2, 1.25} {
				tester.SetPreferences(ui.Preferences{TextScale: scale, ReduceMotion: true})
				if size != 14*scale {
					t.Errorf("theme size=%v at scale=%v, want %v", size, scale, 14*scale)
				}
				tester.SetDark(true)
				tester.Frame()
				if size != 14*scale {
					t.Errorf("theme switch lost scale: size=%v scale=%v", size, scale)
				}
			}
		})
	}
}

func TestTypographySharedRolesScaleRenderedText(t *testing.T) {
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(500).Children(func() {
			heading(c, "章节标题")
			caption(c, "辅助说明")
			quiet(c, "action", "操作", func() {})
		})
	}, 1040, 720)
	base := map[string]ui.Rect{}
	for _, label := range []string{"章节标题", "辅助说明", "操作"} {
		base[label], _ = tester.Find(label)
	}
	tester.SetPreferences(ui.Preferences{TextScale: 2})
	for label, before := range base {
		after, found := tester.Find(label)
		if !found || after.H < before.H*1.6 {
			t.Errorf("%s did not grow with text preference: before=%+v after=%+v", label, before, after)
		}
	}
}

func TestTypographyPosterTitlesAndGroupsReflowAndPosterOpensLibrary(t *testing.T) {
	for _, kind := range []string{"browse", "following"} {
		t.Run(kind, func(t *testing.T) {
			state := uiFixture(t, kind)
			title := strings.Repeat("每周一起寻找失落的星图", 8)
			group := strings.Repeat("星海联合字幕制作组", 5)
			sub := state.Subscriptions[0]
			sub.GroupName = group
			anime := state.anime(sub.AnimeID)
			anime.Title = title
			state.KnownAnime[anime.ID] = anime
			for index := range state.Catalog {
				if state.Catalog[index].ID == anime.ID {
					state.Catalog[index] = anime
				}
			}
			tester := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(spectrumTheme(false))
				ui.Scroll(c).Fill().Children(func() {
					if kind == "browse" {
						state.animeCard(c, anime, 240)
					} else {
						state.followTile(c, sub, 240)
					}
				})
			}, 1040, 720)
			before, found := tester.Find(title)
			if !found || before.H <= 39 {
				t.Fatalf("long title still clipped: %+v found=%v", before, found)
			}
			tester.SetPreferences(ui.Preferences{TextScale: 2})
			after, _ := tester.Find(title)
			if after.H <= before.H*1.6 || after.W > 240 {
				t.Errorf("scaled title did not reflow within poster: before=%+v after=%+v", before, after)
			}
			if kind == "following" {
				tester.Scroll(120, 600, 0, 2000)
				bounds, found := tester.Find(group)
				if !found || bounds.H <= 30 || bounds.W > 240 {
					t.Errorf("long group did not wrap: %+v found=%v", bounds, found)
				}
				tester.Scroll(120, 100, 0, -2000)
				tester.ClickAt(120, 100)
				if state.Page != "episodes" || state.LibraryID != sub.ID {
					t.Errorf("poster click did not open library: page=%s id=%s", state.Page, state.LibraryID)
				}
			}
		})
	}
}

func TestTypographyLibraryHeroReflowsAndScrollsAtMinimumWindow(t *testing.T) {
	state := uiFixture(t, "episodes")
	sub := &state.Subscriptions[0]
	state.LibraryID = sub.ID
	title := strings.Repeat("关于我们每周一起寻找失落星图的故事", 5)
	sub.GroupName = strings.Repeat("联合字幕制作组", 5)
	sub.CheckFailed, sub.LastError = true, strings.Repeat("检查更新失败，请稍后重试。", 3)
	anime := state.anime(sub.AnimeID)
	anime.Title = title
	state.KnownAnime[anime.ID] = anime
	for index := range state.Catalog {
		if state.Catalog[index].ID == anime.ID {
			state.Catalog[index] = anime
		}
	}
	tester := ui.NewTester(state.view, 1040, 720)
	tester.SetPreferences(ui.Preferences{TextScale: 2, ReduceMotion: true})
	titleBounds, found := tester.Find(title)
	if !found || titleBounds.H <= 104 || titleBounds.X+titleBounds.W > 1040 {
		t.Fatalf("hero title clipped or overflowed: %+v found=%v", titleBounds, found)
	}
	more, found := tester.Find("更多操作")
	if !found || more.Y < 64 || more.Y+more.H > 720 {
		t.Fatalf("hero menu unreachable: %+v", more)
	}
	label := "重试检查 " + sub.GroupName
	before, found := tester.Find(label)
	if !found {
		t.Fatal("missing hero retry action")
	}
	tester.Scroll(titleBounds.X+titleBounds.W/2, titleBounds.Y+30, 0, 2000)
	after, _ := tester.Find(label)
	if after.Y >= before.Y || after.Y < 64 || after.Y+after.H > 720 {
		t.Errorf("hero cannot scroll to its last action: before=%+v after=%+v", before, after)
	}
	play, found := tester.Find("播放 render-download-3")
	if !found {
		t.Fatal("missing episode play action")
	}
	tester.Scroll(700, 600, 0, 2000)
	play, found = tester.Find("播放 render-download-3")
	if !found || play.W <= 0 || play.Y+play.H > 720 || play.Y < 64 {
		t.Errorf("episode list cannot scroll to its play action: %+v found=%v", play, found)
	}
	t.Logf("title=%+v menu=%+v retry before=%+v after=%+v play=%+v", titleBounds, more, before, after, play)
	if output := os.Getenv("DIPPER_VISUAL_EVIDENCE"); output != "" {
		if err := os.MkdirAll(output, 0755); err != nil {
			t.Fatal(err)
		}
		if err := writeUIScreen(filepath.Join(output, fmt.Sprintf("library-text200-%dx%d.png", 1040, 720)), tester); err != nil {
			t.Fatal(err)
		}
	}
}
