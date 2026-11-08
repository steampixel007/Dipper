//go:build !windows

package update

import (
	"errors"
	"os/exec"
	"time"
)

type parentProcess struct{}

func processIdentity(pid int) (string, error) {
	return "", errors.New("process identity is unavailable")
}

func openParent(pid int, creation string, limit time.Duration) (*parentProcess, error) {
	return nil, errors.New("update helper is only supported on Windows")
}

func waitParent(parent *parentProcess) error {
	return errors.New("update helper is only supported on Windows")
}

func closeParent(parent *parentProcess) {}

func verifyParentImage(parent *parentProcess, target string) error {
	return errors.New("update helper is only supported on Windows")
}

func hideHelper(command *exec.Cmd) {}

func reportUpdateFailure(err error) {}
