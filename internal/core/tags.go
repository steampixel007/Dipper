package core

import (
	"regexp"
)

// ReleaseTags pulls the technical labels out of a release title and returns
// them in a stable order. Each returned word is a normalized literal that the
// rule engine can match against the title, so recognized and unrecognized
// labels stay the same kind of input for the user.
func ReleaseTags(title string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(w string) {
		w = Normalize(w)
		if w != "" && !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	for _, m := range resolutionPattern.FindAllStringSubmatch(title, -1) {
		add(m[1])
	}
	for _, m := range codecPattern.FindAllStringSubmatch(title, -1) {
		add(m[1])
	}
	for _, m := range containerBundlePattern.FindAllStringSubmatch(title, -1) {
		add(m[1] + "\u53cc\u8bed" + m[2])
		add(m[1])
		add(m[2])
	}
	for _, m := range containerPattern.FindAllStringSubmatch(title, -1) {
		add(m[1])
	}
	for _, m := range sourcePattern.FindAllStringSubmatch(title, -1) {
		add(m[1])
	}
	for _, m := range streamingPattern.FindAllStringSubmatch(title, -1) {
		add(m[1])
	}
	for _, m := range subtitlePattern.FindAllStringSubmatch(title, -1) {
		add(m[1] + m[2])
	}
	for _, m := range audioPattern.FindAllStringSubmatch(title, -1) {
		add(m[1])
	}
	for _, m := range hdrPattern.FindAllStringSubmatch(title, -1) {
		add(m[1])
	}
	for _, m := range flagsPattern.FindAllStringSubmatch(title, -1) {
		add(m[1])
	}
	return out
}

var resolutionPattern = regexp.MustCompile("(?i)\\b(2160p|1440p|1080p|720p|480p|360p|4k)\\b")
var codecPattern = regexp.MustCompile("(?i)\\b(hevc|h\\.?265|x265|av1|avc|h\\.?264|x264)\\b")
var containerBundlePattern = regexp.MustCompile("(?i)(mp4|mkv)(?:/|\uff0f)(mp4|mkv)(?:\\(?pgs\\)?)?")
var containerPattern = regexp.MustCompile("(?i)\\b(mkv|mp4|flac|aac|ac3|eac3|opus|dts)\\b")
var sourcePattern = regexp.MustCompile("(?i)\\b(web-?rip|web-?dl|bdrip|tvrip|dvdrip|hdtv|web)\\b")
var subtitlePattern = regexp.MustCompile("(\u7b80\u7e41\u65e5|\u4e2d\u65e5|\u7e41\u65e5|\u7b80\u65e5|\u7b80\u7e41|\u7b80\u4f53|\u7e41\u4f53|\u7b80|\u7e41|\u65e5|\u82f1|\u4e2d)(\u53cc\u8bed|\u591a\u8bed|\u5185\u5d4c|\u5185\u5c01|\u5916\u6302|\u5b57\u5e55)")
var streamingPattern = regexp.MustCompile("(?i)\\b(b-?global|bahamut|baha|at-?x|netflix|disney|crunchyroll|bilibili)\\b")
var hdrPattern = regexp.MustCompile("(?i)\\b(hdr10\\+|hdr10\\b|hdr\\b|dolby.?vision\\b|dovi\\b|dv\\b|hlg\\b)")
var audioPattern = regexp.MustCompile("(?i)\\b(10bit|8bit|aac|flac|ac3|eac3|opus|dts)\\b")
var flagsPattern = regexp.MustCompile("(?i)\\b(v\\d+|chs|cht|gb|big5|\u7b80|\u7e41|\u7b80\u7e41|\u5916\u6302|\u5185\u5d4c|\u5185\u5c01)\\b")
