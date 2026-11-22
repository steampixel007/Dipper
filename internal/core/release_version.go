package core

import (
	"regexp"
	"strconv"
	"strings"
)

type SubtitleLanguage uint8

const (
	SubtitleUnknown     SubtitleLanguage = 0
	SubtitleSimplified  SubtitleLanguage = 1
	SubtitleTraditional SubtitleLanguage = 2
)

type ReleaseVersion struct {
	Height                                              int
	Language                                            SubtitleLanguage
	Video, Audio, Container, Source, Subtitles, Edition string
	BitDepth                                            int
	HDR, Collection, Ambiguous                          bool
}

type releaseLabel struct {
	pattern *regexp.Regexp
	value   string
	meaning string
}

func releaseLabels(labels ...[3]string) []releaseLabel {
	result := make([]releaseLabel, 0, len(labels))
	for _, label := range labels {
		result = append(result, releaseLabel{regexp.MustCompile(label[0]), label[1], label[2]})
	}
	return result
}

var releaseVideo = releaseLabels(
	[3]string{`\b(?:avc|[hx][. ]?264)\b`, "AVC", "视频编码，通常更容易在旧设备上播放"},
	[3]string{`\b(?:hevc|[hx][. ]?265)\b`, "HEVC", "视频编码，压缩效率较高，部分旧设备可能不支持"},
	[3]string{`\bav1\b`, "AV1", "视频编码，压缩效率较高，需要设备支持"},
)

var releaseAudio = releaseLabels(
	[3]string{`\b(?:heaac|he-aac|aac(?:-lc)?)(?:[257][.]?[01])?\b`, "AAC", "音频编码，和画面清晰度无关"},
	[3]string{`\bflac(?:x\d+)?\b`, "FLAC", "无损音频编码，和画面清晰度无关"},
	[3]string{`\b(?:\d?e-?ac-?3|ddp)(?:[257][.]?[01])?\b|\bdd\+(?:[257][.]?[01])?`, "EAC3", "杜比数字增强音频编码"},
	[3]string{`\b(?:ac-?3|dd)(?:[257][.]?[01])?\b`, "AC3", "杜比数字音频编码"},
	[3]string{`\bdts(?:-hd)?\b`, "DTS", "音频编码，和画面清晰度无关"},
	[3]string{`\btruehd\b`, "TrueHD", "音频编码，和画面清晰度无关"},
	[3]string{`\bopus\b`, "Opus", "音频编码，和画面清晰度无关"},
)

var releaseContainer = releaseLabels(
	[3]string{`\bmp4\b`, "MP4", "视频文件格式，通常更容易在不同设备上播放"},
	[3]string{`\bmkv\b`, "MKV", "视频文件格式，可以封装多条字幕或音轨"},
	[3]string{`\bavi\b`, "AVI", "视频文件格式"},
	[3]string{`\brmvb\b`, "RMVB", "视频文件格式"},
	[3]string{`\bwebm\b`, "WebM", "视频文件格式"},
)

var releaseSource = releaseLabels(
	[3]string{`\b(?:bd[- ]?rip|blu[- ]?ray|(?:bd)?remux|bdmv|jpbd)\b|\[bd\]|\bbd(?:720|1080|2160)[pi]\b`, "BD", "蓝光来源"},
	[3]string{`\bweb[- .]?dl\b`, "WEB-DL", "网络来源，标题标注为直接获取的版本"},
	[3]string{`\bweb[- .]?rip\b`, "WEBRip", "网络来源的转制版本"},
	[3]string{`\b(?:b-?global|baha(?:mut)?|crunchyroll|netflix|bilibili)\b|\[web\]`, "WEB", "网络来源"},
	[3]string{`\b(?:tv-?rip|hdtv|at-?x)\b`, "TV", "电视播出来源"},
	[3]string{`\bdvd[- ]?rip\b|\[dvd\]`, "DVD", "DVD 来源"},
)

var releaseSubtitleFormat = releaseLabels(
	[3]string{`内嵌|內嵌|硬字幕|硬压|硬壓|\b(?:hardsub|hard[ -]sub)\b`, "内嵌", "字幕已压进画面，播放时不能关闭"},
	[3]string{`内封|內封|内挂|內掛|软字幕|軟字幕|\b(?:softsub|soft[ -]sub)\b`, "内封", "字幕封装在视频里，需要播放器支持"},
	[3]string{`外挂|外掛|\b(?:external sub(?:title)?s?)\b`, "外挂", "字幕与视频分开，需要播放器支持加载字幕文件"},
)

var releaseEdition = releaseLabels(
	[3]string{`\b(?:uncensored|unrated)\b|无修正|無修正|无修版|無修版`, "无修正", "标题标注为无修正版"},
	[3]string{`修正版|\bv[2-9]\d*\b`, "修订", "标题标注为修订发布"},
	[3]string{`先行版|\bpreview\b`, "先行", "标题标注为先行发布"},
)

var (
	releaseResolution          = regexp.MustCompile(`\b(?:(?:bd|ai)?(2160|1440|1080|960|900|820|818|816|810|800|768|720|576|540|480|360)[pi]|\d{3,4}\s*[x×*]\s*(2160|1440|1080|960|900|820|818|816|810|800|768|720|576|540|480|360)p?)\b`)
	release4K                  = regexp.MustCompile(`\b(?:4k|uhd)\b`)
	releaseDepth               = regexp.MustCompile(`\b(?:(8|10|12)[- ]?bits?|(?:hi|ma(?:in)?)(10)p)\b`)
	releaseHDR                 = regexp.MustCompile(`\b(?:hdr(?:10\+?)?|hlg|dolby[. -]?vision|dovi)\b`)
	releaseCollection          = regexp.MustCompile(`合[集輯]|全集|全(?:[2-9]|[1-9][0-9]+)[集话話]|批量|\b(?:batch|complete)\b|[\[(]\d{1,4}\s*[-~]\s*\d{1,4}(?:\s*fin|\s*合[集輯])?[\])]|\bs\d+e\d+\s*[-~]\s*\d+\b`)
	releaseLanguageBoth        = regexp.MustCompile(`[简簡]繁|繁[简簡](?:[日中英双雙内內外字]|$|[^[:alnum:]\p{Han}])|[简簡][体體][&+ /]*繁[体體]|[\[(][简簡]\s*[/&+]\s*繁[\])]`)
	releaseLanguageASCII       = regexp.MustCompile(`\b(?:zh[- ]?hans|zh[- ]?cn|chs|gbk?|gb2312)\b`)
	releaseTraditionalASCII    = regexp.MustCompile(`\b(?:zh[- ]?hant|zh[- ]?(?:tw|hk)|cht|big[- ]?5)\b`)
	releaseSimplifiedShort     = regexp.MustCompile(`[\[(/+&]sc[\])/+&]`)
	releaseTraditionalShort    = regexp.MustCompile(`[\[(/+&]tc[\])/+&]`)
	releaseCamera              = regexp.MustCompile(`\b(?:cam(?:-rip)?|telecine|telesync)\b`)
	releaseSizeGB              = regexp.MustCompile(`\d+(?:\.\d+)?\s*gb\b`)
	releaseNoChineseSubtitles  = regexp.MustCompile(`[无無](?:中文字幕|中文字|字幕)|[日英]语无字|[日英]語無字|[\[(]生肉`)
	releaseLanguageSimple      = regexp.MustCompile(`简体|簡體|简中|簡中|[简簡][日英](?:[双雙内內外字]|[\])]|$)|[简簡](?:[双雙]语|[双雙]語|内嵌|內嵌|内封|內封|外挂|外掛|字幕)|[\[(][简簡](?:[\])]|$)`)
	releaseLanguageTraditional = regexp.MustCompile(`繁[体體中]|繁[日英](?:[双雙内內外字]|[\])]|$)|繁(?:[双雙]语|[双雙]語|内嵌|內嵌|内封|內封|外挂|外掛|字幕)|[\[(]繁(?:[\])]|$)`)
	releaseGroupPrefix         = regexp.MustCompile(`^\s*\[[^\]]+\]`)
	releaseTextNormalizer      = strings.NewReplacer("【", "[", "】", "]", "_", " ")
)

func releaseLanguage(text string) SubtitleLanguage {
	text = releaseSizeGB.ReplaceAllString(text, " ")
	if releaseNoChineseSubtitles.MatchString(text) {
		return SubtitleUnknown
	}
	var language SubtitleLanguage
	if releaseLanguageBoth.MatchString(text) {
		return SubtitleSimplified | SubtitleTraditional
	}
	if releaseLanguageSimple.MatchString(text) || releaseLanguageASCII.MatchString(text) || releaseSimplifiedShort.MatchString(text) {
		language |= SubtitleSimplified
	}
	if releaseLanguageTraditional.MatchString(text) || (releaseTraditionalShort.MatchString(text) && !releaseCamera.MatchString(text)) || releaseTraditionalASCII.MatchString(text) {
		language |= SubtitleTraditional
	}
	return language
}

func singleReleaseLabel(text string, labels []releaseLabel) (string, bool) {
	var value string
	for _, label := range labels {
		if !label.pattern.MatchString(text) {
			continue
		}
		if value != "" && value != label.value {
			return "", true
		}
		value = label.value
	}
	return value, false
}

func ParseReleaseVersion(title string) ReleaseVersion {
	text := releaseTextNormalizer.Replace(Normalize(title))
	if prefix := releaseGroupPrefix.FindStringIndex(text); prefix != nil && prefix[1] < len(text) {
		text = text[prefix[1]:]
	}
	var version ReleaseVersion
	version.Language = releaseLanguage(text)
	version.Collection = releaseCollection.MatchString(text)
	version.HDR = releaseHDR.MatchString(text)
	for _, match := range releaseResolution.FindAllStringSubmatch(text, -1) {
		height, _ := strconv.Atoi(match[1] + match[2])
		if version.Height != 0 && version.Height != height {
			version.Ambiguous = true
		}
		version.Height = height
	}
	if release4K.MatchString(text) {
		if version.Height != 0 && version.Height != 2160 {
			version.Ambiguous = true
		}
		version.Height = 2160
	}
	for _, match := range releaseDepth.FindAllStringSubmatch(text, -1) {
		depth, _ := strconv.Atoi(match[1] + match[2])
		if version.BitDepth != 0 && version.BitDepth != depth {
			version.Ambiguous = true
		}
		version.BitDepth = depth
	}
	for _, field := range []struct {
		value  *string
		labels []releaseLabel
	}{
		{&version.Video, releaseVideo},
		{&version.Container, releaseContainer},
		{&version.Subtitles, releaseSubtitleFormat},
	} {
		value, ambiguous := singleReleaseLabel(text, field.labels)
		*field.value = value
		version.Ambiguous = version.Ambiguous || ambiguous
	}
	var audio []string
	for _, label := range releaseAudio {
		if label.pattern.MatchString(text) {
			audio = append(audio, label.value)
		}
	}
	version.Audio = strings.Join(audio, "+")
	for _, label := range releaseSource {
		if label.pattern.MatchString(text) {
			version.Source = label.value
			break
		}
	}
	for _, label := range releaseEdition {
		if label.value != "修订" && label.pattern.MatchString(text) {
			version.Edition = label.value
			break
		}
	}
	return version
}

func ReleaseWordMeaning(word string) string {
	text := releaseTextNormalizer.Replace(Normalize(strings.TrimSpace(word)))
	if releaseResolution.MatchString(text) || release4K.MatchString(text) {
		return "画面分辨率，数字越大通常越清晰"
	}
	if releaseDepth.MatchString(text) {
		return "视频色深，影响色彩过渡和设备兼容性"
	}
	if releaseHDR.MatchString(text) {
		return "高动态范围，需要屏幕和播放器支持"
	}
	switch releaseLanguage("[" + text + "]") {
	case SubtitleSimplified | SubtitleTraditional:
		return "标题标注有简体和繁体中文字幕"
	case SubtitleSimplified:
		return "简体中文字幕"
	case SubtitleTraditional:
		return "繁体中文字幕"
	}
	for _, labels := range [][]releaseLabel{releaseVideo, releaseAudio, releaseContainer, releaseSource, releaseSubtitleFormat, releaseEdition} {
		for _, label := range labels {
			if label.pattern.MatchString(text) {
				return label.meaning
			}
		}
	}
	if releaseCollection.MatchString(text) {
		return "包含多集的发布"
	}
	return "含义未识别"
}
