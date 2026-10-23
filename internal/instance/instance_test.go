package instance

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func acquireTest(t *testing.T, dir string) *Instance {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	i, err := Acquire(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if i != nil {
		t.Cleanup(func() { i.Close() })
	}
	return i
}

func TestForwardActivationWithoutSecondOwner(t *testing.T) {
	dir := t.TempDir()
	owner := acquireTest(t, dir)
	if owner == nil {
		t.Fatal("first launch did not acquire ownership")
	}
	if owner.endpoint.PID != os.Getpid() {
		t.Fatal("activation endpoint must identify the process that needs foreground rights")
	}
	if second := acquireTest(t, dir); second != nil {
		t.Fatal("second launch must activate the owner instead of starting")
	}
	select {
	case <-owner.Activations():
	case <-time.After(time.Second):
		t.Fatal("owner did not receive activation")
	}
}

func TestConcurrentLaunchesHaveOneOwner(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	owners := make(chan *Instance, 8)
	errors := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			i, err := Acquire(ctx, dir)
			if err != nil {
				errors <- err
			} else if i != nil {
				owners <- i
			}
		}()
	}
	wg.Wait()
	close(owners)
	close(errors)
	n := 0
	for owner := range owners {
		n++
		owner.Close()
	}
	for err := range errors {
		t.Error(err)
	}
	if n != 1 {
		t.Fatalf("started %d owners", n)
	}
}

func TestLockedUnresponsiveOwnerCannotStartAnother(t *testing.T) {
	dir := t.TempDir()
	file, err := os.OpenFile(filepath.Join(dir, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	locked, err := tryLock(file)
	if err != nil || !locked {
		t.Fatalf("lock: %v %v", locked, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	i, err := Acquire(ctx, dir)
	if i != nil || err == nil {
		t.Fatalf("must fail without opening a second owner: %v %v", i, err)
	}
}

func TestRejectsUnauthenticatedActivation(t *testing.T) {
	owner := acquireTest(t, t.TempDir())
	conn, err := net.Dial("tcp4", owner.endpoint.Address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	_ = json.NewEncoder(conn).Encode("wrong token")
	_, _ = io.ReadAll(conn)
	select {
	case <-owner.Activations():
		t.Fatal("invalid token activated owner")
	default:
	}
}

func TestInstanceHelperProcess(t *testing.T) {
	if os.Getenv("DIPPER_TEST_INSTANCE") != "1" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	i, err := Acquire(ctx, os.Getenv("DIPPER_TEST_INSTANCE_DIR"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if i == nil {
		fmt.Println("forwarded")
		os.Exit(0)
	}
	defer i.Close()
	fmt.Println("owner")
	for range i.Activations() {
		fmt.Println("activated")
	}
}

func TestProcessActivationAndCrashRecovery(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestInstanceHelperProcess$")
	cmd.Env = append(os.Environ(), "DIPPER_TEST_INSTANCE=1", "DIPPER_TEST_INSTANCE_DIR="+dir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "owner" {
		t.Fatal("child did not own the data directory")
	}
	if i := acquireTest(t, dir); i != nil {
		t.Fatal("existing process was not activated")
	}
	if !scanner.Scan() || scanner.Text() != "activated" {
		t.Fatal("activation did not reach the original process")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if i := acquireTest(t, dir); i == nil {
		t.Fatal("process death did not release the owner lock")
	}
}
