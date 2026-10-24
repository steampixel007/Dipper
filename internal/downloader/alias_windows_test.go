//go:build windows

package downloader

import (
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// folderAlias returns a Windows short (8.3) path that denotes dir but is not
// string-equal to it. EvalSymlinks resolves such components, so an alias that
// survives entry resolution still differs from the stored canonical folder.
func folderAlias(t *testing.T, dir string) (string, bool) {
	t.Helper()
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return "", false
	}
	n, err := windows.GetShortPathName(p, nil, 0)
	if err != nil || n == 0 {
		return "", false
	}
	buf := make([]uint16, n)
	n, err = windows.GetShortPathName(p, &buf[0], n)
	if err != nil || n == 0 {
		return "", false
	}
	short := windows.UTF16ToString(buf[:n])
	if strings.EqualFold(short, dir) {
		return "", false // the volume has 8.3 names disabled
	}
	return short, true
}
