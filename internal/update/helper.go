package update

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const helperFlag = "--software-update-helper"
const completedStageMarker = "complete"

type helperState struct {
	ParentPID                    int
	ParentCreated                string
	Target, TargetHash           string
	Staged, StagedHash, Stage    string
	Args                         []string
	WorkingDirectory             string
	ControlAddress, ControlToken string
	ReadyToken                   string
	Created                      time.Time
}

type Installer struct {
	conn      net.Conn
	command   *exec.Cmd
	directory string
	mu        sync.Mutex
	committed bool
	closed    bool
}

func randomToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func fileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyExclusive(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	return errors.Join(copyErr, syncErr, closeErr)
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func safeRegular(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("expected a clean absolute path")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if !samePath(resolved, path) {
		return errors.New("symbolic links and redirected paths are not supported")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return errors.New("expected a nonempty regular file")
	}
	return nil
}

func hashMatches(path, expected string) error {
	if err := safeRegular(path); err != nil {
		return err
	}
	actual, err := fileHash(path)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return errors.New("executable changed after verification")
	}
	return nil
}

func PreparePortable(ctx context.Context, target string, downloaded Download, args []string) (*Installer, error) {
	if runtime.GOOS != "windows" {
		return nil, errors.New("portable replacement is Windows-only")
	}
	if !strings.EqualFold(filepath.Base(target), "Dipper.exe") {
		return nil, errors.New("the portable executable must be named Dipper.exe")
	}
	if err := safeRegular(target); err != nil {
		return nil, err
	}
	if err := hashMatches(downloaded.Executable, downloaded.SHA256); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".dipper-update-")
	if err != nil {
		return nil, fmt.Errorf("cannot write beside Dipper.exe; move the portable app to a writable folder: %w", err)
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(stage)
		}
	}()
	targetHash, err := fileHash(target)
	if err != nil {
		return nil, err
	}
	created, err := processIdentity(os.Getpid())
	if err != nil {
		return nil, err
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	control, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer control.Close()
	controlToken, err := randomToken()
	if err != nil {
		return nil, err
	}
	readyToken, err := randomToken()
	if err != nil {
		return nil, err
	}
	state := helperState{
		ParentPID: os.Getpid(), ParentCreated: created, Target: target, TargetHash: targetHash,
		Stage: stage, Staged: filepath.Join(stage, "new.exe"), StagedHash: downloaded.SHA256,
		Args: PrivateArguments(args), WorkingDirectory: workingDirectory, ControlAddress: control.Addr().String(),
		ControlToken: controlToken, ReadyToken: readyToken, Created: time.Now(),
	}
	helper := filepath.Join(stage, "helper.exe")
	if err := copyExclusive(target, helper); err != nil {
		return nil, err
	}
	if err := copyExclusive(downloaded.Executable, state.Staged); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	statePath := filepath.Join(stage, "state.json")
	if err := os.WriteFile(statePath, encoded, 0600); err != nil {
		return nil, err
	}
	command := exec.Command(helper, helperFlag, statePath)
	hideHelper(command)
	if err := command.Start(); err != nil {
		return nil, err
	}
	installer := &Installer{command: command, directory: stage}
	defer func() {
		if !success {
			installer.Abort()
		}
	}()
	stop := context.AfterFunc(ctx, func() { control.Close() })
	defer stop()
	control.(*net.TCPListener).SetDeadline(time.Now().Add(15 * time.Second))
	conn, err := acceptToken(control, "ready:"+controlToken, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("updater could not acquire the installation lock or start: %w", err)
	}
	installer.conn = conn
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	success = true
	return installer, nil
}

func (installer *Installer) Commit() error {
	installer.mu.Lock()
	defer installer.mu.Unlock()
	if installer.closed {
		return errors.New("updater is closed")
	}
	if installer.committed {
		return nil
	}
	if err := exchange(installer.conn, "commit", "accepted", 3*time.Second); err != nil {
		return err
	}
	installer.committed = true
	installer.conn.Close()
	go installer.command.Wait()
	return nil
}

func (installer *Installer) Abort() {
	installer.mu.Lock()
	defer installer.mu.Unlock()
	if installer.closed || installer.committed {
		return
	}
	installer.closed = true
	if installer.conn != nil {
		installer.conn.Close()
	}
	if installer.command != nil && installer.command.Process != nil {
		installer.command.Process.Kill()
		installer.command.Wait()
	}
	os.RemoveAll(installer.directory)
}

func validToken(token string) bool {
	value, err := hex.DecodeString(token)
	return err == nil && len(value) == 32
}

func localAddress(address string) bool {
	host, port, err := net.SplitHostPort(address)
	number, parseErr := strconv.Atoi(port)
	return err == nil && parseErr == nil && host == "127.0.0.1" && number > 0 && number <= 65535
}

func exchange(conn net.Conn, request, expected string, timeout time.Duration) error {
	conn.SetDeadline(time.Now().Add(timeout))
	if _, err := fmt.Fprintln(conn, request); err != nil {
		return err
	}
	actual, err := readLine(conn)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return errors.New("updater handshake rejected")
	}
	return nil
}

func readLine(conn net.Conn) (string, error) {
	line, err := bufio.NewReader(io.LimitReader(conn, 1024)).ReadString('\n')
	return strings.TrimSuffix(line, "\n"), err
}

func acceptToken(listener net.Listener, expected string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := listener.Accept()
		if err != nil {
			return nil, err
		}
		conn.SetDeadline(time.Now().Add(time.Second))
		value, err := readLine(conn)
		if err == nil && subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1 {
			return conn, nil
		}
		conn.Close()
	}
	return nil, errors.New("handshake timed out")
}

func RunHelper(statePath string) int {
	if err := runHelperFile(statePath); err != nil {
		reportUpdateFailure(err)
		return 1
	}
	return 0
}

func validateState(state helperState, statePath, executable string) error {
	if !filepath.IsAbs(state.WorkingDirectory) {
		return errors.New("working directory must be absolute")
	}
	info, err := os.Stat(state.WorkingDirectory)
	if err != nil || !info.IsDir() {
		return errors.New("working directory is unavailable")
	}
	if !samePath(state.Stage, filepath.Dir(statePath)) || filepath.Base(statePath) != "state.json" ||
		!samePath(executable, filepath.Join(state.Stage, "helper.exe")) ||
		!strings.HasPrefix(filepath.Base(state.Stage), ".dipper-update-") ||
		!samePath(filepath.Dir(state.Stage), filepath.Dir(state.Target)) ||
		!samePath(state.Staged, filepath.Join(state.Stage, "new.exe")) ||
		!strings.EqualFold(filepath.Base(state.Target), "Dipper.exe") ||
		!validToken(state.TargetHash) || !validToken(state.StagedHash) ||
		!validToken(state.ReadyToken) || !validToken(state.ControlToken) ||
		!localAddress(state.ControlAddress) || state.ParentPID <= 0 || state.ParentCreated == "" ||
		time.Since(state.Created) > 2*time.Minute || time.Until(state.Created) > time.Minute {
		return errors.New("invalid updater inputs")
	}
	if err := hashMatches(executable, state.TargetHash); err != nil {
		return err
	}
	if err := hashMatches(state.Target, state.TargetHash); err != nil {
		return err
	}
	return hashMatches(state.Staged, state.StagedHash)
}

func runHelperFile(statePath string) error {
	if err := safeRegular(statePath); err != nil {
		return err
	}
	file, err := os.Open(statePath)
	if err != nil {
		return err
	}
	var state helperState
	decoder := json.NewDecoder(io.LimitReader(file, 65536))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&state)
	if err == nil && decoder.Decode(new(any)) != io.EOF {
		err = errors.New("unexpected trailing update state")
	}
	file.Close()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if err := validateState(state, statePath, executable); err != nil {
		return err
	}
	parent, err := openParent(state.ParentPID, state.ParentCreated, 90*time.Second)
	if err != nil {
		return err
	}
	defer closeParent(parent)
	if err := verifyParentImage(parent, state.Target); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	gate, err := AcquireLaunchGate(ctx, state.Target)
	if err != nil {
		return err
	}
	defer gate.Close()
	control, err := net.DialTimeout("tcp4", state.ControlAddress, 5*time.Second)
	if err != nil {
		return err
	}
	defer control.Close()
	control.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := fmt.Fprintln(control, "ready:"+state.ControlToken); err != nil {
		return err
	}
	instruction, err := readLine(control)
	if err != nil || instruction != "commit" {
		return nil
	}
	if _, err := fmt.Fprintln(control, "accepted"); err != nil {
		return err
	}
	if err := waitParent(parent); err != nil {
		return fmt.Errorf("Dipper is still running; the executable was not changed: %w", err)
	}
	return replaceAndRestart(state, 40*time.Second, startReplacement)
}

type restartedProcess struct {
	exited <-chan error
}

type restartFunc func(string, []string, string) (restartedProcess, error)

func startReplacement(target string, args []string, workingDirectory string) (restartedProcess, error) {
	command := exec.Command(target, args...)
	command.Dir = workingDirectory
	if err := command.Start(); err != nil {
		return restartedProcess{}, err
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	return restartedProcess{exited: exited}, nil
}

func replaceAndRestart(state helperState, timeout time.Duration, start restartFunc) error {
	if err := hashMatches(state.Target, state.TargetHash); err != nil {
		return err
	}
	if err := hashMatches(state.Staged, state.StagedHash); err != nil {
		return err
	}
	backup := filepath.Join(state.Stage, "old.exe")
	if _, err := os.Lstat(backup); !os.IsNotExist(err) {
		return errors.New("backup path already exists; no executable was changed")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	listener.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))
	if err := os.Rename(state.Target, backup); err != nil {
		return fmt.Errorf("cannot move the old executable; close other Dipper copies and check folder permissions: %w", err)
	}
	restore := func(cause error) error {
		if _, err := os.Lstat(state.Target); err == nil {
			if err := hashMatches(state.Target, state.StagedHash); err != nil {
				return errors.Join(cause, fmt.Errorf("target changed; old executable retained at %s", backup))
			}
			if err := os.Remove(state.Target); err != nil {
				return errors.Join(cause, err)
			}
		} else if !os.IsNotExist(err) {
			return errors.Join(cause, err)
		}
		if err := os.Rename(backup, state.Target); err != nil {
			return errors.Join(cause, fmt.Errorf("restore failed; old executable retained at %s: %w", backup, err))
		}
		return fmt.Errorf("%w; old executable restored, user data was not reverted; review the release notes before reopening", cause)
	}
	if err := os.Rename(state.Staged, state.Target); err != nil {
		return restore(err)
	}
	args := append(PrivateArguments(state.Args), "--software-update-ready-address", listener.Addr().String(), "--software-update-ready-token", state.ReadyToken)
	process, err := start(state.Target, args, state.WorkingDirectory)
	if err != nil {
		return restore(fmt.Errorf("cannot start the new executable: %w", err))
	}
	ready := make(chan error, 1)
	go func() {
		authorize, err := acceptToken(listener, "authorize:"+state.ReadyToken, timeout)
		if err == nil {
			_, err = fmt.Fprintln(authorize, "authorized")
			authorize.Close()
		}
		if err == nil {
			var conn net.Conn
			conn, err = acceptToken(listener, "ready:"+state.ReadyToken, timeout)
			if err == nil {
				_, err = fmt.Fprintln(conn, "accepted")
				conn.Close()
			}
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err == nil {
			os.Remove(backup)
			os.Remove(filepath.Join(state.Stage, "state.json"))
			markStageComplete(state.Stage)
			return nil
		}
		select {
		case <-process.exited:
			return restore(fmt.Errorf("new executable did not become ready: %w", err))
		default:
			return fmt.Errorf("new executable is still running without readiness; no rollback attempted; old executable retained at %s; user data was not reverted", backup)
		}
	case err := <-process.exited:
		listener.Close()
		if readyErr := <-ready; readyErr == nil {
			os.Remove(backup)
			os.Remove(filepath.Join(state.Stage, "state.json"))
			markStageComplete(state.Stage)
			return nil
		}
		return restore(fmt.Errorf("new executable exited before readiness: %v", err))
	}
}

func markStageComplete(stage string) {
	_ = os.WriteFile(filepath.Join(stage, completedStageMarker), []byte("complete\n"), 0600)
}

// CleanupCompleted removes stages whose replacement was acknowledged by the
// restarted application. The marker is written only after that handshake, so
// active or failed update stages are left for inspection and recovery.
func CleanupCompleted(executable string) {
	root := filepath.Clean(filepath.Dir(executable))
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !strings.HasPrefix(name, ".dipper-update-") {
			continue
		}
		stage := filepath.Join(root, name)
		if filepath.Dir(stage) != root {
			continue
		}
		marker, err := os.Lstat(filepath.Join(stage, completedStageMarker))
		if err != nil || !marker.Mode().IsRegular() {
			continue
		}
		files, err := os.ReadDir(stage)
		if err != nil || len(files) != 2 {
			continue
		}
		safe := true
		for _, file := range files {
			if (file.Name() != "helper.exe" && file.Name() != completedStageMarker) ||
				safeRegular(filepath.Join(stage, file.Name())) != nil {
				safe = false
				break
			}
		}
		body, err := os.ReadFile(filepath.Join(stage, completedStageMarker))
		if !safe || err != nil || string(body) != "complete\n" {
			continue
		}
		if err := os.Remove(filepath.Join(stage, "helper.exe")); err != nil {
			continue
		}
		if err := os.Remove(filepath.Join(stage, completedStageMarker)); err != nil {
			continue
		}
		_ = os.Remove(stage)
	}
}

func AuthorizeRestart(address, token string) error {
	return restartHandshake(address, token, "authorize:", "authorized")
}

func NotifyReady(address, token string) error {
	return restartHandshake(address, token, "ready:", "accepted")
}

func restartHandshake(address, token, prefix, expected string) error {
	if address == "" && token == "" {
		return nil
	}
	if !localAddress(address) || !validToken(token) {
		return errors.New("invalid updater restart authentication")
	}
	conn, err := net.DialTimeout("tcp4", address, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return exchange(conn, prefix+token, expected, 5*time.Second)
}

func PrivateArguments(args []string) []string {
	var result []string
	for index := 0; index < len(args); index++ {
		name, _, hasValue := strings.Cut(args[index], "=")
		if strings.HasPrefix(name, "--software-update-") || strings.HasPrefix(name, "-software-update-") {
			if !hasValue && index+1 < len(args) {
				index++
			}
			continue
		}
		result = append(result, args[index])
	}
	return result
}

func gatePath(target string) (string, error) {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	}
	if runtime.GOOS == "windows" {
		absolute = strings.ToLower(absolute)
	}
	hash := sha256.Sum256([]byte(absolute))
	directory := filepath.Join(os.TempDir(), "Dipper-update-locks")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil || !samePath(directory, resolved) {
		return "", errors.New("unsafe update lock directory")
	}
	path := filepath.Join(directory, hex.EncodeToString(hash[:])+".lock")
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return "", errors.New("unsafe update lock file")
	}
	return path, nil
}
