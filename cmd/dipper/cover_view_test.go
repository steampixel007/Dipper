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

func TestUICoverPreservesEveryEdge(t *testing.T) {
	data, err := assets.ReadFile("assets/3887.jpg")
	if err != nil {
		t.Fatal(err)
	}
	photo, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, landscape := range []bool{false, true} {
		orientation := "portrait"
		w, h := photo.Bounds().Dx(), photo.Bounds().Dy()
		if landscape {
			orientation = "landscape"
			w, h = h, w
		}
		poster := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				if landscape {
					poster.Set(x, y, photo.At(y, x))
				} else {
					poster.Set(x, y, photo.At(x, y))
				}
			}
		}
		// Edge markers on a real poster make clipping visible independently of
		// the photograph's colors and of the renderer's resampling filter.
		colors := []color.RGBA{{240, 30, 30, 255}, {30, 180, 30, 255}, {30, 30, 240, 255}, {240, 180, 30, 255}}
		for i, r := range []image.Rectangle{
			image.Rect(0, 0, w, h/10), image.Rect(0, h-h/10, w, h),
			image.Rect(0, 0, w/10, h), image.Rect(w-w/10, 0, w, h),
		} {
			draw.Draw(poster, r, image.NewUniform(colors[i]), image.Point{}, draw.Src)
		}
		for _, size := range [][2]int{{180, 245}, {52, 72}, {68, 94}} {
			t.Run(fmt.Sprintf("%s/%dx%d", orientation, size[0], size[1]), func(t *testing.T) {
				url := t.Name()
				covers[url] = ui.NewBitmap(poster)
				t.Cleanup(func() { delete(covers, url) })
				tt := ui.NewTester(func(c *ui.Context) {
					cover(c, Anime{Cover: url}, float32(size[0]), float32(size[1]))
				}, size[0], size[1])
				scale := min(float64(size[0])/float64(w), float64(size[1])/float64(h))
				iw, ih := float64(w)*scale, float64(h)*scale
				x0, y0 := (float64(size[0])-iw)/2, (float64(size[1])-ih)/2
				margin := image.Pt(0, size[1]/2)
				if y0 > x0 {
					margin = image.Pt(size[0]/2, 0)
				}
				if max(x0, y0) >= 1 {
					if got := tt.Image().RGBAAt(margin.X, margin.Y); got != (color.RGBA{245, 249, 255, 255}) {
						t.Errorf("unused cover space at %v = %v; want blueWash without cropping or stretching", margin, got)
					}
				}
				for i, point := range [][2]float64{{.5, .05}, {.5, .95}, {.05, .5}, {.95, .5}} {
					x, y := int(x0+point[0]*iw), int(y0+point[1]*ih)
					got := tt.Image().RGBAAt(x, y)
					want := colors[i]
					if absChannel(got.R, want.R)+absChannel(got.G, want.G)+absChannel(got.B, want.B) > 30 {
						t.Errorf("cover edge %d cropped or stretched at (%d,%d): got %v, want %v", i, x, y, got, want)
					}
				}
			})
		}
	}
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
	bluePixels := map[image.Point]bool{}
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			p := frame.RGBAAt(x, y)
			if int(p.B) > int(p.R)+70 && int(p.B) > int(p.G)+50 {
				bluePixels[image.Pt(x, y)] = true
			}
		}
	}
	components := 0
	for start := range bluePixels {
		if !bluePixels[start] {
			continue
		}
		components++
		queue := []image.Point{start}
		delete(bluePixels, start)
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			for _, step := range []image.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
				next := p.Add(step)
				if bluePixels[next] {
					delete(bluePixels, next)
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
	if _, ok := tt.Find("正在加载封面"); ok {
		t.Error("loading indicator remains after cover loading ends")
	}
}
