package proxy

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

// stub isolates the test from the machine it runs on: its environment
// variables, its registry, and ProxyFromEnvironment's process-wide cache.
func stub(t *testing.T, mode, custom string, sys System, env *url.URL) {
	t.Helper()
	Configure(mode, custom)
	t.Cleanup(func() { Configure(ModeAuto, "") })
	originalRead := readSys
	originalEnv := envProxy
	readSys = func() System { return sys }
	envProxy = func(*http.Request) (*url.URL, error) { return env, nil }
	sysMu.Lock()
	sysAt, sysVal = time.Time{}, System{}
	sysMu.Unlock()
	t.Cleanup(func() { readSys, envProxy = originalRead, originalEnv })
}

func proxyFor(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Func(req)
	if err != nil {
		t.Fatalf("Func: %v", err)
	}
	return got
}

func TestFuncFollowsConfiguredModes(t *testing.T) {
	env, _ := url.Parse("http://192.0.2.2:7890")
	cases := []struct {
		name     string
		mode     string
		custom   string
		sys      System
		env      *url.URL
		request  string
		want     string
		wantNone bool
	}{
		{name: "custom bare address is http", mode: ModeCustom, custom: "192.0.2.10:7897", request: "https://mikanime.tv", want: "http://192.0.2.10:7897"},
		{name: "custom address keeps its scheme", mode: ModeCustom, custom: "socks5://192.0.2.10:1080", request: "https://mikanime.tv", want: "socks5://192.0.2.10:1080"},
		{name: "custom beats environment and system", mode: ModeCustom, custom: "http://192.0.2.10:7897", env: env, sys: System{URL: "http://192.0.2.1:7897"}, request: "https://mikanime.tv", want: "http://192.0.2.10:7897"},
		{name: "direct never proxies", mode: ModeDirect, sys: System{URL: "http://192.0.2.1:7897"}, request: "https://mikanime.tv", wantNone: true},
		{name: "custom skips loopback", mode: ModeCustom, custom: "192.0.2.10:7897", request: "http://127.0.0.3/torrent", wantNone: true},
		{name: "auto prefers the environment", mode: ModeAuto, sys: System{URL: "http://192.0.2.1:7897"}, env: env, request: "https://mikanime.tv", want: "http://192.0.2.2:7890"},
		{name: "auto uses the system proxy when the environment names none", mode: ModeAuto, sys: System{URL: "http://192.0.2.1:7897"}, request: "https://mikanime.tv", want: "http://192.0.2.1:7897"},
		{name: "auto goes direct when neither names one", mode: ModeAuto, request: "https://mikanime.tv", wantNone: true},
		{name: "auto respects the system bypass list", mode: ModeAuto, sys: System{URL: "http://192.0.2.1:7897", Bypass: []string{"*.mikanime.tv"}}, request: "https://mikanime.tv", wantNone: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub(t, tc.mode, tc.custom, tc.sys, tc.env)
			got := proxyFor(t, tc.request)
			if tc.wantNone {
				if got != nil {
					t.Fatalf("proxy = %q, want none", got)
				}
				return
			}
			if got == nil || got.String() != tc.want {
				t.Fatalf("proxy = %v, want %q", got, tc.want)
			}
		})
	}
}

func TestFuncInvalidCustomAddressIsAnError(t *testing.T) {
	stub(t, ModeCustom, "://not-a-url", System{}, nil)
	req, err := http.NewRequest(http.MethodGet, "https://mikanime.tv", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Func(req); err == nil {
		t.Fatalf("proxy = %v, want an error for a malformed address", got)
	}
}

func TestDescribeReportsEnvironmentAddressWithoutUserInfo(t *testing.T) {
	stub(t, ModeAuto, "", System{}, &url.URL{Scheme: "http", Host: "192.0.2.2:7890", User: url.UserPassword("user", "pass")})
	address, source := Describe()
	if source != "environment" || address != "http://192.0.2.2:7890" {
		t.Fatalf("Describe = %q, %q", address, source)
	}
}

func TestBypassed(t *testing.T) {
	cases := []struct {
		host string
		list []string
		want bool
	}{
		{host: "api.example.com", list: []string{"api.example.com"}, want: true},
		{host: "api.example.com", list: []string{"*.example.com"}, want: true},
		{host: "api.example.com", list: []string{".example.com"}, want: true},
		{host: "example.com", list: []string{".example.com"}, want: true},
		{host: "elsewhere.org", list: []string{"*.example.com"}, want: false},
		{host: "intranet", list: []string{"<local>"}, want: true},
		{host: "api.example.com", list: []string{"<local>"}, want: false},
		{host: "api.example.com", list: nil, want: false},
	}
	for _, tc := range cases {
		if got := bypassed(tc.host, tc.list); got != tc.want {
			t.Errorf("bypassed(%q, %q) = %v, want %v", tc.host, tc.list, got, tc.want)
		}
	}
}

func TestSystemCache(t *testing.T) {
	calls := 0
	original := readSys
	readSys = func() System { calls++; return System{URL: "http://192.0.2.1:7897"} }
	t.Cleanup(func() { readSys = original })
	sysMu.Lock()
	sysAt, sysVal = time.Time{}, System{}
	sysMu.Unlock()
	if got := system(); got.URL != "http://192.0.2.1:7897" {
		t.Fatalf("system = %+v", got)
	}
	readSys = func() System { calls++; return System{} }
	if got := system(); got.URL != "http://192.0.2.1:7897" {
		t.Fatalf("cached value was not reused: %+v", got)
	}
	if calls != 1 {
		t.Fatalf("readSystem calls = %d, want 1", calls)
	}
}

func TestValidateCustomAddress(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		address string
		wantErr bool
	}{
		{name: "auto ignores address", mode: ModeAuto, address: "whatever"},
		{name: "direct ignores address", mode: ModeDirect, address: ""},
		{name: "unknown mode is left to the reader", mode: "future"},
		{name: "custom http", mode: ModeCustom, address: "http://192.0.2.1:7897"},
		{name: "custom https", mode: ModeCustom, address: "https://192.0.2.1:7897"},
		{name: "custom socks5", mode: ModeCustom, address: "socks5://192.0.2.1:1080"},
		{name: "custom bare address means http", mode: ModeCustom, address: "192.0.2.1:7897"},
		{name: "custom requires address", mode: ModeCustom, wantErr: true},
		{name: "custom rejects other schemes", mode: ModeCustom, address: "file://192.0.2.1/share", wantErr: true},
		{name: "custom rejects userinfo", mode: ModeCustom, address: "http://user:pass@192.0.2.1:7897", wantErr: true},
		{name: "custom rejects garbage", mode: ModeCustom, address: "://", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.mode, tc.address)
			if tc.wantErr && err == nil {
				t.Fatalf("Validate(%q, %q) = nil, want error", tc.mode, tc.address)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate(%q, %q) = %v", tc.mode, tc.address, err)
			}
		})
	}
}
