package core

import (
	"bufio"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type corpusRelease struct {
	anime, group, title string
}

func readReleaseCorpus(t *testing.T) []corpusRelease {
	t.Helper()
	file, err := os.Open("testdata/mikan-release-titles.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var releases []corpusRelease
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 3)
		if len(fields) != 3 || fields[2] == "" {
			t.Fatalf("invalid corpus row %d", len(releases)+1)
		}
		releases = append(releases, corpusRelease{fields[0], fields[1], fields[2]})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return releases
}

func TestReleaseCorpusAnnotations(t *testing.T) {
	releases := readReleaseCorpus(t)
	both := SubtitleSimplified | SubtitleTraditional
	for _, test := range []struct {
		name, anime, group, excerpt string
		want                        ReleaseVersion
	}{
		{"size-and-telecine", "227", "534", "[CAM-RIP][TC][WEB-MP4/4.5GB]", ReleaseVersion{Container: "MP4"}},
		{"combined-bluray-resolution", "227", "605", "[BD1080P][日语无字]", ReleaseVersion{Height: 1080, Source: "BD"}},
		{"explicit-no-chinese", "227", "1231", "[IQIYI WebRip 2160p H265 Veryslow AAC][无中文字幕]", ReleaseVersion{Height: 2160, Source: "WEBRip", Video: "HEVC", Audio: "AAC"}},
		{"simplified-only", "227", "10", "[HDTV][1080P][简体MP4]", ReleaseVersion{Height: 1080, Language: SubtitleSimplified, Source: "TV", Container: "MP4"}},
		{"legacy-bilingual", "227", "22", "[简日][HDRip][X264-AAC][720P][MP4]", ReleaseVersion{Height: 720, Language: SubtitleSimplified, Video: "AVC", Audio: "AAC", Container: "MP4"}},
		{"mixed-files", "227", "161", "[WEBRIP][简日双语MP4/繁日双语MP4/简繁日多语MKV][1080P]", ReleaseVersion{Height: 1080, Language: both, Source: "WEBRip", Ambiguous: true}},
		{"one-episode", "227", "466", "[BIG5][全1话][1280X720P]", ReleaseVersion{Height: 720, Language: SubtitleTraditional, Source: "BD", Container: "MP4"}},
		{"sources-and-reversed-codecs", "3298", "583", "[1080P][Baha][WEB-DL][AAC AVC][CHT][MP4]", ReleaseVersion{Height: 1080, Language: SubtitleTraditional, Source: "WEB-DL", Video: "AVC", Audio: "AAC", Container: "MP4"}},
		{"underscore-audio-and-revision", "3298", "622", "[x265_EAC3][简繁内封][v2]", ReleaseVersion{Language: both, Source: "WEBRip", Video: "HEVC", Audio: "EAC3", Subtitles: "内封"}},
		{"separated-scripts-and-audio-tracks", "3522", "202", "EP01~EP10", ReleaseVersion{Height: 1080, Language: both, Video: "AVC", Audio: "AAC+EAC3", Collection: true}},
		{"single-episode-audio-tracks", "3522", "202", "EP12 END", ReleaseVersion{Height: 1080, Language: both, Video: "AVC", Audio: "AAC+EAC3"}},
		{"nonstandard-height", "3536", "34", "BIG5 AVC_aac 816p", ReleaseVersion{Height: 816, Language: SubtitleTraditional, Video: "AVC", Audio: "AAC"}},
		{"ambiguous-dual-resolution", "3640", "202", "[01-24][818P][820P][Bilibili][WEB-DL][AVC AAC][中文版&日语版]", ReleaseVersion{Height: 820, Ambiguous: true, Collection: true, Source: "WEB-DL", Video: "AVC", Audio: "AAC"}},
		{"external-subs", "3449", "370", "[01-12 合集][WebRip 1080p HEVC-10bit AAC][简繁外挂字幕]", ReleaseVersion{Height: 1080, Language: both, Video: "HEVC", Audio: "AAC", BitDepth: 10, Source: "WEBRip", Subtitles: "外挂", Collection: true}},
		{"script-order-reversed", "4071", "202", "(1080P 繁简中字 多国字幕)", ReleaseVersion{Height: 1080, Language: both}},
		{"channel-count-and-depth", "4120", "1257", "[Ma10p_2160p][x265_2eac3][日英双语-多国字幕]", ReleaseVersion{Height: 2160, Source: "WEBRip", Video: "HEVC", Audio: "EAC3", BitDepth: 10}},
	} {
		t.Run(test.name, func(t *testing.T) {
			count := 0
			for _, release := range releases {
				if release.anime != test.anime || release.group != test.group || !strings.Contains(release.title, test.excerpt) {
					continue
				}
				count++
				if got := ParseReleaseVersion(release.title); got != test.want {
					t.Fatalf("%q: got %+v, want %+v", release.title, got, test.want)
				}
			}
			if count == 0 {
				t.Fatal("annotated title missing from corpus")
			}
			t.Logf("%d matching titles", count)
		})
	}
}

func TestReleaseCorpusCoverage(t *testing.T) {
	type coverage struct {
		total, height, language, video, audio, container, source, uncertain int
	}
	add := func(counts *coverage, version ReleaseVersion) {
		counts.total++
		for _, field := range []struct {
			count *int
			known bool
		}{
			{&counts.height, version.Height > 0 && !version.Ambiguous},
			{&counts.language, version.Language != SubtitleUnknown},
			{&counts.video, version.Video != ""},
			{&counts.audio, version.Audio != ""},
			{&counts.container, version.Container != ""},
			{&counts.source, version.Source != ""},
			{&counts.uncertain, version.Ambiguous},
		} {
			if field.known {
				(*field.count)++
			}
		}
	}
	var overall, outsideConan coverage
	byAnime := map[string]*coverage{}
	groups := map[string]bool{}
	for _, release := range readReleaseCorpus(t) {
		version := ParseReleaseVersion(release.title)
		if byAnime[release.anime] == nil {
			byAnime[release.anime] = new(coverage)
		}
		add(byAnime[release.anime], version)
		add(&overall, version)
		if release.anime != "227" {
			add(&outsideConan, version)
		}
		groups[release.group] = true
	}
	var animeIDs []string
	for id := range byAnime {
		animeIDs = append(animeIDs, id)
	}
	slices.Sort(animeIDs)
	for _, id := range animeIDs {
		t.Logf("anime %s: %+v", id, *byAnime[id])
	}
	t.Logf("%d anime, %d groups, overall %+v, excluding anime 227 %+v", len(byAnime), len(groups), overall, outsideConan)
	if overall.total < 2500 || len(byAnime) < 18 || len(groups) < 55 {
		t.Fatal("corpus lost its source diversity")
	}
}

func TestReleaseCorpusSeasonEpisodes(t *testing.T) {
	seasonEpisode := regexp.MustCompile(`(?i)\bS\d{1,2}[\s._-]*E\s*P?\s*(\d{1,4}(?:\.\d+)?)`)
	count := 0
	for _, release := range readReleaseCorpus(t) {
		match := seasonEpisode.FindStringSubmatch(release.title)
		if match == nil {
			continue
		}
		count++
		number, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			t.Fatalf("%q: season episode %q: %v", release.title, match[1], err)
		}
		want := strconv.FormatFloat(number, 'f', -1, 64)
		if got := ParseEpisode(release.title); got != want {
			t.Errorf("ParseEpisode(%q) = %q, want %q", release.title, got, want)
		}
	}
	if count < 40 {
		t.Fatalf("corpus season episodes = %d, want at least 40", count)
	}
}

func TestReleaseVersionAudioAliases(t *testing.T) {
	for _, title := range []string{
		"[Group] Show - 01 [1080p AVC EAC3]",
		"[Group] Show - 01 [1080p AVC E-AC3]",
		"[Group] Show - 01 [1080p AVC E-AC-3]",
		"[Group] Show - 01 [1080p AVC DD+2.0]",
	} {
		if got := ParseReleaseVersion(title).Audio; got != "EAC3" {
			t.Errorf("ParseReleaseVersion(%q).Audio = %q, want EAC3", title, got)
		}
	}
	for _, test := range []struct {
		title string
		want  string
	}{
		{"[Group] Show - 01 [1080p AVC EAC3+AC3]", "EAC3+AC3"},
		{"[Group] Show - 01 [1080p AVC DDP2.0+AC3]", "EAC3+AC3"},
	} {
		if got := ParseReleaseVersion(test.title).Audio; got != test.want {
			t.Errorf("ParseReleaseVersion(%q).Audio = %q, want %q", test.title, got, test.want)
		}
	}
}

func TestReleaseVersionEpisodePrefixedCollections(t *testing.T) {
	for _, title := range []string{
		"[Group] Show - EP01~EP10 [1080p AVC AAC]",
		"[Group] Show - EP14-EP20 [1080p AVC AAC]",
	} {
		if !ParseReleaseVersion(title).Collection {
			t.Errorf("ParseReleaseVersion(%q).Collection = false, want true", title)
		}
	}
	if got := ParseReleaseVersion("[Group] Show - EP07 [1080p AVC AAC]").Collection; got {
		t.Error("single prefixed episode was classified as a collection")
	}
}
