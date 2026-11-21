package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestUICoverShowsWholePosterWithoutStretching(t *testing.T) {
	gray := color.RGBA{90, 90, 100, 255}
	marker := color.RGBA{30, 190, 40, 255}
	for _, dimensions := range [][2]int{{400, 560}, {400, 700}, {700, 400}, {400, 400}} {
		w, h := dimensions[0], dimensions[1]
		poster := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(poster, poster.Bounds(), image.NewUniform(gray), image.Point{}, draw.Src)
		edges := []struct {
			name  string
			rect  image.Rectangle
			color color.RGBA
		}{
			{"top", image.Rect(w/3, 0, 2*w/3, 32), color.RGBA{220, 30, 40, 255}},
			{"bottom", image.Rect(w/3, h-32, 2*w/3, h), color.RGBA{30, 60, 220, 255}},
			{"left", image.Rect(0, h/3, 32, 2*h/3), color.RGBA{220, 150, 30, 255}},
			{"right", image.Rect(w-32, h/3, w, 2*h/3), color.RGBA{30, 190, 210, 255}},
		}
		for _, edge := range edges {
			draw.Draw(poster, edge.rect, image.NewUniform(edge.color), image.Point{}, draw.Src)
		}
		const arm, bar = 100, 32
		for _, r := range []image.Rectangle{
			image.Rect(w/2-arm, h/2-bar/2, w/2+arm, h/2+bar/2),
			image.Rect(w/2-bar/2, h/2-arm, w/2+bar/2, h/2+arm),
		} {
			draw.Draw(poster, r, image.NewUniform(marker), image.Point{}, draw.Src)
		}
		for _, size := range [][2]int{{180, 245}, {52, 72}, {68, 94}} {
			for _, dark := range []bool{false, true} {
				t.Run(fmt.Sprintf("%dx%d/%dx%d/dark%v", w, h, size[0], size[1], dark), func(t *testing.T) {
					url := t.Name()
					covers[url] = ui.NewBitmap(poster)
					t.Cleanup(func() { delete(covers, url) })
					tt := ui.NewTester(func(c *ui.Context) {
						installTheme(c, dark)
						cover(c, Anime{Cover: url}, float32(size[0]), float32(size[1]))
					}, size[0], size[1])
					frame := tt.Image()
					scale := min(float32(size[0])/float32(w), float32(size[1])/float32(h))
					offsetX := (float32(size[0]) - float32(w)*scale) / 2
					offsetY := (float32(size[1]) - float32(h)*scale) / 2
					for _, edge := range edges {
						centerX := float32(edge.rect.Min.X+edge.rect.Max.X) / 2
						centerY := float32(edge.rect.Min.Y+edge.rect.Max.Y) / 2
						point := image.Pt(int(offsetX+centerX*scale), int(offsetY+centerY*scale))
						if got := frame.RGBAAt(point.X, point.Y); !near(got, edge.color, 45) {
							t.Errorf("%s poster edge is cropped or misplaced at %v: got %v, want %v", edge.name, point, got, edge.color)
						}
					}
					palette := lightPalette
					if dark {
						palette = darkPalette
					}
					background := color.RGBA{palette.sunken.R, palette.sunken.G, palette.sunken.B, palette.sunken.A}
					if offsetX >= 3 {
						for _, point := range []image.Point{{X: 1, Y: size[1] / 2}, {X: size[0] - 2, Y: size[1] / 2}} {
							if got := frame.RGBAAt(point.X, point.Y); !near(got, background, 15) {
								t.Errorf("side margin at %v = %v, want neutral background %v", point, got, background)
							}
						}
					}
					if offsetY >= 3 {
						for _, point := range []image.Point{{X: size[0] / 2, Y: 1}, {X: size[0] / 2, Y: size[1] - 2}} {
							if got := frame.RGBAAt(point.X, point.Y); !near(got, background, 15) {
								t.Errorf("vertical margin at %v = %v, want neutral background %v", point, got, background)
							}
						}
					}
					minX, minY, maxX, maxY := size[0], size[1], 0, 0
					for y := range size[1] {
						for x := range size[0] {
							if near(frame.RGBAAt(x, y), marker, 45) {
								minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
							}
						}
					}
					if minX > maxX {
						t.Fatal("centered cross marker is not visible")
					}
					if armX, armY := maxX-minX+1, maxY-minY+1; max(armX-armY, armY-armX) > 2 {
						t.Errorf("cover cross measures %d×%d; want equal arms (no stretching)", armX, armY)
					}
				})
			}
		}
	}
}

func near(got color.RGBA, want color.RGBA, tol int) bool {
	return absChannel(got.R, want.R)+absChannel(got.G, want.G)+absChannel(got.B, want.B) <= tol
}

func absChannel(a, b uint8) int {
	return max(int(a)-int(b), int(b)-int(a))
}

func TestUICoverLoadingShowsNineBlocks(t *testing.T) {
	s := uiFixture(t, "browse")
	s.Catalog[0].Cover = t.Name()
	url := s.Catalog[0].Cover
	loadingCovers[url] = true
	t.Cleanup(func() { delete(loadingCovers, url) })
	tt := ui.NewTester(s.view, 1320, 860)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	r, ok := tt.Find("正在加载封面")
	if !ok {
		t.Fatal("loading cover has no accessible indicator")
	}
	frame := tt.Image()
	region := image.Rect(int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
	ink := lightPalette.muted
	blockColor := color.RGBA{R: ink.R, G: ink.G, B: ink.B, A: 255}
	blockPixels := map[image.Point]bool{}
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			if frame.RGBAAt(x, y) == blockColor {
				blockPixels[image.Pt(x, y)] = true
			}
		}
	}
	components := 0
	for start := range blockPixels {
		if !blockPixels[start] {
			continue
		}
		components++
		queue := []image.Point{start}
		delete(blockPixels, start)
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			for _, step := range []image.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
				next := p.Add(step)
				if blockPixels[next] {
					delete(blockPixels, next)
					queue = append(queue, next)
				}
			}
		}
	}
	if components != 9 {
		t.Errorf("loading cover shows %d separate shapes; want nine Blocks", components)
	}
	before := append([]byte(nil), frame.Pix...)
	tt.Advance(400 * time.Millisecond)
	if !bytes.Equal(before, tt.Image().Pix) {
		t.Error("loading cover moves despite the system reduce-motion preference")
	}
	delete(loadingCovers, url)
	tt.Frame()
	if _, ok := tt.Find("正在加载封面"); !ok {
		t.Error("missing cover stopped showing the loading indicator")
	}
}

func TestUICoverMissingBitmapKeepsLoading(t *testing.T) {
	uiFixture(t, "following")
	for _, size := range [][2]int{{180, 245}, {52, 72}, {68, 94}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			a := Anime{Cover: t.Name()}
			tt := ui.NewTester(func(c *ui.Context) {
				cover(c, a, float32(size[0]), float32(size[1]))
			}, size[0], size[1])
			if _, ok := tt.Find("正在加载封面"); !ok {
				t.Fatal("cover URL without a bitmap shows the bird instead of loading")
			}
			covers[a.Cover] = ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 4, 4)))
			url := a.Cover
			t.Cleanup(func() { delete(covers, url) })
			tt.Frame()
			if _, ok := tt.Find("正在加载封面"); ok {
				t.Error("loaded cover still shows the loading indicator")
			}
			delete(covers, a.Cover)
			a.Cover = ""
			tt.Frame()
			if _, ok := tt.Find("正在加载封面"); ok {
				t.Error("anime with no cover URL shows a loading indicator")
			}
		})
	}
}
