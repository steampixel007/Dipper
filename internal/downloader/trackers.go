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
