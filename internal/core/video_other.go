//go:build !windows

package core

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func userVideoDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Movies"), nil
	}
	if runtime.GOOS == "linux" {
		if config, err := os.UserConfigDir(); err == nil {
			if file, err := os.Open(filepath.Join(config, "user-dirs.dirs")); err == nil {
				defer file.Close()
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
					if !ok || strings.TrimSpace(key) != "XDG_VIDEOS_DIR" {
						continue
					}
					dir, err := strconv.Unquote(strings.TrimSpace(value))
					if err != nil {
						continue
					}
					dir = strings.ReplaceAll(strings.ReplaceAll(dir, "${HOME}", home), "$HOME", home)
					if filepath.IsAbs(dir) {
						return filepath.Clean(dir), nil
					}
				}
			}
		}
	}
	return filepath.Join(home, "Videos"), nil
}
