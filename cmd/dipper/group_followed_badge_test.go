package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIGroupPickerMarksFollowedGroup(t *testing.T) {
	s := uiFixture(t, "detail")
	s.ChoosingGroup = true
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "正在追")

	s.Subscriptions = nil
	tt.Frame()
	if tt.HasText("正在追") {
		t.Fatalf("groups without a follow must not show the badge; texts=%q", tt.Texts())
	}
}
