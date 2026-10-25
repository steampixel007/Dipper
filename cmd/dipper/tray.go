package main

import (
	"log/slog"

	"github.com/egoist/mygo"
)

type desktopTray interface {
	Bounds() mygo.Rectangle
	Destroy()
}

func trayMenuLabel(platform, label string) string {
	// MyGo measures Windows 11 menu items tightly enough to crowd the last glyph.
	if platform == "windows" {
		return label + "  "
	}
	return label
}

type trayController struct {
	current  desktopTray
	create   func() (desktopTray, error)
	platform string
}

// NewTray can succeed before Windows has registered the icon. Bounds queries
// the shell itself, so a failed registration never strands a hidden window.
func (t *trayController) ensure() bool {
	if t.current != nil {
		if t.registered() {
			return true
		}
		t.destroy()
	}
	icon, err := t.create()
	if err != nil {
		slog.Warn("tray unavailable", "error", err)
		return false
	}
	if icon == nil {
		return false
	}
	t.current = icon
	if !t.registered() {
		slog.Warn("tray registration could not be confirmed")
		t.destroy()
		return false
	}
	slog.Info("tray ready", "platform", t.platform)
	return true
}

func (t *trayController) registered() bool {
	// AppIndicator on Linux exposes no bounds; Windows queries the shell.
	if t.platform != "windows" {
		return true
	}
	r := t.current.Bounds()
	return r.Width > 0 && r.Height > 0
}

func (t *trayController) destroy() {
	if t.current != nil {
		t.current.Destroy()
		t.current = nil
	}
}
