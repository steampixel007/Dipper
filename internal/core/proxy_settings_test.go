package core

import (
	"path/filepath"
	"testing"
)

func TestProxySettingsPersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	c := openTest(t, path, Options{})
	settings, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.ProxyMode != "" && settings.ProxyMode != "auto" {
		t.Fatalf("default proxy mode = %q, want auto or empty", settings.ProxyMode)
	}
	settings.ProxyMode, settings.ProxyURL = "custom", "http://192.0.2.1:7897"
	if err := c.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	c = openTest(t, path, Options{})
	defer c.Close()
	got, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.ProxyMode != "custom" || got.ProxyURL != "http://192.0.2.1:7897" {
		t.Fatalf("proxy settings = %q, %q", got.ProxyMode, got.ProxyURL)
	}
	got.ProxyURL = "://invalid"
	if err := c.SetSettings(got); err == nil {
		t.Fatal("invalid proxy address was accepted")
	}
}
