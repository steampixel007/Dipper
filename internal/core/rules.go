package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Normalize folds only ASCII case and full-width ASCII; script and punctuation
// differences outside that range retain their literal matching meaning.
func Normalize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 0xff01 && r <= 0xff5e {
			r -= 0xfee0
		}
		if r == 0x3000 {
			r = ' '
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		return r
	}, s)
}

func cleanWords(words []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range words {
		w = strings.TrimSpace(Normalize(w))
		if w != "" && !seen[w] {
			out = append(out, w)
			seen[w] = true
		}
	}
	return out
}

func NormalizeRule(r Rule) Rule {
	out := Rule{All: cleanWords(r.All), None: cleanWords(r.None)}
	for _, group := range r.Any {
		if words := cleanWords(group); len(words) > 0 {
			out.Any = append(out.Any, words)
		}
	}
	return out
}

func Match(title string, rule Rule) MatchResult {
	title = Normalize(title)
	rule = NormalizeRule(rule)
	for _, w := range rule.All {
		if !strings.Contains(title, w) {
			return MatchResult{false, "缺少必需词：" + w}
		}
	}
	for _, w := range rule.None {
		if strings.Contains(title, w) {
			return MatchResult{false, "包含排除词：" + w}
		}
	}
	for _, group := range rule.Any {
		found := false
		for _, w := range group {
			if strings.Contains(title, w) {
				found = true
				break
			}
		}
		if !found {
			return MatchResult{false, "缺少任意词组：" + strings.Join(group, " / ")}
		}
	}
	return MatchResult{true, "符合条件"}
}

func (r Rule) Match(title string) MatchResult { return Match(title, r) }

var episodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`第\s*(\d{1,3}(?:\.\d+)?)\s*[集话話]`),
	regexp.MustCompile(`(?i)(?:^|[\s\[(_-])(?:EP?|Episode)\s*(\d{1,3}(?:\.\d+)?)(?:[\s\])_.-]|$)`),
	regexp.MustCompile(`(?:\s-\s|\[)(\d{1,3}(?:\.\d+)?)(?:v\d+)?(?:[\s\]]|$)`),
}

func ParseEpisode(title string) string {
	for _, re := range episodePatterns {
		if m := re.FindStringSubmatch(Normalize(title)); m != nil {
			v, err := strconv.ParseFloat(m[1], 64)
			if err == nil {
				return strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
	}
	return ""
}

var suggestionPattern = regexp.MustCompile(`(?i)\b(?:2160p|1080p|720p|480p|HEVC|AVC|x265|x264|H[.]?265|H[.]?264|AAC|FLAC|MKV|MP4|CHS|CHT|GB|BIG5|WEB-DL|BDRip)\b|简日内嵌|繁日内嵌|简体|繁体|簡體|繁體|内封|內封|内嵌|內嵌|合集`)

func SuggestWords(title string) []string {
	return cleanWords(suggestionPattern.FindAllString(Normalize(title), -1))
}

func SubscriptionID(animeID, groupID string) string {
	return fmt.Sprintf("%d:%s%s", len(animeID), animeID, groupID)
}
