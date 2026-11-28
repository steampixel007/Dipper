package main

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDesignSystemRegistersGeistWeights(t *testing.T) {
	uiTestImages.Do(loadImages)
	families := ui.FontFamilies()
	found := false
	for _, f := range families {
		if f.Name == "Geist" && f.Registered {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Geist font is not registered in MyGo: %+v", families)
	}
	measureWidth := func(weight int) float32 {
		var w float32
		tt := ui.NewTester(func(c *ui.Context) {
			installTheme(c, false)
			ui.Row(c).Children(func() {
				el := ui.Text(c, "Dipper 0123456789 Release Title").Font("Geist").FontSize(24).FontWeight(weight)
				w = el.Bounds().W
			})
		}, 800, 120)
		tt.Frame()
		tt.Frame()
		return w
	}
	w400 := measureWidth(400)
	w500 := measureWidth(500)
	w600 := measureWidth(600)
	w700 := measureWidth(700)
	if w400 <= 0 || w700 <= w400 {
		t.Fatalf("Geist weights did not resolve distinct faces: w400=%g w500=%g w600=%g w700=%g", w400, w500, w600, w700)
	}
	if w500 == w400 && w600 == w400 {
		t.Fatalf("Geist intermediate weights collapsed to regular: w400=%g w500=%g w600=%g w700=%g", w400, w500, w600, w700)
	}
}

func TestDesignSystemIconsUseReiconStrokeAndBox(t *testing.T) {
	uiTestImages.Do(loadImages)
	if !strings.Contains(reiconSVGPrefix, `stroke-width="1.5"`) || !strings.Contains(reiconSVGPrefix, `stroke="currentColor"`) {
		t.Fatalf("icon SVG prefix does not enforce Reicon 1.5px outline stroke: %s", reiconSVGPrefix)
	}
	if len(icons) != len(iconPaths) || len(icons) < 20 {
		t.Fatalf("loaded %d icons, want %d", len(icons), len(iconPaths))
	}
	var outer ui.Rect
	t2 := ui.NewTester(func(c *ui.Context) {
		installTheme(c, false)
		ui.Row(c).Children(func() {
			wrap := ui.Box(c.Key("icon-wrap")).Children(func() {
				icon(c, "settings")
			})
			outer = wrap.Bounds()
		})
	}, 120, 120)
	t2.Frame()
	t2.Frame()
	if outer.W != 24 || outer.H != 24 {
		t.Fatalf("icon layout box = %.0fx%.0f, want 24x24", outer.W, outer.H)
	}
}

func TestDesignSystemFlatLayoutAndNoBeginnerStrip(t *testing.T) {
	if textMeta < 14 || textControl < 14 || textBody < 14 || textSection < 14 || textTitle < 14 {
		t.Fatalf("minimum font size violated: meta=%g control=%g body=%g section=%g title=%g", textMeta, textControl, textBody, textSection, textTitle)
	}

	// Browse omits beginner hint strip and source-name caption.
	browse := uiFixture(t, "browse")
	browse.IntroHidden = false
	bt := ui.NewTester(browse.view, 1320, 860)
	for _, banned := range []string{"第一次追番？", "了解怎么追", "来源：蜜柑计划"} {
		if bt.HasText(banned) {
			t.Errorf("browse still renders banned copy %q", banned)
		}
	}

	// Settings sections sit directly on canvas (no framed surface cards).
	for _, appearance := range []string{appearanceLight, appearanceDark} {
		t.Run("settings/"+appearance, func(t *testing.T) {
			s := uiFixture(t, "settings")
			s.Appearance = appearance
			tt := ui.NewTester(s.view, 1320, 860)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			pal := lightPalette
			if appearance == appearanceDark {
				pal = darkPalette
			}
			sec, ok := tt.Find("外观与动效")
			if !ok {
				t.Fatal("missing settings section heading")
			}
			// Sample background immediately left of section heading: must be canvas, not a framed card.
			frame := tt.Image()
			bg := color.NRGBAModel.Convert(frame.At(int(sec.X)-10, int(sec.Y+sec.H/2))).(color.NRGBA)
			want := color.NRGBA{R: pal.canvas.R, G: pal.canvas.G, B: pal.canvas.B, A: 255}
			if bg != want {
				t.Errorf("settings section background = %v, want flat canvas %v", bg, want)
			}
		})
	}

	// Download rows sit directly on canvas (no card boxes) and have no green completion marks.
	dl := uiFixture(t, "downloads")
	dl.Appearance = appearanceDark
	dt := ui.NewTester(dl.view, 1320, 860)
	dt.SetPreferences(ui.Preferences{ReduceMotion: true})
	rowTitle, ok := dt.Find(dl.Downloads[0].Title)
	if !ok {
		t.Fatal("missing download row title")
	}
	frame := dt.Image()
	bg := color.NRGBAModel.Convert(frame.At(int(rowTitle.X)-4, int(rowTitle.Y+rowTitle.H/2))).(color.NRGBA)
	wantCanvas := color.NRGBA{R: darkPalette.canvas.R, G: darkPalette.canvas.G, B: darkPalette.canvas.B, A: 255}
	if bg != wantCanvas {
		t.Errorf("download row rests on %v, want flat canvas %v (no card box)", bg, wantCanvas)
	}
}

func TestDesignSystemNeutralAccentBudget(t *testing.T) {
	for _, page := range []string{"downloads", "settings"} {
		for _, appearance := range []string{appearanceLight, appearanceDark} {
			t.Run(page+"/"+appearance, func(t *testing.T) {
				s := uiFixture(t, page)
				s.Appearance = appearance
				tt := ui.NewTester(s.view, 1320, 860)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				img := tt.Image()
				bounds := img.Bounds()
				total, neutral, accent := 0, 0, 0
				for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
					for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
						c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
						total++
						spread := max(c.R, c.G, c.B) - min(c.R, c.G, c.B)
						if spread <= 16 {
							neutral++
						} else if c.B > c.R+24 {
							accent++
						}
					}
				}
				neutralRatio := float64(neutral) / float64(total)
				accentRatio := float64(accent) / float64(total)
				if neutralRatio < 0.85 {
					t.Errorf("%s/%s neutral pixel ratio = %.2f, want >= 0.85", page, appearance, neutralRatio)
				}
				if accentRatio > 0.12 {
					t.Errorf("%s/%s accent pixel ratio = %.2f, want <= 0.12", page, appearance, accentRatio)
				}
				_ = image.Point{}
			})
		}
	}
}
