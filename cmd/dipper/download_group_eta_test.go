package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUIDownloadGroupShowsCombinedETA(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads[1].Failed = true
	s.Downloads[1].Error = "连接超时，请重试"
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "3 个下载任务，下载中 1，分享中 1，失败 1，剩余约 1 分钟")
}
