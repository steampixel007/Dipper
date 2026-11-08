//go:build windows

package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/steampixel007/Dipper/internal/instance"
	"golang.org/x/sys/windows"
)

type fixturePrepared struct {
	PID   int
	Stage string
}

type fixtureRestarted struct {
	PID     int
	Created string
	Cwd     string
}

func TestMain(tests *testing.M) {
	if os.Getenv("DIPPER_UPDATE_FIXTURE") == "1" {
		var err error
		switch {
		case len(os.Args) == 3 && os.Args[1] == helperFlag:
			err = runHelperFile(os.Args[2])
		case len(os.Args) > 2 && os.Args[1] == "--fixture-parent":
			err = fixtureParent(os.Args[2])
		case len(os.Args) > 2 && os.Args[1] == "--fixture-restart":
			err = fixtureRestart(os.Args[2])
		default:
			err = errors.New("unrecognized fixture invocation")
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(tests.Run())
}

func fixtureParent(address string) error {
	target, err := os.Executable()
	if err != nil {
		return err
	}
	owner, err := instance.Acquire(context.Background(), "relative-data")
	if err != nil || owner == nil {
		return fmt.Errorf("fixture parent ownership: %v", err)
	}
	defer owner.Close()
	hash, err := Digest("incoming.exe")
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	installer, err := PreparePortable(ctx, target, Download{Executable: filepath.Join(cwd, "incoming.exe"), SHA256: hash},
		[]string{"--fixture-restart", os.Getenv("DIPPER_UPDATE_SCENARIO"), "--data", "relative-data", "--download-root=relative-downloads"})
	if err != nil {
		return err
	}
	defer installer.Abort()
	if os.Getenv("DIPPER_UPDATE_SCENARIO") == "abort" {
		installer.Abort()
		return nil
	}
	conn, err := net.DialTimeout("tcp4", address, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	if err := installer.Commit(); err != nil {
		return err
	}
	if err := json.NewEncoder(conn).Encode(fixturePrepared{PID: installer.command.Process.Pid, Stage: installer.directory}); err != nil {
		return err
	}
	if value, err := readLine(conn); err != nil || value != "exit" {
		return fmt.Errorf("fixture parent release: %s %v", value, err)
	}
	return nil
}

func fixtureRestart(scenario string) error {
	if scenario == "exit" {
		return errors.New("deliberate harmless fixture startup failure")
	}
	var address, token string
	for index, arg := range os.Args {
		if arg == "--software-update-ready-address" {
			address = os.Args[index+1]
		}
		if arg == "--software-update-ready-token" {
			token = os.Args[index+1]
		}
	}
	if err := AuthorizeRestart(address, token); err != nil {
		return err
	}
	owner, err := instance.Acquire(context.Background(), "relative-data")
	if err != nil || owner == nil {
		return fmt.Errorf("fixture restart did not acquire original instance: %v", err)
	}
	defer owner.Close()
	cwd, _ := os.Getwd()
	body, err := os.ReadFile(filepath.Join("relative-data", "preferences.json"))
	if err != nil || string(body) != "preserved preferences" {
		return errors.New("fixture restart lost user data")
	}
	if !strings.Contains(strings.Join(os.Args, "|"), "--download-root=relative-downloads") {
		return errors.New("fixture restart lost download context")
	}
	created, err := processIdentity(os.Getpid())
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(fixtureRestarted{PID: os.Getpid(), Created: created, Cwd: cwd})
	if err != nil {
		return err
	}
	if err := os.WriteFile("restarted.json", encoded, 0600); err != nil {
		return err
	}
	return NotifyReady(address, token)
}

func TestPortableHelperRealProcess(t *testing.T) {
	for _, scenario := range []string{"success", "exit", "abort"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			source, _ := os.Executable()
			target, incoming := filepath.Join(dir, "Dipper.exe"), filepath.Join(dir, "incoming.exe")
			if err := copyExclusive(source, target); err != nil {
				t.Fatal(err)
			}
			if err := copyExclusive(source, incoming); err != nil {
				t.Fatal(err)
			}
			file, _ := os.OpenFile(incoming, os.O_APPEND|os.O_WRONLY, 0600)
			file.WriteString("harmless different fixture image")
			file.Close()
			oldHash, _ := Digest(target)
			newHash, _ := Digest(incoming)
			os.Mkdir(filepath.Join(dir, "relative-data"), 0700)
			os.WriteFile(filepath.Join(dir, "relative-data", "preferences.json"), []byte("preserved preferences"), 0600)
			os.WriteFile(filepath.Join(dir, "relative-data", "dipper.db"), []byte("preserved follows"), 0600)
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			listener.(*net.TCPListener).SetDeadline(time.Now().Add(25 * time.Second))
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, target, "--fixture-parent", listener.Addr().String())
			command.Dir = dir
			command.Env = append(os.Environ(), "DIPPER_UPDATE_FIXTURE=1", "DIPPER_UPDATE_SCENARIO="+scenario)
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { command.Process.Kill() })
			if scenario == "abort" {
				if err := command.Wait(); err != nil {
					t.Fatal("fixture abort:", err)
				}
				stages, _ := filepath.Glob(filepath.Join(dir, ".dipper-update-*"))
				if len(stages) != 0 {
					t.Fatal("abort left a helper stage")
				}
			} else {
				conn, err := listener.Accept()
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(15 * time.Second))
				var prepared fixturePrepared
				if err := json.NewDecoder(conn).Decode(&prepared); err != nil {
					t.Fatal(err)
				}
				identity, err := processIdentity(prepared.PID)
				if err != nil {
					t.Fatal(err)
				}
				helper, err := openParent(prepared.PID, identity, 20*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer closeParent(helper)
				if err := hashMatches(target, oldHash); err != nil {
					t.Fatal("helper modified still-running original:", err)
				}
				blocked, stop := context.WithTimeout(context.Background(), 80*time.Millisecond)
				gate, gateErr := AcquireLaunchGate(blocked, target)
				stop()
				if gateErr == nil {
					gate.Close()
					t.Fatal("concurrent ordinary relaunch bypassed updater lock")
				}
				fmt.Fprintln(conn, "exit")
				if err := command.Wait(); err != nil {
					t.Fatal(err)
				}
				if err := waitParent(helper); err != nil {
					t.Fatal("helper did not finish:", err)
				}
				if scenario == "success" {
					var restarted fixtureRestarted
					body, _ := os.ReadFile(filepath.Join(dir, "restarted.json"))
					if json.Unmarshal(body, &restarted) != nil || !samePath(restarted.Cwd, dir) || hashMatches(target, newHash) != nil {
						t.Fatal("new image did not restart with original working directory")
					}
					if err := waitProcessExited(restarted.PID, restarted.Created, 10*time.Second); err != nil {
						t.Fatal("restarted fixture was not joined:", err)
					}
					if _, err := os.Stat(filepath.Join(prepared.Stage, "old.exe")); !os.IsNotExist(err) {
						t.Fatal("ready startup retained executable backup")
					}
				}
			}
			if scenario != "success" && hashMatches(target, oldHash) != nil {
				t.Fatal("failed or aborted helper did not preserve old executable")
			}
			for name, expected := range map[string]string{"preferences.json": "preserved preferences", "dipper.db": "preserved follows"} {
				body, _ := os.ReadFile(filepath.Join(dir, "relative-data", name))
				if string(body) != expected {
					t.Fatal("helper changed user data:", name)
				}
			}
		})
	}
}

func waitProcessExited(pid int, created string, timeout time.Duration) error {
	process, err := openParent(pid, created, timeout)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil
	}
	if err != nil {
		return err
	}
	defer closeParent(process)
	return waitParent(process)
}

func TestWindowsExactParentAndLockedTarget(t *testing.T) {
	identity, err := processIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if parent, err := openParent(os.Getpid(), identity+"wrong", time.Second); err == nil {
		closeParent(parent)
		t.Fatal("accepted reused PID with a different creation identity")
	}
	parent, err := openParent(os.Getpid(), identity, 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer closeParent(parent)
	if err := waitParent(parent); err == nil {
		t.Fatal("bounded exact process wait ignored running process")
	}
	if err := verifyParentImage(parent, filepath.Join(t.TempDir(), "Dipper.exe")); err == nil {
		t.Fatal("accepted parent image unrelated to update target")
	}
	state := replacementFixture(t)
	name, _ := windows.UTF16PtrFromString(state.Target)
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	err = replaceAndRestart(state, time.Second, func(string, []string, string) (restartedProcess, error) {
		t.Error("started replacement of locked target")
		return restartedProcess{}, nil
	})
	if err == nil || hashMatches(state.Target, state.TargetHash) != nil || !strings.Contains(err.Error(), "folder permissions") {
		t.Fatal("locked executable was changed or lacked actionable error:", err)
	}
}
