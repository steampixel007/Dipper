package core

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// SeedRatioOptions are the accepted share-ratio stop targets, in display order.
var SeedRatioOptions = []string{"1:1", "2:1", "3:1", "不限"}

// trackerSchemes are the announce URL schemes the embedded BT client speaks.
var trackerSchemes = map[string]bool{"udp": true, "http": true, "https": true, "ws": true, "wss": true}

// ValidateSeedRatio accepts a share-ratio stop label from SeedRatioOptions or
// the empty default.
func ValidateSeedRatio(label string) error {
	if label == "" {
		return nil
	}
	for _, option := range SeedRatioOptions {
		if option == label {
			return nil
		}
	}
	return fmt.Errorf("invalid seed ratio %q", label)
}

// SeedRatioTarget maps a share-ratio label to the engine's stop multiple: -1
// stops only on seeding time, and unknown labels select the default 1:1.
func SeedRatioTarget(label string) float64 {
	switch label {
	case "2:1":
		return 2
	case "3:1":
		return 3
	case "不限":
		return -1
	}
	return 1
}

// ValidateTrackers rejects raw tracker text that carries a line no announce
// URL can be parsed from.
func ValidateTrackers(text string) error {
	for _, line := range trackerLines(text) {
		if trackerURL(line) == "" {
			return fmt.Errorf("invalid tracker %q", line)
		}
	}
	return nil
}

// ParseTrackers splits raw tracker text into unique announce URLs, dropping
// separators, surrounding space and blank entries.
func ParseTrackers(text string) []string {
	var trackers []string
	seen := make(map[string]bool)
	for _, line := range trackerLines(text) {
		if tracker := trackerURL(line); tracker != "" && !seen[tracker] {
			seen[tracker] = true
			trackers = append(trackers, tracker)
		}
	}
	return trackers
}

func trackerLines(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ';' || unicode.IsSpace(r) })
}

func trackerURL(line string) string {
	line = strings.TrimSpace(line)
	parsed, err := url.Parse(line)
	if err != nil || !trackerSchemes[strings.ToLower(parsed.Scheme)] || parsed.Host == "" {
		return ""
	}
	return line
}
