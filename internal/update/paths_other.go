//go:build !windows

package update

import "path/filepath"

func resolveUpdatePath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
