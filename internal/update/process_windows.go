//go:build windows

package update

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type parentProcess struct {
	handle windows.Handle
	limit  time.Duration
}

func processIdentity(pid int) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return "", err
	}
	return strconv.FormatInt(creation.Nanoseconds(), 10), nil
}

func openParent(pid int, creation string, limit time.Duration) (*parentProcess, error) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return nil, err
	}
	var actual, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &actual, &exit, &kernel, &user); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	if strconv.FormatInt(actual.Nanoseconds(), 10) != creation {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("parent process identity changed")
	}
	return &parentProcess{handle: handle, limit: limit}, nil
}

func waitParent(parent *parentProcess) error {
	status, err := windows.WaitForSingleObject(parent.handle, uint32(parent.limit/time.Millisecond))
	if err != nil {
		return err
	}
	if status != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("original process did not exit within timeout")
	}
	return nil
}

func verifyParentImage(parent *parentProcess, target string) error {
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(parent.handle, 0, &buffer[0], &size); err != nil {
		return err
	}
	if !samePath(windows.UTF16ToString(buffer[:size]), target) {
		return fmt.Errorf("parent executable does not match the update target")
	}
	return nil
}

func closeParent(parent *parentProcess) {
	windows.CloseHandle(parent.handle)
}

func hideHelper(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

func reportUpdateFailure(err error) {
	title, _ := windows.UTF16PtrFromString(helperFailureTitle)
	message, _ := windows.UTF16PtrFromString(helperFailureText + err.Error())
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
