package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIDownloadGroupShowsCombinedETA(t *testing.T) {
	s := uiFixture(t, "downloads")
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "3 个下载任务，下载中 1，已完成 1，失败 1，剩余约 1 分钟")
}
