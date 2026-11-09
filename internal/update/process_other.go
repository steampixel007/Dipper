//go:build !windows

package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type parentProcess struct {
	pid      int
	identity string
	limit    time.Duration
}

func openParent(pid int, creation string, limit time.Duration) (*parentProcess, error) {
	if pid <= 0 || creation == "" {
		return nil, errors.New("invalid parent process identity")
	}
	actual, err := processIdentity(pid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &parentProcess{pid: pid, identity: creation, limit: limit}, nil
		}
		return nil, err
	}
	if actual != creation {
		return nil, errors.New("parent process identity changed")
	}
	return &parentProcess{pid: pid, identity: creation, limit: limit}, nil
}

func waitParent(parent *parentProcess) error {
	timer := time.NewTimer(parent.limit)
	defer timer.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		actual, err := processIdentity(parent.pid)
		if errors.Is(err, os.ErrNotExist) || (err == nil && actual != parent.identity) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("cannot verify original process exit: %w", err)
		}
		select {
		case <-timer.C:
			return errors.New("original process did not exit within timeout")
		case <-ticker.C:
		}
	}
}

func closeParent(parent *parentProcess) {}

func verifyParentImage(parent *parentProcess, target string) error {
	image, err := processImage(parent.pid)
	if err != nil {
		return err
	}
	actual, err := processIdentity(parent.pid)
	if err != nil {
		return err
	}
	if actual != parent.identity || !samePath(image, target) {
		return errors.New("parent executable does not match the update target")
	}
	return nil
}

func hideHelper(command *exec.Cmd) {}

func reportUpdateFailure(err error) {
	fmt.Fprintln(os.Stderr, helperFailureText+err.Error())
}
