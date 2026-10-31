package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUISettingsShowBuiltinTrackers(t *testing.T) {
	s := uiFixture(t, "settings")
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "内置 Tracker", "udp://tracker.opentrackr.org:1337/announce")
}
