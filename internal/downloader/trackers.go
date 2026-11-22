package downloader

import "strings"

var legacyBuiltinTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"https://tracker.tamersunion.org:443/announce",
	"http://tracker.opentrackr.org:1337/announce",
	"udp://open.demonii.com:1337/announce",
}

// normalizeTrackers trims user-supplied announce URLs, drops empties and
// keeps the first copy of every URL.
func normalizeTrackers(trackers []string) []string {
	var clean []string
	seen := make(map[string]bool)
	for _, tracker := range trackers {
		tracker = strings.TrimSpace(tracker)
		if tracker == "" || seen[tracker] {
			continue
		}
		seen[tracker] = true
		clean = append(clean, tracker)
	}
	return clean
}

type trackerTiers struct {
	user, remote, legacyBuiltin []string
}

func reconciledTiers(current [][]string, previous, configured trackerTiers, disabled []string) ([][]string, trackerTiers) {
	drop := make(map[string]bool)
	for _, tier := range [][]string{previous.user, previous.remote, previous.legacyBuiltin} {
		for _, tracker := range tier {
			drop[tracker] = true
		}
	}
	excluded := make(map[string]bool)
	for _, tracker := range disabled {
		excluded[tracker] = true
	}
	var tiers [][]string
	seen := make(map[string]bool)
	add := func(tier []string, skip map[string]bool) []string {
		var clean []string
		for _, tracker := range tier {
			if skip[tracker] || tracker == "" || seen[tracker] {
				continue
			}
			seen[tracker] = true
			clean = append(clean, tracker)
		}
		if len(clean) > 0 {
			tiers = append(tiers, clean)
		}
		return clean
	}
	for _, tier := range current {
		add(tier, drop)
	}
	injected := trackerTiers{
		user:   add(configured.user, excluded),
		remote: add(configured.remote, excluded),
	}
	return tiers, injected
}
