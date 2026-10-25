package appdata

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type Paths struct {
	DataDir   string
	ConfigDir string
	CoverDir  string
	LogDir    string
}

func (p Paths) Preferences() string { return filepath.Join(p.ConfigDir, "preferences.json") }
func (p Paths) Log() string         { return filepath.Join(p.LogDir, "app.log") }

func Resolve(explicit string) (Paths, error) {
	if explicit != "" {
		dir, err := filepath.Abs(explicit)
		if err != nil {
			return Paths{}, fmt.Errorf("解析数据目录：%w", err)
		}
		return Paths{DataDir: dir, ConfigDir: dir, CoverDir: filepath.Join(dir, "covers"), LogDir: dir}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("读取用户目录：%w", err)
	}
	local := ""
	if runtime.GOOS == "windows" {
		if local, err = platformLocalData(); err != nil {
			return Paths{}, fmt.Errorf("读取本地应用数据目录：%w", err)
		}
	}
	return resolve(runtime.GOOS, home, local, os.Getenv)
}

// userConfigDir and userCacheDir are the standard-library directory lookups;
// they are variables only so tests can inject values on any host.
var (
	userConfigDir = os.UserConfigDir
	userCacheDir  = os.UserCacheDir
)

func resolve(goos, home, local string, getenv func(string) string) (Paths, error) {
	switch goos {
	case "windows":
		if local == "" || !filepath.IsAbs(local) {
			return Paths{}, fmt.Errorf("本地应用数据目录无效：%q", local)
		}
		data := filepath.Join(local, "Dipper")
		return Paths{DataDir: data, ConfigDir: data, CoverDir: filepath.Join(data, "Cache", "covers"), LogDir: filepath.Join(data, "Logs")}, nil
	case "darwin":
		if home == "" {
			return Paths{}, fmt.Errorf("用户目录为空")
		}
		support, err := userConfigDir()
		if err != nil {
			return Paths{}, fmt.Errorf("读取应用支持目录：%w", err)
		}
		cache, err := userCacheDir()
		if err != nil {
			return Paths{}, fmt.Errorf("读取缓存目录：%w", err)
		}
		data := filepath.Join(support, "Dipper")
		return Paths{DataDir: data, ConfigDir: data, CoverDir: filepath.Join(cache, "Dipper", "covers"), LogDir: filepath.Join(home, "Library", "Logs", "Dipper")}, nil
	case "linux":
		if home == "" || !strings.HasPrefix(home, "/") {
			return Paths{}, fmt.Errorf("用户目录无效：%q", home)
		}
		xdg := func(key, fallback string) string {
			if value := getenv(key); strings.HasPrefix(value, "/") {
				return path.Clean(value)
			}
			return path.Join(home, fallback)
		}
		// No stdlib API covers XDG_DATA_HOME or XDG_STATE_HOME.
		config, err := userConfigDir()
		if err != nil {
			// A relative XDG value is invalid; the spec says to disregard it.
			config = path.Join(home, ".config")
		}
		cache, err := userCacheDir()
		if err != nil {
			cache = path.Join(home, ".cache")
		}
		return Paths{
			DataDir:   path.Join(xdg("XDG_DATA_HOME", ".local/share"), "dipper"),
			ConfigDir: path.Join(config, "dipper"),
			CoverDir:  path.Join(cache, "dipper", "covers"),
			LogDir:    path.Join(xdg("XDG_STATE_HOME", ".local/state"), "dipper"),
		}, nil
	default:
		return Paths{}, fmt.Errorf("不支持的系统：%s", goos)
	}
}
