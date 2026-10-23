//go:build !windows

package downloader

import (
	"net"
	"time"
)

var awakeStart = time.Now()

// Interface availability does not prove internet reachability. Core can override
// this with OS connectivity events through SetNetworkAvailable.
func networkAvailable() bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range interfaces {
		if iface.Flags&(net.FlagUp|net.FlagRunning) != net.FlagUp|net.FlagRunning || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.IsGlobalUnicast() && !ip.IsLoopback() {
				return true
			}
		}
	}
	return false
}

func awakeTime() time.Duration { return time.Since(awakeStart) }
