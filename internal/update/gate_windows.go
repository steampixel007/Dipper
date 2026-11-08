//go:build windows

package update

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

type Gate struct {
	file *os.File
}

func AcquireLaunchGate(ctx context.Context, executable string) (*Gate, error) {
	path, err := gatePath(executable)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		var overlapped windows.Overlapped
		err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
		if err == nil {
			return &Gate{file: file}, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			file.Close()
			return nil, err
		}
		if ctx.Err() != nil {
			file.Close()
			return nil, errors.New("an update is in progress")
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (gate *Gate) Close() error {
	var overlapped windows.Overlapped
	_ = windows.UnlockFileEx(windows.Handle(gate.file.Fd()), 0, 1, 0, &overlapped)
	return gate.file.Close()
}
