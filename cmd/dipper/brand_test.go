package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestTrayIconFollowsTaskbarAppearance(t *testing.T) {
	light := decodeIconPNG(t, brandTrayPNG)
	dark := decodeIconPNG(t, brandTrayDarkPNG)
	if got := trayIconPNG(false); !bytes.Equal(got, brandTrayPNG) {
		t.Error("a light taskbar must show the ink cut, not the dark cut")
	}
	if got := trayIconPNG(true); !bytes.Equal(got, brandTrayDarkPNG) {
		t.Error("a dark taskbar must show the dark cut, not the ink cut")
	}
	if l := meanGray(light); l >= 128 {
		t.Errorf("the light cut must be dark ink, mean gray %d", l)
	}
	if l := meanGray(dark); l < 128 {
		t.Errorf("the dark cut must be light ink, mean gray %d", l)
	}
	var shown [][]byte
	isDark := true
	set := func(png []byte) error {
		shown = append(shown, png)
		return nil
	}
	trayIconSetter(func() bool { return isDark }, set)()
	isDark = false
	trayIconSetter(func() bool { return isDark }, set)()
	if len(shown) != 2 || !bytes.Equal(shown[0], brandTrayDarkPNG) || !bytes.Equal(shown[1], brandTrayPNG) {
		t.Errorf("the tray must reshow the cut for the bar: %d shown", len(shown))
	}
}

func decodeIconPNG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode tray icon: %v", err)
	}
	return img
}

func meanGray(img image.Image) int {
	var sum, n uint64
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A == 0 {
				continue
			}
			sum += uint64(c.R)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return int(sum / n)
}

func TestWindowsSysoEmbedsAppIcon(t *testing.T) {
	for _, tc := range []struct {
		path    string
		machine uint16
	}{
		{"dipper_windows_amd64.syso", 0x8664},
		{"dipper_windows_arm64.syso", 0xAA64},
	} {
		b, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("%s: the executable's icon resource is missing: %v", tc.path, err)
		}
		if len(b) < 20+40+16 {
			t.Fatalf("%s is %d bytes, too small for a COFF header and a section", tc.path, len(b))
		}
		if machine := uint16(b[0]) | uint16(b[1])<<8; machine != tc.machine {
			t.Errorf("%s targets machine %#x, want %#x", tc.path, machine, tc.machine)
		}
		name := string(bytes.TrimRight(b[20:28], "\x00"))
		if name != ".rsrc" {
			t.Errorf("%s holds section %q, not .rsrc", tc.path, name)
		}
		for _, size := range []int{16, 24, 32, 48, 64, 128, 256} {
			var found bool
			for i := 0; i+len(pngHeader) <= len(b) && !found; i++ {
				if !bytes.Equal(b[i:i+len(pngHeader)], pngHeader) {
					continue
				}
				cfg, err := png.DecodeConfig(bytes.NewReader(b[i:]))
				if err == nil && cfg.Width == size && cfg.Height == size {
					found = true
				}
			}
			if !found {
				t.Errorf("%s has no %d×%d icon image", tc.path, size, size)
			}
		}
	}
}

var pngHeader = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
