package core

import (
	"reflect"
	"testing"
)

func TestReleaseTagsRecognizeCommonLabels(t *testing.T) {
	cases := []struct {
		title string
		want  []string
	}{
		{"[组] 剧名 - 03 [1080P][HEVC][简日内嵌][MKV]", []string{"1080p", "hevc", "mkv", "简日内嵌"}},
		{"[ANi] 剧名 - 02 [1080P][Baha][WEB-DL][AAC AVC][CHT][MP4]", []string{"1080p", "avc", "aac", "mp4", "web-dl", "cht"}},
		{"[组] 剧名 [WebRip 1080p HEVC-10bit AAC][简繁内封字幕]", []string{"1080p", "hevc", "aac", "webrip", "简繁内封", "10bit"}},
		{"[组] 剧名 - 01 [简日双语MP4/繁日双语MP4/简繁日多语MKV][1080P]", []string{"1080p", "mp4", "mkv", "简日双语", "繁日双语", "简繁日多语"}},
		{"[组] 剧名 数码重映第450集 [TVRIP][简日双语MP4/繁日双语MP4/简繁日多语MKV(PGS)][1080P]", []string{"1080p", "mp4", "mkv", "tvrip", "简日双语", "繁日双语", "简繁日多语"}},
		{"[组] 剧名 [720p][CHS][AVC].mp4", []string{"720p", "avc", "mp4", "chs"}},
		{"[组] 剧名 [1080P][V2][720P]", []string{"1080p", "720p", "v2"}},
		{"[组] 剧名 [4K][HEVC][HDR]", []string{"4k", "hevc"}},
		{"[组] 剧名 [1920x1080][x264][FLAC]", []string{"x264", "flac"}},
	}
	for _, tc := range cases {
		got := ReleaseTags(tc.title)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %v, want %v", tc.title, got, tc.want)
		}
	}
}
