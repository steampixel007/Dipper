package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCustomWindowControlsRenderAndDispatch(t *testing.T) {
	s := uiFixture(t, "browse")
	s.CustomWindowControls = true
	minimized, maximized, closed := 0, 0, 0
	s.WindowMinimize = func() { minimized++ }
	s.WindowMaximize = func() { maximized++ }
	s.WindowClose = func() { closed++ }
	s.WindowIsMaximized = func() bool { return false }

	tt := ui.NewTester(s.view, 1320, 860)
	for _, label := range []string{"最小化窗口", "最大化窗口", "关闭窗口"} {
		bounds := requireLayoutBound(t, tt, label, 1320, 860)
		if bounds.X < 1100 || bounds.Y != 0 {
			t.Fatalf("%q is not flush with the top-right edge: %+v", label, bounds)
		}
	}
	closeBounds := requireLayoutBound(t, tt, "关闭窗口", 1320, 860)
	if closeBounds.X+closeBounds.W != 1320 {
		t.Fatalf("close button does not sit flush with the right edge: %+v", closeBounds)
	}
	uiClick(t, tt, "最小化窗口")
	uiClick(t, tt, "最大化窗口")
	uiClick(t, tt, "关闭窗口")
	if minimized != 1 || maximized != 1 || closed != 1 {
		t.Fatalf("window callbacks = minimize %d, maximize %d, close %d", minimized, maximized, closed)
	}
	if tt.HasText("还原窗口") {
		t.Fatal("a normal window must not offer restore")
	}

	s.WindowIsMaximized = func() bool { return true }
	tt.Frame()
	uiRequireText(t, tt, "还原窗口")
	if tt.HasText("最大化窗口") {
		t.Fatal("a maximized window must offer restore instead of maximize")
	}
}

func TestWindowControlsUseDistinctIcons(t *testing.T) {
	for _, name := range []string{"minimize", "maximize", "restore", "close"} {
		if iconPaths[name] == "" {
			t.Fatalf("missing window control icon %q", name)
		}
	}
}
