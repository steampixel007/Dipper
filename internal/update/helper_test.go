package update

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func replacementFixture(t *testing.T) helperState {
	t.Helper()
	dir := t.TempDir()
	stage, err := os.MkdirTemp(dir, ".dipper-update-")
	if err != nil {
		t.Fatal(err)
	}
	state := helperState{
		Target: filepath.Join(dir, "Dipper.exe"), Stage: stage,
		Staged: filepath.Join(stage, "new.exe"), WorkingDirectory: dir,
		ParentPID: os.Getpid(), ParentCreated: "fixture-identity",
		ControlAddress: "127.0.0.1:12345", Created: time.Now(),
		Args: []string{"--data", "relative-data", "--download-root=relative-downloads", "--software-update-ready-token=stale"},
	}
	state.ControlToken, _ = randomToken()
	state.ReadyToken, _ = randomToken()
	for path, body := range map[string]string{
		state.Target: "old executable fixture", state.Staged: "new executable fixture",
		filepath.Join(stage, "helper.exe"):     "old executable fixture",
		filepath.Join(stage, "state.json"):     "fixture",
		filepath.Join(dir, "preferences.json"): "user preferences",
		filepath.Join(dir, "state.sqlite"):     "user follows and downloads",
		filepath.Join(dir, "unrelated.txt"):    "unrelated user file",
	} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state.TargetHash, _ = fileHash(state.Target)
	state.StagedHash, _ = fileHash(state.Staged)
	return state
}

func TestReplaceRestartPreservesContextAndData(t *testing.T) {
	state := replacementFixture(t)
	finished := make(chan error, 1)
	err := replaceAndRestart(state, 5*time.Second, func(target string, args []string, working string) (restartedProcess, error) {
		if target != state.Target || working != state.WorkingDirectory || !reflect.DeepEqual(PrivateArguments(args), PrivateArguments(state.Args)) {
			t.Errorf("lost original restart context: %s %v %s", target, args, working)
		}
		if err := hashMatches(target, state.StagedHash); err != nil {
			t.Error(err)
		}
		var address, token string
		for index, arg := range args {
			if arg == "--software-update-ready-address" {
				address = args[index+1]
			}
			if arg == "--software-update-ready-token" {
				token = args[index+1]
			}
		}
		go func() {
			conn, err := net.Dial("tcp4", address)
			if err == nil {
				fmt.Fprintln(conn, "authorize:"+strings.Repeat("0", 64))
				conn.Close()
			}
			if err == nil {
				err = AuthorizeRestart(address, token)
			}
			if err == nil {
				err = NotifyReady(address, token)
			}
			finished <- err
		}()
		return restartedProcess{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if err := hashMatches(state.Target, state.StagedHash); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state.Stage, "old.exe")); !os.IsNotExist(err) {
		t.Fatal("successful acknowledged startup retained old executable")
	}
	if _, err := os.Stat(filepath.Join(state.Stage, "state.json")); !os.IsNotExist(err) {
		t.Fatal("successful acknowledged startup retained update state")
	}
	entries, err := os.ReadDir(state.Stage)
	if err != nil || len(entries) != 2 {
		t.Fatalf("successful update retained unexpected stage files: %v %v", entries, err)
	}
	found := map[string]bool{}
	for _, entry := range entries {
		found[entry.Name()] = true
	}
	if !found["helper.exe"] || !found[completedStageMarker] {
		t.Fatalf("successful acknowledgement did not mark the stage for cleanup: %v", found)
	}
	for name, expected := range map[string]string{"preferences.json": "user preferences", "state.sqlite": "user follows and downloads"} {
		body, _ := os.ReadFile(filepath.Join(state.WorkingDirectory, name))
		if string(body) != expected {
			t.Fatalf("update changed user data %s: %s", name, body)
		}
	}
	if body, err := os.ReadFile(filepath.Join(state.WorkingDirectory, "unrelated.txt")); err != nil || string(body) != "unrelated user file" {
		t.Fatalf("update changed an unrelated file: %q %v", body, err)
	}
}

func TestCleanupCompletedUpdateStages(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Dipper.exe")
	completed := filepath.Join(dir, ".dipper-update-completed")
	active := filepath.Join(dir, ".dipper-update-active")
	invalid := filepath.Join(dir, ".dipper-update-invalid")
	unrelated := filepath.Join(dir, "user-folder")
	for path, body := range map[string]string{
		target: "old executable fixture",
		filepath.Join(completed, completedStageMarker):             "complete\n",
		filepath.Join(completed, "helper.exe"):                     "helper fixture",
		filepath.Join(active, "helper.exe"):                        "helper fixture",
		filepath.Join(active, "state.json"):                        "active update state",
		filepath.Join(invalid, completedStageMarker, "nested.txt"): "not a regular marker",
		filepath.Join(unrelated, completedStageMarker):             "unrelated marker",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	stageFile := filepath.Join(dir, ".dipper-update-file")
	if err := os.WriteFile(stageFile, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}

	CleanupCompleted(target)

	if _, err := os.Stat(completed); !os.IsNotExist(err) {
		t.Fatalf("acknowledged stage was not removed: %v", err)
	}
	for _, path := range []string{active, invalid, unrelated, stageFile} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("cleanup affected %s: %v", path, err)
		}
	}
}

func TestCleanupPreservesUnexpectedStageFiles(t *testing.T) {
	for _, scenario := range []string{"file", "directory", "marker"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			stage := filepath.Join(dir, ".dipper-update-completed")
			if err := os.Mkdir(stage, 0700); err != nil {
				t.Fatal(err)
			}
			for name, body := range map[string]string{"helper.exe": "helper fixture", completedStageMarker: "complete\n"} {
				if err := os.WriteFile(filepath.Join(stage, name), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			protected := filepath.Join(stage, "user-notes.txt")
			if scenario == "directory" {
				protected = filepath.Join(stage, "user-data", "notes.txt")
				if err := os.Mkdir(filepath.Dir(protected), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "marker" {
				protected = filepath.Join(stage, completedStageMarker)
			}
			if err := os.WriteFile(protected, []byte("preserved fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			CleanupCompleted(filepath.Join(dir, "Dipper.exe"))
			if body, err := os.ReadFile(protected); err != nil || string(body) != "preserved fixture" {
				t.Fatalf("cleanup removed unexpected stage content: %q %v", body, err)
			}
			if _, err := os.Stat(filepath.Join(stage, "helper.exe")); err != nil {
				t.Fatalf("cleanup partially removed an unrecognized stage: %v", err)
			}
		})
	}
}

func TestReplaceFailuresPreserveUsableOldExecutable(t *testing.T) {
	for _, scenario := range []string{"start", "exit", "timeout", "changed", "backup", "replacement"} {
		t.Run(scenario, func(t *testing.T) {
			state := replacementFixture(t)
			started := false
			switch scenario {
			case "changed":
				os.WriteFile(state.Staged, []byte("unverified new bytes"), 0600)
			case "backup":
				os.WriteFile(filepath.Join(state.Stage, "old.exe"), []byte("unrelated existing backup"), 0600)
			case "replacement":
				os.Remove(state.Staged)
			}
			err := replaceAndRestart(state, 50*time.Millisecond, func(string, []string, string) (restartedProcess, error) {
				started = true
				if scenario == "start" {
					return restartedProcess{}, errors.New("fixture start denied")
				}
				exited := make(chan error, 1)
				if scenario == "exit" {
					exited <- errors.New("fixture startup failed")
				}
				return restartedProcess{exited: exited}, nil
			})
			if err == nil {
				t.Fatal("failed update reported success")
			}
			if scenario == "changed" || scenario == "backup" || scenario == "replacement" {
				if started {
					t.Fatal("started an unsafe replacement")
				}
			}
			if scenario == "timeout" {
				if err := hashMatches(filepath.Join(state.Stage, "old.exe"), state.TargetHash); err != nil {
					t.Fatal("lost old executable while new process was still running", err)
				}
				if !strings.Contains(err.Error(), "no rollback attempted") {
					t.Fatalf("claimed unsafe rollback of a running process: %v", err)
				}
			} else if err := hashMatches(state.Target, state.TargetHash); err != nil {
				t.Fatal("failed update lost usable old executable", err)
			}
			if scenario == "start" || scenario == "exit" {
				if !strings.Contains(err.Error(), "user data was not reverted") {
					t.Fatal("rollback omitted data compatibility boundary", err)
				}
			}
		})
	}
}

func TestHelperInputAndRestartAuthentication(t *testing.T) {
	for _, scenario := range []string{"valid", "target", "stage", "staged", "hash", "token", "address", "old", "future", "cwd", "helper"} {
		t.Run(scenario, func(t *testing.T) {
			state := replacementFixture(t)
			statePath := filepath.Join(state.Stage, "state.json")
			helper := filepath.Join(state.Stage, "helper.exe")
			switch scenario {
			case "target":
				state.Target = filepath.Join(filepath.Dir(state.Target), "other.exe")
			case "stage":
				state.Stage = filepath.Dir(state.Stage)
			case "staged":
				state.Staged = state.Target
			case "hash":
				state.StagedHash = strings.Repeat("0", 64)
			case "token":
				state.ReadyToken = "stale"
			case "address":
				state.ControlAddress = "192.0.2.1:80"
			case "old":
				state.Created = time.Now().Add(-time.Hour)
			case "future":
				state.Created = time.Now().Add(time.Hour)
			case "cwd":
				state.WorkingDirectory = "."
			case "helper":
				os.WriteFile(helper, []byte("changed helper"), 0600)
			}
			err := validateState(state, statePath, helper)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("input validation %s: %v", scenario, err)
			}
		})
	}
	for _, address := range []string{"127.0.0.1:0", "127.0.0.1:-1", "127.0.0.1:99999", "127.0.0.1:service", "localhost:80"} {
		if err := AuthorizeRestart(address, strings.Repeat("1", 64)); err == nil {
			t.Errorf("accepted unsafe restart address %q", address)
		}
	}
	input := []string{"--data=relative", "--software-update-ready-address", "127.0.0.1:1", "--download-root", "downloads", "-software-update-ready-token=old"}
	want := []string{"--data=relative", "--download-root", "downloads"}
	if got := PrivateArguments(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("private arguments leaked or context lost: %v", got)
	}
}

func TestLaunchGateSerializesAndCancels(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Dipper.exe")
	first, err := AcquireLaunchGate(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if gate, err := AcquireLaunchGate(ctx, target); err == nil {
		gate.Close()
		t.Fatal("canceled operation acquired launch gate")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if gate, err := AcquireLaunchGate(ctx, target); err == nil {
		gate.Close()
		t.Fatal("concurrent launch bypassed installation gate")
	}
	first.Close()
	next, err := AcquireLaunchGate(context.Background(), target)
	if err != nil {
		t.Fatal("released update gate stayed locked", err)
	}
	next.Close()
}
