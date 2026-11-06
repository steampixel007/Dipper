package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dipper.local/dipper/internal/core"
	"dipper.local/dipper/internal/downloader"
	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

func TestResourceSizesKeepRawBytes(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		resources []Resource
		want      string
	}{
		{"empty", nil, "大小未知"},
		{"known", []Resource{{Bytes: 1_250_001}, {Bytes: 2_250_001}}, "3.5 MB"},
		{"round after summing", []Resource{{Bytes: 1_004_999, MB: 2}, {Bytes: 1_004_999, MB: 2}}, "2.01 MB"},
		{"small byte values", []Resource{{Bytes: 499}, {Bytes: 500}}, "999 B"},
		{"all unknown", []Resource{{}, {}}, "2 条大小未知"},
		{"mixed", []Resource{{Bytes: 1_250_001}, {}}, "已知大小 1.25 MB，另有 1 条大小未知"},
		{"zero", []Resource{{Bytes: 0}}, "1 条大小未知"},
		{"legacy MB", []Resource{{MB: 486}, {MB: 236}}, "722 MB"},
		{"raw overrides legacy", []Resource{{Bytes: 1, MB: 486}}, "1 B"},
		{"invalid negative", []Resource{{Bytes: -1, MB: 486}, {MB: -1}}, "2 条大小未知"},
		{"invalid legacy overflow", []Resource{{MB: int(^uint(0) >> 1)}}, "1 条大小未知"},
		{"large", []Resource{{Bytes: math.MaxInt64}}, "9.22 EB"},
		{"overflow", []Resource{{Bytes: math.MaxInt64}, {Bytes: 1}}, "总大小超出可显示范围"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if got := resourceSizes(scenario.resources); got != scenario.want {
				t.Fatalf("resource sizes = %q, want %q", got, scenario.want)
			}
		})
	}
}

func TestSizeTextCarriesRoundedBoundary(t *testing.T) {
	for _, scenario := range []struct {
		bytes int64
		want  string
	}{
		{999_994, "999.99 KB"},
		{999_995, "1 MB"},
		{999_999, "1 MB"},
		{1_004_999, "1 MB"},
		{1_005_001, "1.01 MB"},
	} {
		if got := byteSizeText(scenario.bytes); got != scenario.want {
			t.Errorf("size text for %d = %q, want %q", scenario.bytes, got, scenario.want)
		}
	}
}

func TestSizeTextDistinguishesZeroAndInvalidValues(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		bytes int64
		exact string
		human string
	}{
		{"empty file", 0, "0 B", "大小未知"},
		{"negative", -1, "大小未知", "大小未知"},
		{"minimum", math.MinInt64, "大小未知", "大小未知"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if got := exactByteText(scenario.bytes); got != scenario.exact {
				t.Errorf("exact size for %d = %q, want %q", scenario.bytes, got, scenario.exact)
			}
			if got := byteSizeText(scenario.bytes); got != scenario.human {
				t.Errorf("human size for %d = %q, want %q", scenario.bytes, got, scenario.human)
			}
		})
	}
}

func TestUploadLimitRoundTripsEveryStoredByteRate(t *testing.T) {
	for _, scenario := range []struct {
		input   string
		bytes   int64
		display string
	}{
		{"42 B/s", 42, "42 B/s"},
		{"512 B/s", 512, "512 B/s"},
		{"0.5 KiB/s", 512, "512 B/s"},
		{"256 KiB/s", 256 * 1024, "256 KiB/s"},
		{"1.5 MiB/s", 3 * 512 * 1024, "1536 KiB/s"},
		{"4 MiB/s", 4 * 1024 * 1024, "4 MiB/s"},
	} {
		t.Run(scenario.input, func(t *testing.T) {
			got, ok := parseByteRate(scenario.input)
			if !ok || got != scenario.bytes {
				t.Fatalf("parse %q = %d, %v; want %d, true", scenario.input, got, ok, scenario.bytes)
			}
			if label := uploadLabel(scenario.bytes); label != scenario.display {
				t.Fatalf("display %d bytes/s = %q, want %q", scenario.bytes, label, scenario.display)
			}
			if rate := uploadLimit(uploadLabel(scenario.bytes)); rate != scenario.bytes {
				t.Fatalf("round trip %d bytes/s = %d", scenario.bytes, rate)
			}
		})
	}
	for _, label := range []string{"NaN MiB/s", "+Inf GiB/s", "-1 KiB/s", "1.5", "0 B/s"} {
		if rate, ok := parseByteRate(label); ok {
			t.Errorf("parse invalid %q = %d, true", label, rate)
		}
	}
}

func TestResourceRowUsesRawBytes(t *testing.T) {
	s := uiFixture(t, "detail")
	r := s.ResourceList[0]
	r.Bytes, r.MB = 1_250_001, 486
	s.ResourceList = []Resource{r}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "1.25 MB")
	if tt.HasText("486 MB") {
		t.Fatal("resource row ignored its exact byte size")
	}
}

func TestDownloadSizeProjectionPreservesBytes(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		size      int64
		snapshot  *downloader.Snapshot
		wantBytes int64
		wantDone  int64
	}{
		{"source bytes", 1_000_001, nil, 1_000_001, 0},
		{"source large", math.MaxInt64, nil, math.MaxInt64, 0},
		{"snapshot bytes", 1_000_001, &downloader.Snapshot{TotalBytes: 2_000_003, CompletedBytes: 1_000_001, Progress: .5, UploadedBytes: 3_000_007, DownloadedBytes: 2_000_009}, 2_000_003, 1_000_001},
		{"metadata absent", 1_000_001, &downloader.Snapshot{}, 1_000_001, 0},
		{"invalid snapshot", 1_000_001, &downloader.Snapshot{TotalBytes: -1}, -1, 0},
		{"unknown", 0, nil, 0, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := &State{}
			rows := coreResources([]source.Resource{{ID: "raw-size", Title: "Episode", Size: scenario.size}})
			if rows[0].Size != scenario.size {
				t.Fatalf("source-to-core bytes = %d, want %d", rows[0].Size, scenario.size)
			}
			state := core.State{Tasks: []core.TaskRecord{{Task: core.DownloadTask{Hash: "raw-size", Resource: rows[0]}}}}
			var snapshots []downloader.Snapshot
			if scenario.snapshot != nil {
				snapshot := *scenario.snapshot
				snapshot.ID = "raw-size"
				snapshots = append(snapshots, snapshot)
			}
			s.project(state, snapshots)
			d := s.Downloads[0]
			if d.Bytes != scenario.wantBytes || downloadDoneBytes(d) != scenario.wantDone {
				t.Fatalf("projected total=%d done=%d, want total=%d done=%d", d.Bytes, downloadDoneBytes(d), scenario.wantBytes, scenario.wantDone)
			}
			if scenario.snapshot != nil && d.UploadedBytes != scenario.snapshot.UploadedBytes {
				t.Fatalf("projected upload=%d, want %d bytes", d.UploadedBytes, scenario.snapshot.UploadedBytes)
			}
			if scenario.snapshot != nil && d.DownloadedBytes != scenario.snapshot.DownloadedBytes {
				t.Fatalf("projected download=%d, want %d bytes", d.DownloadedBytes, scenario.snapshot.DownloadedBytes)
			}
		})
	}
}

func TestHistoryResourceProjectionPreservesBytes(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("..", "..", "internal", "source", "testdata", "probe-b4063-g370-take65.html"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Home/ExpandEpisodeTable" {
			t.Errorf("unexpected source request %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(html)
	}))
	defer server.Close()
	s := uiFixture(t, "detail")
	s.Source = source.New(server.URL)
	s.Ctx = t.Context()
	s.AnimeID, s.Group, s.GroupIDs = "4063", 0, []string{"370"}
	s.History, s.ResourceList = []source.Resource{{ID: "stale", Size: 1}}, []Resource{{ID: "stale", MB: 1}}
	updates := make(chan func(), 1)
	s.Update = func(update func()) { updates <- update }
	s.refreshHistory()
	select {
	case update := <-updates:
		update()
	case <-time.After(5 * time.Second):
		t.Fatal("history projection did not complete")
	}
	s.historyCancel()
	if s.HistoryError != "" || s.HistoryLoading || len(s.ResourceList) != 3 || len(s.History) != 3 {
		t.Fatalf("history error=%q loading=%v resources=%d source=%d", s.HistoryError, s.HistoryLoading, len(s.ResourceList), len(s.History))
	}
	for index, resource := range s.ResourceList {
		if resource.Bytes != s.History[index].Size || resource.Bytes <= 0 {
			t.Errorf("resource %d has %d bytes, source has %d", index, resource.Bytes, s.History[index].Size)
		}
	}
}

func TestDownloadGroupSizesKeepExactCompletion(t *testing.T) {
	s := &State{Downloads: []Download{
		{Bytes: 1_004_999, MB: 2, CompletedBytes: 500_001, Progress: .5},
		{Bytes: 1_004_999, MB: 2, CompletedBytes: 500_002, Progress: .5},
		{},
	}}
	total, done, unknown, overflow := s.downloadGroupSizes([]int{0, 1, 2})
	if total != 2_009_998 || done != 1_000_003 || unknown != 1 || overflow {
		t.Fatalf("group totals=%d done=%d unknown=%d overflow=%v", total, done, unknown, overflow)
	}
}

func TestUIDownloadGroupSizeLabels(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		downloads []Download
		want      string
	}{
		{"known rounding", []Download{{Bytes: 1_004_999, CompletedBytes: 500_001, Progress: .5}, {Bytes: 1_004_999, CompletedBytes: 500_002, Progress: .5}}, "1 MB / 2.01 MB"},
		{"mixed", []Download{{Bytes: 1_000_001, CompletedBytes: 500_001, Progress: .5}, {}}, "已下载 500 KB / 1 MB（另有 1 条大小未知）"},
		{"unknown", []Download{{}, {}}, "2 条大小未知"},
		{"large", []Download{{Bytes: math.MaxInt64, Progress: 1, Stopped: true}}, "9.22 EB / 9.22 EB"},
		{"overflow", []Download{{Bytes: math.MaxInt64, Progress: .5}, {Bytes: 1}}, "总大小超出可显示范围"},
		{"zero completion", []Download{{Bytes: 10_000}}, "0 B / 10 KB"},
		{"negative completion falls back", []Download{{Bytes: 10_000, CompletedBytes: -1, Progress: .25}}, "2.5 KB / 10 KB"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := uiFixture(t, "downloads")
			s.Downloads = scenario.downloads
			for index := range s.Downloads {
				s.Downloads[index].ID = string(rune('a' + index))
				s.Downloads[index].AnimeID = s.AnimeID
			}
			tt := ui.NewTester(s.view, 1320, 860)
			uiRequireText(t, tt, scenario.want)
		})
	}
}

func TestDownloadETAKeepsRawBytes(t *testing.T) {
	d := Download{Bytes: 1_000_001, MB: 2, CompletedBytes: 1, Progress: .000001, DownSpeed: .01}
	if got := downloadETA(d); got != "剩余约 1 分钟" {
		t.Fatalf("raw-size ETA = %q, want 剩余约 1 分钟", got)
	}
	s := &State{Downloads: []Download{d}}
	if got := s.downloadGroupETA([]int{0}); got != "剩余约 1 分钟" {
		t.Fatalf("raw-size group ETA = %q, want 剩余约 1 分钟", got)
	}
}

func TestDownloadETARejectsInvalidRemainingTime(t *testing.T) {
	for _, scenario := range []struct {
		name string
		d    Download
	}{
		{"completed", Download{Bytes: 1_000_000, CompletedBytes: 1_000_000, Progress: .5, DownSpeed: .1}},
		{"zero speed", Download{Bytes: 1_000_000, Progress: .5}},
		{"negative speed", Download{Bytes: 1_000_000, Progress: .5, DownSpeed: -1}},
		{"NaN speed", Download{Bytes: 1_000_000, Progress: .5, DownSpeed: math.NaN()}},
		{"infinite speed", Download{Bytes: 1_000_000, Progress: .5, DownSpeed: math.Inf(1)}},
		{"NaN progress", Download{Bytes: 1_000_000, Progress: math.NaN(), DownSpeed: 1}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if got := downloadETA(scenario.d); got != "" {
				t.Fatalf("invalid ETA = %q, want no estimate", got)
			}
			s := &State{Downloads: []Download{scenario.d}}
			if got := s.downloadGroupETA([]int{0}); got != "" {
				t.Fatalf("invalid group ETA = %q, want no estimate", got)
			}
		})
	}
}

func TestDownloadGroupSizesRejectOverflowWithoutPartialTotal(t *testing.T) {
	s := &State{Downloads: []Download{{Bytes: math.MaxInt64}, {Bytes: 1}}}
	total, done, unknown, overflow := s.downloadGroupSizes([]int{0, 1})
	if total != 0 || done != 0 || unknown != 0 || !overflow {
		t.Fatalf("overflowing totals=%d done=%d unknown=%d overflow=%v", total, done, unknown, overflow)
	}
}

func TestUIDownloadSharingRatioUsesRawBytes(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		downloaded int64
		want       string
	}{
		{"import fallback", 0, "2.00"},
		{"downloaded denominator", 2_000_002, "1.00"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			s := uiFixture(t, "downloads")
			s.Downloads = []Download{{ID: "ratio", AnimeID: s.AnimeID, Bytes: 1_000_001, MB: 2, DownloadedBytes: scenario.downloaded, UploadedBytes: 2_000_002, UploadedMB: 3, Progress: 1}}
			tt := ui.NewTester(s.view, 1320, 860)
			uiRequireText(t, tt, "分享率 "+scenario.want+"，分享时间 0 分钟")
		})
	}
}

func TestSeedTimeDoesNotReportPositiveDurationAsZero(t *testing.T) {
	for _, scenario := range []struct {
		seconds int
		want    string
	}{
		{0, "分享时间 0 分钟"},
		{1, "分享时间不足 1 分钟"},
		{59, "分享时间不足 1 分钟"},
		{60, "分享时间 1 分钟"},
		{3599, "分享时间 59 分钟"},
	} {
		if got := seedTimeText(scenario.seconds); got != scenario.want {
			t.Errorf("seedTimeText(%d) = %q, want %q", scenario.seconds, got, scenario.want)
		}
	}
}

func TestUIDownloadPositiveSharingTimeUsesReadableUnit(t *testing.T) {
	s := uiFixture(t, "downloads")
	s.Downloads = []Download{{
		ID: "sharing-time", AnimeID: s.AnimeID,
		Bytes: 1_000_000, DownloadedBytes: 1_000_000,
		UploadedBytes: 250_000, Progress: 1, SeedSeconds: 1,
	}}
	tt := ui.NewTester(s.view, 1320, 860)
	uiRequireText(t, tt, "分享率 0.25，分享时间不足 1 分钟")
	if tt.HasText("分享时间 0 分钟") {
		t.Fatal("a positive sharing duration was reported as zero")
	}
}
