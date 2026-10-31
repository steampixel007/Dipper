package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIFollowingListFloatsFailedChecks(t *testing.T) {
	s := uiFixture(t, "following")
	tt := ui.NewTester(s.view, 1320, 860)

	joined := strings.Join(tt.Texts(), "\n")
	failed := strings.Index(joined, "取消追番 回归测试组乙")
	healthy := strings.Index(joined, "取消追番 回归测试组甲")
	if failed > healthy {
		t.Fatalf("a failed check must float above healthy follows: failed=%d healthy=%d", failed, healthy)
	}

	s.Subscriptions[1].CheckFailed = false
	tt.Frame()
	joined = strings.Join(tt.Texts(), "\n")
	failed = strings.Index(joined, "取消追番 回归测试组乙")
	healthy = strings.Index(joined, "取消追番 回归测试组甲")
	if failed < healthy {
		t.Fatalf("healthy follows must keep their saved order: failed=%d healthy=%d", failed, healthy)
	}
}
