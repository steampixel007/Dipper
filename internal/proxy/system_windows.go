//go:build windows

package proxy

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// readSystem reads Internet Options' manual proxy, which is what proxy
// clients on Windows set.
func readSystem() System {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return System{}
	}
	defer key.Close()
	if on, _, err := key.GetIntegerValue("ProxyEnable"); err != nil || on == 0 {
		return System{}
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return System{}
	}
	override, _, _ := key.GetStringValue("ProxyOverride")
	return parseWindows(server, override)
}

// parseWindows reads ProxyServer ("host:port", or
// "http=h:p;https=h:p;socks=h:p") and ProxyOverride ("a;*.b;<local>").
func parseWindows(server, override string) System {
	server = strings.TrimSpace(server)
	if server == "" {
		return System{}
	}
	bypass := strings.Split(override, ";")
	if !strings.Contains(server, "=") {
		if !strings.Contains(server, "://") {
			server = "http://" + server
		}
		return System{URL: server, Bypass: bypass}
	}
	byProtocol := map[string]string{}
	for _, part := range strings.Split(server, ";") {
		if key, value, ok := strings.Cut(part, "="); ok {
			byProtocol[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
		}
	}
	entry := func(key, scheme string) string {
		value := byProtocol[key]
		if value != "" && !strings.Contains(value, "://") {
			value = scheme + "://" + value
		}
		return value
	}
	for _, value := range []string{entry("https", "http"), entry("http", "http"), entry("socks", "socks5")} {
		if value != "" {
			return System{URL: value, Bypass: bypass}
		}
	}
	return System{}
}
