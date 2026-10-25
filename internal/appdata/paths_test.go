package appdata

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// injectDirs replaces the standard-library lookups with fakes so every branch
// can run on any host; the real lookups are exercised on Linux by
// TestResolveUsesStandardLibraryOnLinux.
func injectDirs(t *testing.T, config, cache string, configErr, cacheErr error) {
	t.Helper()
	previousConfig, previousCache := userConfigDir, userCacheDir
	userConfigDir = func() (string, error) { return config, configErr }
	userCacheDir = func() (string, error) { return cache, cacheErr }
	t.Cleanup(func() { userConfigDir, userCacheDir = previousConfig, previousCache })
}

func noEnv(string) string { return "" }

func TestResolvePlatformDirectories(t *testing.T) {
	home := "/home/tester"
	injectDirs(t, home+"/.config", home+"/.cache", nil, nil)
	linux, err := resolve("linux", home, "", noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if linux.DataDir != "/home/tester/.local/share/dipper" || linux.ConfigDir != "/home/tester/.config/dipper" || linux.CoverDir != "/home/tester/.cache/dipper/covers" || linux.LogDir != "/home/tester/.local/state/dipper" {
		t.Fatalf("Linux paths: %+v", linux)
	}

	macHome := "/Users/tester"
	injectDirs(t, macHome+"/Library/Application Support", macHome+"/Library/Caches", nil, nil)
	mac, err := resolve("darwin", macHome, "", noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if mac.DataDir != filepath.FromSlash("/Users/tester/Library/Application Support/Dipper") || mac.ConfigDir != mac.DataDir || mac.CoverDir != filepath.FromSlash("/Users/tester/Library/Caches/Dipper/covers") || mac.LogDir != filepath.FromSlash("/Users/tester/Library/Logs/Dipper") {
		t.Fatalf("macOS paths: %+v", mac)
	}

	local := filepath.Join(t.TempDir(), "Local")
	win, err := resolve("windows", "", local, noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if win.DataDir != filepath.Join(local, "Dipper") || win.ConfigDir != win.DataDir || win.CoverDir != filepath.Join(local, "Dipper", "Cache", "covers") || win.LogDir != filepath.Join(local, "Dipper", "Logs") {
		t.Fatalf("Windows paths: %+v", win)
	}
}

func TestResolveLinuxXDGOverrides(t *testing.T) {
	injectDirs(t, "/custom/config", "/custom/cache", nil, nil)
	values := map[string]string{"XDG_DATA_HOME": "/custom/data", "XDG_STATE_HOME": "/custom/state"}
	paths, err := resolve("linux", "/home/tester", "", func(name string) string { return values[name] })
	if err != nil {
		t.Fatal(err)
	}
	if paths.DataDir != "/custom/data/dipper" || paths.ConfigDir != "/custom/config/dipper" || paths.CoverDir != "/custom/cache/dipper/covers" || paths.LogDir != "/custom/state/dipper" {
		t.Fatalf("XDG overrides: %+v", paths)
	}
}

func TestResolveLinuxInvalidXDGValueFallsBack(t *testing.T) {
	injectDirs(t, "", "", errors.New("XDG path is relative"), errors.New("XDG path is relative"))
	paths, err := resolve("linux", "/home/tester", "", noEnv)
	if err != nil {
		t.Fatal(err)
	}
	if paths.ConfigDir != "/home/tester/.config/dipper" || paths.CoverDir != "/home/tester/.cache/dipper/covers" {
		t.Fatalf("invalid XDG fallback: %+v", paths)
	}
}

func TestResolveUsesStandardLibraryOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the standard library resolves XDG paths only where its lookup applies")
	}
	t.Setenv("HOME", "/home/tester")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	paths, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if paths.DataDir != "/home/tester/.local/share/dipper" || paths.ConfigDir != "/home/tester/.config/dipper" || paths.CoverDir != "/home/tester/.cache/dipper/covers" || paths.LogDir != "/home/tester/.local/state/dipper" {
		t.Fatalf("standard-library resolution: %+v", paths)
	}
}

func TestExplicitDataDirKeepsSingleRoot(t *testing.T) {
	dir := t.TempDir()
	paths, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if paths.DataDir != dir || paths.ConfigDir != dir || paths.Preferences() != filepath.Join(dir, "preferences.json") || paths.CoverDir != filepath.Join(dir, "covers") || paths.Log() != filepath.Join(dir, "app.log") {
		t.Fatalf("explicit paths: %+v", paths)
	}
	if _, err := os.Stat(filepath.Join(dir, "preferences.json")); !os.IsNotExist(err) {
		t.Fatalf("resolving paths wrote a file: %v", err)
	}
	if _, err := resolve("windows", "", "", os.Getenv); err == nil {
		t.Fatal("missing Windows local data directory was accepted")
	}
}
