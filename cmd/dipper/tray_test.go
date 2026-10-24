package main

import (
	"errors"
	"testing"

	"github.com/egoist/mygo"
)

type testTray struct {
	bounds    mygo.Rectangle
	destroyed bool
}

func (t *testTray) Bounds() mygo.Rectangle { return t.bounds }
func (t *testTray) Destroy()               { t.destroyed = true }

func TestTrayRequiresShellRegistrationBeforeHiding(t *testing.T) {
	for _, bounds := range []mygo.Rectangle{{}, {Width: 16}, {Height: 16}} {
		icon := &testTray{bounds: bounds}
		trays := &trayController{platform: "windows", create: func() (desktopTray, error) { return icon, nil }}
		if trays.ensure() || !icon.destroyed || trays.current != nil {
			t.Fatalf("an unregistered tray must be unavailable and cleaned up: %+v", bounds)
		}
	}
}

func TestTrayRecoversLostRegistrationAndKeepsExistingIcon(t *testing.T) {
	first := &testTray{bounds: mygo.Rectangle{Width: 16, Height: 16}}
	second := &testTray{bounds: first.bounds}
	created := 0
	trays := &trayController{platform: "windows", create: func() (desktopTray, error) {
		created++
		if created == 1 {
			return first, nil
		}
		return second, nil
	}}
	if !trays.ensure() || !trays.ensure() || created != 1 || first.destroyed {
		t.Fatal("a registered icon must be retained across close requests")
	}
	first.bounds = mygo.Rectangle{}
	if !trays.ensure() || !first.destroyed || trays.current != second || created != 2 {
		t.Fatal("a lost registration must be replaced before the window is hidden")
	}
	trays.destroy()
	trays.destroy()
	if !second.destroyed || trays.current != nil {
		t.Fatal("shutdown must remove the tray icon")
	}
}

func TestTrayCreationFailureCanRetryAndOtherPlatformsNeedNoBounds(t *testing.T) {
	for _, nilIcon := range []bool{false, true} {
		created := 0
		trays := &trayController{platform: "windows", create: func() (desktopTray, error) {
			created++
			if created == 1 {
				if nilIcon {
					return nil, nil
				}
				return nil, errors.New("notification area unavailable")
			}
			return &testTray{bounds: mygo.Rectangle{Width: 16, Height: 16}}, nil
		}}
		if trays.ensure() || !trays.ensure() {
			t.Fatal("failed tray creation must remain unavailable and retry on the next request")
		}
	}
	for _, platform := range []string{"linux", "darwin"} {
		icon := &testTray{}
		trays := &trayController{platform: platform, create: func() (desktopTray, error) { return icon, nil }}
		if !trays.ensure() || icon.destroyed {
			t.Fatalf("%s must not use Windows shell bounds to decide availability", platform)
		}
	}
}
