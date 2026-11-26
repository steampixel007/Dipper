package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIPosterTilesFitArtworkWithoutMargins(t *testing.T) {
	for _, page := range []string{"browse", "following"} {
		for _, dimensions := range [][2]int{{400, 560}, {400, 700}, {700, 400}, {400, 400}} {
			for _, dark := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%dx%d/dark%v", page, dimensions[0], dimensions[1], dark), func(t *testing.T) {
					state := uiFixture(t, page)
					anime := state.Catalog[0]
					anime.Cover = t.Name()
					state.KnownAnime[anime.ID] = anime
					poster := image.NewRGBA(image.Rect(0, 0, dimensions[0], dimensions[1]))
					artwork := color.RGBA{130, 100, 170, 255}
					draw.Draw(poster, poster.Bounds(), image.NewUniform(artwork), image.Point{}, draw.Src)
					edges := []struct {
						rect  image.Rectangle
						color color.RGBA
					}{
						{image.Rect(dimensions[0]/3, 0, dimensions[0]*2/3, 32), color.RGBA{220, 30, 40, 255}},
						{image.Rect(dimensions[0]/3, dimensions[1]-32, dimensions[0]*2/3, dimensions[1]), color.RGBA{30, 60, 220, 255}},
						{image.Rect(0, dimensions[1]/3, 32, dimensions[1]*2/3), color.RGBA{220, 150, 30, 255}},
						{image.Rect(dimensions[0]-32, dimensions[1]/3, dimensions[0], dimensions[1]*2/3), color.RGBA{30, 190, 210, 255}},
					}
					for _, edge := range edges {
						draw.Draw(poster, edge.rect, image.NewUniform(edge.color), image.Point{}, draw.Src)
					}
					t.Cleanup(func() { delete(covers, anime.Cover) })
					const width = 180
					tester := ui.NewTester(func(c *ui.Context) {
						installTheme(c, dark)
						if page == "browse" {
							state.animeCard(c, anime, width)
						} else {
							state.followTile(c, Subscription{ID: "poster", AnimeID: anime.ID, GroupName: "字幕组"}, width)
						}
					}, width, 600)
					tester.SetPreferences(ui.Preferences{ReduceMotion: true})
					if _, ok := tester.Find("正在加载封面"); !ok {
						t.Fatal("unloaded poster must show the loading indicator")
					}
					covers[anime.Cover] = ui.NewBitmap(poster)
					tester.Frame()
					if _, ok := tester.Find("正在加载封面"); ok {
						t.Fatal("loaded poster still shows the loading indicator")
					}
					height := float32(width) * float32(dimensions[1]) / float32(dimensions[0])
					frame := tester.Image()
					for _, edge := range edges {
						centerX := float32(edge.rect.Min.X+edge.rect.Max.X) / 2
						centerY := float32(edge.rect.Min.Y+edge.rect.Max.Y) / 2
						point := image.Pt(int(centerX*width/float32(dimensions[0])), int(centerY*width/float32(dimensions[0])))
						if got := frame.RGBAAt(point.X, point.Y); !near(got, edge.color, 45) {
							t.Errorf("poster must reach its natural bounds without cropping or margins at %v: got %v, want %v", point, got, edge.color)
						}
					}
					title, ok := tester.Find(anime.Title)
					if !ok {
						t.Fatal("poster title missing")
					}
					if gap := title.Y - height; gap < 11 || gap > 13 {
						t.Errorf("title must follow the artwork without an empty frame: artwork height=%g title=%+v", height, title)
					}
				})
			}
		}
	}
}

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
