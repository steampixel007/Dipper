//go:build !windows

package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

// folderAlias returns a symlink that denotes dir but is not string-equal to it.
func folderAlias(t *testing.T, dir string) (string, bool) {
	t.Helper()
	link := dir + "-link"
	if err := os.Symlink(dir, link); err != nil {
		return "", false
	}
	t.Cleanup(func() { os.Remove(link) })
	resolvedLink, errLink := filepath.EvalSymlinks(link)
	resolvedDir, errDir := filepath.EvalSymlinks(dir)
	if errLink != nil || errDir != nil || resolvedLink != resolvedDir {
		return "", false
	}
	return link, true
}
