package downloader

import (
	"slices"
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
// BuiltinTrackers returns the announce URLs every task receives after its own
// tiers and the user's extra tier.
func BuiltinTrackers() []string {
	return slices.Clone(builtinTrackers)
}

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

// reconciledTiers rebuilds a torrent's announce list when a dynamic tracker
// tier changes: the torrent's own metadata tiers stay, the previously applied
// user and remote URLs and the built-ins are stripped, and the current user,
// remote and built-in tiers are appended in that order. It also reports which
// configured URLs each dynamic tier actually injected, so a later settings
// change removes only what the tiers added.
func reconciledTiers(current [][]string, previousUser, user, previousRemote, remote []string) ([][]string, []string, []string) {
	drop := make(map[string]bool)
	for _, tracker := range previousUser {
		drop[tracker] = true
	}
	for _, tracker := range previousRemote {
		drop[tracker] = true
	}
	for _, tracker := range builtinTrackers {
		drop[tracker] = true
	}
	var tiers [][]string
	seen := make(map[string]bool)
	add := func(tier []string, bypassDrop bool) {
		var clean []string
		for _, tracker := range tier {
			if (drop[tracker] && !bypassDrop) || tracker == "" || seen[tracker] {
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
		add(tier, false)
	}
	// URLs from a new tier that the surviving own tiers already carry are
	// not that tier's additions; only genuinely injected ones may be removed
	// by a later settings change.
	injectedUser := newlyInjected(user, seen)
	add(user, true)
	injectedRemote := newlyInjected(remote, seen)
	// The new dynamic tiers and the built-ins are intentional; they bypass
	// the removal filter and deduplicate against whatever survived it.
	add(remote, true)
	add(builtinTrackers, true)
	return tiers, injectedUser, injectedRemote
}

func newlyInjected(trackers []string, seen map[string]bool) []string {
	var injected []string
	for _, tracker := range trackers {
		if tracker != "" && !seen[tracker] {
			injected = append(injected, tracker)
		}
	}
	return injected
}
