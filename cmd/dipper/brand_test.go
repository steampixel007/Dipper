package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
		if machine := binary.LittleEndian.Uint16(b[0:]); machine != tc.machine {
			t.Errorf("%s targets machine %#x, want %#x", tc.path, machine, tc.machine)
		}
		if name := string(bytes.TrimRight(b[20:28], "\x00")); name != ".rsrc" {
			t.Errorf("%s holds section %q, not .rsrc", tc.path, name)
		}
		rawAt := int(binary.LittleEndian.Uint32(b[40:]))
		rawSize := int(binary.LittleEndian.Uint32(b[36:]))
		if rawAt <= 0 || rawAt+rawSize > len(b) {
			t.Fatalf("%s .rsrc raw data out of bounds: at=%d size=%d len=%d", tc.path, rawAt, rawSize, len(b))
		}
		icons := walkSysoResources(t, b[rawAt:rawAt+rawSize], tc.path)
		for _, size := range []int{16, 24, 32, 48, 64, 128, 256} {
			cfg, ok := icons[uint16(size)]
			if !ok {
				t.Errorf("%s has no %dx%d icon image", tc.path, size, size)
				continue
			}
			if cfg.Width != size || cfg.Height != size {
				t.Errorf("%s icon for %dpx is %dx%d", tc.path, size, cfg.Width, cfg.Height)
			}
		}
	}
}

// sysoDir reads one resource directory and returns its entries' name-or-id
// fields. wantID asks for nonzero ids: a misplaced directory is all zeros,
// which is exactly the corruption this walk must catch.
func sysoDir(t *testing.T, raw []byte, path, name string, at uint32, wantID bool) []uint32 {
	t.Helper()
	if int(at)+16 > len(raw) {
		t.Fatalf("%s directory %s at %d out of bounds", path, name, at)
	}
	n := int(binary.LittleEndian.Uint16(raw[at+12:])) + int(binary.LittleEndian.Uint16(raw[at+14:]))
	if n == 0 {
		t.Fatalf("%s directory %s at %d has no entries", path, name, at)
	}
	ids := make([]uint32, 0, n)
	for i := 0; i < n; i++ {
		p := int(at) + 16 + 8*i
		if p+8 > len(raw) {
			t.Fatalf("%s directory %s entry %d out of bounds", path, name, i)
		}
		id := binary.LittleEndian.Uint32(raw[p:])
		if wantID && id == 0 {
			t.Fatalf("%s directory %s entry %d has id 0", path, name, i)
		}
		ids = append(ids, id)
	}
	return ids
}

// sysoSubdir returns entry i's offset with the directory bit stripped.
func sysoSubdir(t *testing.T, raw []byte, at uint32, i int, path, name string) uint32 {
	t.Helper()
	off := binary.LittleEndian.Uint32(raw[at+16+8*uint32(i)+4:])
	if off&0x80000000 == 0 {
		t.Fatalf("%s %s entry %d points at data, not a directory", path, name, i)
	}
	return off &^ 0x80000000
}

// walkSysoResources follows the .rsrc tree Windows reads - root, type, id,
// language, data entry - and returns the RT_ICON images it reaches, keyed by
// width. A PNG the tree cannot reach is invisible to Windows, so the walk
// must find every icon; scanning for PNG signatures would pass a corrupt tree.
func walkSysoResources(t *testing.T, raw []byte, path string) map[uint16]image.Config {
	t.Helper()
	const rtIcon, rtGroupIcon = uint32(3), uint32(14)
	roots := sysoDir(t, raw, path, "root", 0, true)
	var iconDirAt, groupDirAt uint32
	for i, id := range roots {
		at := sysoSubdir(t, raw, 0, i, path, "root")
		switch id {
		case rtIcon:
			iconDirAt = at
		case rtGroupIcon:
			groupDirAt = at
		}
	}
	if iconDirAt == 0 || groupDirAt == 0 {
		t.Fatalf("%s root entries %v lack RT_ICON or RT_GROUP_ICON", path, roots)
	}
	icons := make(map[uint16]image.Config)
	iconIDs := sysoDir(t, raw, path, "RT_ICON", iconDirAt, true)
	for i, id := range iconIDs {
		idAt := sysoSubdir(t, raw, iconDirAt, i, path, "RT_ICON")
		langs := sysoDir(t, raw, path, "RT_ICON id", idAt, true)
		if len(langs) != 1 || langs[0] != 0x0409 {
			t.Fatalf("%s RT_ICON %d languages: %v", path, id, langs)
		}
		cfg := sysoPNGAt(t, raw, idAt, path, fmt.Sprintf("RT_ICON %d", id))
		icons[uint16(cfg.Width)] = cfg
	}
	groupIDs := sysoDir(t, raw, path, "RT_GROUP_ICON", groupDirAt, true)
	if len(groupIDs) != 1 {
		t.Fatalf("%s RT_GROUP_ICON ids: %v", path, groupIDs)
	}
	groupAt := sysoSubdir(t, raw, groupDirAt, 0, path, "RT_GROUP_ICON")
	groupLangs := sysoDir(t, raw, path, "RT_GROUP_ICON id", groupAt, true)
	if len(groupLangs) != 1 || groupLangs[0] != 0x0409 {
		t.Fatalf("%s RT_GROUP_ICON languages: %v", path, groupLangs)
	}
	group := sysoDataAt(t, raw, groupAt, path, "RT_GROUP_ICON 1")
	if got := binary.LittleEndian.Uint16(group[4:]); got != uint16(len(iconIDs)) {
		t.Fatalf("%s group icon lists %d entries, want %d", path, got, len(iconIDs))
	}
	return icons
}

// sysoDataAt follows an id directory's single language entry to its data
// entry and returns the resource bytes there.
func sysoDataAt(t *testing.T, raw []byte, idAt uint32, path, name string) []byte {
	t.Helper()
	langEntryOff := binary.LittleEndian.Uint32(raw[idAt+16+4:])
	if langEntryOff&0x80000000 != 0 {
		t.Fatalf("%s %s language entry points at a directory", path, name)
	}
	dataEntryAt := langEntryOff
	rva := binary.LittleEndian.Uint32(raw[dataEntryAt:])
	size := binary.LittleEndian.Uint32(raw[dataEntryAt+4:])
	if int(rva)+int(size) > len(raw) {
		t.Fatalf("%s %s data out of bounds: rva=%d size=%d len=%d", path, name, rva, size, len(raw))
	}
	return raw[int(rva) : int(rva)+int(size)]
}

func sysoPNGAt(t *testing.T, raw []byte, idAt uint32, path, name string) image.Config {
	t.Helper()
	data := sysoDataAt(t, raw, idAt, path, name)
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s %s is not a PNG: %v", path, name, err)
	}
	return cfg
}
