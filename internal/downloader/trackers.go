package downloader

import (
	"strings"

	"github.com/anacrolix/torrent"
)

// builtinTrackers are stable public HTTP and UDP trackers appended to every
// task after its own announce tiers. They add redundancy for torrents whose
// announce list is empty, unreachable or blocked by an earlier tracker
// failure. They are deliberately few and well-known; peer exchange and DHT
// stay primary.
var builtinTrackers = []string{
	"udp://tracker.opentrackr.org:1337/announce",
	"https://tracker.tamersunion.org:443/announce",
	"http://tracker.opentrackr.org:1337/announce",
	"udp://open.demonii.com:1337/announce",
}

// appendTrackers appends each tier as its own announce tier, skipping URLs an
// earlier tier already carries, and mutates the spec in place.
func appendTrackers(spec *torrent.TorrentSpec, tiers ...[]string) {
	if spec == nil {
		return
	}
	seen := make(map[string]bool)
	for _, tier := range spec.Trackers {
		for _, tracker := range tier {
			seen[tracker] = true
		}
	}
	for _, tier := range tiers {
		var extra []string
		for _, tracker := range tier {
			if !seen[tracker] {
				extra = append(extra, tracker)
			}
		}
		if len(extra) > 0 {
			spec.Trackers = append(spec.Trackers, extra)
			for _, tracker := range extra {
				seen[tracker] = true
			}
		}
	}
}

func appendBuiltinTrackers(spec *torrent.TorrentSpec) {
	appendTrackers(spec, builtinTrackers)
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

// reconciledTiers rebuilds a torrent's announce list when the user tracker
// tier changes: the torrent's own metadata tiers stay, the previously applied
// user URLs and the built-ins are stripped, and the current user tier plus
// the built-ins are appended as their own tiers. It also reports which
// configured URLs the rebuilt list actually injected as the user tier.
func reconciledTiers(current [][]string, previous, extras []string) ([][]string, []string) {
	drop := make(map[string]bool)
	for _, tracker := range previous {
		drop[tracker] = true
	}
	for _, tracker := range builtinTrackers {
		drop[tracker] = true
	}
	var tiers [][]string
	seen := make(map[string]bool)
	add := func(tier []string) {
		var clean []string
		for _, tracker := range tier {
			if drop[tracker] || seen[tracker] {
				continue
			}
			seen[tracker] = true
			clean = append(clean, tracker)
		}
		if len(clean) > 0 {
			tiers = append(tiers, clean)
		}
	}
	for _, tier := range current {
		add(tier)
	}
	// URLs from the new user tier that the surviving own tiers already carry
	// are not user additions; only genuinely injected ones may be removed by a
	// later settings change.
	var injected []string
	for _, tracker := range extras {
		if !seen[tracker] {
			injected = append(injected, tracker)
		}
	}
	// The new user tier and the built-ins are intentional; they bypass the
	// removal filter and deduplicate against whatever survived it.
	for _, tier := range [][]string{extras, builtinTrackers} {
		var clean []string
		for _, tracker := range tier {
			if tracker == "" || seen[tracker] {
				continue
			}
			seen[tracker] = true
			clean = append(clean, tracker)
		}
		if len(clean) > 0 {
			tiers = append(tiers, clean)
		}
	}
	return tiers, injected
}

// injectedExtra reports which configured tracker URLs the admission append
// actually added: URLs the torrent's own metadata already carries are not
// user additions, so clearing the setting must not remove them.
func injectedExtra(spec *torrent.TorrentSpec, extras []string) []string {
	own := make(map[string]bool)
	for _, tier := range spec.Trackers {
		for _, tracker := range tier {
			own[tracker] = true
		}
	}
	var injected []string
	for _, tracker := range extras {
		if !own[tracker] {
			injected = append(injected, tracker)
		}
	}
	return injected
}
