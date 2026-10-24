// Package proxy resolves the proxy Dipper's outbound HTTP requests use.
// A desktop app launched from the Start menu has no *_PROXY in its
// environment, so without this every request to the source goes direct
// even when the user's system proxy is the only way to reach it.
package proxy

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	ModeAuto   = "auto"
	ModeDirect = "direct"
	ModeCustom = "custom"
)

// System is one proxy address plus the hosts that bypass it.
type System struct {
	URL    string
	Bypass []string
}

var (
	mu       sync.Mutex
	mode     = ModeAuto
	custom   string
	sysMu    sync.Mutex
	sysAt    time.Time
	sysVal   System
	readSys  = readSystem
	envProxy = http.ProxyFromEnvironment
)

// Configure stores the user's choice. Mode and address are published under
// one lock so a request cannot mix the old choice with a new address.
func Configure(modeName, customURL string) {
	modeName = strings.TrimSpace(modeName)
	switch modeName {
	case ModeDirect:
		modeName, customURL = ModeDirect, ""
	case ModeCustom:
		customURL = strings.TrimSpace(customURL)
	default:
		modeName, customURL = ModeAuto, ""
	}
	mu.Lock()
	mode, custom = modeName, customURL
	mu.Unlock()
}

// Validate checks a mode and address as they are persisted. An unknown mode
// is not an error: it is stored as read, and an app too old to know it
// falls back to auto.
func Validate(modeName, address string) error {
	if modeName != ModeCustom {
		return nil
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return errors.New("custom proxy requires an address")
	}
	u, err := parse(address)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") || u.Hostname() == "" || u.User != nil {
		return errors.New("invalid proxy address: want http, https or socks5 without userinfo")
	}
	return nil
}

// Func is an http.Transport Proxy function. Loopback never goes through a
// proxy: tests and local tooling must be able to observe requests.
func Func(req *http.Request) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}
	host := req.URL.Hostname()
	if loopback(host) {
		return nil, nil
	}
	m, address := snapshot()
	switch m {
	case ModeDirect:
		return nil, nil
	case ModeCustom:
		return parse(address)
	}
	if p, err := envProxy(req); p != nil || err != nil {
		return p, err
	}
	sys := system()
	if sys.URL == "" || bypassed(host, sys.Bypass) {
		return nil, nil
	}
	return parse(sys.URL)
}

func snapshot() (string, string) {
	mu.Lock()
	defer mu.Unlock()
	return mode, custom
}

// parse reads a proxy address; a bare host:port is an HTTP proxy.
func parse(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	return url.Parse(s)
}

func loopback(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// bypassed matches the system exception list: exact hosts, "*.example.com"
// or ".example.com" suffixes, and "<local>" for dotless names.
func bypassed(host string, list []string) bool {
	host = strings.ToLower(host)
	for _, b := range list {
		b = strings.ToLower(strings.TrimSpace(b))
		switch {
		case b == "":
		case b == "<local>":
			if !strings.Contains(host, ".") {
				return true
			}
		case strings.HasPrefix(b, "*."), strings.HasPrefix(b, "."):
			suffix := strings.TrimPrefix(b, "*")
			if strings.HasSuffix(host, suffix) || host == suffix[1:] {
				return true
			}
		case host == b:
			return true
		}
	}
	return false
}

// System reads the system proxy, at most every 15 seconds while one is set
// and every 2 while none is. An app started at login comes up before the
// proxy client does, and a long cache would leave its requests direct for
// minutes after the user turned the proxy on.
func system() System {
	sysMu.Lock()
	defer sysMu.Unlock()
	keep := 15 * time.Second
	if sysVal.URL == "" {
		keep = 2 * time.Second
	}
	if sysAt.IsZero() || time.Since(sysAt) > keep {
		sysVal, sysAt = readSys(), time.Now()
	}
	return sysVal
}

// Describe says what auto mode uses right now: the proxy address, where it
// came from ("environment" or "system"), or "" with "none" when it goes
// direct. A userinfo part is left out of the address shown.
func Describe() (address, source string) {
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "mikanime.tv"}}
	if p, err := envProxy(req); err == nil && p != nil {
		return redact(p.String()), "environment"
	}
	if sys := system(); sys.URL != "" {
		return redact(sys.URL), "system"
	}
	return "", "none"
}

func redact(raw string) string {
	u, err := parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}
