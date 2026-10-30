package downloader

import "github.com/anacrolix/torrent"

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

// appendBuiltinTrackers appends the built-in tracker list as its own announce
// tier, skipping URLs any existing tier already carries, and mutates the spec
// in place.
func appendBuiltinTrackers(spec *torrent.TorrentSpec) {
	if spec == nil {
		return
	}
	seen := make(map[string]bool)
	for _, tier := range spec.Trackers {
		for _, tracker := range tier {
			seen[tracker] = true
		}
	}
	var extra []string
	for _, tracker := range builtinTrackers {
		if !seen[tracker] {
			extra = append(extra, tracker)
		}
	}
	if len(extra) > 0 {
		spec.Trackers = append(spec.Trackers, extra)
	}
}
