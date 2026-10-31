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
		{"[ANi] 剧名 - 02 [1080P][Baha][WEB-DL][AAC AVC][CHT][MP4]", []string{"1080p", "avc", "aac", "mp4", "web-dl", "baha", "cht"}},
		{"[组] 剧名 [WebRip 1080p HEVC-10bit AAC][简繁内封字幕]", []string{"1080p", "hevc", "aac", "webrip", "简繁内封", "10bit"}},
		{"[组] 剧名 - 01 [简日双语MP4/繁日双语MP4/简繁日多语MKV][1080P]", []string{"1080p", "mp4", "mkv", "简日双语", "繁日双语", "简繁日多语"}},
		{"[组] 剧名 数码重映第450集 [TVRIP][简日双语MP4/繁日双语MP4/简繁日多语MKV(PGS)][1080P]", []string{"1080p", "mp4", "mkv", "tvrip", "简日双语", "繁日双语", "简繁日多语"}},
		{"[组] 剧名 [720p][CHS][AVC].mp4", []string{"720p", "avc", "mp4", "chs"}},
		{"[组] 剧名 [1080P][V2][720P]", []string{"1080p", "720p", "v2"}},
		{"[组] 剧名 [4K][HEVC][HDR]", []string{"4k", "hevc", "hdr"}},
		{"[组] 剧名 [Baha][WEB-DL][1080P][HEVC][HDR10+][简体内嵌]", []string{"1080p", "hevc", "web-dl", "baha", "简体内嵌", "hdr10+"}},
		{"[组] 剧名 [B-Global][1080P][Dolby Vision][MKV]", []string{"1080p", "mkv", "b-global", "dolby vision"}},
		{"[组] 剧名 [AT-X][720p][HLG][MP4]", []string{"720p", "mp4", "at-x", "hlg"}},
		{"[组] 剧名 [1920x1080][x264][FLAC]", []string{"x264", "flac"}},
		{"[组] 剧名 [1080P][MP4/MKV]", []string{"1080p", "mp4", "mkv"}},
		{"[组] 剧名 [1080P][繁体][MP4]", []string{"1080p", "mp4", "繁体"}},
		{"[组] 剧名 [720P][简体MP4]", []string{"720p", "mp4", "简体"}},
	}
	for _, tc := range cases {
		got := ReleaseTags(tc.title)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestReleaseTagsRecognizeCollections(t *testing.T) {
	cases := []struct {
		title string
		want  []string
	}{
		{"[组] 剧名 合集 [01-12][1080P][HEVC]", []string{"1080p", "hevc", "合集"}},
		{"[组] 剧名 全集合集 [1080P]", []string{"1080p", "全集", "合集"}},
		{"[组] 剧名 第01-12集 合集 [1080P][简日内嵌][MKV]", []string{"1080p", "mkv", "简日内嵌", "合集"}},
		{"[组] 剧名 - 01 [1080P][HEVC]", []string{"1080p", "hevc"}},
	}
	for _, tc := range cases {
		got := ReleaseTags(tc.title)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %v, want %v", tc.title, got, tc.want)
		}
	}
}
