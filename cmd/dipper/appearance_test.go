package main

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

func TestAppearanceSettingsClickPersistsAndAppliesAfterSave(t *testing.T) {
	s := uiFixture(t, "settings")
	s.IntroHidden = true
	s.CloseBehavior = closeBackground
	s.SourceRoute = s.SourceRoutes[1].Label
	var applied []string
	s.ApplyAppearance = func(value string) {
		pref, err := readPreferences(s.ConfigDir)
		if err != nil || pref.Appearance != value {
			t.Errorf("native appearance applied before preference was saved: value=%q preferences=%+v error=%v", value, pref, err)
		}
		applied = append(applied, value)
	}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, appearanceText.title, appearanceText.light, appearanceText.dark, appearanceText.system)
	for i, choice := range []struct{ label, value string }{
		{appearanceText.dark, appearanceDark},
		{appearanceText.system, appearanceSystem},
		{appearanceText.light, appearanceLight},
	} {
		uiClick(t, tt, choice.label)
		if s.Appearance != choice.value || s.SaveError != "" {
			t.Fatalf("click %q: appearance=%q error=%q", choice.label, s.Appearance, s.SaveError)
		}
		if len(applied) != i+1 || applied[i] != choice.value {
			t.Fatalf("click %q did not apply native appearance exactly once: %v", choice.label, applied)
		}
		restarted := newState(s.Dir)
		restarted.ConfigDir = s.ConfigDir
		restarted.Source = source.New("http://127.0.0.1")
		if err := restarted.loadPreferences(); err != nil {
			t.Fatal(err)
		}
		if restarted.Appearance != choice.value {
			t.Fatalf("restart after %q: appearance=%q, want %q", choice.label, restarted.Appearance, choice.value)
		}
		if !restarted.IntroHidden || !restarted.ReduceEffects || restarted.CloseBehavior != closeBackground || restarted.Source.Origin() != s.SourceRoutes[1].Origin {
			t.Fatalf("appearance change lost other preferences: %+v origin=%q", restarted.Saved, restarted.Source.Origin())
		}
	}
}

func TestAppearanceMissingEmptyAndUnknownPreferencesDefaultToLight(t *testing.T) {
	uiTestImages.Do(loadImages)
	for _, tc := range []struct{ name, content string }{
		{"missing-file", ""},
		{"legacy", `{"IntroHidden":true}`},
		{"empty", `{"Appearance":""}`},
		{"unknown", `{"Appearance":"future-value"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newState(t.TempDir())
			if s.darkAppearance(true) {
				t.Fatal("new state must default to light even on a dark host")
			}
			if tc.content != "" {
				if err := os.WriteFile(filepath.Join(s.ConfigDir, "preferences.json"), []byte(tc.content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			s.Appearance = appearanceDark
			s.Source = source.New("http://127.0.0.1")
			if err := s.loadPreferences(); err != nil {
				t.Fatal(err)
			}
			if s.Appearance != appearanceLight {
				t.Fatalf("unspecified/unknown appearance=%q, want light", s.Appearance)
			}
			tt := ui.NewTester(s.view, 1320, 860)
			tt.SetDark(true)
			requireAppearanceSurfaces(t, tt, false)
		})
	}
}

func TestAppearanceSaveFailureRestoresSelectionWithoutApplying(t *testing.T) {
	s := uiFixture(t, "settings")
	s.Appearance = appearanceDark
	if err := s.savePreferences(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.ConfigDir, "preferences.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".tmp", 0755); err != nil {
		t.Fatal(err)
	}
	applied := 0
	s.ApplyAppearance = func(string) { applied++ }
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, appearanceText.system)
	if s.Appearance != appearanceDark || applied != 0 || !strings.HasPrefix(s.SaveError, appearanceText.saveError) {
		t.Fatalf("failed save: appearance=%q applied=%d error=%q", s.Appearance, applied, s.SaveError)
	}
	uiRequireText(t, tt, s.SaveError)
	requireAppearanceSurfaces(t, tt, true)
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("failed appearance save changed existing preferences: error=%v before=%s after=%s", err, before, after)
	}
}

func TestAppearanceBusySettingsIgnoreRadioClicks(t *testing.T) {
	s := uiFixture(t, "settings")
	s.Appearance = appearanceDark
	s.Busy = true
	applied := 0
	s.ApplyAppearance = func(string) { applied++ }
	tt := ui.NewTester(s.view, 1320, 860)
	for _, label := range []string{appearanceText.light, appearanceText.system} {
		uiClick(t, tt, label)
	}
	if s.Appearance != appearanceDark || applied != 0 || s.SaveError != "" {
		t.Fatalf("busy radio changed appearance: appearance=%q applied=%d error=%q", s.Appearance, applied, s.SaveError)
	}
	if _, err := os.Stat(filepath.Join(s.ConfigDir, "preferences.json")); !os.IsNotExist(err) {
		t.Fatalf("busy appearance click wrote preferences: %v", err)
	}
}

func TestAppearanceRenderedPagesUseDarkSurfacesAndReadableText(t *testing.T) {
	for _, page := range []struct{ id, heading, caption string }{
		{"browse", "2025 年春季番剧", "找到喜欢的番剧，选好版本，后续更新交给 Dipper。"},
		{"detail", "资源预览", "周一 · 来源：蜜柑计划"},
		{"following", "喜欢的故事，不错过更新。", "暂停追番只停止检查新资源，已开始的下载会继续。"},
		{"downloads", "更新到了，安心等它下好。", "查看下载进度、传输速度与做种状态。"},
		{"settings", "让 Dipper 按你的习惯工作。", appearanceText.detail},
	} {
		t.Run(page.id, func(t *testing.T) {
			s := uiFixture(t, page.id)
			s.Appearance = appearanceDark
			tt := ui.NewTester(s.view, 1320, 860)
			requireAppearanceSurfaces(t, tt, true)
			requireAppearanceText(t, tt, page.heading, true)
			requireAppearanceText(t, tt, page.caption, true)
		})
	}
}

func TestAppearanceFollowsHostOnlyInSystemModeWithoutLeaking(t *testing.T) {
	system := uiFixture(t, "settings")
	system.Appearance = appearanceSystem
	var systemPalette *colorPalette
	systemTester := ui.NewTester(func(c *ui.Context) {
		system.view(c)
		systemPalette = colors(c)
	}, 1320, 860)
	for _, mode := range []string{appearanceLight, appearanceDark} {
		t.Run(mode, func(t *testing.T) {
			forced := uiFixture(t, "settings")
			forced.Appearance = mode
			var forcedPalette *colorPalette
			forcedTester := ui.NewTester(func(c *ui.Context) {
				forced.view(c)
				forcedPalette = colors(c)
			}, 1320, 860)
			for _, hostDark := range []bool{false, true, false} {
				systemTester.SetDark(hostDark)
				requireAppearanceSurfaces(t, systemTester, hostDark)
				forcedTester.SetDark(!hostDark)
				forcedTester.Frame()
				requireAppearanceSurfaces(t, forcedTester, mode == appearanceDark)
				systemTester.Frame()
				requireAppearanceSurfaces(t, systemTester, hostDark)
				if gotDark := appearanceColorLuminance(systemPalette.paper) < .15; gotDark != hostDark {
					t.Errorf("system colors(context) dark=%v, host dark=%v", gotDark, hostDark)
				}
				if gotDark := appearanceColorLuminance(forcedPalette.paper) < .15; gotDark != (mode == appearanceDark) {
					t.Errorf("forced %s colors(context) dark=%v", mode, gotDark)
				}
			}
		})
	}
}

func TestAppearanceDarkDialogAndPopoverRemainReadable(t *testing.T) {
	s := uiFixture(t, "detail")
	s.Appearance = appearanceDark
	tt := ui.NewTester(s.view, 1320, 860)
	uiClick(t, tt, "筛选 1080p")
	requireAppearanceText(t, tt, "怎么筛选「1080p」？", true)
	requireAppearanceText(t, tt, "标题必须出现这个词", true)
	requireAppearanceTextSurface(t, tt, "怎么筛选「1080p」？", true)
	uiClick(t, tt, "只要含「1080p」的")
	uiClick(t, tt, "下载并追番")
	requireAppearanceText(t, tt, "确认开始追番", true)
	requireAppearanceText(t, tt, "保持 Dipper 运行即可自动追番和下载；退出后停止后台活动。", true)
	requireAppearanceTextSurface(t, tt, "确认开始追番", true)
}

func TestAppearanceDarkTextAndStatusColorsHaveReadableContrast(t *testing.T) {
	ui.NewTester(func(c *ui.Context) {
		c.SetTheme(spectrumTheme(true))
		p := colors(c)
		for _, pair := range []struct {
			name            string
			ink, background ui.Color
		}{
			{"body", p.ink, p.paper},
			{"caption", p.muted, p.paper},
			{"sidebar caption", p.muted, p.chrome},
			{"recessed caption", p.muted, p.canvas},
			{"primary action", p.accentText, p.blue},
			{"include", p.blue, p.blueWash},
			{"exclude", p.avoidInk, p.avoidWash},
			{"success", p.green, p.greenWash},
			{"warning", p.amber, p.amberWash},
			{"error", p.errorInk, p.errorWash},
		} {
			a, b := appearanceColorLuminance(pair.ink), appearanceColorLuminance(pair.background)
			contrast := (max(a, b) + .05) / (min(a, b) + .05)
			if contrast < 4.5 {
				t.Errorf("%s text contrast=%.2f:1, want at least 4.5:1", pair.name, contrast)
			}
		}
	}, 20, 20)
}

func appearanceColorLuminance(c ui.Color) float64 {
	return appearanceLuminance(color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A})
}

func appearanceLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	linear := func(v uint32) float64 {
		x := float64(v) / 65535
		if x <= .04045 {
			return x / 12.92
		}
		return math.Pow((x+.055)/1.055, 2.4)
	}
	return .2126*linear(r) + .7152*linear(g) + .0722*linear(b)
}

func requireAppearanceSurfaces(t *testing.T, tt *ui.Tester, dark bool) {
	t.Helper()
	img := tt.Image()
	for _, sample := range []struct {
		name  string
		point image.Point
	}{
		{"sidebar", image.Pt(8, img.Bounds().Dy()/2)},
		{"title bar", image.Pt(img.Bounds().Dx()-8, 8)},
		{"page", image.Pt(210, img.Bounds().Dy()-8)},
	} {
		lum := appearanceLuminance(img.At(sample.point.X, sample.point.Y))
		if (dark && lum >= .15) || (!dark && lum <= .8) {
			t.Errorf("%s surface luminance=%.3f, want dark=%v", sample.name, lum, dark)
		}
	}
}

func requireAppearanceText(t *testing.T, tt *ui.Tester, label string, dark bool) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing readable text %q; texts=%q", label, tt.Texts())
	}
	img := tt.Image()
	bounds := image.Rect(int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H)).Intersect(img.Bounds())
	ink := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			lum := appearanceLuminance(img.At(x, y))
			if (dark && lum > .45) || (!dark && lum < .3) {
				ink++
			}
		}
	}
	if ink < 5 {
		t.Errorf("text %q has only %d contrasting foreground pixels, want dark=%v", label, ink, dark)
	}
}

func requireAppearanceTextSurface(t *testing.T, tt *ui.Tester, label string, dark bool) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing panel heading %q", label)
	}
	lum := appearanceLuminance(tt.Image().At(int(r.X+r.W/2), int(r.Y)-2))
	if (dark && lum >= .15) || (!dark && lum <= .8) {
		t.Errorf("panel behind %q has luminance=%.3f, want dark=%v", label, lum, dark)
	}
}
