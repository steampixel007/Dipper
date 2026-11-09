//go:build darwin || linux

package update

import (
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestProcessIdentityTracksCurrentProcess(t *testing.T) {
	identity, err := processIdentity(os.Getpid())
	if err != nil || identity == "" {
		t.Fatalf("current process identity unavailable: %q %v", identity, err)
	}
	parent, err := openParent(os.Getpid(), identity, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyParentImage(parent, image); err != nil {
		t.Fatal(err)
	}
	if err := waitParent(parent); err == nil {
		t.Fatal("live process reported as exited")
	}
	closeParent(parent)
}

func TestProcessIdentityRejectsClosedChild(t *testing.T) {
	command := helperFixtureProcess(t)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	identity, err := processIdentity(command.Process.Pid)
	if err != nil {
		command.Process.Kill()
		command.Wait()
		t.Fatal(err)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	command.Wait()
	parent, err := openParent(command.Process.Pid, identity, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := waitParent(parent); err != nil {
		t.Fatal(err)
	}
	closeParent(parent)
	_, err = processIdentity(command.Process.Pid)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dead process identity is unknown instead of exited: %v", err)
	}
}

func helperFixtureProcess(t *testing.T) *exec.Cmd {
	t.Helper()
	return exec.Command("sleep", "30")
}
