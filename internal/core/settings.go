package core

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// trackerSchemes are the announce URL schemes the embedded BT client speaks.
var trackerSchemes = map[string]bool{"udp": true, "http": true, "https": true, "ws": true, "wss": true}

const (
	MaxTrackerLists    = 16
	MaxTrackersPerList = 200
)

func ValidTrackerListURL(address string) bool {
	parsed, err := url.Parse(address)
	return err == nil && len(address) <= 2048 && parsed.Hostname() != "" &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.User == nil && parsed.Fragment == ""
}

func validateTrackerLists(lists []TrackerList) error {
	if len(lists) > MaxTrackerLists {
		return fmt.Errorf("too many tracker list subscriptions")
	}
	seen := make(map[string]bool)
	for _, list := range lists {
		if !ValidTrackerListURL(list.URL) || seen[list.URL] || len(list.Trackers) == 0 || len(list.Trackers) > MaxTrackersPerList {
			return fmt.Errorf("invalid tracker list subscription")
		}
		seen[list.URL] = true
		for _, tracker := range list.Trackers {
			if tracker == "" || trackerURL(tracker) != tracker {
				return &TrackerError{Line: tracker}
			}
		}
	}
	return nil
}

func (s Settings) ActiveExtraTrackers() []string {
	urls := ParseTrackers(s.ExtraTrackers)
	seen := make(map[string]bool)
	for _, tracker := range urls {
		seen[tracker] = true
	}
	for _, list := range s.TrackerLists {
		if list.Disabled {
			continue
		}
		for _, tracker := range list.Trackers {
			if !seen[tracker] {
				seen[tracker] = true
				urls = append(urls, tracker)
			}
		}
	}
	return urls
}

func ValidateSeedRatio(label string) error {
	if label == "" || label == "不限" {
		return nil
	}
	if _, ok := parseSeedRatio(label); ok {
		return nil
	}
	return fmt.Errorf("invalid seed ratio %q", label)
}

// SeedRatioTarget maps a share-ratio label to the engine's stop multiple: -1
// stops only on seeding time, and unknown labels select the default 1:1.
func SeedRatioTarget(label string) float64 {
	if label == "不限" {
		return -1
	}
	if target, ok := parseSeedRatio(label); ok {
		return target
	}
	return 1
}

func parseSeedRatio(label string) (float64, bool) {
	number, ok := strings.CutSuffix(label, ":1")
	if !ok || strings.ContainsAny(number, "xXpP_") {
		return 0, false
	}
	target, err := strconv.ParseFloat(number, 64)
	return target, err == nil && target > 0 && !math.IsInf(target, 0)
}

func (s Settings) EffectiveUploadLimit() int64 {
	if s.UploadUnlimited {
		return -1
	}
	return s.UploadLimitBytes
}

func (s Settings) EffectiveSeedDuration() time.Duration {
	if s.SeedTimeUnlimited || s.SeedMinutes == -1 {
		return -1
	}
	if s.SeedMinutes <= 0 || s.SeedMinutes > int64(math.MaxInt64)/int64(time.Minute) {
		return 0
	}
	return time.Duration(s.SeedMinutes) * time.Minute
}

func (s Settings) EffectiveSeedRatio() float64 {
	if s.SeedRatioUnlimited {
		return -1
	}
	return SeedRatioTarget(s.SeedRatio)
}

// TrackerError reports a tracker line whose announce URL cannot be parsed.
// The UI bridge translates it for display while keeping the offending line.
type TrackerError struct{ Line string }

func (e *TrackerError) Error() string {
	return "invalid tracker " + strconv.Quote(e.Line)
}

// ValidateTrackers rejects raw tracker text that carries a line no announce
// URL can be parsed from.
func ValidateTrackers(text string) error {
	for _, line := range trackerLines(text) {
		if trackerURL(line) == "" {
			return &TrackerError{Line: line}
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
