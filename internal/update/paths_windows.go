//go:build windows

package update

import (
	"strings"

	"golang.org/x/sys/windows"
)

func resolveUpdatePath(path string) (string, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 512)
	for {
		size, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if err != nil {
			return "", err
		}
		if size >= uint32(len(buffer)) {
			buffer = make([]uint16, size+1)
			continue
		}
		resolved := windows.UTF16ToString(buffer[:size])
		if strings.HasPrefix(resolved, `\\?\UNC\`) {
			return `\\` + strings.TrimPrefix(resolved, `\\?\UNC\`), nil
		}
		return strings.TrimPrefix(resolved, `\\?\`), nil
	}
}
