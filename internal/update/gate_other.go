//go:build !windows

package update

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
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
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return &Gate{file: file}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
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
	_ = syscall.Flock(int(gate.file.Fd()), syscall.LOCK_UN)
	return gate.file.Close()
}
