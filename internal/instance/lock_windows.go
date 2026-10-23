package instance

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func tryLock(file *os.File) (bool, error) {
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return err == nil, err
}

var allowForeground = windows.NewLazySystemDLL("user32.dll").NewProc("AllowSetForegroundWindow")

func grantForeground(pid int) {
	if pid <= 0 || allowForeground.Find() != nil {
		return
	}
	// Explorer gives the newly launched process foreground rights. Transfer them
	// to the existing owner before asking it to show and focus its window.
	_, _, _ = allowForeground.Call(uintptr(uint32(pid)))
}
