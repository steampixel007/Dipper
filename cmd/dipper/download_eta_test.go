package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDownloadETAText(t *testing.T) {
	cases := []struct {
		name string
		d    Download
		want string
	}{
		{"running minutes", Download{MB: 240, Progress: 0.5, DownSpeed: 1}, "剩余约 2 分钟"},
		{"under a minute", Download{MB: 100, Progress: 0.99, DownSpeed: 1}, "剩余不足 1 分钟"},
		{"half hour", Download{MB: 3600, Progress: 0.5, DownSpeed: 1}, "剩余约 30 分钟"},
		{"one hour", Download{MB: 7200, Progress: 0.5, DownSpeed: 1}, "剩余约 1 小时"},
		{"hour with minutes", Download{MB: 9000, Progress: 0.5, DownSpeed: 1}, "剩余约 1 小时 15 分钟"},
		{"no speed", Download{MB: 240, Progress: 0.5, DownSpeed: 0}, ""},
		{"no size", Download{Progress: 0.5, DownSpeed: 1}, ""},
		{"complete", Download{MB: 240, Progress: 1, DownSpeed: 1}, ""},
		{"paused", Download{MB: 240, Progress: 0.5, DownSpeed: 1, Paused: true}, ""},
		{"stopped", Download{MB: 240, Progress: 0.5, DownSpeed: 1, Stopped: true}, ""},
		{"failed", Download{MB: 240, Progress: 0.5, DownSpeed: 1, Failed: true}, ""},
	}
	for _, c := range cases {
		if got := downloadETA(c.d); got != c.want {
			t.Errorf("%s: downloadETA = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestUIDownloadRowShowsETA(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads[0].Progress = 0.5
	s.Downloads[0].MB = 240
	s.Downloads[0].DownSpeed = 1
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "剩余约 2 分钟")
}

func TestDownloadGroupETAMixesActiveTransfers(t *testing.T) {
	s := &State{Downloads: []Download{
		{MB: 200, Progress: .5, DownSpeed: 2},
		{MB: 280, Progress: .5, DownSpeed: 2},
		{MB: 100, Progress: 0, DownSpeed: 9, Paused: true},
		{MB: 100, Progress: .5, DownSpeed: 9, Failed: true},
	}}
	if got := s.downloadGroupETA([]int{0, 1, 2, 3}); got != "剩余约 1 分钟" {
		t.Fatalf("group ETA = %q, want 剩余约 1 分钟", got)
	}
}

func TestDownloadGroupETAEmptyWithoutActiveTransfers(t *testing.T) {
	s := &State{Downloads: []Download{
		{MB: 100, Progress: .5, DownSpeed: 2, Paused: true},
		{MB: 100, Progress: .5, Failed: true},
		{MB: 100, Progress: 1, DownSpeed: 2},
	}}
	if got := s.downloadGroupETA([]int{0, 1, 2}); got != "" {
		t.Fatalf("group ETA = %q, want empty", got)
	}
}

func TestDownloadGroupETAEmptyWhenActiveSizeUnknown(t *testing.T) {
	s := &State{Downloads: []Download{
		{MB: 240, Progress: .5, DownSpeed: 1},
		{MB: 0, Progress: .1, DownSpeed: 1},
	}}
	if got := s.downloadGroupETA([]int{0, 1}); got != "" {
		t.Fatalf("group ETA = %q, want empty while an active transfer has unknown size", got)
	}
}
