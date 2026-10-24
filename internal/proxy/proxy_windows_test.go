//go:build windows

package proxy

import "testing"

func TestParseWindows(t *testing.T) {
	cases := []struct {
		name   string
		server string
		want   string
	}{
		{name: "one server for every protocol", server: "127.0.0.1:7897", want: "http://127.0.0.1:7897"},
		{name: "per-protocol entries prefer https", server: "http=127.0.0.1:8080;https=127.0.0.1:7897", want: "http://127.0.0.1:7897"},
		{name: "socks is used when http entries are absent", server: "socks=127.0.0.1:1080", want: "socks5://127.0.0.1:1080"},
		{name: "empty is none", server: "  ", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseWindows(tc.server, ""); got.URL != tc.want {
				t.Fatalf("parseWindows(%q) URL = %q, want %q", tc.server, got.URL, tc.want)
			}
		})
	}
}
