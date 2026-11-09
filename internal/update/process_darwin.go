package update

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

func processIdentity(pid int) (string, error) {
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) || errors.Is(err, unix.EIO) {
			return "", os.ErrNotExist
		}
		return "", err
	}
	if info.Proc.P_pid != int32(pid) || info.Proc.P_stat == 5 {
		return "", os.ErrNotExist
	}
	return fmt.Sprintf("%d:%d", info.Proc.P_starttime.Sec, info.Proc.P_starttime.Usec), nil
}

func processImage(pid int) (string, error) {
	buffer := make([]byte, 4096)
	_, _, errno := unix.Syscall6(unix.SYS_PROC_INFO, 2, uintptr(pid), 11, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if errno != 0 {
		return "", errno
	}
	if buffer[0] == 0 {
		return "", errors.New("parent process image is unavailable")
	}
	return unix.ByteSliceToString(buffer), nil
}
